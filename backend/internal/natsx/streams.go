package natsx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

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
	// System marks streams that back KV buckets and object stores.
	System bool `json:"system"`
}

// StreamSpec is the editable subset of a stream configuration.
type StreamSpec struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Subjects       []string `json:"subjects"`
	Storage        string   `json:"storage"`   // file | memory
	Retention      string   `json:"retention"` // limits | interest | workqueue
	Discard        string   `json:"discard"`   // old | new
	Replicas       int      `json:"replicas"`
	MaxMsgs        int64    `json:"maxMsgs"`
	MaxBytes       int64    `json:"maxBytes"`
	MaxAgeSecs     int64    `json:"maxAgeSecs"`
	MaxMsgSize     int32    `json:"maxMsgSize"`
	DuplicatesSecs int64    `json:"duplicatesSecs"`
}

type StreamDetail struct {
	StreamSummary
	Spec StreamSpec `json:"spec"`
}

func summarize(info *jetstream.StreamInfo) StreamSummary {
	return StreamSummary{
		Name: info.Config.Name, Subjects: info.Config.Subjects,
		Retention: info.Config.Retention.String(), Storage: info.Config.Storage.String(),
		Messages: info.State.Msgs, Bytes: info.State.Bytes, Consumers: info.State.Consumers,
		FirstSeq: info.State.FirstSeq, LastSeq: info.State.LastSeq, Created: info.Created,
		System: strings.HasPrefix(info.Config.Name, "KV_") || strings.HasPrefix(info.Config.Name, "OBJ_"),
	}
}

func specOf(c jetstream.StreamConfig) StreamSpec {
	spec := StreamSpec{
		Name: c.Name, Description: c.Description, Subjects: c.Subjects,
		Storage: "file", Retention: "limits", Discard: "old", Replicas: c.Replicas,
		MaxMsgs: c.MaxMsgs, MaxBytes: c.MaxBytes, MaxMsgSize: c.MaxMsgSize,
		MaxAgeSecs: int64(c.MaxAge / time.Second), DuplicatesSecs: int64(c.Duplicates / time.Second),
	}
	if c.Storage == jetstream.MemoryStorage {
		spec.Storage = "memory"
	}
	switch c.Retention {
	case jetstream.InterestPolicy:
		spec.Retention = "interest"
	case jetstream.WorkQueuePolicy:
		spec.Retention = "workqueue"
	}
	if c.Discard == jetstream.DiscardNew {
		spec.Discard = "new"
	}
	return spec
}

func (spec StreamSpec) config() (jetstream.StreamConfig, error) {
	if spec.Name == "" {
		return jetstream.StreamConfig{}, errors.New("stream name is required")
	}
	for _, subject := range spec.Subjects {
		if err := ValidateSubject(subject, true); err != nil {
			return jetstream.StreamConfig{}, fmt.Errorf("subject %q: %w", subject, err)
		}
	}
	c := jetstream.StreamConfig{
		Name: spec.Name, Description: spec.Description, Subjects: spec.Subjects,
		Replicas: max(spec.Replicas, 1), MaxMsgs: orUnlimited(spec.MaxMsgs), MaxBytes: orUnlimited(spec.MaxBytes),
		MaxMsgSize:   int32(orUnlimited(int64(spec.MaxMsgSize))),
		MaxAge:       time.Duration(spec.MaxAgeSecs) * time.Second,
		Duplicates:   time.Duration(spec.DuplicatesSecs) * time.Second,
		MaxConsumers: -1, MaxMsgsPerSubject: -1,
	}
	if spec.Storage == "memory" {
		c.Storage = jetstream.MemoryStorage
	}
	switch spec.Retention {
	case "", "limits":
	case "interest":
		c.Retention = jetstream.InterestPolicy
	case "workqueue":
		c.Retention = jetstream.WorkQueuePolicy
	default:
		return c, fmt.Errorf("unknown retention %q", spec.Retention)
	}
	if spec.Discard == "new" {
		c.Discard = jetstream.DiscardNew
	}
	return c, nil
}

func orUnlimited(v int64) int64 {
	if v <= 0 {
		return -1
	}
	return v
}

func (m *Manager) Streams(ctx context.Context, connectionID string) ([]StreamSummary, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	out := []StreamSummary{}
	lister := s.js.ListStreams(ctx)
	for info := range lister.Info() {
		out = append(out, summarize(info))
	}
	if err := lister.Err(); err != nil && !jetStreamUnavailable(err) {
		return nil, err
	}
	return out, nil
}

