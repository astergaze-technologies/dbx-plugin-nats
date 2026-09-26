package natsx

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// runServer starts an in-process NATS server with JetStream on a random port.
func runServer(t *testing.T, mutate func(*server.Options)) Config {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	opts.StoreDir = t.TempDir()
	if mutate != nil {
		mutate(&opts)
	}
	srv := natsserver.RunServer(&opts)
	t.Cleanup(srv.Shutdown)
	addr := srv.Addr().(*net.TCPAddr)
	return Config{Name: "test", Host: "127.0.0.1", Port: addr.Port, Timeout: 2 * time.Second}
}

func connect(t *testing.T, cfg Config) (*Manager, string) {
	t.Helper()
	m := NewManager()
	t.Cleanup(m.CloseAll)
	if err := m.Connect("conn-1", cfg); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return m, "conn-1"
}

// seed creates a stream with messages and a KV bucket using a plain client.
func seed(t *testing.T, cfg Config) {
	t.Helper()
	nc, err := Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	ctx := context.Background()
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}}); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		if _, err := js.Publish(ctx, "orders.created", []byte{'a' + byte(i)}); err != nil {
			t.Fatal(err)
		}
	}
	// Binary payload must come back base64-encoded.
	if _, err := js.Publish(ctx, "orders.bin", []byte{0xff, 0xfe}); err != nil {
		t.Fatal(err)
	}
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "config", History: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"a", "b"} {
		if _, err := kv.PutString(ctx, k, "value-"+k); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTestAndAuth(t *testing.T) {
	cfg := runServer(t, func(o *server.Options) { o.Username, o.Password = "admin", "s3cret" })

	if _, err := Test(cfg); err == nil {
		t.Fatal("expected authorization failure without credentials")
	}
	bad := cfg
	bad.Auth, bad.Username, bad.Password = AuthPassword, "admin", "wrong"
	if _, err := Test(bad); err == nil || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("expected failure that does not leak the password, got %v", err)
	}
	good := cfg
	good.Auth, good.Username, good.Password = AuthPassword, "admin", "s3cret"
	msg, err := Test(good)
	if err != nil || !strings.Contains(msg, "Connected to") {
		t.Fatalf("Test = %q, %v", msg, err)
	}

	token := runServer(t, func(o *server.Options) { o.Authorization = "tok" })
	token.Auth, token.Token = AuthToken, "tok"
	if _, err := Test(token); err != nil {
		t.Fatalf("token auth: %v", err)
	}
	if _, err := (Config{Auth: AuthToken}).options(); err == nil {
		t.Fatal("token auth without a token must fail")
	}
}

func TestOverviewStreamsAndMessages(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	ctx := context.Background()

	ov, err := m.Overview(ctx, id)
	if err != nil || ov.Server.Version == "" || ov.JetStream == nil || ov.JetStream.Streams < 1 {
		t.Fatalf("overview = %+v, %v", ov, err)
	}

	streams, err := m.Streams(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var orders *StreamSummary
	for i := range streams {
		if streams[i].Name == "ORDERS" {
			orders = &streams[i]
		}
	}
	if orders == nil || orders.Messages != 6 || orders.Subjects[0] != "orders.>" {
		t.Fatalf("ORDERS summary = %+v", orders)
	}

	// Newest first, paged by `before`.
	page, err := m.StreamMessages(ctx, id, "ORDERS", 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 4 || page.Messages[0].Sequence != 6 || page.NextBefore != 3 {
		t.Fatalf("first page = %+v", page)
	}
	if bin := page.Messages[0]; bin.Encoding != "base64" || bin.Data != "//4=" {
		t.Fatalf("binary payload = %+v", bin.Payload)
	}
	if page.Messages[1].Encoding != "utf8" || page.Messages[1].Data != "e" {
		t.Fatalf("text payload = %+v", page.Messages[1].Payload)
	}
	next, err := m.StreamMessages(ctx, id, "ORDERS", page.NextBefore, 4)
	if err != nil || len(next.Messages) != 2 || next.Messages[1].Sequence != 1 || next.NextBefore != 0 {
		t.Fatalf("second page = %+v, %v", next, err)
	}
	if _, err := m.StreamMessages(ctx, id, "MISSING", 0, 5); err == nil {
		t.Fatal("expected error for unknown stream")
	}
}

func TestKeyValue(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	ctx := context.Background()

	buckets, err := m.Buckets(ctx, id)
	if err != nil || len(buckets) != 1 || buckets[0].Bucket != "config" || buckets[0].History != 3 || buckets[0].TTL != "none" {
		t.Fatalf("buckets = %+v, %v", buckets, err)
	}
	keys, err := m.Keys(ctx, id, "config")
	if err != nil || len(keys.Keys) != 2 || keys.Truncated {
		t.Fatalf("keys = %+v, %v", keys, err)
	}
	v, err := m.Get(ctx, id, "config", "b")
	if err != nil || v.Data != "value-b" || v.Revision == 0 || v.Operation != "PUT" {
		t.Fatalf("get = %+v, %v", v, err)
	}
}

func TestNoJetStream(t *testing.T) {
	cfg := runServer(t, func(o *server.Options) { o.JetStream = false })
	m, id := connect(t, cfg)
	ctx := context.Background()
	ov, err := m.Overview(ctx, id)
	if err != nil || ov.JetStream != nil {
		t.Fatalf("overview without JetStream = %+v, %v", ov, err)
	}
	if streams, err := m.Streams(ctx, id); err != nil || len(streams) != 0 {
		t.Fatalf("streams without JetStream = %v, %v", streams, err)
	}
	if buckets, err := m.Buckets(ctx, id); err != nil || len(buckets) != 0 {
		t.Fatalf("buckets without JetStream = %v, %v", buckets, err)
	}
}

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
	if _, err := m.Request(id, "svc.nobody", "x", nil, time.Second); err == nil || !strings.Contains(err.Error(), "no service") {
		t.Fatalf("expected no-responders error, got %v", err)
	}
}

type recorder struct {
	mu     sync.Mutex
	events []struct {
		method string
		params any
	}
}

func (r *recorder) emit(method string, params any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, struct {
		method string
		params any
	}{method, params})
}

