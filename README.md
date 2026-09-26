# litefaas

Poor man's serverless: a minimal **CLI + API** control plane to build and deploy **functions**, **backends** (Java / Go / Python), and a **lightweight frontend**, self-hosted on a single node.

> Status: RFC accepted — Phases 0–6 are in tree. Functions, always-on backends (including `runtime: dockerfile`), a static frontend, a persisted path route table, bearer tokens, log streaming, and optional `stack.yaml` run on one Docker host.

## Docs

- **[RFC-0001: Architecture](docs/RFC-0001-architecture.md)** — design, manifests (`litefaas.yaml` / `stack.yaml`), API, phases
- Kinds: `function` | `backend` | `frontend`
- Runtimes: `go` | `java` | `python` | `dockerfile` | `static`
- Presets: `spring-boot` (Java), `fastapi` (Python)

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
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data --no-auth
```

`--no-auth` keeps the local walkthrough open. Omit it and litefaasd writes a bearer token to `./data/token` (see [Auth tokens](#auth-tokens)).

**Terminal 2 — init, build, deploy, invoke**

```bash
./lf init hello --runtime go
cd hello
../lf build
../lf deploy --gateway http://127.0.0.1:8080
../lf invoke hello -d '{"name":"litefaas"}'
../lf logs hello --tail 50
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
| `lf invoke` | API | `POST /v1/invoke/{name}` reverse-proxies to the container (wakes a stopped function) |
| `lf logs` | API + Docker | `GET /v1/functions/{name}/logs` tails `docker logs` |

Re-run `lf build && lf deploy` after editing `handler.go`. `lf delete hello` removes the resource and stops the container.

## Java and Python (Phase 3)

Same verbs as Go. Generic HTTP templates bind `0.0.0.0:$PORT` and serve `GET /healthz`. One preset per language:

```bash
./lf init hello-java --runtime java
./lf init hello-py --runtime python
./lf init orders --runtime java --preset spring-boot
./lf init api --runtime python --preset fastapi

cd hello-py && ../lf build && ../lf deploy --gateway http://127.0.0.1:8080
../lf invoke hello-py -d '{"name":"litefaas"}'
# {"message":"hello from litefaas","function":"hello-py"}
```

| Runtime / preset | Template |
|------------------|----------|
| `java` | `templates/runtimes/java/http` (`com.sun.net.httpserver`) |
| `java --preset spring-boot` | `templates/presets/java/spring-boot` |
| `python` | `templates/runtimes/python/http` (stdlib `http.server`) |
| `python --preset fastapi` | `templates/presets/python/fastapi` |
| `static` (`--kind frontend`) | `templates/frontend/static` (nginx + SPA `try_files`) |
| `dockerfile` (`--kind backend` default) | `templates/meta/dockerfile` (replace the Dockerfile) |

## Demo: static frontend + API (Phase 4)

On one Docker host, with the binaries built and `litefaasd` listening on `127.0.0.1:8080`:

```bash
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data --no-auth

# other shell — one stack.yaml, or the per-service dirs
./lf build examples && ./lf deploy examples --gateway http://127.0.0.1:8080
# same as: lf build/deploy examples/web, examples/api, examples/orders

curl -s http://127.0.0.1:8080/          # static index.html (SPA fallback)
curl -s http://127.0.0.1:8080/api/      # python backend {"ok":true,"service":"api"}
curl -s http://127.0.0.1:8080/orders/   # dockerfile backend
```

`kind: frontend` + `runtime: static` is nginx serving files (`try_files` → `index.html`). Triggers become the edge route table: `/` → web, `/api` → api (`strip_prefix: true`). Control-plane paths (`/healthz`, `/version`, `/v1/*`) stay on litefaasd.

Or scaffold your own:

```bash
./lf init web --runtime static --kind frontend
./lf init api --runtime python --kind backend
# set api/litefaas.yaml trigger path /api and strip_prefix: true
```

## Backends, dockerfile, and routes (Phase 5)

`kind: backend` is always-on: Docker `--restart unless-stopped`, no platform per-request timeout, no scale-to-zero. `runtime: dockerfile` is the escape hatch for anything else (Django, Next SSR, …) — default kind is `backend`. Triggers become the edge table; `strip_prefix: true` forwards `/orders/x` as `/x` and sets `X-Forwarded-Prefix`.

```bash
./lf init orders --runtime dockerfile
# created with kind=backend, trigger path /orders, strip_prefix: true
cd orders && ../lf build && ../lf deploy --gateway http://127.0.0.1:8080

# or the in-tree example
./lf build examples/orders && ./lf deploy examples/orders --gateway http://127.0.0.1:8080
curl -s http://127.0.0.1:8080/orders/
# {"ok":true,"service":"orders","path":"/"}

./lf routes --gateway http://127.0.0.1:8080
```

`GET /v1/routes` is derived from each resource's HTTP triggers. `PUT /v1/routes` replaces that table (persisted in sqlite); `DELETE /v1/routes` (or `lf routes clear`) drops the override.

