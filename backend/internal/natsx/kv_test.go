package natsx

import (
	"context"
	"testing"
)

func TestKeyValueReadWrite(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	ctx := context.Background()

	buckets, err := m.Buckets(ctx, id)
	if err != nil || len(buckets) != 1 || buckets[0].Bucket != "config" || buckets[0].History != 3 {
		t.Fatalf("buckets = %+v, %v", buckets, err)
	}
	if rev, err := m.Put(ctx, id, "config", "a", []byte("v2")); err != nil || rev == 0 {
		t.Fatalf("put = %d, %v", rev, err)
	}
	v, err := m.Get(ctx, id, "config", "a")
	if err != nil || v.Data != "v2" || v.Operation != "PUT" {
		t.Fatalf("get = %+v, %v", v, err)
	}
	history, err := m.History(ctx, id, "config", "a")
	if err != nil || len(history) != 2 || history[0].Data != "v2" || history[1].Data != "value-a" {
		t.Fatalf("history = %+v, %v", history, err)
	}
	if err := m.DeleteKey(ctx, id, "config", "b", false); err != nil {
		t.Fatal(err)
	}
	keys, err := m.Keys(ctx, id, "config")
	if err != nil || len(keys.Keys) != 1 || keys.Keys[0] != "a" {
		t.Fatalf("keys after delete = %+v, %v", keys, err)
	}
	if err := m.DeleteKey(ctx, id, "config", "a", true); err != nil {
		t.Fatal(err)
	}
	if keys, _ := m.Keys(ctx, id, "config"); len(keys.Keys) != 0 {
		t.Fatalf("keys after purge = %+v", keys)
	}
}

func TestBucketLifecycle(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	ctx := context.Background()
	if err := m.CreateBucket(ctx, id, BucketSpec{Bucket: "sessions", History: 5, TTLSecs: 60}); err != nil {
		t.Fatal(err)
	}
	buckets, err := m.Buckets(ctx, id)
	if err != nil || len(buckets) != 1 || buckets[0].History != 5 || buckets[0].TTL != "1m0s" {
		t.Fatalf("buckets = %+v, %v", buckets, err)
	}
	if err := m.DeleteBucket(ctx, id, "sessions"); err != nil {
		t.Fatal(err)
	}
	if buckets, _ := m.Buckets(ctx, id); len(buckets) != 0 {
		t.Fatalf("bucket not deleted: %+v", buckets)
	}
}

func TestWatchKV(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	rec := &recorder{}
	feed, err := m.WatchKV(id, "config", "a", rec.emit)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, _ = m.Put(ctx, id, "config", "a", []byte("changed"))
	_, _ = m.Put(ctx, id, "config", "b", []byte("ignored"))
	waitFor(t, func() bool { return rec.count(EventKVChange) == 1 })
	change := rec.all(EventKVChange)[0].(KVChange)
	if change.FeedID != feed || change.Key != "a" || change.Data != "changed" {
		t.Fatalf("change = %+v", change)
	}
	if err := m.StopFeed(id, feed); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return rec.count(EventFeedClose) == 1 })
}
