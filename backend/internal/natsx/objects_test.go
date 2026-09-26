package natsx

import (
	"bytes"
	"context"
	"testing"
)

func TestObjectStore(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	ctx := context.Background()

	if err := m.CreateObjectStore(ctx, id, "files", "uploads"); err != nil {
		t.Fatal(err)
	}
	stores, err := m.ObjectStores(ctx, id)
	if err != nil || len(stores) != 1 || stores[0].Store != "files" || stores[0].Description != "uploads" {
		t.Fatalf("stores = %+v, %v", stores, err)
	}
	if objs, err := m.Objects(ctx, id, "files"); err != nil || len(objs) != 0 {
		t.Fatalf("empty store = %+v, %v", objs, err)
	}
	payload := bytes.Repeat([]byte("x"), 300*1024) // spans several chunks
	if _, err := m.PutObject(ctx, id, "files", "reports/q1.csv", payload); err != nil {
		t.Fatal(err)
	}
	objs, err := m.Objects(ctx, id, "files")
	if err != nil || len(objs) != 1 || objs[0].Name != "reports/q1.csv" || objs[0].Size != uint64(len(payload)) {
		t.Fatalf("objects = %+v, %v", objs, err)
	}
	got, err := m.GetObject(ctx, id, "files", "reports/q1.csv")
	if err != nil || !bytes.Equal(got.Data, payload) {
		t.Fatalf("get object: %v (len %d)", err, len(got.Data))
	}
	if _, err := m.PutObject(ctx, id, "files", "big", make([]byte, MaxObjectBytes+1)); err == nil {
		t.Fatal("oversized upload must be rejected")
	}
	if err := m.DeleteObject(ctx, id, "files", "reports/q1.csv"); err != nil {
		t.Fatal(err)
	}
	if objs, _ := m.Objects(ctx, id, "files"); len(objs) != 0 {
		t.Fatalf("object not deleted: %+v", objs)
	}
	if err := m.DeleteObjectStore(ctx, id, "files"); err != nil {
		t.Fatal(err)
	}
}
