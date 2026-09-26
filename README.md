# NATS for DBX

A [DBX](https://dbxio.com) plugin for [NATS](https://nats.io) servers: JetStream streams, consumers, key-value buckets, object stores, micro services, publish/request and live subscriptions — inside DBX.

Plugin ID `com.astergaze.nats` · publisher `astergaze` · Go sidecar + Vue workbench.

## Features

The workbench sits next to DBX's sidebar with no extra column: a section bar (Server, Streams, Key-Value, Object Store, Services, Publish, Subscribe) on top, and each stream, bucket or store you open gets its own closable tab. DBX plugins cannot add nodes to the DBX sidebar itself.

| Area | Read | Write (disabled on read-only connections) |
| --- | --- | --- |
| **Server** | name, version, cluster, TLS, RTT; JetStream usage against account limits | — |
| **Streams** | messages newest first with paging, consumers (pending, ack pending, redelivered), configuration | create, edit, purge, delete stream; delete message or consumer |
| **Key-Value** | keys, value history per key, live watch | create/delete bucket, put, delete key |
| **Object Store** | objects, preview | create/delete store, upload (≤ 4 MiB), delete object |
| **Services** | NATS micro services via `$SRV.INFO` / `$SRV.STATS` with per-endpoint stats | — |
| **Publish / Subscribe** | live subscriptions (wildcards, queue groups) | publish, request-reply |

Destructive actions ask for confirmation; deleting or purging a stream, bucket or store requires typing its name.
Streams backing KV buckets and object stores (`KV_*`, `OBJ_*`) are hidden unless "Show KV / Object Store streams" is ticked.

**Browse as files** opens DBX's native file browser (`nats:///streams|kv|objects/...`) through a filesystem provider.

### Connection options

- Host / port (DBX SSH tunnels and proxies are honoured — the sidecar dials the endpoint DBX hands it)
- Authentication: none, user/password, token, or a `.creds` file (JWT + seed)
- TLS, with an optional "skip verification" for self-signed test servers
- Connect timeout; DBX's read-only flag is enforced by the sidecar

Passwords, tokens and credentials are stored in DBX's secret store, never in the connection JSON.

### Limits (deliberate)

- Live subscriptions and KV watches forward at most **100 messages/second** each; the excess is counted as "dropped". The UI keeps the latest 500.
- Message bodies over 64 KiB are truncated in the UI (the size is still shown); key listings stop at 1,000 keys.
- Request-reply timeouts are capped at 60 s.

## Architecture

```
manifest.json                 connection form, workbench, filesystem provider, permissions
src/                          workbench UI (Vue 3 + TypeScript); Vite builds it into ui/ (generated, not committed)
  api/                        DBX bridge (invoke, events) and response types
  components/                 data table, dialogs, message card, buttons
  stores/                     open tabs, dialogs, navigation helpers
  views/<area>/               one folder per section
public/index.html             workbench shell, copied into ui/ by the build
backend/
  main.go                     DBX protocol wiring (JSON-RPC over stdio via the DBX Go SDK)
  internal/rpc/               method routing, params, lifecycle, filesystem provider
  internal/natsx/             NATS logic, one file per primitive, tested against an in-process nats-server
  third_party/dbx-plugin-sdk/ vendored DBX Go SDK (see its README)
```

Sidecar methods are registered in `backend/internal/rpc/nats.go`; events are `nats/message`, `nats/kvChange`, `nats/feedClosed` and `nats/connectionChanged`.

The workbench iframe's CSP allows only inline and same-origin classic scripts, so the UI is bundled into a single IIFE (no ES module imports at runtime). It is also sandboxed without `allow-forms`: use click/Enter handlers, never `<form>` submission.

## Develop

Requirements: Go 1.26+ and Node.js 22+.

```bash
npm install                            # also installs the DBX plugin CLI locally
npm run package                        # typecheck + bundle UI, build sidecar -> dist/com.astergaze.nats-<version>-<target>.dbxp
npm run dev                            # browser dev host on :5190 with the real sidecar (rebuilds the UI on change)
cd backend && go test -race ./...      # unit + integration tests (embedded nats-server)
```

### Try it against a local server

```bash
docker run --rm -p 4222:4222 nats:latest -js
nats stream add ORDERS --subjects 'orders.>' --defaults
nats pub orders.created '{"id":1}' --count 20
nats kv add config && nats kv put config feature.dark_mode on
nats object add assets && nats object put assets ./README.md
```

Then either use the dev host, or install into DBX: `npm run package`, open DBX → Plugin Center → Settings,
enable development-only unsigned packages, install the `.dbxp`, and create a **NATS** connection to `localhost:4222`.

`npm run dev` goes through `scripts/dev.mjs`: plugin CLI 0.1.9's dev host writes a `go.work` pinned to `go 1.22`,
which fails for this module, so the script runs the dev host from a patched copy that uses the Go version in `backend/go.mod`.

Connection form fields use `select`, not `radio`: DBX desktop does not render `radio` fields (the dev host does).

Keep `version` in `backend/main.go` equal to `manifest.json` — DBX rejects a sidecar whose identity does not match.

## Release

1. Bump `version` in `manifest.json` and `backend/main.go`, tag, and publish a GitHub Release.
   `.github/workflows/plugin-release.yml` builds unsigned `.dbxp` candidates for every target.
2. Open a PR against [`t8y2/dbx-store`](https://github.com/t8y2/dbx-store) (`main`) adding
   `publishers/astergaze.json` (first release only) and `candidates/com.astergaze.nats.json` with the release URLs,
   SHA-256 and sizes from the generated `*.artifact.json` files.
3. DBX Store reviews, signs with the official key and finalizes the catalog in the same PR.

## License

Apache-2.0. The vendored DBX SDK in `backend/third_party/dbx-plugin-sdk` is © the DBX authors, Apache-2.0.
