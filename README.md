# litefaas

Poor man's serverless: a minimal **CLI + API** control plane to build and deploy **functions**, **backends** (Java / Go / Python), and a **lightweight frontend**, self-hosted on a single node.

> Status: RFC accepted — Phases 0–2 are in tree (skeleton, sqlite API, Go builder + Docker runner). Java, Python, and frontend land in later phases.

## Docs

- **[RFC-0001: Architecture](docs/RFC-0001-architecture.md)** — design, manifests (`litefaas.yaml`), API, phases
- Kinds: `function` | `backend` | `frontend`
- Runtimes: `go` | `java` | `python` | `dockerfile` | `static`

## Build

Requires [Go 1.22+](https://go.dev/dl/). Phase 2 **build / deploy / invoke** also requires [Docker Engine](https://docs.docker.com/engine/install/) on the same Linux host (`docker` CLI on `PATH`, daemon reachable). No image registry is required — local tags such as `hello:latest` are enough.

```bash
git clone https://github.com/wolvever/litefaas.git
cd litefaas
go build -o lf ./cmd/lf
go build -o litefaasd ./cmd/litefaasd
```

`go build ./...` from the repo root must succeed (it does not need Docker). Override the reported version at link time if you want:

```bash
go build -ldflags "-X github.com/wolvever/litefaas/internal/version.Version=v0.1.0-alpha" -o lf ./cmd/lf
```

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

## End-to-end: Go function on one Docker host

This is the Phase 2 demo. Everything stays on a single node; images are local Docker names.

1. Build the two binaries (see above) and start the daemon:

   ```bash
   ./litefaasd --addr 127.0.0.1:8080 --data-dir ./data
   ```

2. In another shell, scaffold a Go HTTP function (listens on `$PORT`, serves `GET /healthz`):

   ```bash
   ./lf init hello --runtime go
   ```

   That writes `hello/litefaas.yaml`, a multi-stage `Dockerfile`, and `handler.go` from `templates/runtimes/go/http`.

3. Build the image, register the function, and run the container:

   ```bash
   ./lf build ./hello
   ./lf deploy ./hello --gateway http://127.0.0.1:8080
   ```

   `lf deploy` `POST`s `/v1/functions` (or `PUT`s if the name already exists) then `POST /v1/functions/hello/deploy`. litefaasd replaces any previous `litefaas-hello` container and publishes it on `127.0.0.1`.

4. Sync invoke (`kind=function` only):

   ```bash
   ./lf invoke hello -d 'hello from litefaas' --gateway http://127.0.0.1:8080
   ```

   The CLI `POST`s `/v1/invoke/hello`; the daemon forwards the body to the container.

`lf init --runtime java|python|dockerfile|static` and framework presets are not implemented yet (Phases 3–4).

## Resource API (RFC-0001 §9 subset)

Base path `/v1`. State is sqlite under `--data-dir` (file `litefaas.db`; default `~/.litefaas`).

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Daemon health |
| GET | `/version` | Build identity |
| POST | `/v1/functions` | Register resource metadata |
| GET | `/v1/functions` | List |
| GET | `/v1/functions/{name}` | Get (includes revisions) |
| PUT | `/v1/functions/{name}` | Replace metadata |
| DELETE | `/v1/functions/{name}` | Delete (stops the container) |
| POST | `/v1/functions/{name}/deploy` | Deploy image and run the Docker container |
| POST | `/v1/invoke/{name}` | Sync HTTP invoke (`kind=function`) |

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

## Goals (v0.1)

See GitHub milestone [v0.1.0-alpha](https://github.com/wolvever/litefaas/milestone/1). Phases 0–4 are the first public alpha.

## Inspiration

Fn Project, OpenFaaS/faasd — same verbs and template idea, smaller surface.

## License

[MIT](LICENSE)
