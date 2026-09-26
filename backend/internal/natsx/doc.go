// Package natsx holds the NATS side of the plugin: per-connection sessions,
// JetStream/KV reads, publish/request and live subscriptions. It has no DBX SDK
// dependency so it can be tested with a plain `go test`.
package natsx
