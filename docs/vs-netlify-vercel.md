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
- Preflight before up/deploy (`lf check`, `lf up --check`) — optional pack `host.build`, image build, one-shot container smoke (tear down); fails closed before litefaasd
- Deploy summary with edge URLs (no secret values)
- Install UX (`install.sh` + release binaries) and `make demo`
- Local rebuild loop (`lf watch`) — debounce → Docker build → API deploy cutover (Netlify Dev-feel; **not** framework HMR inside the container, and **not** `netlify watch` waiting on remote CDN deploys)
- `_redirects` + thin `netlify.toml` `[[redirects]]` / `[[headers]]` → gateway edge rules (data, not Netlify SaaS; force/`Role`/`Query` skipped in MVP)

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
