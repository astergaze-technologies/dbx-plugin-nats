package natsx

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// MaxPayloadBytes caps message bodies sent to the UI; larger bodies are truncated.
const MaxPayloadBytes = 64 * 1024

// Payload is a message body as the UI receives it.
type Payload struct {
	Data      string `json:"data"`
	Encoding  string `json:"encoding"` // "utf8" or "base64"
	Size      int    `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}

func encodePayload(data []byte) Payload {
	p := Payload{Size: len(data), Encoding: "utf8"}
	if len(data) > MaxPayloadBytes {
		data, p.Truncated = data[:MaxPayloadBytes], true
	}
	if utf8.Valid(data) {
		p.Data = string(data)
	} else {
		p.Encoding, p.Data = "base64", base64.StdEncoding.EncodeToString(data)
	}
	return p
}

func flattenHeaders(h nats.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = strings.Join(v, ", ")
	}
	return out
}

func toHeader(h map[string]string) nats.Header {
	if len(h) == 0 {
		return nil
	}
	out := nats.Header{}
	for k, v := range h {
		out.Set(k, v)
	}
	return out
}

// ValidateSubject checks a subject; wildcards (* and >) are allowed only when subscribing.
func ValidateSubject(subject string, allowWildcards bool) error {
	if subject == "" {
		return errors.New("subject is required")
	}
	if strings.ContainsAny(subject, " \t\r\n") {
		return errors.New("subject must not contain whitespace")
	}
	tokens := strings.Split(subject, ".")
	for i, t := range tokens {
		switch {
		case t == "":
			return errors.New("subject has an empty token")
		case t == ">" && i != len(tokens)-1:
			return errors.New("'>' is only allowed as the last token")
		case (t == "*" || t == ">") && !allowWildcards:
			return errors.New("wildcards are not allowed when publishing")
		}
	}
	return nil
}

// ---- server overview ----

type Overview struct {
	Server    ServerInfo    `json:"server"`
	JetStream *JetStreamUse `json:"jetstream,omitempty"`
}

type ServerInfo struct {
	Name       string  `json:"name"`
	ID         string  `json:"id"`
	Version    string  `json:"version"`
	Cluster    string  `json:"cluster,omitempty"`
	URL        string  `json:"url"`
	RTTMillis  float64 `json:"rttMs"`
	MaxPayload int64   `json:"maxPayload"`
	Headers    bool    `json:"headers"`
	TLS        bool    `json:"tls"`
}

type JetStreamUse struct {
	Memory    uint64 `json:"memory"`
	Storage   uint64 `json:"storage"`
	Streams   int    `json:"streams"`
	Consumers int    `json:"consumers"`
	MaxMemory int64  `json:"maxMemory"`
	MaxStore  int64  `json:"maxStore"`
}

func (m *Manager) Overview(ctx context.Context, connectionID string) (*Overview, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	rtt, _ := s.nc.RTT()
	_, tlsErr := s.nc.TLSConnectionState()
	out := &Overview{Server: ServerInfo{
		Name:       s.nc.ConnectedServerName(),
		ID:         s.nc.ConnectedServerId(),
		Version:    s.nc.ConnectedServerVersion(),
		Cluster:    s.nc.ConnectedClusterName(),
		URL:        s.nc.ConnectedUrlRedacted(),
		RTTMillis:  float64(rtt.Microseconds()) / 1000,
		MaxPayload: s.nc.MaxPayload(),
		Headers:    s.nc.HeadersSupported(),
		TLS:        tlsErr == nil,
	}}
	// JetStream is optional; a server without it still gets an overview.
	if info, err := s.js.AccountInfo(ctx); err == nil {
		out.JetStream = &JetStreamUse{
			Memory: info.Memory, Storage: info.Store,
			Streams: info.Streams, Consumers: info.Consumers,
			MaxMemory: info.Limits.MaxMemory, MaxStore: info.Limits.MaxStore,
		}
	}
	return out, nil
}

// jetStreamUnavailable reports errors meaning "this server/account has no JetStream";
// a server without JetStream at all answers API requests with no responders.
func jetStreamUnavailable(err error) bool {
	return errors.Is(err, jetstream.ErrJetStreamNotEnabled) ||
		errors.Is(err, jetstream.ErrJetStreamNotEnabledForAccount) ||
		errors.Is(err, nats.ErrNoResponders)
}

// ---- streams ----

type StreamSummary struct {
	Name      string    `json:"name"`
	Subjects  []string  `json:"subjects"`
	Retention string    `json:"retention"`
	Storage   string    `json:"storage"`
	Messages  uint64    `json:"messages"`
	Bytes     uint64    `json:"bytes"`
	Consumers int       `json:"consumers"`
	FirstSeq  uint64    `json:"firstSeq"`
	LastSeq   uint64    `json:"lastSeq"`
	Created   time.Time `json:"created"`
}

func (m *Manager) Streams(ctx context.Context, connectionID string) ([]StreamSummary, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	out := []StreamSummary{}
	lister := s.js.ListStreams(ctx)
	for info := range lister.Info() {
		out = append(out, StreamSummary{
			Name: info.Config.Name, Subjects: info.Config.Subjects,
			Retention: info.Config.Retention.String(), Storage: info.Config.Storage.String(),
			Messages: info.State.Msgs, Bytes: info.State.Bytes, Consumers: info.State.Consumers,
			FirstSeq: info.State.FirstSeq, LastSeq: info.State.LastSeq, Created: info.Created,
		})
	}
	if err := lister.Err(); err != nil {
		if jetStreamUnavailable(err) {
			return out, nil
		}
		return nil, err
	}
	return out, nil
}

type StoredMessage struct {
	Sequence uint64            `json:"seq"`
	Subject  string            `json:"subject"`
	Time     time.Time         `json:"time"`
	Headers  map[string]string `json:"headers,omitempty"`
	Payload
}

type MessagePage struct {
	Messages []StoredMessage `json:"messages"`
	// NextBefore is the sequence to pass as `before` for the next (older) page; 0 when done.
	NextBefore uint64 `json:"nextBefore"`
}

// MaxPageSize bounds one page of stream messages.
const MaxPageSize = 200

// StreamMessages reads up to limit messages, newest first, with sequence < before
// (before == 0 means "from the end of the stream").
func (m *Manager) StreamMessages(ctx context.Context, connectionID, stream string, before uint64, limit int) (*MessagePage, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > MaxPageSize {
		limit = 50
	}
	st, err := s.js.Stream(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("stream %q: %w", stream, err)
	}
	info, err := st.Info(ctx)
	if err != nil {
		return nil, err
	}
	first, last := info.State.FirstSeq, info.State.LastSeq
	seq := last
	if before != 0 && before-1 < last {
		seq = before - 1
	}
	page := &MessagePage{Messages: []StoredMessage{}}
	if seq < first || seq == 0 {
		return page, nil
	}
	// Read the window [lo, seq] with bounded parallel direct gets: one request per
	// message costs a full round trip, so sequential reads are unusable on remote
	// servers. Deleted sequences are gaps and simply make the page shorter.
	lo := first
	if seq-first+1 > uint64(limit) {
		lo = seq - uint64(limit) + 1
	}
	found := make([]*StoredMessage, seq-lo+1)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	slots := make(chan struct{}, fetchParallelism)
	for n := lo; n <= seq; n++ {
		wg.Add(1)
		slots <- struct{}{}
		go func(n uint64) {
			defer func() { <-slots; wg.Done() }()
			msg, err := st.GetMsg(ctx, n)
			if err != nil {
				if !errors.Is(err, jetstream.ErrMsgNotFound) {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
				return
			}
			found[n-lo] = &StoredMessage{
				Sequence: msg.Sequence, Subject: msg.Subject, Time: msg.Time,
				Headers: flattenHeaders(msg.Header), Payload: encodePayload(msg.Data),
			}
		}(n)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	for i := len(found) - 1; i >= 0; i-- { // newest first
		if found[i] != nil {
			page.Messages = append(page.Messages, *found[i])
		}
	}
	if lo > first {
		page.NextBefore = lo
	}
	return page, nil
}

// fetchParallelism bounds concurrent direct gets for one page.
const fetchParallelism = 16

// ---- key-value ----

type BucketSummary struct {
	Bucket     string `json:"bucket"`
	Values     uint64 `json:"values"`
	History    int64  `json:"history"`
	TTL        string `json:"ttl"`
	Bytes      uint64 `json:"bytes"`
	Storage    string `json:"storage"`
	Compressed bool   `json:"compressed"`
}

func (m *Manager) Buckets(ctx context.Context, connectionID string) ([]BucketSummary, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	out := []BucketSummary{}
	lister := s.js.KeyValueStores(ctx)
	for st := range lister.Status() {
		ttl := "none"
		if st.TTL() > 0 {
			ttl = st.TTL().String()
		}
		out = append(out, BucketSummary{
			Bucket: st.Bucket(), Values: st.Values(), History: st.History(), TTL: ttl,
			Bytes: st.Bytes(), Storage: st.BackingStore(), Compressed: st.IsCompressed(),
		})
	}
	if err := lister.Error(); err != nil {
		if jetStreamUnavailable(err) {
			return out, nil
		}
		return nil, err
	}
	return out, nil
}

// MaxKeys bounds one key listing.
const MaxKeys = 1000

type KeyList struct {
	Keys      []string `json:"keys"`
	Truncated bool     `json:"truncated"`
}

func (m *Manager) Keys(ctx context.Context, connectionID, bucket string) (*KeyList, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket %q: %w", bucket, err)
	}
	lister, err := kv.ListKeys(ctx)
	if err != nil {
		return nil, err
	}
	defer lister.Stop()
	out := &KeyList{Keys: []string{}}
	for key := range lister.Keys() {
		if len(out.Keys) == MaxKeys {
			out.Truncated = true
			break
		}
		out.Keys = append(out.Keys, key)
	}
	return out, nil
}

type KeyValue struct {
	Key       string    `json:"key"`
	Revision  uint64    `json:"revision"`
	Created   time.Time `json:"created"`
	Operation string    `json:"operation"`
	Payload
}

func (m *Manager) Get(ctx context.Context, connectionID, bucket, key string) (*KeyValue, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket %q: %w", bucket, err)
	}
	entry, err := kv.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("key %q: %w", key, err)
	}
	return &KeyValue{
		Key: entry.Key(), Revision: entry.Revision(), Created: entry.Created(),
		Operation: operationName(entry.Operation()), Payload: encodePayload(entry.Value()),
	}, nil
}

func operationName(op jetstream.KeyValueOp) string {
	switch op {
	case jetstream.KeyValuePut:
		return "PUT"
	case jetstream.KeyValueDelete:
		return "DEL"
	case jetstream.KeyValuePurge:
		return "PURGE"
	}
	return op.String()
}

// ---- publish / request ----

func (m *Manager) Publish(connectionID, subject, data string, headers map[string]string) error {
	s, err := m.session(connectionID)
	if err != nil {
		return err
	}
	if err := ValidateSubject(subject, false); err != nil {
		return err
	}
	msg := &nats.Msg{Subject: subject, Data: []byte(data), Header: toHeader(headers)}
	if err := s.nc.PublishMsg(msg); err != nil {
		return err
	}
	return s.nc.FlushTimeout(5 * time.Second)
}

type Reply struct {
	Subject   string            `json:"subject"`
	Headers   map[string]string `json:"headers,omitempty"`
	ElapsedMs float64           `json:"elapsedMs"`
	Payload
}

// MaxRequestTimeout bounds request/reply waits.
const MaxRequestTimeout = 60 * time.Second

func (m *Manager) Request(connectionID, subject, data string, headers map[string]string, timeout time.Duration) (*Reply, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	if err := ValidateSubject(subject, false); err != nil {
		return nil, err
	}
	if timeout <= 0 || timeout > MaxRequestTimeout {
		timeout = 5 * time.Second
	}
	start := time.Now()
	msg, err := s.nc.RequestMsg(&nats.Msg{Subject: subject, Data: []byte(data), Header: toHeader(headers)}, timeout)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, fmt.Errorf("no service is listening on %q", subject)
		}
		return nil, err
	}
	return &Reply{
		Subject: msg.Subject, Headers: flattenHeaders(msg.Header),
		ElapsedMs: float64(time.Since(start).Microseconds()) / 1000, Payload: encodePayload(msg.Data),
	}, nil
}
