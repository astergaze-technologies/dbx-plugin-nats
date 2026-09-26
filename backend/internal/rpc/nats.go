package rpc

import (
	"context"
	"time"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
)

// params is the union of workbench request fields; each method reads what it needs.
type params struct {
	ConnectionID string            `json:"connectionId"`
	Stream       string            `json:"stream"`
	Consumer     string            `json:"consumer"`
	Subject      string            `json:"subject"`
	Seq          uint64            `json:"seq"`
	Before       uint64            `json:"before"`
	Limit        int               `json:"limit"`
	Bucket       string            `json:"bucket"`
	Key          string            `json:"key"`
	Pattern      string            `json:"pattern"`
	Purge        bool              `json:"purge"`
	Store        string            `json:"store"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Queue        string            `json:"queue"`
	Data         string            `json:"data"`
	DataBase64   []byte            `json:"dataBase64"`
	Headers      map[string]string `json:"headers"`
	TimeoutMs    int               `json:"timeoutMs"`
	FeedID       string            `json:"feedId"`
	Update       bool              `json:"update"`
	StreamSpec   natsx.StreamSpec  `json:"streamSpec"`
	BucketSpec   natsx.BucketSpec  `json:"bucketSpec"`
}

func (p params) connection() string { return p.ConnectionID }

type method func(ctx context.Context, p params, emit natsx.Emit) (any, error)

func (r *Router) register(methods map[string]method) {
	for name, fn := range methods {
		on(r, name, fn)
	}
}

func (r *Router) registerServer() {
	m := r.nats
	r.register(map[string]method{
		"nats/overview": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.Overview(ctx, p.ConnectionID)
		},
		"nats/services": func(_ context.Context, p params, _ natsx.Emit) (any, error) {
			services, err := m.Services(p.ConnectionID)
			return map[string]any{"services": services}, err
		},
	})
}

func (r *Router) registerStreams() {
	m := r.nats
	r.register(map[string]method{
		"nats/streams": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			streams, err := m.Streams(ctx, p.ConnectionID)
			return map[string]any{"streams": streams}, err
		},
		"nats/stream": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.Stream(ctx, p.ConnectionID, p.Stream)
		},
		"nats/streamSave": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.SaveStream(ctx, p.ConnectionID, p.StreamSpec, p.Update)
		},
		"nats/streamPurge": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.PurgeStream(ctx, p.ConnectionID, p.Stream, p.Subject))
		},
		"nats/streamDelete": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.DeleteStream(ctx, p.ConnectionID, p.Stream))
		},
		"nats/streamMessages": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.StreamMessages(ctx, p.ConnectionID, p.Stream, p.Before, p.Limit)
		},
		"nats/messageDelete": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.DeleteMessage(ctx, p.ConnectionID, p.Stream, p.Seq))
		},
		"nats/consumers": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			consumers, err := m.Consumers(ctx, p.ConnectionID, p.Stream)
			return map[string]any{"consumers": consumers}, err
		},
		"nats/consumerDelete": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.DeleteConsumer(ctx, p.ConnectionID, p.Stream, p.Consumer))
		},
	})
}

func (r *Router) registerKV() {
	m := r.nats
	r.register(map[string]method{
		"nats/kvBuckets": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			buckets, err := m.Buckets(ctx, p.ConnectionID)
			return map[string]any{"buckets": buckets}, err
		},
		"nats/kvBucketCreate": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.CreateBucket(ctx, p.ConnectionID, p.BucketSpec))
		},
		"nats/kvBucketDelete": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.DeleteBucket(ctx, p.ConnectionID, p.Bucket))
		},
		"nats/kvKeys": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.Keys(ctx, p.ConnectionID, p.Bucket)
		},
		"nats/kvGet": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.Get(ctx, p.ConnectionID, p.Bucket, p.Key)
		},
		"nats/kvHistory": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			history, err := m.History(ctx, p.ConnectionID, p.Bucket, p.Key)
			return map[string]any{"history": history}, err
		},
		"nats/kvPut": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			revision, err := m.Put(ctx, p.ConnectionID, p.Bucket, p.Key, []byte(p.Data))
			return map[string]any{"revision": revision}, err
		},
		"nats/kvDelete": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.DeleteKey(ctx, p.ConnectionID, p.Bucket, p.Key, p.Purge))
		},
		"nats/kvWatch": func(_ context.Context, p params, emit natsx.Emit) (any, error) {
			feed, err := m.WatchKV(p.ConnectionID, p.Bucket, p.Pattern, emit)
			return map[string]any{"feedId": feed}, err
		},
	})
}

func (r *Router) registerObjects() {
	m := r.nats
	r.register(map[string]method{
		"nats/objectStores": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			stores, err := m.ObjectStores(ctx, p.ConnectionID)
			return map[string]any{"stores": stores}, err
		},
		"nats/objectStoreCreate": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.CreateObjectStore(ctx, p.ConnectionID, p.Store, p.Description))
		},
		"nats/objectStoreDelete": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.DeleteObjectStore(ctx, p.ConnectionID, p.Store))
		},
		"nats/objects": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			objects, err := m.Objects(ctx, p.ConnectionID, p.Store)
			return map[string]any{"objects": objects}, err
		},
		"nats/objectGet": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.GetObject(ctx, p.ConnectionID, p.Store, p.Name)
		},
		"nats/objectPut": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return m.PutObject(ctx, p.ConnectionID, p.Store, p.Name, p.DataBase64)
		},
		"nats/objectDelete": func(ctx context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.DeleteObject(ctx, p.ConnectionID, p.Store, p.Name))
		},
	})
}

func (r *Router) registerMessaging() {
	m := r.nats
	r.register(map[string]method{
		"nats/publish": func(_ context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.Publish(p.ConnectionID, p.Subject, p.Data, p.Headers))
		},
		"nats/request": func(_ context.Context, p params, _ natsx.Emit) (any, error) {
			return m.Request(p.ConnectionID, p.Subject, p.Data, p.Headers, time.Duration(p.TimeoutMs)*time.Millisecond)
		},
		"nats/subscribe": func(_ context.Context, p params, emit natsx.Emit) (any, error) {
			feed, err := m.Subscribe(p.ConnectionID, p.Subject, p.Queue, emit)
			return map[string]any{"feedId": feed}, err
		},
		"nats/feedStop": func(_ context.Context, p params, _ natsx.Emit) (any, error) {
			return ok(m.StopFeed(p.ConnectionID, p.FeedID))
		},
	})
}
