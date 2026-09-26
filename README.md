# litefaas

Poor man's serverless: a minimal **CLI + API** control plane to build and deploy **functions**, **backends** (Java / Go / Python), and a **lightweight frontend**, self-hosted on a single node.

> Status: Phases 0–3 — control plane, Go/Java/Python HTTP runtimes, Spring Boot and FastAPI presets.

## Docs

- **[RFC-0001: Architecture](docs/RFC-0001-architecture.md)** — design, manifests (`litefaas.yaml`), API, phases
- Kinds: `function` | `backend` | `frontend`
- Runtimes: `go` | `java` | `python` | `dockerfile` | `static`

## Build

Requires [Go 1.22+](https://go.dev/dl/) on Linux. Docker is required from Phase 2 on (build + run workloads). No Kubernetes.

```bash
git clone https://github.com/wolvever/litefaas.git
cd litefaas
go build -o lf ./cmd/lf
go build -o litefaasd ./cmd/litefaasd
```

`go build ./...` from the repo root must succeed.

## End-to-end: Go function on one Docker host

```bash
# 1. control plane
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data

# 2. scaffold + image (another shell)
./lf init hello --runtime go
./lf build hello

# 3. register + start container, then sync invoke
./lf deploy hello --gateway http://127.0.0.1:8080
./lf invoke hello -d '{"hello":"litefaas"}'
# → {"hello":"litefaas"}
```

`lf init --runtime go` copies `templates/runtimes/go/http` (Dockerfile + handler that binds `0.0.0.0:$PORT` and serves `GET /healthz`). Same flow for Java and Python:

```bash
./lf init hello-java --runtime java
./lf init hello-py --runtime python
# optional presets
./lf init orders --runtime java --preset spring-boot
./lf init api --runtime python --preset fastapi
./lf build hello-py && ./lf deploy hello-py --gateway http://127.0.0.1:8080
./lf invoke hello-py -d '{"hello":"python"}'
```

`lf build` runs `docker build` and tags `litefaas/<name>:latest`. `lf deploy` POSTs metadata then `POST /v1/functions/{name}/deploy`; litefaasd `docker run`s the local image (no registry). `lf invoke` is `POST /invoke/{name}` and forwards the body to the container.

## Health and version

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/version
./lf version
./lf health --gateway http://127.0.0.1:8080
```

`GET /healthz` returns `{"status":"ok","version":"..."}`. There is no UI.

## Resource API (RFC-0001 §9 subset)

Base path `/v1`. State is sqlite under `--data-dir` (file `litefaas.db`; default `~/.litefaas`).

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Daemon health |
| GET | `/version` | Build identity |
| POST | `/v1/functions` | Register or update resource metadata |
| GET | `/v1/functions` | List |
| GET | `/v1/functions/{name}` | Get (includes revisions + instance) |
| DELETE | `/v1/functions/{name}` | Delete and stop container |
| POST | `/v1/functions/{name}/deploy` | Run image via Docker |
| POST | `/invoke/{name}` | Sync invoke (functions) |

Auth: if `LITEFAAS_TOKEN` or `--token` is set, send `Authorization: Bearer <token>`. `/healthz` and `/version` stay open.

## CLI context

```bash
lf context create local --gateway http://127.0.0.1:8080
lf context use local
lf list
```

Context file: `~/.litefaas/config.yaml` (override with `--config-dir`).

## Goals (v0.1)

See GitHub milestone [v0.1.0-alpha](https://github.com/wolvever/litefaas/milestone/1). Phases 0–4 are the first public alpha.

## Inspiration

Fn Project, OpenFaaS/faasd — same verbs and template idea, smaller surface.

## License

[MIT](LICENSE)
