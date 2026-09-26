package rpc

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/astergaze-solutions/dbx-plugin-nats/internal/natsx"
)

// The filesystem provider exposes NATS primitives to DBX's native file browser:
//
//	nats:///kv/<bucket>/<key>
//	nats:///objects/<store>/<object>
//	nats:///streams/<stream>/<seq>
//
// Segments are path-escaped because keys and object names may contain '/'.
const scheme = "nats://"

type fsParams struct {
	ConnectionID string `json:"connectionId"`
	URI          string `json:"uri"`
	Cursor       string `json:"cursor"`
	Limit        int    `json:"limit"`
	DataBase64   []byte `json:"dataBase64"`
}

func (p fsParams) connection() string { return p.ConnectionID }

type entry struct {
	Name       string     `json:"name"`
	URI        string     `json:"uri"`
	Kind       string     `json:"kind"`
	Size       *uint64    `json:"size,omitempty"`
	ModifiedAt *time.Time `json:"modifiedAt,omitempty"`
}

var errUnsupported = errors.New("not supported at this location")

func uri(segments ...string) string {
	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}
	return scheme + "/" + strings.Join(escaped, "/")
}

func dir(name string, segments ...string) entry {
	return entry{Name: name, URI: uri(segments...), Kind: "directory"}
}

func file(name string, size uint64, modified time.Time, segments ...string) entry {
	e := entry{Name: name, URI: uri(segments...), Kind: "file", Size: &size}
	if !modified.IsZero() {
		e.ModifiedAt = &modified
	}
	return e
}

func parseURI(raw string) ([]string, error) {
	if !strings.HasPrefix(raw, scheme) {
		return nil, invalidParams{errors.New("uri must start with nats://")}
	}
	var out []string
	for _, part := range strings.Split(strings.Trim(strings.TrimPrefix(raw, scheme), "/"), "/") {
		if part == "" {
			continue
		}
		s, err := url.PathUnescape(part)
		if err != nil {
			return nil, invalidParams{err}
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *Router) registerFilesystem() {
	on(r, "filesystem/list", r.fsList)
	on(r, "filesystem/read", r.fsRead)
	on(r, "filesystem/write", r.fsWrite)
	on(r, "filesystem/delete", r.fsDelete)
}

func (r *Router) fsList(ctx context.Context, p fsParams, _ natsx.Emit) (any, error) {
	path, err := parseURI(p.URI)
	if err != nil {
		return nil, err
	}
	m, id := r.nats, p.ConnectionID
	entries := []entry{}
	var next string
	switch {
	case len(path) == 0:
		entries = []entry{dir("streams", "streams"), dir("kv", "kv"), dir("objects", "objects")}
	case len(path) == 1 && path[0] == "streams":
		streams, err := m.Streams(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, s := range streams {
			entries = append(entries, dir(s.Name, "streams", s.Name))
		}
	case len(path) == 2 && path[0] == "streams":
		before, _ := strconv.ParseUint(p.Cursor, 10, 64)
		page, err := m.StreamMessages(ctx, id, path[1], before, p.Limit)
		if err != nil {
			return nil, err
		}
		for _, msg := range page.Messages {
			seq := strconv.FormatUint(msg.Sequence, 10)
			entries = append(entries, file(seq+" · "+msg.Subject, uint64(msg.Size), msg.Time, "streams", path[1], seq))
		}
		if page.NextBefore > 0 {
			next = strconv.FormatUint(page.NextBefore, 10)
		}
	case len(path) == 1 && path[0] == "kv":
		buckets, err := m.Buckets(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, b := range buckets {
			entries = append(entries, dir(b.Bucket, "kv", b.Bucket))
		}
	case len(path) == 2 && path[0] == "kv":
		keys, err := m.Keys(ctx, id, path[1])
		if err != nil {
			return nil, err
		}
		slices.Sort(keys.Keys)
		for _, k := range keys.Keys {
			entries = append(entries, file(url.PathEscape(k), 0, time.Time{}, "kv", path[1], k))
		}
	case len(path) == 1 && path[0] == "objects":
		stores, err := m.ObjectStores(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, s := range stores {
			entries = append(entries, dir(s.Store, "objects", s.Store))
		}
	case len(path) == 2 && path[0] == "objects":
		objects, err := m.Objects(ctx, id, path[1])
		if err != nil {
			return nil, err
		}
		for _, o := range objects {
			entries = append(entries, file(url.PathEscape(o.Name), o.Size, o.Modified, "objects", path[1], o.Name))
		}
	default:
		return nil, errUnsupported
	}
	result := map[string]any{"entries": entries}
	if next != "" {
		result["nextCursor"] = next
	}
	return result, nil
}

func (r *Router) fsRead(ctx context.Context, p fsParams, _ natsx.Emit) (any, error) {
	path, err := parseURI(p.URI)
	if err != nil {
		return nil, err
	}
	if len(path) != 3 {
		return nil, errUnsupported
	}
	m, id := r.nats, p.ConnectionID
	var (
		data      []byte
		truncated bool
	)
	switch path[0] {
	case "streams":
		seq, err := strconv.ParseUint(path[2], 10, 64)
		if err != nil {
			return nil, invalidParams{err}
		}
		msg, err := m.StreamMessage(ctx, id, path[1], seq)
		if err != nil {
			return nil, err
		}
		data, truncated = decode(msg.Payload)
	case "kv":
		v, err := m.Get(ctx, id, path[1], path[2])
		if err != nil {
			return nil, err
		}
		data, truncated = decode(v.Payload)
	case "objects":
		o, err := m.GetObject(ctx, id, path[1], path[2])
		if err != nil {
			return nil, err
		}
		data = o.Data
	default:
		return nil, errUnsupported
	}
	return map[string]any{"dataBase64": data, "truncated": truncated}, nil
}

func (r *Router) fsWrite(ctx context.Context, p fsParams, _ natsx.Emit) (any, error) {
	path, err := parseURI(p.URI)
	if err != nil {
		return nil, err
	}
	if len(path) != 3 {
		return nil, errUnsupported
	}
	switch path[0] {
	case "kv":
		_, err = r.nats.Put(ctx, p.ConnectionID, path[1], path[2], p.DataBase64)
	case "objects":
		_, err = r.nats.PutObject(ctx, p.ConnectionID, path[1], path[2], p.DataBase64)
	default:
		err = errUnsupported
	}
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

func (r *Router) fsDelete(ctx context.Context, p fsParams, _ natsx.Emit) (any, error) {
	path, err := parseURI(p.URI)
	if err != nil {
		return nil, err
	}
	if len(path) != 3 {
		return nil, errUnsupported
	}
	switch path[0] {
	case "streams":
		seq, perr := strconv.ParseUint(path[2], 10, 64)
		if perr != nil {
			return nil, invalidParams{perr}
		}
		err = r.nats.DeleteMessage(ctx, p.ConnectionID, path[1], seq)
	case "kv":
		err = r.nats.DeleteKey(ctx, p.ConnectionID, path[1], path[2], false)
	case "objects":
		err = r.nats.DeleteObject(ctx, p.ConnectionID, path[1], path[2])
	default:
		err = errUnsupported
	}
	if err != nil {
		return nil, err
	}
	return map[string]bool{"success": true}, nil
}

// decode restores bytes from a payload, which is capped at natsx.MaxPayloadBytes.
func decode(p natsx.Payload) ([]byte, bool) {
	if p.Encoding == "base64" {
		b, _ := base64.StdEncoding.DecodeString(p.Data)
		return b, p.Truncated
	}
	return []byte(p.Data), p.Truncated
}
