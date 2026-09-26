package natsx

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
)

func TestStreamMessagesPaging(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	ctx := context.Background()

	page, err := m.StreamMessages(ctx, id, "ORDERS", 0, 4)
	if err != nil || len(page.Messages) != 4 || page.Messages[0].Sequence != 6 || page.NextBefore != 3 {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	if bin := page.Messages[0]; bin.Encoding != "base64" || bin.Data != "//4=" {
		t.Fatalf("binary payload = %+v", bin.Payload)
	}
	next, err := m.StreamMessages(ctx, id, "ORDERS", page.NextBefore, 4)
	if err != nil || len(next.Messages) != 2 || next.Messages[1].Sequence != 1 || next.NextBefore != 0 {
		t.Fatalf("second page = %+v, %v", next, err)
	}
	one, err := m.StreamMessage(ctx, id, "ORDERS", 2)
	if err != nil || one.Data != "b" {
		t.Fatalf("single message = %+v, %v", one, err)
	}
}

func TestSystemStreamsFlagged(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	streams, err := m.Streams(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	system := map[string]bool{}
	for _, s := range streams {
		system[s.Name] = s.System
	}
	if system["ORDERS"] || !system["KV_config"] {
		t.Fatalf("system flags = %v", system)
	}
}

func TestStreamMessagesSkipDeleted(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	ctx := context.Background()
	for _, seq := range []uint64{4, 5} {
		if err := m.DeleteMessage(ctx, id, "ORDERS", seq); err != nil {
			t.Fatal(err)
		}
	}
	page, err := m.StreamMessages(ctx, id, "ORDERS", 0, 4)
	if err != nil || len(page.Messages) != 2 || page.Messages[0].Sequence != 6 || page.Messages[1].Sequence != 3 {
		t.Fatalf("page with gaps = %+v, %v", page, err)
	}
}

func TestStreamLifecycle(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	ctx := context.Background()

	spec := StreamSpec{Name: "EVENTS", Subjects: []string{"events.>"}, Storage: "memory", Retention: "limits", MaxMsgs: 100}
	created, err := m.SaveStream(ctx, id, spec, false)
	if err != nil || created.Spec.Storage != "memory" || created.Spec.MaxMsgs != 100 {
		t.Fatalf("create = %+v, %v", created, err)
	}
	spec.Subjects = []string{"events.>", "audit.>"}
	spec.Description = "all events"
	updated, err := m.SaveStream(ctx, id, spec, true)
	if err != nil || len(updated.Spec.Subjects) != 2 || updated.Spec.Description != "all events" {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	if _, err := m.SaveStream(ctx, id, StreamSpec{Name: "BAD", Subjects: []string{"a b"}}, false); err == nil {
		t.Fatal("invalid subject must be rejected")
	}

	js := jsClient(t, cfg)
	for range 3 {
		_, _ = js.Publish(ctx, "events.x", []byte("1"))
	}
	_, _ = js.Publish(ctx, "audit.y", []byte("2"))
	if err := m.PurgeStream(ctx, id, "EVENTS", "events.x"); err != nil {
		t.Fatal(err)
	}
	detail, err := m.Stream(ctx, id, "EVENTS")
	if err != nil || detail.Messages != 1 {
		t.Fatalf("after subject purge = %+v, %v", detail, err)
	}
	if err := m.DeleteStream(ctx, id, "EVENTS"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stream(ctx, id, "EVENTS"); err == nil {
		t.Fatal("stream should be gone")
	}
}

func TestConsumers(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	js := jsClient(t, cfg)
	ctx := context.Background()
	if _, err := js.CreateOrUpdateConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{
		Durable: "billing", FilterSubject: "orders.created", AckPolicy: jetstream.AckExplicitPolicy,
	}); err != nil {
		t.Fatal(err)
	}
	m, id := connect(t, cfg)
	list, err := m.Consumers(ctx, id, "ORDERS")
	if err != nil || len(list) != 1 {
		t.Fatalf("consumers = %+v, %v", list, err)
	}
	c := list[0]
	if c.Name != "billing" || !c.Durable || !c.Pull || c.NumPending != 5 || c.FilterSubjects[0] != "orders.created" {
		t.Fatalf("consumer = %+v", c)
	}
	if err := m.DeleteConsumer(ctx, id, "ORDERS", "billing"); err != nil {
		t.Fatal(err)
	}
	if list, _ := m.Consumers(ctx, id, "ORDERS"); len(list) != 0 {
		t.Fatalf("consumer not deleted: %+v", list)
	}
}
