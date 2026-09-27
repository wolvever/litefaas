# RFC-0001: litefaas — Poor Man's Serverless Control Plane

- **Status:** Accepted for implementation
- **Created:** 2026-09-26
- **Authors:** Cheng Ju / Lightweight Lib
- **Target:** Self-hosted, CLI + API driven, Go control plane

## 1. Motivation

Existing FaaS/self-host stacks (Fn, OpenFaaS/faasd, Coolify) share a useful shape — manifest, language templates, containers, CLI talking to an HTTP API — but are either heavier than needed, license-constrained, UI-first, or weak on polyglot **frameworks** plus a light **frontend** path.

We want a minimal appliance that:

1. Is **CLI and API first** (no UI required for v0.1).
2. Builds and deploys **functions** and long-running **backends**.
3. Supports **Java, Go, and Python** with broad framework coverage.
4. Adds **lightweight frontend** hosting (static-first).
5. Stays **self-hosted** and **very lightweight** (single node, Go binaries).
6. Complements (does not replace) tools like Coolify and pglite-sidecar.

## 2. Goals

| ID | Goal |
|----|------|
| G1 | One Go CLI (`litefaas` / `lf`) and one Go daemon (`litefaasd`) sharing one HTTP API |
| G2 | Deploy `kind: function`, `kind: backend`, and `kind: frontend` |
| G3 | Runtimes `go`, `java`, `python`, plus `dockerfile` and `static` escape hatches |
| G4 | Framework support via **HTTP port contract** + optional **presets**, not per-framework runners |
| G5 | Single-node runner on Docker (v0.1); containerd optional later |
| G6 | Path-based HTTP routing for frontend + APIs + functions |
| G7 | sqlite (or bbolt) for control-plane state; no external DB required |

## 3. Non-goals (v0.1)

- Multi-node clustering / HA
- Kubernetes
- Full local hot-reload for every framework
- Embedding every framework's CLI
- Multi-tenant hard isolation (beyond container boundaries)
- Managed cloud offering
- Replacing Coolify for general Compose apps

## 4. Design principles

1. **Contract over frameworks.** Anything that listens on `$PORT` and logs to stdout is a first-class citizen.
2. **Templates are copy-on-init.** Presets land in the user repo; the daemon never special-cases Spring vs Gin at invoke time.
3. **CLI ↔ API parity.** Every CLI verb is a thin client over the same REST API CI can call.
4. **Static frontend by default.** Node SSR is just another backend/Dockerfile.
5. **Steal verbs, not surface area.** init / build / deploy / invoke / logs — Fn/OpenFaaS muscle memory without their full ecosystems.

## 5. Architecture

```
┌─────────────┐     HTTPS/HTTP      ┌──────────────────┐
│  lf CLI /   │ ──────────────────► │    litefaasd     │
│  CI / curl  │                     │  API + store     │
└─────────────┘                     │  builder, runner │
                                    │  proxy manager   │
                                    └────────┬─────────┘
                                             │ Docker API
                                    ┌────────▼─────────┐
                                    │  containers      │
                                    │  + edge proxy    │
                                    │  (Caddy/Traefik  │
                                    │   or tiny Go)    │
                                    └──────────────────┘
```

### 5.1 Components

| Component | Responsibility |
|-----------|----------------|
| `lf` CLI | `init`, `build`, `deploy`, `invoke`, `logs`, `route`, context/config |
| `litefaasd` | REST API, job queue, image build orchestration, container lifecycle, proxy config |
| Store | Apps/functions/backends/frontends, revisions, secrets refs, routes |
| Builder | Resolve runtime/preset/Dockerfile → `docker build` |
| Runner | Create/replace/stop containers; resource limits; health checks |
| Proxy | Path/host routing to containers; TLS later |

### 5.2 Process model

- **function:** invoke-oriented; timeout enforced; scale-to-zero allowed (stop when idle).
- **backend:** min replicas ≥ 1; no per-request timeout from the platform (client timeouts only).
- **frontend:** static file server container (or prebuilt nginx image); SPA fallback optional.

## 6. HTTP contracts

All `go` / `java` / `python` / `dockerfile` workloads MUST:

1. Bind `0.0.0.0:$PORT` (default `8080`).
2. Emit logs to stdout/stderr.
3. Prefer `GET /healthz` → `200` (recommended; configurable).
4. Read configuration from environment variables.

The platform does not import application frameworks. Framework choice only affects scaffold and Dockerfile.

## 7. Manifest

