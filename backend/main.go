// Command dbx-plugin-nats is the DBX sidecar for NATS. stdout carries the protocol; logs go to stderr.
package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
	"github.com/astergaze-solutions/dbx-plugin-nats/internal/rpc"
)

// version must match manifest.json; DBX rejects a mismatched sidecar identity.
var version = "0.1.0"

const pluginID = "com.astergaze.nats"

type handler struct{ router *rpc.Router }

func (h handler) Handle(_ dbxpluginsdk.RequestContext, method string, params json.RawMessage, emitter *dbxpluginsdk.Emitter) (any, *dbxpluginsdk.PluginError) {
	emit := func(event string, payload any) {
		if err := emitter.Event(event, payload); err != nil {
			log.Printf("emit %s: %s", event, err.Message)
		}
	}
	result, err := h.router.Call(method, params, emit)
	if err != nil {
		return nil, dbxpluginsdk.NewError(err.Code, err.Message)
	}
	return result, nil
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[" + pluginID + "] ")

	manager := natsx.NewManager()
	go func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		<-signals
		manager.CloseAll()
		os.Exit(0)
	}()

	metadata := dbxpluginsdk.Metadata{ID: pluginID, Version: version, Capabilities: []string{"connections", "events"}}
	err := dbxpluginsdk.NewServer(metadata, handler{rpc.NewRouter(manager)}).Serve()
	manager.CloseAll()
	if err != nil {
		log.Fatal(err)
	}
}
