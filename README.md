# litefaas

Poor man's serverless: a minimal **CLI + API** control plane to build and deploy **functions**, **backends** (Java / Go / Python), and a **lightweight frontend**, self-hosted on a single node.

> Status: Phases 0–1 implemented (skeleton + sqlite store + resource CRUD). Builder, Docker runner, and `litefaas.yaml` come in later phases.

## Docs

- **[RFC-0001: Architecture](docs/RFC-0001-architecture.md)** — design, manifests, API, phases

## Build

Requires Go 1.22+.

```bash
go build -o bin/litefaasd ./cmd/litefaasd
go build -o bin/lf ./cmd/lf
```

`go build ./...` also succeeds (no output binaries).

## Run

Start the daemon. State lives under `--data-dir` (default `~/.litefaas`) as `litefaas.db` and survives restarts.

```bash
export LITEFAAS_TOKEN=devtoken
./bin/litefaasd --listen 127.0.0.1:8080 --data-dir ~/.litefaas
```

In another terminal:

```bash
export LITEFAAS_TOKEN=devtoken
./bin/lf context set --gateway http://127.0.0.1:8080
./bin/lf health
./bin/lf version
./bin/lf create --name hello --kind function --runtime go --image hello:dev
./bin/lf list
./bin/lf delete hello
```

`lf` also honors `--gateway`, `--token`, `LITEFAAS_GATEWAY`, and `LITEFAAS_TOKEN`.

## HTTP API (Phase 1)

RFC §9 subset. `GET /healthz` and `GET /version` are unauthenticated. Everything under `/v1` requires `Authorization: Bearer $LITEFAAS_TOKEN` when a token is configured.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Health + version |
| GET | `/version` | Daemon version |
| POST | `/v1/functions` | Register resource metadata |
| GET | `/v1/functions` | List |
| GET | `/v1/functions/{name}` | Get (includes revisions) |
| DELETE | `/v1/functions/{name}` | Delete |
| POST | `/v1/functions/{name}/deploy` | Record deploy metadata (no runner yet) |

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s -H "Authorization: Bearer $LITEFAAS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"hello","kind":"function","runtime":"go"}' \
  http://127.0.0.1:8080/v1/functions
```

## Layout

```
cmd/litefaasd    daemon
cmd/lf           CLI
internal/api     HTTP handlers
internal/store   sqlite resources + revisions
internal/client  CLI HTTP client
```

## Goals (v0.1)

See GitHub milestones/issues. Phases 0–4 are the first public alpha.

## Inspiration

Fn Project, OpenFaaS/faasd — same verbs and template idea, smaller surface.

## License

MIT
