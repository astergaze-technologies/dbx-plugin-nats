package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
)

// lifecycleParams is the payload DBX sends with connection/test, /connect and /disconnect.
// `connection` is DBX's ConnectionConfig: common fields plus `external_config`
// (config-bound form fields) and `connection_secrets` (secret-bound form fields).
type lifecycleParams struct {
	Connection struct {
		ID                string            `json:"id"`
		Name              string            `json:"name"`
		Host              string            `json:"host"`
		Port              json.Number       `json:"port"`
		Username          string            `json:"username"`
		Password          string            `json:"password"`
		ExternalConfig    map[string]any    `json:"external_config"`
		ConnectionSecrets map[string]string `json:"connection_secrets"`
	} `json:"connection"`
	// Runtime is the final endpoint after DBX transport layers (SSH tunnels etc.).
	Runtime struct {
		Host string      `json:"host"`
		Port json.Number `json:"port"`
	} `json:"runtime"`
}

func decodeLifecycle(raw json.RawMessage) (*lifecycleParams, error) {
	var p lifecycleParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.New("invalid lifecycle parameters")
	}
	return &p, nil
}

// config builds the dial config. The runtime endpoint wins so tunnels are honoured.
func (p *lifecycleParams) config() (natsx.Config, error) {
	c := p.Connection
	cfg := natsx.Config{
		Name:        c.Name,
		Host:        c.Host,
		Auth:        stringConfig(c.ExternalConfig, "auth", natsx.AuthNone),
		Username:    c.Username,
		Password:    firstNonEmpty(c.Password, c.ConnectionSecrets["password"]),
		Token:       c.ConnectionSecrets["token"],
		Creds:       c.ConnectionSecrets["creds"],
		CredsPath:   stringConfig(c.ExternalConfig, "creds_path", ""),
		TLS:         boolConfig(c.ExternalConfig, "tls"),
		TLSInsecure: boolConfig(c.ExternalConfig, "tls_insecure"),
		Timeout:     time.Duration(intConfig(c.ExternalConfig, "connect_timeout_secs", 10)) * time.Second,
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func stringConfig(config map[string]any, key, fallback string) string {
	if v, ok := config[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

// boolConfig accepts real booleans and their string forms ("true").
func boolConfig(config map[string]any, key string) bool {
	switch v := config[key].(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

func intConfig(config map[string]any, key string, fallback int) int {
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
