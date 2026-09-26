// Command dbx-plugin-nats is the DBX sidecar for NATS connections.
// stdout carries the DBX protocol only; diagnostics go to stderr.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

// version must match manifest.json; DBX rejects a backend identity mismatch.
var version = "0.1.0"

const pluginID = "com.astergaze.nats"

// JSON-RPC error codes.
const (
	codeInvalidParams = -32602
	codeNotConnected  = -32010
	codeFailed        = -32000
)

// requestTimeout bounds JetStream/KV reads issued from the workbench.
const requestTimeout = 15 * time.Second

type plugin struct {
	nats *natsx.Manager
}

// workbenchParams covers every workbench method; each uses the fields it needs.
type workbenchParams struct {
	ConnectionID   string            `json:"connectionId"`
	Stream         string            `json:"stream"`
	Before         uint64            `json:"before"`
	Limit          int               `json:"limit"`
	Bucket         string            `json:"bucket"`
	Key            string            `json:"key"`
	Subject        string            `json:"subject"`
	Queue          string            `json:"queue"`
	Data           string            `json:"data"`
	Headers        map[string]string `json:"headers"`
	TimeoutMs      int               `json:"timeoutMs"`
	SubscriptionID string            `json:"subscriptionId"`
}

func (p *plugin) Handle(
	_ dbxpluginsdk.RequestContext,
	method string,
	params json.RawMessage,
	emitter *dbxpluginsdk.Emitter,
) (any, *dbxpluginsdk.PluginError) {
	switch method {
	case "connection/test", "connection/connect", "connection/disconnect":
		return p.lifecycle(method, params, emitter)
	}

	var w workbenchParams
	if err := json.Unmarshal(params, &w); err != nil {
		return nil, dbxpluginsdk.NewError(codeInvalidParams, "Invalid request parameters")
	}
	if w.ConnectionID == "" {
		return nil, dbxpluginsdk.NewError(codeInvalidParams, "Missing connectionId")
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	var (
		result any
		err    error
	)
	switch method {
	case "nats/overview":
		result, err = p.nats.Overview(ctx, w.ConnectionID)
	case "nats/streams":
		streams, e := p.nats.Streams(ctx, w.ConnectionID)
		result, err = map[string]any{"streams": streams}, e
	case "nats/streamMessages":
		result, err = p.nats.StreamMessages(ctx, w.ConnectionID, w.Stream, w.Before, w.Limit)
	case "nats/kvBuckets":
		buckets, e := p.nats.Buckets(ctx, w.ConnectionID)
		result, err = map[string]any{"buckets": buckets}, e
	case "nats/kvKeys":
		result, err = p.nats.Keys(ctx, w.ConnectionID, w.Bucket)
	case "nats/kvGet":
		result, err = p.nats.Get(ctx, w.ConnectionID, w.Bucket, w.Key)
	case "nats/publish":
		err = p.nats.Publish(w.ConnectionID, w.Subject, w.Data, w.Headers)
		result = map[string]any{"ok": err == nil}
	case "nats/request":
		result, err = p.nats.Request(w.ConnectionID, w.Subject, w.Data, w.Headers,
			time.Duration(w.TimeoutMs)*time.Millisecond)
	case "nats/subscribe":
		emit := func(event string, payload any) {
			if e := emitter.Event(event, payload); e != nil {
				log.Printf("emit %s: %s", event, e.Message)
			}
		}
		id, e := p.nats.Subscribe(w.ConnectionID, w.Subject, w.Queue, emit)
		result, err = map[string]any{"subscriptionId": id}, e
	case "nats/unsubscribe":
		err = p.nats.Unsubscribe(w.ConnectionID, w.SubscriptionID)
		result = map[string]any{"ok": err == nil}
	default:
		return nil, dbxpluginsdk.MethodNotFound(method)
	}
	if err != nil {
		return nil, toPluginError(err)
	}
	return result, nil
}

func (p *plugin) lifecycle(method string, raw json.RawMessage, emitter *dbxpluginsdk.Emitter) (any, *dbxpluginsdk.PluginError) {
	params, err := decodeLifecycle(raw)
	if err != nil {
		return nil, dbxpluginsdk.NewError(codeInvalidParams, err.Error())
	}
	id := params.Connection.ID

	if method == "connection/disconnect" {
		if id == "" {
			return nil, dbxpluginsdk.NewError(codeInvalidParams, "Missing connection id")
		}
		p.nats.Disconnect(id)
		_ = emitter.Event("nats/connectionChanged", map[string]any{"connectionId": id, "state": "disconnected"})
		return map[string]any{"success": true}, nil
	}

	cfg, err := params.config()
	if err != nil {
		return nil, dbxpluginsdk.NewError(codeInvalidParams, err.Error())
	}
	if method == "connection/test" {
		message, err := natsx.Test(cfg)
		if err != nil {
			return map[string]any{"success": false, "message": err.Error()}, nil
		}
		return map[string]any{"success": true, "message": message}, nil
	}

	if id == "" {
		return nil, dbxpluginsdk.NewError(codeInvalidParams, "Missing connection id")
	}
	if err := p.nats.Connect(id, cfg); err != nil {
		return nil, dbxpluginsdk.NewError(codeFailed, err.Error())
	}
	_ = emitter.Event("nats/connectionChanged", map[string]any{"connectionId": id, "state": "connected"})
	return map[string]any{"success": true}, nil
}

func toPluginError(err error) *dbxpluginsdk.PluginError {
	if errors.Is(err, natsx.ErrNotConnected) {
		return dbxpluginsdk.NewError(codeNotConnected, err.Error())
	}
	return dbxpluginsdk.NewError(codeFailed, err.Error())
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[" + pluginID + "] ")

	p := &plugin{nats: natsx.NewManager()}
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
		p.nats.CloseAll()
		os.Exit(0)
	}()

	metadata := dbxpluginsdk.Metadata{
		ID:           pluginID,
		Version:      version,
		Capabilities: []string{"connections", "events"},
	}
	err := dbxpluginsdk.NewServer(metadata, p).Serve()
	p.nats.CloseAll()
	if err != nil {
		log.Fatal(err)
	}
}
