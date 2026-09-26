# litefaas

Poor man's serverless: a minimal **CLI + API** control plane to build and deploy **functions**, **backends** (Java / Go / Python), and a **lightweight frontend**, self-hosted on a single node.

> Status: RFC accepted — Phases 0–2 are in tree. A Go function can be initialized, built, deployed, and invoked on one Docker host.

## Docs

- **[RFC-0001: Architecture](docs/RFC-0001-architecture.md)** — design, manifests (`litefaas.yaml`), API, phases
- Kinds: `function` | `backend` | `frontend`
- Runtimes: `go` (Phase 2) · `java` | `python` | `dockerfile` | `static` (later phases)

## Build

Requires [Go 1.22+](https://go.dev/dl/). Docker is required only for `lf build` / `lf deploy` / the live invoke path.

```bash
git clone https://github.com/wolvever/litefaas.git
cd litefaas
go build -o lf ./cmd/lf
go build -o litefaasd ./cmd/litefaasd
```

`go build ./...` from the repo root must succeed. Override the reported version at link time if you want:

```bash
go build -ldflags "-X github.com/wolvever/litefaas/internal/version.Version=v0.1.0-alpha" -o lf ./cmd/lf
```

## End-to-end: Go function on one Docker host

This is the Phase 2 demo. You need the Go toolchain, a local Docker daemon, and two terminals.

**Terminal 1 — start the daemon**

```bash
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data
```

**Terminal 2 — init, build, deploy, invoke**

```bash
./lf init hello --runtime go
cd hello
../lf build
../lf deploy --gateway http://127.0.0.1:8080
../lf invoke hello -d '{"name":"litefaas"}'
```

Expected invoke body (pretty-printed here):

```json
{"message":"hello from litefaas","function":"hello"}
```

The same invoke is `POST /v1/invoke/hello`:

```bash
curl -s -X POST http://127.0.0.1:8080/v1/invoke/hello \
  -H 'Content-Type: application/json' \
  -d '{"name":"litefaas"}'
```

What each step does:

| Step | Where it runs | What happens |
|------|----------------|--------------|
| `lf init` | local files | Copies `templates/runtimes/go/http/` (handler, Dockerfile, `litefaas.yaml`) |
| `lf build` | local Docker | `docker build -t <image> .` (default image `hello:latest`; no registry) |
| `lf deploy` | API + Docker | `POST /v1/functions` (or `PUT` if it exists) then `POST /v1/functions/{name}/deploy` — create/replace container `litefaas-<name>`, `PORT`, memory, env; publish `127.0.0.1:<ephemeral>→$PORT` |
| `lf invoke` | API | `POST /v1/invoke/{name}` reverse-proxies to the container |

Re-run `lf build && lf deploy` after editing `handler.go`. `lf delete hello` removes the resource and stops the container.

## Health and version

```bash
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data
# in another shell
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/version
./lf version
./lf health --gateway http://127.0.0.1:8080
```

`GET /healthz` returns `{"status":"ok","version":"..."}`. There is no UI; the CLI (`lf`) and HTTP API are the interface.

## Resource API (RFC-0001 §9 subset)

Base path `/v1`. State is sqlite under `--data-dir` (file `litefaas.db`; default `~/.litefaas`).

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Daemon health |
| GET | `/version` | Build identity |
| POST | `/v1/functions` | Register resource metadata |
| PUT | `/v1/functions/{name}` | Update resource metadata |
| GET | `/v1/functions` | List |
| GET | `/v1/functions/{name}` | Get (includes revisions) |
| DELETE | `/v1/functions/{name}` | Delete resource and stop its container |
| POST | `/v1/functions/{name}/deploy` | Deploy/replace the Docker container |
| POST | `/v1/invoke/{name}` | Sync invoke (kind=function) |

Auth: if `LITEFAAS_TOKEN` or `--token` is set, send `Authorization: Bearer <token>`. `/healthz` and `/version` stay open.

```bash
curl -s -X POST http://127.0.0.1:8080/v1/functions \
  -H 'Content-Type: application/json' \
  -d '{"name":"orders-api","kind":"backend","runtime":"java","image":"localhost:5000/orders-api:0.1.0"}'
./lf list --gateway http://127.0.0.1:8080
./lf delete orders-api --gateway http://127.0.0.1:8080
```

## CLI context

```bash
lf context create local --gateway http://127.0.0.1:8080
lf context list
lf context use local
lf list
lf delete orders-api
```

Context file: `~/.litefaas/config.yaml` (override with `--config-dir`).

## Tests

Unit and integration tests do **not** require Docker. They mock the docker CLI and, for invoke, use `httptest` as the function:

```bash
go test ./...
go build ./...
```

Docker-required smoke (same as the demo above): `litefaasd` running, then `lf init` → `lf build` → `lf deploy` → `lf invoke` on a host with a working Docker daemon. `lf build` / `lf deploy` fail with a clear error if `docker` is missing.

## Manifest (`litefaas.yaml`)

Parsed fields for a Go function (RFC-0001 §7): `name`, `kind`, `runtime`, `handler`, `image`, `port`, `memory` (MiB), `timeout`, `health`, `env`.

## Goals (v0.1)

See GitHub milestone [v0.1.0-alpha](https://github.com/wolvever/litefaas/milestone/1). Phases 0–4 are the first public alpha.

## Inspiration

Fn Project, OpenFaaS/faasd — same verbs and template idea, smaller surface.

## License

[MIT](LICENSE)
