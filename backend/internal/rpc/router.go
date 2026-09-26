// Package rpc maps DBX JSON-RPC methods onto natsx operations.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
)

const (
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeFailed         = -32000
	CodeNotConnected   = -32010
	CodeReadOnly       = -32011
)

const callTimeout = 20 * time.Second

type Error struct {
	Code    int
	Message string
}

type handler func(ctx context.Context, raw json.RawMessage, emit natsx.Emit) (any, error)

type Router struct {
	nats     *natsx.Manager
	handlers map[string]handler
}

func NewRouter(m *natsx.Manager) *Router {
	r := &Router{nats: m, handlers: map[string]handler{}}
	r.registerLifecycle()
	r.registerServer()
	r.registerStreams()
	r.registerKV()
	r.registerObjects()
	r.registerMessaging()
	r.registerFilesystem()
	return r
}

func (r *Router) Call(method string, raw json.RawMessage, emit natsx.Emit) (any, *Error) {
	h, ok := r.handlers[method]
	if !ok {
		return nil, &Error{CodeMethodNotFound, "Method not found: " + method}
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	result, err := h(ctx, raw, emit)
	if err != nil {
		return nil, toError(err)
	}
	return result, nil
}

type invalidParams struct{ error }

func toError(err error) *Error {
	var invalid invalidParams
	switch {
	case errors.As(err, &invalid):
		return &Error{CodeInvalidParams, err.Error()}
	case errors.Is(err, natsx.ErrNotConnected):
		return &Error{CodeNotConnected, err.Error()}
	case errors.Is(err, natsx.ErrReadOnly):
		return &Error{CodeReadOnly, err.Error()}
	}
	return &Error{CodeFailed, err.Error()}
}

// connected is implemented by params that must name an open connection.
type connected interface{ connection() string }

// on registers a handler whose params decode into P.
func on[P any](r *Router, method string, fn func(ctx context.Context, p P, emit natsx.Emit) (any, error)) {
	r.handlers[method] = func(ctx context.Context, raw json.RawMessage, emit natsx.Emit) (any, error) {
		var p P
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidParams{errors.New("invalid request parameters")}
		}
		if c, ok := any(p).(connected); ok && c.connection() == "" {
			return nil, invalidParams{errors.New("missing connectionId")}
		}
		return fn(ctx, p, emit)
	}
}

func ok(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}