func (m *Manager) Stream(ctx context.Context, connectionID, name string) (*StreamDetail, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	st, err := s.js.Stream(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("stream %q: %w", name, err)
	}
	info, err := st.Info(ctx)
	if err != nil {
		return nil, err
	}
	return &StreamDetail{StreamSummary: summarize(info), Spec: specOf(info.Config)}, nil
}

// SaveStream creates the stream, or updates it when update is true.
func (m *Manager) SaveStream(ctx context.Context, connectionID string, spec StreamSpec, update bool) (*StreamDetail, error) {
	s, err := m.writable(connectionID)
	if err != nil {
		return nil, err
	}
	cfg, err := spec.config()
	if err != nil {
		return nil, err
	}
	save := s.js.CreateStream
	if update {
		save = s.js.UpdateStream
	}
	st, err := save(ctx, cfg)
	if err != nil {
		return nil, err
	}
	info := st.CachedInfo()
	return &StreamDetail{StreamSummary: summarize(info), Spec: specOf(info.Config)}, nil
}

func (m *Manager) PurgeStream(ctx context.Context, connectionID, name, subject string) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	st, err := s.js.Stream(ctx, name)
	if err != nil {
		return err
	}
	var opts []jetstream.StreamPurgeOpt
	if subject != "" {
		opts = append(opts, jetstream.WithPurgeSubject(subject))
	}
	return st.Purge(ctx, opts...)
}

func (m *Manager) DeleteStream(ctx context.Context, connectionID, name string) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	return s.js.DeleteStream(ctx, name)
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
	// NextBefore is the `before` value for the next (older) page; 0 when done.
	NextBefore uint64 `json:"nextBefore"`
}

const (
	MaxPageSize      = 200
	fetchParallelism = 16
)

func storedMessage(msg *jetstream.RawStreamMsg) StoredMessage {
	return StoredMessage{
		Sequence: msg.Sequence, Subject: msg.Subject, Time: msg.Time,
		Headers: flattenHeaders(msg.Header), Payload: encodePayload(msg.Data),
	}
}

// StreamMessages reads up to limit messages with sequence < before (0 = from the end), newest first.
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
	lo := first
	if seq-first+1 > uint64(limit) {
		lo = seq - uint64(limit) + 1
	}
	found, err := fetchRange(ctx, st, lo, seq)
	if err != nil {
		return nil, err
	}
	for i := len(found) - 1; i >= 0; i-- {
		if found[i] != nil {
			page.Messages = append(page.Messages, *found[i])
		}
	}
	if lo > first {
		page.NextBefore = lo
	}
	return page, nil
}

// fetchRange reads [lo, hi] with bounded parallel direct gets; deleted sequences stay nil.
func fetchRange(ctx context.Context, st jetstream.Stream, lo, hi uint64) ([]*StoredMessage, error) {
	found := make([]*StoredMessage, hi-lo+1)
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	slots := make(chan struct{}, fetchParallelism)
	for n := lo; n <= hi; n++ {
		wg.Add(1)
		slots <- struct{}{}
		go func(n uint64) {
			defer func() { <-slots; wg.Done() }()
			msg, err := st.GetMsg(ctx, n)
			if err == nil {
				m := storedMessage(msg)
				found[n-lo] = &m
				return
			}
			if !errors.Is(err, jetstream.ErrMsgNotFound) {
				mu.Lock()
				firstErr = firstError(firstErr, err)
				mu.Unlock()
			}
		}(n)
	}
	wg.Wait()
	return found, firstErr
}

func firstError(current, next error) error {
	if current != nil {
		return current
	}
	return next
}

func (m *Manager) StreamMessage(ctx context.Context, connectionID, stream string, seq uint64) (*StoredMessage, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	st, err := s.js.Stream(ctx, stream)
	if err != nil {
		return nil, err
	}
	msg, err := st.GetMsg(ctx, seq)
	if err != nil {
		return nil, err
	}
	out := storedMessage(msg)
	return &out, nil
}

func (m *Manager) DeleteMessage(ctx context.Context, connectionID, stream string, seq uint64) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	st, err := s.js.Stream(ctx, stream)
	if err != nil {
		return err
	}
	return st.DeleteMsg(ctx, seq)
}
