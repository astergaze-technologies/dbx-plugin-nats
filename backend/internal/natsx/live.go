package natsx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Live feeds (subscriptions, KV watches) forward at most RateLimit events per
// second each; the rest are counted as dropped instead of flooding the host.
const (
	RateLimit      = 100
	liveBuffer     = 256
	EventMessage   = "nats/message"
	EventKVChange  = "nats/kvChange"
	EventFeedClose = "nats/feedClosed"
)

// Emit sends a host event; it must be safe for concurrent use.
type Emit func(method string, params any)

type LiveMessage struct {
	FeedID       string            `json:"feedId"`
	ConnectionID string            `json:"connectionId"`
	Subject      string            `json:"subject"`
	Reply        string            `json:"reply,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	ReceivedAt   time.Time         `json:"receivedAt"`
	Dropped      uint64            `json:"dropped,omitempty"`
	Payload
}

type KVChange struct {
	FeedID       string `json:"feedId"`
	ConnectionID string `json:"connectionId"`
	Bucket       string `json:"bucket"`
	Dropped      uint64 `json:"dropped,omitempty"`
	KeyValue
}

// liveFeed queues event builders; each receives the dropped count since the last event.
type liveFeed struct {
	id, connectionID string
	method           string
	emit             Emit
	queue            chan func(dropped uint64) any
	cancel           func()
	mu               sync.Mutex
	dropped          uint64
	done             chan struct{}
	once             sync.Once
}

func newFeedID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) startFeed(s *session, connectionID, method string, emit Emit) *liveFeed {
	feed := &liveFeed{
		id: newFeedID(), connectionID: connectionID, method: method, emit: emit,
		queue: make(chan func(uint64) any, liveBuffer), done: make(chan struct{}),
	}
	m.mu.Lock()
	s.live[feed.id] = feed
	m.mu.Unlock()
	go feed.forward()
	return feed
}

func (f *liveFeed) push(build func(uint64) any) {
	select {
	case f.queue <- build:
	default:
		f.mu.Lock()
		f.dropped++
		f.mu.Unlock()
	}
}

func (f *liveFeed) forward() {
	ticker := time.NewTicker(time.Second / RateLimit)
	defer ticker.Stop()
	for {
		select {
		case <-f.done:
			return
		case build := <-f.queue:
			select {
			case <-f.done:
				return
			case <-ticker.C:
			}
			f.mu.Lock()
			dropped := f.dropped
			f.dropped = 0
			f.mu.Unlock()
			f.emit(f.method, build(dropped))
		}
	}
}

func (f *liveFeed) stop(reason string) {
	f.once.Do(func() {
		if f.cancel != nil {
			f.cancel()
		}
		close(f.done)
		f.emit(EventFeedClose, map[string]any{"feedId": f.id, "connectionId": f.connectionID, "reason": reason})
	})
}

// Subscribe starts a live core-NATS subscription and returns its feed ID.
func (m *Manager) Subscribe(connectionID, subject, queueGroup string, emit Emit) (string, error) {
	if err := ValidateSubject(subject, true); err != nil {
		return "", err
	}
	s, err := m.session(connectionID)
	if err != nil {
		return "", err
	}
	feed := m.startFeed(s, connectionID, EventMessage, emit)
	handler := func(msg *nats.Msg) {
		received := time.Now().UTC()
		feed.push(func(dropped uint64) any {
			return LiveMessage{FeedID: feed.id, ConnectionID: connectionID, Subject: msg.Subject, Reply: msg.Reply,
				Headers: flattenHeaders(msg.Header), ReceivedAt: received, Dropped: dropped, Payload: encodePayload(msg.Data)}
		})
	}
	var sub *nats.Subscription
	if queueGroup != "" {
		sub, err = s.nc.QueueSubscribe(subject, queueGroup, handler)
	} else {
		sub, err = s.nc.Subscribe(subject, handler)
	}
	if err != nil {
		m.dropFeed(s, feed, "error")
		return "", err
	}
	feed.cancel = func() { _ = sub.Unsubscribe() }
	return feed.id, nil
}

// WatchKV streams future changes to keys matching pattern (">" for all).
func (m *Manager) WatchKV(connectionID, bucket, pattern string, emit Emit) (string, error) {
	kv, err := m.bucket(context.Background(), connectionID, bucket, false)
	if err != nil {
		return "", err
	}
	s, err := m.session(connectionID)
	if err != nil {
		return "", err
	}
	if pattern == "" {
		pattern = ">"
	}
	ctx, cancel := context.WithCancel(context.Background())
	watcher, err := kv.Watch(ctx, pattern, jetstream.UpdatesOnly())
	if err != nil {
		cancel()
		return "", err
	}
	feed := m.startFeed(s, connectionID, EventKVChange, emit)
	feed.cancel = func() { _ = watcher.Stop(); cancel() }
	go func() {
		for entry := range watcher.Updates() {
			if entry == nil {
				continue
			}
			change := keyValue(entry)
			feed.push(func(dropped uint64) any {
				return KVChange{FeedID: feed.id, ConnectionID: connectionID, Bucket: bucket, Dropped: dropped, KeyValue: change}
			})
		}
	}()
	return feed.id, nil
}

// StopFeed ends a subscription or watch; unknown IDs are a no-op.
func (m *Manager) StopFeed(connectionID, feedID string) error {
	s, err := m.session(connectionID)
	if err != nil {
		return err
	}
	m.mu.Lock()
	feed := s.live[feedID]
	m.mu.Unlock()
	if feed != nil {
		m.dropFeed(s, feed, "stopped")
	}
	return nil
}

func (m *Manager) dropFeed(s *session, feed *liveFeed, reason string) {
	m.mu.Lock()
	delete(s.live, feed.id)
	m.mu.Unlock()
	feed.stop(reason)
}
