# litefaas

Poor man's serverless: a minimal **CLI + API** control plane to build and deploy **functions**, **backends** (Java / Go / Python), and a **lightweight frontend**, self-hosted on a single node.

> Status: RFC accepted — Phase 0 skeleton and Phase 1 store/API are in tree. Docker workloads start in Phase 2.

## Docs

- **[RFC-0001: Architecture](docs/RFC-0001-architecture.md)** — design, manifests (`litefaas.yaml`), API, phases
- Kinds: `function` | `backend` | `frontend`
- Runtimes: `go` | `java` | `python` | `dockerfile` | `static`

## Build

Requires [Go 1.22+](https://go.dev/dl/) on Linux (or any `GOOS` for the control plane; Docker workloads come in later phases).

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
| GET | `/v1/functions` | List |
| GET | `/v1/functions/{name}` | Get (includes revisions) |
| DELETE | `/v1/functions/{name}` | Delete |
| POST | `/v1/functions/{name}/deploy` | Record a deploy revision (runner is a stub until Phase 2) |

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