File name: `litefaas.yaml` (per service) or `stack.yaml` (multi-service). v0.1 implements per-directory `litefaas.yaml`. Phase 7 makes the per-service file **optional**: `lf build` / `lf deploy` on a directory with no manifest fingerprint the tree, pick a stack pack, and synthesize name/kind/runtime/image.

```yaml
name: orders-api
kind: backend                 # function | backend | frontend
runtime: java                 # go | java | python | dockerfile | static
preset: spring-boot           # optional
stack: java-spring-mybatis    # optional Phase 7 pack id; --stack wins over this
handler: ./orders-api         # source or static output dir
image: localhost:5000/orders-api:0.1.0
port: 8080
memory: 512                   # MiB
timeout: 60s                  # functions only
health: /healthz
triggers:
  - type: http
    path: /api/orders
    strip_prefix: true
env:
  DATABASE_URL: ${secret:db}
build:
  # optional; frontend Node build or custom
  command: []
  output: ""
```

Frontend example:

```yaml
name: web
kind: frontend
runtime: static
handler: ./web/dist
image: localhost:5000/web:0.1.0
triggers:
  - type: http
    path: /
    spa: true
```

## 8. Template layout

```
templates/
  runtimes/
    go/http/
    java/http/
    python/http/
  presets/
    go/gin/
    go/fiber/
    java/spring-boot/
    java/quarkus/
    python/fastapi/
    python/flask/
  frontend/
    static/
  meta/
    dockerfile/
  stacks/                     # Phase 7 fingerprint packs (data, not Go)
    java-spring-mybatis/
    java-spring-jpa/
    python-fastapi/
    python-flask-sqlalchemy/
    go-gin-gorm/
    node-express-prisma/
    node-nestjs-prisma/
    node-nextjs/
    python-django/
```

Each runtime/preset template provides: `template.yml`, `Dockerfile` (and optionally build stage), and a minimal handler stub. Each stack pack provides `stack.yml` (fingerprints + defaults + sidecar/env hints) and a best-practice `Dockerfile`.

**Framework coverage strategy**

- Runtimes = universal HTTP.
- Ship 1–2 presets per language in v0.1.
- `runtime: dockerfile` for everything else (Django, Micronaut, Next SSR, …).
- Community presets later as separate git pulls (OpenFaaS-style), not core bloat.

### 8.1 Stack packs (Phase 7)

A **stack** is language + web framework + ORM/SQL (+ optional KV/Redis hints). Packs are YAML + Dockerfile under `templates/stacks/` (embedded in `lf`, overridable via `LITEFAAS_STACKS_DIR`). `litefaasd` does not import packs or frameworks.

Detection is a small AND/OR fingerprint matcher (`files` must exist; `contains.any` is a substring check). No rules engine in the daemon.

Precedence for `lf build` / `lf deploy`:

1. `--stack <id>`
2. `stack:` in `litefaas.yaml` (one-liner is enough)
3. Explicit `runtime` / `preset` / existing `Dockerfile` in `litefaas.yaml`
4. Fingerprint detection
5. Error (suggest `--stack` or a manifest)

Existing `stack.yaml` (multi-service) stays explicit. Sidecar/env hints are documented soft defaults — the platform does not start Postgres or Redis.

## 9. API (v0.1 sketch)

Base: `/v1`

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Daemon health |
| POST | `/functions` | Register resource metadata |
| GET | `/functions` | List |
| GET | `/functions/{name}` | Get |
| DELETE | `/functions/{name}` | Delete |
| POST | `/functions/{name}/deploy` | Deploy image revision |
| POST | `/invoke/{name}` | Sync invoke (functions) |
| GET | `/functions/{name}/logs` | Log tail |
| PUT | `/routes` | Replace route table (or derived from manifests) |
| POST | `/secrets/{name}` | Store secret (local encrypted or file-backed) |

CLI maps 1:1. Auth for v0.1: shared bearer token in env (`LITEFAAS_TOKEN`).

## 10. CLI verbs

```text
lf init <name> --runtime go|java|python|dockerfile|static [--preset ...] [--kind ...]
lf build [path] [--stack ID]
lf deploy [path] [--stack ID] [--gateway http://127.0.0.1:8080]
lf invoke <name> [-d payload]
lf logs <name>
lf list
lf delete <name>
lf up                  # start local litefaasd if needed (optional)
```

## 11. Build & deploy flow

