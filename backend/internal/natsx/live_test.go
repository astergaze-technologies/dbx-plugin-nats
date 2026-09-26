package natsx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

func TestPublishAndRequest(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	nc, _ := Dial(cfg)
	defer nc.Close()
	got := make(chan *nats.Msg, 1)
	_, _ = nc.Subscribe("greet", func(msg *nats.Msg) { got <- msg })
	_, _ = nc.Subscribe("svc.echo", func(msg *nats.Msg) {
		_ = msg.RespondMsg(&nats.Msg{Data: append([]byte("echo:"), msg.Data...), Header: nats.Header{"X-Svc": {"1"}}})
	})
	_ = nc.Flush()

	if err := m.Publish(id, "greet", "hi", map[string]string{"X-Test": "yes"}); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-got:
		if string(msg.Data) != "hi" || msg.Header.Get("X-Test") != "yes" {
			t.Fatalf("published = %q %v", msg.Data, msg.Header)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("publish not received")
	}
	if err := m.Publish(id, "greet.*", "x", nil); err == nil {
		t.Fatal("publishing to a wildcard must fail")
	}
	reply, err := m.Request(id, "svc.echo", "ping", nil, time.Second)
	if err != nil || reply.Data != "echo:ping" || reply.Headers["X-Svc"] != "1" {
		t.Fatalf("request = %+v, %v", reply, err)
	}
	if _, err := m.Request(id, "svc.nobody", "x", nil, time.Second); err == nil {
		t.Fatal("expected no-responders error")
	}
}

func TestSubscribeRateLimitAndCleanup(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	rec := &recorder{}
	feed, err := m.Subscribe(id, "live.>", "", rec.emit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Subscribe(id, "bad subject", "", rec.emit); err == nil {
		t.Fatal("invalid subject must be rejected")
	}
	nc, _ := Dial(cfg)
	defer nc.Close()
	for i := range 2000 {
		_ = nc.Publish("live.tick", []byte{byte(i)})
	}
	_ = nc.Flush()

	waitFor(t, func() bool {
		var dropped uint64
		for _, p := range rec.all(EventMessage) {
			dropped += p.(LiveMessage).Dropped
		}
		return rec.count(EventMessage) >= 20 && dropped > 0
	})
	first := rec.all(EventMessage)[0].(LiveMessage)
	if first.FeedID != feed || first.Subject != "live.tick" {
		t.Fatalf("first event = %+v", first)
	}
	m.Disconnect(id)
	m.Disconnect(id)
	waitFor(t, func() bool { return rec.count(EventFeedClose) == 1 })
	if _, err := m.Overview(context.Background(), id); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("after disconnect: %v", err)
	}
}

func TestQueueGroupAndStop(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	rec := &recorder{}
	feed, err := m.Subscribe(id, "jobs", "workers", rec.emit)
	if err != nil {
		t.Fatal(err)
	}
	nc, _ := Dial(cfg)
	defer nc.Close()
	_ = nc.Publish("jobs", []byte("one"))
	_ = nc.Flush()
	waitFor(t, func() bool { return rec.count(EventMessage) == 1 })
	_ = m.StopFeed(id, feed)
	_ = m.StopFeed(id, feed)
	_ = nc.Publish("jobs", []byte("two"))
	_ = nc.Flush()
	time.Sleep(100 * time.Millisecond)
	if rec.count(EventMessage) != 1 || rec.count(EventFeedClose) != 1 {
		t.Fatalf("after stop: %d messages, %d closed", rec.count(EventMessage), rec.count(EventFeedClose))
	}
}

func TestReconnectReplacesSession(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	rec := &recorder{}
	if _, err := m.Subscribe(id, "x", "", rec.emit); err != nil {
		t.Fatal(err)
	}
	if err := m.Connect(id, cfg); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return rec.count(EventFeedClose) == 1 })
}

func TestServiceDiscovery(t *testing.T) {
	cfg := runServer(t, nil)
	nc, _ := Dial(cfg)
	defer nc.Close()
	svc, err := micro.AddService(nc, micro.Config{
		Name: "billing", Version: "1.2.0", Description: "invoices",
		Endpoint: &micro.EndpointConfig{Subject: "billing.invoice", Handler: micro.HandlerFunc(func(r micro.Request) { _ = r.Respond([]byte("ok")) })},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Stop()
	if _, err := nc.Request("billing.invoice", nil, time.Second); err != nil {
		t.Fatal(err)
	}

	m, id := connect(t, cfg)
	services, err := m.Services(id)
	if err != nil || len(services) != 1 {
		t.Fatalf("services = %+v, %v", services, err)
	}
	s := services[0]
	if s.Name != "billing" || s.Version != "1.2.0" || len(s.Endpoints) != 1 || s.Endpoints[0].Requests != 1 {
		t.Fatalf("service = %+v", s)
	}
}
