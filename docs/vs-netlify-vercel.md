# litefaas vs Netlify / Vercel

## What litefaas is

A **CLI + HTTP API** control plane for **functions**, **backends**, and a **lightweight frontend** on **one Docker host**. Stack packs are **data** (YAML + Dockerfiles under `templates/stacks/`). The daemon (`litefaasd`) stays dumb: no framework imports, no managed databases, no ACME, no UI.

## What Netlify / Vercel optimize for

Global frontend DX: CDN edge, git-OAuth previews, team billing, managed platform SaaS. Excellent for static/JAMstack and serverless at the edge — not a single-node self-hosted control plane.

## When to choose which

| Need | Prefer |
|------|--------|
| Self-host on one Docker box; own the API | **litefaas** |
| Global CDN + preview deploys from git | **Netlify / Vercel** |
| Brownfield backend fingerprints (Gin, Spring, Nest, …) | **litefaas** packs |
| Team UI, billing, analytics SaaS | **Netlify / Vercel** |
| Encrypt-at-rest secrets + `${secret:name}` env | **litefaas** |
| Zero-ops multi-region frontend | **Netlify / Vercel** |

## What we steal (product DX, not architecture)

- Local `lf up` (bring the control plane up in one terminal)
- Detect / plan before build (`lf detect`, `lf build --plan`)
- Preflight before up/deploy (`lf check`, `lf up --check`) — optional pack `host.build`, image build, one-shot container smoke (tear down); fails closed before litefaasd. Host recipes are **pack-declared** data; use `lf check --skip-host` when the toolchain is not installed locally.
- Deploy summary with edge URLs (no secret values)
- Install UX (`install.sh` + release binaries) and `make demo`
- Local rebuild loop (`lf watch`) — debounce → Docker build → API deploy cutover (Netlify Dev-feel; **not** framework HMR inside the container, and **not** `netlify watch` waiting on remote CDN deploys)
- `_redirects` + thin `netlify.toml` `[[redirects]]` / `[[headers]]` + `vercel.json` redirects/rewrites/headers/`routes` → gateway edge rules (data, not SaaS; force/`Role`/`Query`/middleware skipped)
- Path placeholders: trailing `/*` + `:splat`, `:param` segments, mixed `/shop/:cat/*`, cheap `(.*)`/`$1` → splat; optional Vercel `cleanUrls` → single-segment `/:page.html` → `/:page`

## Explicit non-goals (core)

Matching [GOAL #42](https://github.com/wolvever/litefaas/issues/42) and the RFC:

- CDN / global edge network
- Team UI / billing / pack-store SaaS
- Managed Postgres/Redis **inside** the core (hints and sidecars are docs only)
- ACME / auto-cert **inside** litefaasd (put **Caddy** / Traefik / nginx in front)
- Kubernetes control plane
- Framework HMR / live-reload inside the running container
- `netlify watch`-style waiting on remote git/CDN deploys

Put TLS in front:

```caddyfile
example.com {
    reverse_proxy 127.0.0.1:8080
}
```


## Edge rules subset (honest)

| Supported | Notes |
|-----------|--------|
| `_redirects` from/to/status | force/`Role`/`Query` ignored |
| `netlify.toml` `[[redirects]]` / `[[headers]]` | build/plugins/functions ignored |
| `vercel.json` redirects / rewrites / headers | builds/functions/crons/images ignored |
| `vercel.json` `routes[]` | `src`/`dest`/`status`/`headers` only; no `middleware`/`handle`/`has` |
| Placeholders | `/*`+`:splat`, `:param`, mixed `:param`+`/*`, `(.*)`→`/*`, `$1`→`:splat` |
| `cleanUrls: true` | single-segment `/:page.html` → `/:page` only; nested `.html` needs explicit rules |

### Still non-goals (edge / platform)

- CDN / global edge network / ISR / image optimization
- Vercel / Netlify **middleware** SaaS and Edge Middleware
- Full PCRE route engine beyond cheap `(.*)` / `:param` / `/*`
- Host-based draft aliases / multi-tenant preview hostnames
- Per-project edge-rule merge provenance (deploy still global PUT)
