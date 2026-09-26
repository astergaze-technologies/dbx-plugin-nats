# NATS for DBX

A [DBX](https://dbxio.com) plugin for [NATS](https://nats.io) servers: browse JetStream streams and their messages, key-value buckets, publish and request-reply, and watch subjects live — inside DBX.

Plugin ID `com.astergaze.nats` · publisher `astergaze` · Go sidecar + sandboxed workbench UI.

## Features

| Tab | What it does |
| --- | --- |
| **Overview** | Server name, version, cluster, TLS, RTT, max payload; JetStream storage/memory used against account limits. |
| **Streams** | Filterable list of streams; open one to read its messages newest first, with paging. JSON payloads are pretty-printed, binary payloads shown as base64. |
| **Key-Value** | Buckets with entries, size, history and TTL; filter keys and read a key's latest value and revision. |
| **Publish** | Publish a message with headers, or send a request and show the reply. |
| **Subscribe** | Several live subscriptions at once (wildcards and queue groups), pause, clear, per-subscription counts. |

JetStream and KV access is **read-only**: the plugin never creates, edits or deletes streams, consumers, buckets or keys. Servers without JetStream still get Overview, Publish and Subscribe.

### Connection options

- Host / port (DBX SSH tunnels and proxies are honoured — the sidecar dials the endpoint DBX hands it)
- Authentication: none, user/password, token, or a `.creds` file (JWT + seed)
- TLS, with an optional "skip verification" for self-signed test servers
- Connect timeout

Passwords, tokens and credentials are stored in DBX's secret store, never in the connection JSON.

### Limits (deliberate)

- Live subscriptions forward at most **100 messages/second** each; the excess is counted and shown as "dropped" rather than flooding DBX. The UI keeps the latest 500.
- Message bodies over 64 KiB are truncated in the UI (the size is still shown).
- A stream page holds up to 200 messages; key listings stop at 1,000 keys.
- Request-reply timeouts are capped at 60 s.

## Architecture

```
manifest.json            connection form, workbench, permissions (host.events only)
ui/index.html            workbench: vanilla JS, DBX UI kit + theme tokens, no build step
backend/
  main.go                DBX protocol wiring (JSON-RPC over stdio via the DBX Go SDK)
  lifecycle.go           DBX connection payload -> dial config
  internal/natsx/        all NATS logic, no SDK dependency, tested against an in-process nats-server
  third_party/dbx-plugin-sdk/   vendored DBX Go SDK (see its README)
```

Sidecar methods (called by the workbench with `connectionId`):

| Method | Params | Result |
| --- | --- | --- |
| `nats/overview` | — | server info, JetStream usage |
| `nats/streams` | — | `{ streams }` |
| `nats/streamMessages` | `stream`, `before?`, `limit?` | `{ messages, nextBefore }` newest first |
| `nats/kvBuckets` | — | `{ buckets }` |
| `nats/kvKeys` | `bucket` | `{ keys, truncated }` |
| `nats/kvGet` | `bucket`, `key` | value, revision, operation |
| `nats/publish` | `subject`, `data`, `headers?` | `{ ok }` |
| `nats/request` | `subject`, `data`, `headers?`, `timeoutMs?` | reply |
| `nats/subscribe` | `subject`, `queue?` | `{ subscriptionId }` |
| `nats/unsubscribe` | `subscriptionId` | `{ ok }` |

Events: `nats/message`, `nats/subscriptionClosed`, `nats/connectionChanged`.

## Develop

Requirements: Go 1.26+, Node.js 22+ and the DBX plugin CLI (`npm i -g @dbx-app/plugin-cli`, or use `npx @dbx-app/plugin-cli`).

```bash
cd backend && go test -race ./...     # unit + integration tests (embedded nats-server)
dbx-plugin package .                   # dist/com.astergaze.nats-<version>-<target>.dbxp
dbx-plugin dev --path . --port 5190    # browser dev host with the real sidecar
```

> **`dbx-plugin dev` and newer Go modules.** CLI 0.1.9's dev host writes a `go.work` declaring `go 1.22`,
> so Go builds of modules that need a newer Go fail with *"module . listed in go.work file requires go >= 1.26.0"*.
> `dbx-plugin package` is unaffected (it builds with `GOWORK=off`). Until that is fixed upstream, run the dev host
> with a patched runtime: copy the CLI's `dev-runtime/` folder, change `go 1.22` to `go 1.26.0` in `runtime.mjs`,
> and start with `DBX_PLUGIN_DEV_RUNTIME=/path/to/dev-runtime/runtime.mjs dbx-plugin dev --path .`.

The workbench iframe is sandboxed without `allow-forms`: use click/Enter handlers, never `<form>` submission.
All server data is rendered with `textContent`.

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