func (r *recorder) messages() (msgs []LiveMessage, closed int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		switch e.method {
		case EventMethod:
			msgs = append(msgs, e.params.(LiveMessage))
		case EventClosed:
			closed++
		}
	}
	return msgs, closed
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for condition")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSubscribeRateLimitAndCleanup(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	rec := &recorder{}

	subID, err := m.Subscribe(id, "live.>", "", rec.emit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Subscribe(id, "bad subject", "", rec.emit); err == nil {
		t.Fatal("invalid subject must be rejected")
	}

	nc, _ := Dial(cfg)
	defer nc.Close()
	// Burst far beyond the buffer: the plugin must drop, not block or balloon.
	const burst = 2000
	for i := range burst {
		_ = nc.Publish("live.tick", []byte{byte(i)})
	}
	_ = nc.Flush()

	waitFor(t, func() bool { msgs, _ := rec.messages(); return len(msgs) >= 20 })
	msgs, _ := rec.messages()
	if msgs[0].SubscriptionID != subID || msgs[0].ConnectionID != id || msgs[0].Subject != "live.tick" {
		t.Fatalf("first event = %+v", msgs[0])
	}
	// Everything is either delivered eventually or counted as dropped.
	waitFor(t, func() bool {
		msgs, _ := rec.messages()
		var dropped uint64
		for _, msg := range msgs {
			dropped += msg.Dropped
		}
		return len(msgs) > 0 && dropped > 0
	})

	// Disconnect stops the subscription exactly once and emits a close event.
	m.Disconnect(id)
	m.Disconnect(id) // idempotent
	waitFor(t, func() bool { _, closed := rec.messages(); return closed == 1 })
	if _, err := m.Overview(context.Background(), id); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("after disconnect: %v", err)
	}
}

func TestQueueGroupAndUnsubscribe(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	rec := &recorder{}
	subID, err := m.Subscribe(id, "jobs", "workers", rec.emit)
	if err != nil {
		t.Fatal(err)
	}
	nc, _ := Dial(cfg)
	defer nc.Close()
	_ = nc.Publish("jobs", []byte("one"))
	_ = nc.Flush()
	waitFor(t, func() bool { msgs, _ := rec.messages(); return len(msgs) == 1 })

	if err := m.Unsubscribe(id, subID); err != nil {
		t.Fatal(err)
	}
	if err := m.Unsubscribe(id, subID); err != nil {
		t.Fatalf("second unsubscribe should be a no-op: %v", err)
	}
	_ = nc.Publish("jobs", []byte("two"))
	_ = nc.Flush()
	time.Sleep(100 * time.Millisecond)
	msgs, closed := rec.messages()
	if len(msgs) != 1 || closed != 1 {
		t.Fatalf("after unsubscribe: %d messages, %d closed", len(msgs), closed)
	}
}

func TestValidateSubject(t *testing.T) {
	cases := []struct {
		subject  string
		wildcard bool
		ok       bool
	}{
		{"orders.created", false, true},
		{"orders.*", true, true},
		{"orders.>", true, true},
		{"orders.>", false, false},
		{">.orders", true, false},
		{"orders..x", true, false},
		{"", true, false},
		{"has space", true, false},
	}
	for _, c := range cases {
		if err := ValidateSubject(c.subject, c.wildcard); (err == nil) != c.ok {
			t.Errorf("ValidateSubject(%q, %v) = %v, want ok=%v", c.subject, c.wildcard, err, c.ok)
		}
	}
}

func TestConnectReplacesSession(t *testing.T) {
	cfg := runServer(t, nil)
	m, id := connect(t, cfg)
	rec := &recorder{}
	if _, err := m.Subscribe(id, "x", "", rec.emit); err != nil {
		t.Fatal(err)
	}
	if err := m.Connect(id, cfg); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { _, closed := rec.messages(); return closed == 1 })
}

func TestStreamMessagesSkipsDeletedSequences(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	nc, _ := Dial(cfg)
	defer nc.Close()
	js, _ := jetstream.New(nc)
	ctx := context.Background()
	st, _ := js.Stream(ctx, "ORDERS")
	for _, seq := range []uint64{4, 5} {
		if err := st.DeleteMsg(ctx, seq); err != nil {
			t.Fatal(err)
		}
	}
	m, id := connect(t, cfg)
	page, err := m.StreamMessages(ctx, id, "ORDERS", 0, 4) // window 3..6, with 4 and 5 deleted
	if err != nil {
		t.Fatal(err)
	}
	var seqs []uint64
	for _, msg := range page.Messages {
		seqs = append(seqs, msg.Sequence)
	}
	if len(seqs) != 2 || seqs[0] != 6 || seqs[1] != 3 || page.NextBefore != 3 {
		t.Fatalf("seqs = %v, nextBefore = %d", seqs, page.NextBefore)
	}
}
