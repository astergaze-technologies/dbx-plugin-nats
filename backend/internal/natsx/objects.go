package natsx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// MaxObjectBytes bounds object reads/writes through the UI bridge.
const MaxObjectBytes = 4 << 20

type ObjectStoreSummary struct {
	Store       string `json:"store"`
	Description string `json:"description,omitempty"`
	Size        uint64 `json:"size"`
	Storage     string `json:"storage"`
	Sealed      bool   `json:"sealed"`
	TTL         string `json:"ttl"`
}

type ObjectSummary struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Size        uint64    `json:"size"`
	Chunks      uint32    `json:"chunks"`
	Digest      string    `json:"digest"`
	Modified    time.Time `json:"modified"`
}

type ObjectData struct {
	ObjectSummary
	Data []byte `json:"data"` // base64 in JSON
}

func (m *Manager) ObjectStores(ctx context.Context, connectionID string) ([]ObjectStoreSummary, error) {
	s, err := m.session(connectionID)
	if err != nil {
		return nil, err
	}
	out := []ObjectStoreSummary{}
	lister := s.js.ObjectStores(ctx)
	for st := range lister.Status() {
		ttl := "none"
		if st.TTL() > 0 {
			ttl = st.TTL().String()
		}
		out = append(out, ObjectStoreSummary{
			Store: st.Bucket(), Description: st.Description(), Size: st.Size(),
			Storage: st.Storage().String(), Sealed: st.Sealed(), TTL: ttl,
		})
	}
	if err := lister.Error(); err != nil && !jetStreamUnavailable(err) {
		return nil, err
	}
	return out, nil
}

func (m *Manager) CreateObjectStore(ctx context.Context, connectionID, store, description string) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	if store == "" {
		return errors.New("store name is required")
	}
	_, err = s.js.CreateObjectStore(ctx, jetstream.ObjectStoreConfig{Bucket: store, Description: description})
	return err
}

func (m *Manager) DeleteObjectStore(ctx context.Context, connectionID, store string) error {
	s, err := m.writable(connectionID)
	if err != nil {
		return err
	}
	return s.js.DeleteObjectStore(ctx, store)
}

func objectSummary(info *jetstream.ObjectInfo) ObjectSummary {
	return ObjectSummary{Name: info.Name, Description: info.Description, Size: info.Size,
		Chunks: info.Chunks, Digest: info.Digest, Modified: info.ModTime}
}

func (m *Manager) Objects(ctx context.Context, connectionID, store string) ([]ObjectSummary, error) {
	obs, err := m.objectStore(ctx, connectionID, store, false)
	if err != nil {
		return nil, err
	}
	infos, err := obs.List(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoObjectsFound) {
			return []ObjectSummary{}, nil
		}
		return nil, err
	}
	out := make([]ObjectSummary, 0, len(infos))
	for _, info := range infos {
		out = append(out, objectSummary(info))
	}
	return out, nil
}

func (m *Manager) GetObject(ctx context.Context, connectionID, store, name string) (*ObjectData, error) {
	obs, err := m.objectStore(ctx, connectionID, store, false)
	if err != nil {
		return nil, err
	}
	info, err := obs.GetInfo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("object %q: %w", name, err)
	}
	if info.Size > MaxObjectBytes {
		return nil, fmt.Errorf("object %q is %d bytes; the preview limit is %d", name, info.Size, MaxObjectBytes)
	}
	data, err := obs.GetBytes(ctx, name)
	if err != nil {
		return nil, err
	}
	return &ObjectData{ObjectSummary: objectSummary(info), Data: data}, nil
}

func (m *Manager) PutObject(ctx context.Context, connectionID, store, name string, data []byte) (*ObjectSummary, error) {
	if len(data) > MaxObjectBytes {
		return nil, fmt.Errorf("upload limit is %d bytes", MaxObjectBytes)
	}
	obs, err := m.objectStore(ctx, connectionID, store, true)
	if err != nil {
		return nil, err
	}
	info, err := obs.PutBytes(ctx, name, data)
	if err != nil {
		return nil, err
	}
	out := objectSummary(info)
	return &out, nil
}

func (m *Manager) DeleteObject(ctx context.Context, connectionID, store, name string) error {
	obs, err := m.objectStore(ctx, connectionID, store, true)
	if err != nil {
		return err
	}
	return obs.Delete(ctx, name)
}

func (m *Manager) objectStore(ctx context.Context, connectionID, store string, write bool) (jetstream.ObjectStore, error) {
	get := m.session
	if write {
		get = m.writable
	}
	s, err := get(connectionID)
	if err != nil {
		return nil, err
	}
	obs, err := s.js.ObjectStore(ctx, store)
	if err != nil {
		return nil, fmt.Errorf("object store %q: %w", store, err)
	}
	return obs, nil
}
