package rpc

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/test"

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
	return r
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
