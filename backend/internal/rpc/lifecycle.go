package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
)

// lifecycleParams is DBX's connection/test|connect|disconnect payload.
type lifecycleParams struct {
	Connection struct {
		ID                string            `json:"id"`
		Name              string            `json:"name"`
		Host              string            `json:"host"`
		Port              json.Number       `json:"port"`
		Username          string            `json:"username"`
		Password          string            `json:"password"`
		ReadOnly          bool              `json:"read_only"`
		ExternalConfig    map[string]any    `json:"external_config"`
		ConnectionSecrets map[string]string `json:"connection_secrets"`
	} `json:"connection"`
	// Runtime is the endpoint after DBX transport layers (SSH tunnels, proxies).
	Runtime struct {
		Host string      `json:"host"`
		Port json.Number `json:"port"`
	} `json:"runtime"`
}

func (p lifecycleParams) config() (natsx.Config, error) {
	c := p.Connection
	cfg := natsx.Config{
		Name:        c.Name,
		Host:        c.Host,
		Auth:        stringValue(c.ExternalConfig, "auth", natsx.AuthNone),
		Username:    c.Username,
		Password:    firstNonEmpty(c.Password, c.ConnectionSecrets["password"]),
		Token:       c.ConnectionSecrets["token"],
		Creds:       c.ConnectionSecrets["creds"],
		CredsPath:   stringValue(c.ExternalConfig, "creds_path", ""),
		TLS:         boolValue(c.ExternalConfig, "tls"),
		TLSInsecure: boolValue(c.ExternalConfig, "tls_insecure"),
		Timeout:     time.Duration(intValue(c.ExternalConfig, "connect_timeout_secs", 10)) * time.Second,
		ReadOnly:    c.ReadOnly,
	}
	port := c.Port
	if p.Runtime.Host != "" {
		cfg.Host, port = p.Runtime.Host, p.Runtime.Port
	}
	n, err := strconv.Atoi(port.String())
	if err != nil || n <= 0 || n > 65535 {
		return cfg, fmt.Errorf("invalid port %q", port.String())
	}
	cfg.Port = n
	return cfg, nil
}

func (r *Router) registerLifecycle() {
	on(r, "connection/test", func(_ context.Context, p lifecycleParams, _ natsx.Emit) (any, error) {
		cfg, err := p.config()
		if err != nil {
			return nil, invalidParams{err}
		}
		message, err := natsx.Test(cfg)
		if err != nil {
			return map[string]any{"success": false, "message": err.Error()}, nil
		}
		return map[string]any{"success": true, "message": message}, nil
	})
	on(r, "connection/connect", func(_ context.Context, p lifecycleParams, emit natsx.Emit) (any, error) {
		cfg, err := p.config()
		if err != nil {
			return nil, invalidParams{err}
		}
		if p.Connection.ID == "" {
			return nil, invalidParams{errors.New("missing connection id")}
		}
		if err := r.nats.Connect(p.Connection.ID, cfg); err != nil {
			return nil, err
		}
		emit("nats/connectionChanged", map[string]string{"connectionId": p.Connection.ID, "state": "connected"})
		return map[string]bool{"success": true}, nil
	})
	on(r, "connection/disconnect", func(_ context.Context, p lifecycleParams, emit natsx.Emit) (any, error) {
		if p.Connection.ID == "" {
			return nil, invalidParams{errors.New("missing connection id")}
		}
		r.nats.Disconnect(p.Connection.ID)
		emit("nats/connectionChanged", map[string]string{"connectionId": p.Connection.ID, "state": "disconnected"})
		return map[string]bool{"success": true}, nil
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func stringValue(config map[string]any, key, fallback string) string {
	if v, ok := config[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func boolValue(config map[string]any, key string) bool {
	switch v := config[key].(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

func intValue(config map[string]any, key string, fallback int) int {
	switch v := config[key].(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case string:
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