1. Read `litefaas.yaml` / `stack.yaml`, or detect a stack pack (Phase 7).
2. Materialize build context from stack-pack Dockerfile, runtime/preset, or user Dockerfile.
3. Optional `build.command` (e.g. `npm ci && npm run build`) in a build container.
4. `docker build` → tag `image`.
5. Push to configured registry **or** `docker load` on single-node local mode.
6. API deploy: stop old container (if any), start new with env/secrets/limits, attach to proxy network.
7. Update routes from triggers.

## 12. Frontend design

- **Default:** `kind: frontend` + `runtime: static` → nginx:alpine (or Caddy) serving files; `spa: true` rewrites to `index.html`.
- **Build:** prefer CI or developer `npm run build` producing `handler` dir; optional in-daemon Node build stage later.
- **SSR:** use `kind: backend` + `runtime: dockerfile` (or a future `node` preset). Not required for v0.1 lightness.

Edge routing example:

```text
/           → web (frontend)
/api/*      → java/go/python backends
/fn/*       → functions
```

## 13. Storage & secrets

- Control plane DB: sqlite file under `~/.litefaas/` or `--data-dir`.
- Secrets: v0.1 store encrypted-at-rest with a host key file, inject as env at deploy. No external vault required.
- App data volumes: optional named Docker volumes declared later; out of scope for first functions demo.

## 14. Relation to other work

| Project | Relation |
|---------|----------|
| [pglite-sidecar](https://github.com/wolvever/pglite-sidecar) | Optional local Postgres sidecar; deploy as `kind: backend` or external `DATABASE_URL` |
| Coolify | Heavier Git/Compose PaaS; litefaas can run *under* Coolify later or stay standalone on a VPS |
| Fn / OpenFaaS | Inspiration for verbs, templates, manifests — not a fork |

## 15. Implementation phases (goals)

### Phase 0 — Skeleton
- Repo layout, `go.mod`, `cmd/lf`, `cmd/litefaasd`, README pointing at this RFC
- Health endpoint + version

### Phase 1 — Core API + store
- sqlite store for resources/revisions
- CRUD + deploy metadata (stub runner OK)
- CLI: `list`, `delete`, config/context

### Phase 2 — Builder + Go runtime
- `lf init --runtime go`
- `lf build` / `lf deploy` with Docker
- Sync `lf invoke` for functions

### Phase 3 — Java + Python runtimes
- Generic HTTP templates + Dockerfiles
- One preset each (Spring Boot or Quarkus; FastAPI)

### Phase 4 — Frontend static
- `kind: frontend`, `runtime: static`, SPA routes
- Demo: static web + one API backend

### Phase 5 — Backends + routing polish
- `kind: backend` (no scale-to-zero)
- Path routing table, strip_prefix
- Dockerfile escape hatch

### Phase 6 — Hardening
- Tokens, basic metrics, log streaming, timeouts/memory enforcement
- `stack.yaml` multi-service (optional)

### Phase 7 — Zero-config stack detection (post-v0.1)
- Data-only stack packs: fingerprints + Dockerfile recipes
- First-party: `java-spring-mybatis`, `java-spring-jpa`, `python-fastapi`, `python-flask-sqlalchemy`, `python-django`, `go-gin-gorm`, `node-express-prisma`, `node-nestjs-prisma`, `node-nextjs`
- `lf build` / `lf deploy` work with little or no `litefaas.yaml`; `--stack` / `stack:` override
- Control plane stays HTTP `$PORT` + Docker; adding a pack does not grow `litefaasd`
- `runtime: node` is accepted for stack packs (no `lf init` template yet; use a pack or `dockerfile`)

## 16. Success criteria (v0.1 exit)

1. On a single Linux host with Docker: install two Go binaries (or one with subcommands).
2. `lf init` + `build` + `deploy` works for Go, Java, and Python HTTP samples.
3. A static frontend is reachable on `/` and an API on `/api/*`.
4. A function can be invoked via CLI and HTTP.
5. README alone is enough to reproduce the demo without a UI.
6. Idle control plane is small (target: tens of MB RSS for `litefaasd` excluding workloads).

## 17. Open questions

1. Proxy: embed Caddy/Traefik vs write a tiny Go reverse proxy for v0.1?
2. Local registry vs Docker-only local image names on one node?
3. Rename binary (`lf` vs `litefaas`) before first public release?
4. Scale-to-zero strategy for functions (idle TTL)?

**Proposal for v0.1:** tiny Go reverse proxy; local Docker image names without registry; CLI name `lf`; function idle TTL default 5m (configurable).

## 18. Decision

Proceed to implement Phases 0–4 as the first public milestone (`v0.1.0-alpha`), tracked as GitHub issues, executed by an implementation loop (cloud agent / project) against this RFC.
