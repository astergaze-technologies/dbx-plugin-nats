package natsx

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Live subscriptions forward messages to the UI as host events. A busy subject
// (think `>`) would flood the host's bounded event buffer, so each subscription
// forwards at most RateLimit messages per second and counts what it drops.
const (
	RateLimit   = 100
	bufferSize  = 256
	EventMethod = "nats/message"
	// EventClosed is emitted once when a subscription ends for any reason.
	EventClosed = "nats/subscriptionClosed"
)

// Emit sends a host event. It must be safe for concurrent use.
type Emit func(method string, params any)

// LiveMessage is the payload of a nats/message event.
type LiveMessage struct {
	SubscriptionID string            `json:"subscriptionId"`
	ConnectionID   string            `json:"connectionId"`
	Subject        string            `json:"subject"`
	Reply          string            `json:"reply,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	ReceivedAt     time.Time         `json:"receivedAt"`
	// Dropped counts messages discarded by the rate limit since the previous event.
	Dropped uint64 `json:"dropped,omitempty"`
	Payload
}

type subscription struct {
	id, connectionID string
	sub              *nats.Subscription
	queue            chan *nats.Msg
	emit             Emit
	mu               sync.Mutex
	dropped          uint64
	done             chan struct{}
	once             sync.Once
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Subscribe starts a live subscription and returns its ID.
func (m *Manager) Subscribe(connectionID, subject, queueGroup string, emit Emit) (string, error) {
	if err := ValidateSubject(subject, true); err != nil {
		return "", err
	}
	s, err := m.session(connectionID)
	if err != nil {
		return "", err
	}
	sub := &subscription{
		id: newID(), connectionID: connectionID, emit: emit,
		queue: make(chan *nats.Msg, bufferSize), done: make(chan struct{}),
	}
	handler := func(msg *nats.Msg) {
		select {
		case sub.queue <- msg:
		default:
			sub.mu.Lock()
			sub.dropped++
			sub.mu.Unlock()
		}
	}
	if queueGroup != "" {
		sub.sub, err = s.nc.QueueSubscribe(subject, queueGroup, handler)
	} else {
		sub.sub, err = s.nc.Subscribe(subject, handler)
	}
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	s.subs[sub.id] = sub
	m.mu.Unlock()
	go sub.forward()
	return sub.id, nil
}

// Unsubscribe stops a subscription. Unknown IDs are a no-op.
func (m *Manager) Unsubscribe(connectionID, subscriptionID string) error {
	s, err := m.session(connectionID)
	if err != nil {
		return err
	}
	m.mu.Lock()
	sub := s.subs[subscriptionID]
	delete(s.subs, subscriptionID)
	m.mu.Unlock()
	if sub != nil {
		sub.stop("unsubscribed")
	}
	return nil
}

func (sub *subscription) forward() {
	ticker := time.NewTicker(time.Second / RateLimit)
	defer ticker.Stop()
	for {
		select {
		case <-sub.done:
			return
		case msg := <-sub.queue:
			select {
			case <-sub.done:
				return
			case <-ticker.C:
			}
			sub.mu.Lock()
			dropped := sub.dropped
			sub.dropped = 0
			sub.mu.Unlock()
			sub.emit(EventMethod, LiveMessage{
				SubscriptionID: sub.id, ConnectionID: sub.connectionID,
				Subject: msg.Subject, Reply: msg.Reply, Headers: flattenHeaders(msg.Header),
				ReceivedAt: time.Now().UTC(), Dropped: dropped, Payload: encodePayload(msg.Data),
			})
		}
	}
}

func (sub *subscription) stop(reason string) {
	sub.once.Do(func() {
		_ = sub.sub.Unsubscribe()
		close(sub.done)
		sub.emit(EventClosed, map[string]any{
			"subscriptionId": sub.id, "connectionId": sub.connectionID, "reason": reason,
		})
	})
}