```bash
# replace the table (path + name + strip_prefix + spa)
printf '%s\n' '[{"path":"/api","name":"api","strip_prefix":true},{"path":"/","name":"web","spa":true}]' > /tmp/routes.json
./lf routes set /tmp/routes.json --gateway http://127.0.0.1:8080
./lf routes clear --gateway http://127.0.0.1:8080
```

## Auth tokens (Phase 6)

Control-plane routes (`/v1/*`) take a **shared bearer token**. `/healthz`, `/version`, and edge routes stay open.

Resolution on the daemon:

1. `--token` or `LITEFAAS_TOKEN`
2. else `{data-dir}/token` (created on first start, mode `0600`)
3. `--no-auth` disables the check (local demo only)

`lf` sends `Authorization: Bearer <token>`. Resolution: `--token` > `LITEFAAS_TOKEN` > context `token:` > `{config-dir}/token` (default `~/.litefaas/token`). `X-Litefaas-Token` is also accepted.

```bash
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data
export LITEFAAS_TOKEN=$(cat ./data/token)
./lf token --config-dir ./data          # token=… source=file:./data/token
./lf list --gateway http://127.0.0.1:8080 --token "$LITEFAAS_TOKEN"
curl -s -H "Authorization: Bearer $LITEFAAS_TOKEN" http://127.0.0.1:8080/v1/functions
# or store it on a context
./lf context create local --gateway http://127.0.0.1:8080 --token "$LITEFAAS_TOKEN"
```

If daemon and CLI share `~/.litefaas` (the default `--data-dir` / `--config-dir`), `lf` picks up the file automatically.

## Logs and metrics

```bash
./lf logs hello --tail 100
./lf logs hello -f --gateway http://127.0.0.1:8080
curl -s -H "Authorization: Bearer $LITEFAAS_TOKEN" \
  'http://127.0.0.1:8080/v1/functions/hello/logs?tail=100'
./lf metrics --gateway http://127.0.0.1:8080
```

`GET /v1/functions/{name}/logs?follow=1&tail=100` streams `docker logs --timestamps` (plain text). `GET /v1/metrics` is a small JSON snapshot: request/invoke/deploy/error counters, resource count, uptime, `auth`, `idle_ttl`.

## Limits and idle functions

`memory` is MiB (16–8192; default 128). Docker gets `--memory` and `--memory-swap` set to the same value (no extra swap). `timeout` is functions-only, max `5m` (default `30s`), enforced on `POST /v1/invoke/{name}`.

Functions are `--restart no`. After `--idle-ttl` (default `5m`; `0` disables) without invoke/deploy, litefaasd stops the container. The next invoke redeploys it from the stored image. Backends and frontends stay always-on (`--restart unless-stopped`).

## stack.yaml

A directory with `stack.yaml` (and no `litefaas.yaml`) is a multi-service unit. `lf build` / `lf deploy` walk `services`. Handler paths are relative to the stack file. A per-service `litefaas.yaml` in the same directory still wins.

```yaml
services:
  web:
    kind: frontend
    runtime: static
    handler: ./web
    image: web:latest
    triggers:
      - type: http
        path: /
        spa: true
  api:
    kind: backend
    runtime: python
    handler: ./api
    image: api:latest
    triggers:
      - type: http
        path: /api
        strip_prefix: true
```

`services` may also be a list of objects with `name:`. See `examples/stack.yaml`. This is **not** Phase 7 zero-config stack detection.

## Health and version

```bash
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data --no-auth
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
| POST | `/v1/invoke/{name}` | Sync invoke (kind=function; wakes if idle-stopped) |
| GET | `/v1/functions/{name}/logs` | Log tail (`follow`, `tail` query params) |
| GET | `/v1/metrics` | Basic counters |
| GET | `/v1/routes` | Edge routes (override, or derived from triggers) |
| PUT | `/v1/routes` | Replace the persisted route table |
| DELETE | `/v1/routes` | Clear the override; derive from manifests again |

Auth: `Authorization: Bearer <token>` or `X-Litefaas-Token`. `/healthz` and `/version` stay open. See [Auth tokens](#auth-tokens).

```bash
curl -s -X POST http://127.0.0.1:8080/v1/functions \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $LITEFAAS_TOKEN" \
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

Parsed fields for a Go function (RFC-0001 §7): `name`, `kind`, `runtime`, `handler`, `image`, `port`, `memory` (MiB, 16–8192), `timeout` (max 5m), `health`, `env`. Multi-service: `stack.yaml` (see above).

## Goals (v0.1)

See GitHub milestone [v0.1.0-alpha](https://github.com/wolvever/litefaas/milestone/1). Phases 0–6 (v0.1.0-alpha) are in tree. Phase 7 zero-config stack detection is post-v0.1.

## Inspiration

Fn Project, OpenFaaS/faasd — same verbs and template idea, smaller surface.

## License

[MIT](LICENSE)
