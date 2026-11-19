package rpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
)

func noEmit(string, any) {}

func call(t *testing.T, r *Router, method string, params any) (map[string]any, *Error) {
	t.Helper()
	raw, _ := json.Marshal(params)
	result, rpcErr := r.Call(method, raw, noEmit)
	if rpcErr != nil {
		return nil, rpcErr
	}
	b, _ := json.Marshal(result)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func TestLifecycleConfig(t *testing.T) {
	var p lifecycleParams
	_ = json.Unmarshal([]byte(`{
		"connection": {
			"id": "c1", "name": "Prod", "host": "nats.internal", "port": 4222,
			"username": "app", "password": "pw", "read_only": true,
			"external_config": {"auth": "password", "tls": "true", "connect_timeout_secs": 7},
			"connection_secrets": {"token": "t"}
		},
		"runtime": {"host": "127.0.0.1", "port": 49152}
	}`), &p)
	cfg, err := p.config()
	if err != nil {
		t.Fatal(err)
	}
	want := natsx.Config{Name: "Prod", Host: "127.0.0.1", Port: 49152, Auth: natsx.AuthPassword,
		Username: "app", Password: "pw", Token: "t", TLS: true, Timeout: 7 * time.Second, ReadOnly: true}
	if cfg != want {
		t.Fatalf("config = %+v\nwant     %+v", cfg, want)
	}

	var fallback lifecycleParams
	_ = json.Unmarshal([]byte(`{"connection":{"id":"c","host":"localhost","port":4222}}`), &fallback)
	if cfg, err := fallback.config(); err != nil || cfg.Host != "localhost" || cfg.Auth != natsx.AuthNone {
		t.Fatalf("defaults = %+v, %v", cfg, err)
	}
	var bad lifecycleParams
	_ = json.Unmarshal([]byte(`{"connection":{"id":"c","host":"h","port":0}}`), &bad)
	if _, err := bad.config(); err == nil {
		t.Fatal("port 0 must be rejected")
	}
}

func TestRouterErrors(t *testing.T) {
	r := NewRouter(natsx.NewManager())
	if _, err := call(t, r, "nats/nope", map[string]string{}); err == nil || err.Code != CodeMethodNotFound {
		t.Fatalf("unknown method: %+v", err)
	}
	if _, err := call(t, r, "nats/streams", map[string]string{}); err == nil || err.Code != CodeInvalidParams {
		t.Fatalf("missing connectionId: %+v", err)
	}
	if _, err := call(t, r, "nats/streams", map[string]string{"connectionId": "x"}); err == nil || err.Code != CodeNotConnected {
		t.Fatalf("not connected: %+v", err)
	}
}

func connectedRouter(t *testing.T, readOnly bool) *Router {
	t.Helper()
	r, _ := connectedRouterURL(t, readOnly)
	return r
}

func connectedRouterURL(t *testing.T, readOnly bool) (*Router, string) {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port, opts.JetStream, opts.StoreDir = -1, true, t.TempDir()
	srv := natsserver.RunServer(&opts)
	t.Cleanup(srv.Shutdown)
	m := natsx.NewManager()
	t.Cleanup(m.CloseAll)
	r := NewRouter(m)
	lifecycle := map[string]any{
		"connection": map[string]any{"id": "c1", "host": "127.0.0.1", "port": srv.Addr().(*net.TCPAddr).Port, "read_only": readOnly},
	}
	if res, err := call(t, r, "connection/connect", lifecycle); err != nil || res["success"] != true {
		t.Fatalf("connect: %+v %+v", res, err)
	}
	return r, srv.ClientURL()
}

// Sequences past 2^53 must cross the bridge as strings or JS rounds them.
func TestSequencesBeyondFloat64Precision(t *testing.T) {
	r, url := connectedRouterURL(t, false)
	const first = uint64(1)<<60 + 1
	seqStr := func(n uint64) string { return strconv.FormatUint(n, 10) }

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	ctx := context.Background()
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "BIG", Subjects: []string{"big.>"}, FirstSeq: first}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := js.Publish(ctx, "big.x", []byte(`{"n":`+strconv.Itoa(i)+`}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := js.CreateConsumer(ctx, "BIG", jetstream.ConsumerConfig{Durable: "d"}); err != nil {
		t.Fatal(err)
	}
	with := func(extra map[string]any) map[string]any {
		extra["connectionId"] = "c1"
		return extra
	}
	seqs := func(page map[string]any) (out []any) {
		for _, m := range page["messages"].([]any) {
			out = append(out, m.(map[string]any)["seq"])
		}
		return out
	}

	detail, rpcErr := call(t, r, "nats/stream", with(map[string]any{"stream": "BIG"}))
	if rpcErr != nil || detail["firstSeq"] != seqStr(first) || detail["lastSeq"] != seqStr(first+2) {
		t.Fatalf("stream detail = %+v %+v", detail, rpcErr)
	}

	page, rpcErr := call(t, r, "nats/streamMessages", with(map[string]any{"stream": "BIG", "limit": 2}))
	if rpcErr != nil || !reflect.DeepEqual(seqs(page), []any{seqStr(first + 2), seqStr(first + 1)}) {
		t.Fatalf("page = %+v %+v", page, rpcErr)
	}
	next, rpcErr := call(t, r, "nats/streamMessages", with(map[string]any{"stream": "BIG", "limit": 2, "before": page["nextBefore"]}))
	if rpcErr != nil || !reflect.DeepEqual(seqs(next), []any{seqStr(first)}) {
		t.Fatalf("older page = %+v %+v", next, rpcErr)
	}
	if _, ok := next["nextBefore"]; ok {
		t.Fatalf("last page must omit nextBefore, got %v", next["nextBefore"])
	}

	if _, rpcErr := call(t, r, "nats/messageDelete", with(map[string]any{"stream": "BIG", "seq": seqStr(first + 1)})); rpcErr != nil {
		t.Fatal(rpcErr.Message)
	}
	after, _ := call(t, r, "nats/streamMessages", with(map[string]any{"stream": "BIG"}))
	if !reflect.DeepEqual(seqs(after), []any{seqStr(first + 2), seqStr(first)}) {
		t.Fatalf("after delete = %+v (exactly seq %d must be gone)", seqs(after), first+1)
	}

	consumers, rpcErr := call(t, r, "nats/consumers", with(map[string]any{"stream": "BIG"}))
	if rpcErr != nil {
		t.Fatal(rpcErr.Message)
	}
	if c := consumers["consumers"].([]any)[0].(map[string]any); c["deliveredSeq"] != seqStr(first-1) || c["ackFloorSeq"] != seqStr(first-1) {
		t.Fatalf("consumer seqs must be strings: %+v", c)
	}

	if _, rpcErr := call(t, r, "nats/kvBucketCreate", with(map[string]any{"bucketSpec": map[string]any{"bucket": "cfg"}})); rpcErr != nil {
		t.Fatal(rpcErr.Message)
	}
	put, rpcErr := call(t, r, "nats/kvPut", with(map[string]any{"bucket": "cfg", "key": "k", "data": "v"}))
	if rpcErr != nil || put["revision"] != "1" {
		t.Fatalf("kvPut = %+v %+v", put, rpcErr)
	}
	got, rpcErr := call(t, r, "nats/kvGet", with(map[string]any{"bucket": "cfg", "key": "k"}))
	if rpcErr != nil || got["revision"] != "1" {
		t.Fatalf("kvGet = %+v %+v", got, rpcErr)
	}
}

func TestReadOnlyCode(t *testing.T) {
	r := connectedRouter(t, true)
	_, err := call(t, r, "nats/kvBucketCreate", map[string]any{"connectionId": "c1", "bucketSpec": map[string]any{"bucket": "b"}})
	if err == nil || err.Code != CodeReadOnly {
		t.Fatalf("write on read-only connection: %+v", err)
	}
}

func TestFilesystemProvider(t *testing.T) {
	r := connectedRouter(t, false)
	id := map[string]any{"connectionId": "c1"}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{"connectionId": "c1"}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	if _, err := call(t, r, "nats/kvBucketCreate", with(map[string]any{"bucketSpec": map[string]any{"bucket": "cfg"}})); err != nil {
		t.Fatal(err.Message)
	}
	_ = id

	root, err := call(t, r, "filesystem/list", with(map[string]any{"uri": "nats:///"}))
	if err != nil || len(root["entries"].([]any)) != 3 {
		t.Fatalf("root = %+v %+v", root, err)
	}
	keyURI := uri("kv", "cfg", "app/theme")
	data := base64.StdEncoding.EncodeToString([]byte("dark"))
	if _, err := call(t, r, "filesystem/write", with(map[string]any{"uri": keyURI, "dataBase64": data})); err != nil {
		t.Fatal(err.Message)
	}
	listing, err := call(t, r, "filesystem/list", with(map[string]any{"uri": uri("kv", "cfg")}))
	if err != nil {
		t.Fatal(err.Message)
	}
	entries := listing["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["uri"] != keyURI {
		t.Fatalf("kv listing = %+v", entries)
	}
	read, err := call(t, r, "filesystem/read", with(map[string]any{"uri": keyURI}))
	if err != nil || read["dataBase64"] != data {
		t.Fatalf("read = %+v %+v", read, err)
	}
	if _, err := call(t, r, "filesystem/delete", with(map[string]any{"uri": keyURI})); err != nil {
		t.Fatal(err.Message)
	}
	if _, err := call(t, r, "filesystem/write", with(map[string]any{"uri": uri("streams", "S", "1"), "dataBase64": data})); err == nil {
		t.Fatal("writing into a stream must be rejected")
	}
	if _, err := call(t, r, "filesystem/list", with(map[string]any{"uri": "s3://x"})); err == nil || err.Code != CodeInvalidParams {
		t.Fatalf("foreign scheme: %+v", err)
	}
}
