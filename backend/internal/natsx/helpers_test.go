package natsx

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go/jetstream"
)

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
	return Config{Name: "test", Host: "127.0.0.1", Port: srv.Addr().(*net.TCPAddr).Port, Timeout: 2 * time.Second}
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

func jsClient(t *testing.T, cfg Config) jetstream.JetStream {
	t.Helper()
	nc, err := Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	return js
}

// seed creates stream ORDERS (5 text + 1 binary message) and KV bucket "config".
func seed(t *testing.T, cfg Config) {
	t.Helper()
	js := jsClient(t, cfg)
	ctx := context.Background()
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}}); err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		if _, err := js.Publish(ctx, "orders.created", []byte{'a' + byte(i)}); err != nil {
			t.Fatal(err)
		}
	}
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

type event struct {
	method string
	params any
}

type recorder struct {
	mu     sync.Mutex
	events []event
}

func (r *recorder) emit(method string, params any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event{method, params})
}

func (r *recorder) count(method string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.method == method {
			n++
		}
	}
	return n
}

func (r *recorder) all(method string) []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []any
	for _, e := range r.events {
		if e.method == method {
			out = append(out, e.params)
		}
	}
	return out
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
