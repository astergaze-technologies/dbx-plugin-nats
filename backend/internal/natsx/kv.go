package natsx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type BucketSummary struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description,omitempty"`
	Values      uint64 `json:"values"`
	History     int64  `json:"history"`
	TTL         string `json:"ttl"`
	Bytes       uint64 `json:"bytes"`
	Storage     string `json:"storage"`
	Compressed  bool   `json:"compressed"`
}

type BucketSpec struct {
	Bucket      string `json:"bucket"`
	Description string `json:"description,omitempty"`
	History     uint8  `json:"history"`
	TTLSecs     int64  `json:"ttlSecs"`
	MaxBytes    int64  `json:"maxBytes"`
	Storage     string `json:"storage"`
	Replicas    int    `json:"replicas"`
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
			Bucket: st.Bucket(), Description: st.Config().Description, Values: st.Values(),
			History: st.History(), TTL: ttl, Bytes: st.Bytes(), Storage: st.BackingStore(), Compressed: st.IsCompressed(),
		})
	}
	if err := lister.Error(); err != nil && !jetStreamUnavailable(err) {
		return nil, err
	}
	return out, nil
}

func (m *Manager) CreateBucket(ctx context.Context, connectionID string, spec BucketSpec) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	if spec.Bucket == "" {
		return errors.New("bucket name is required")
	}
	cfg := jetstream.KeyValueConfig{
		Bucket: spec.Bucket, Description: spec.Description, History: max(spec.History, 1),
		TTL: time.Duration(spec.TTLSecs) * time.Second, MaxBytes: orUnlimited(spec.MaxBytes), Replicas: max(spec.Replicas, 1),
	}
	if spec.Storage == "memory" {
		cfg.Storage = jetstream.MemoryStorage
	}
	_, err = s.js.CreateKeyValue(ctx, cfg)
	return err
}

func (m *Manager) DeleteBucket(ctx context.Context, connectionID, bucket string) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	return s.js.DeleteKeyValue(ctx, bucket)
}

const MaxKeys = 1000

type KeyList struct {
	Keys      []string `json:"keys"`
	Truncated bool     `json:"truncated"`
}

func (m *Manager) Keys(ctx context.Context, connectionID, bucket string) (*KeyList, error) {
	kv, err := m.bucket(ctx, connectionID, bucket, false)
	if err != nil {
		return nil, err
	}
	lister, err := kv.ListKeys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return &KeyList{Keys: []string{}}, nil
		}
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

func keyValue(e jetstream.KeyValueEntry) KeyValue {
	return KeyValue{Key: e.Key(), Revision: e.Revision(), Created: e.Created(),
		Operation: operationName(e.Operation()), Payload: encodePayload(e.Value())}
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

func (m *Manager) Get(ctx context.Context, connectionID, bucket, key string) (*KeyValue, error) {
	kv, err := m.bucket(ctx, connectionID, bucket, false)
	if err != nil {
		return nil, err
	}
	entry, err := kv.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("key %q: %w", key, err)
	}
	out := keyValue(entry)
	return &out, nil
}

// History returns all retained revisions of key, newest first.
func (m *Manager) History(ctx context.Context, connectionID, bucket, key string) ([]KeyValue, error) {
	kv, err := m.bucket(ctx, connectionID, bucket, false)
	if err != nil {
		return nil, err
	}
	entries, err := kv.History(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("key %q: %w", key, err)
	}
	out := make([]KeyValue, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		out = append(out, keyValue(entries[i]))
	}
	return out, nil
}

func (m *Manager) Put(ctx context.Context, connectionID, bucket, key string, value []byte) (uint64, error) {
	kv, err := m.bucket(ctx, connectionID, bucket, true)
	if err != nil {
		return 0, err
	}
	return kv.Put(ctx, key, value)
}

// DeleteKey soft-deletes key; purge also removes its history.
func (m *Manager) DeleteKey(ctx context.Context, connectionID, bucket, key string, purge bool) error {
	kv, err := m.bucket(ctx, connectionID, bucket, true)
	if err != nil {
		return err
	}
	if purge {
		return kv.Purge(ctx, key)
	}
	return kv.Delete(ctx, key)
}

func (m *Manager) bucket(ctx context.Context, connectionID, bucket string, write bool) (jetstream.KeyValue, error) {
	get := m.session
	if write {
		get = m.writable
	}
	s, err := get(connectionID)
	if err != nil {
		return nil, err
	}
	kv, err := s.js.KeyValue(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket %q: %w", bucket, err)
	}
	return kv, nil
}
