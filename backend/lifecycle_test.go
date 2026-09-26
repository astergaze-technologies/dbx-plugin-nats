package main

import (
	"testing"
	"time"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
)

func TestLifecycleConfig(t *testing.T) {
	raw := []byte(`{
		"provider": {"id": "com.astergaze.nats.connection", "databaseType": "nats"},
		"connection": {
			"id": "c1", "name": "Prod", "host": "nats.internal", "port": 4222,
			"username": "app", "password": "pw",
			"external_config": {"auth": "password", "tls": "true", "connect_timeout_secs": 7},
			"connection_secrets": {"token": "t"}
		},
		"runtime": {"host": "127.0.0.1", "port": 49152},
		"operationId": "x"
	}`)
	p, err := decodeLifecycle(raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := p.config()
	if err != nil {
		t.Fatal(err)
	}
	want := natsx.Config{
		Name: "Prod", Host: "127.0.0.1", Port: 49152, Auth: natsx.AuthPassword,
		Username: "app", Password: "pw", Token: "t", TLS: true, Timeout: 7 * time.Second,
	}
	if cfg != want {
		t.Fatalf("config = %+v\nwant     %+v", cfg, want)
	}
	if cfg.URL() != "tls://127.0.0.1:49152" {
		t.Fatalf("url = %s", cfg.URL())
	}
}

func TestLifecycleDefaultsAndPortFallback(t *testing.T) {
	p, err := decodeLifecycle([]byte(`{"connection":{"id":"c","host":"localhost","port":4222},"runtime":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := p.config()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "localhost" || cfg.Port != 4222 || cfg.Auth != natsx.AuthNone || cfg.TLS || cfg.Timeout != 10*time.Second {
		t.Fatalf("defaults = %+v", cfg)
	}

	bad, _ := decodeLifecycle([]byte(`{"connection":{"id":"c","host":"h","port":0}}`))
	if _, err := bad.config(); err == nil {
		t.Fatal("port 0 must be rejected")
	}
	if _, err := decodeLifecycle([]byte(`{`)); err == nil {
		t.Fatal("invalid JSON must be rejected")
	}
}
