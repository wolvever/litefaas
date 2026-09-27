# Using pglite-sidecar with litefaas

[pglite-sidecar](https://github.com/wolvever/pglite-sidecar) is an optional local Postgres wire-protocol sidecar. litefaas does **not** start it — you run it yourself and point apps at it via `DATABASE_URL`.

## Typical flow

```bash
# terminal A — sidecar (see upstream README); listen on host :5432

# terminal B — control plane
./litefaasd --addr 127.0.0.1:8080 --data-dir ./data --no-auth

./lf secret set db --value 'postgres://app:app@host.docker.internal:5432/app'

# in the app litefaas.yaml:
# env:
#   DATABASE_URL: ${secret:db}

./lf build examples/stacks/catalog
./lf deploy examples/stacks/catalog --gateway http://127.0.0.1:8080
```

## Why `host.docker.internal`?

litefaas publishes containers on `127.0.0.1:<ephemeral>→$PORT`. From **inside** the container, `localhost` is the container itself. The runner adds `--add-host=host.docker.internal:host-gateway` so Linux Docker can reach Postgres (or pglite-sidecar) on the host.

Pack `stack.yml` hints still show `localhost` for humans running processes on the host; containerized apps should use `host.docker.internal`.
