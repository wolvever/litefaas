# Zero-config stack examples (Phase 7)

These directories look like ordinary apps: **no `litefaas.yaml`**. `lf build` / `lf deploy` fingerprint the tree, pick a first-party stack pack, and materialize that pack’s Dockerfile.

```bash
# from repo root (Docker mocked in unit tests; real daemon for a live demo)
./lf build examples/stacks/shop
./lf build examples/stacks/catalog
./lf build examples/stacks/inventory
./lf build examples/stacks/notes
./lf build examples/stacks/tickets
./lf build examples/stacks/library
./lf build examples/stacks/portal
./lf build examples/stacks/blog
./lf build examples/stacks/tasks

# pin if detection is wrong
./lf build examples/stacks/catalog --stack python-fastapi

# or a one-line override in the app dir
# stack: go-gin-gorm
```

| Dir | Detected pack | Language |
|-----|---------------|----------|
| `shop` | `java-spring-mybatis` | Spring Boot + MyBatis (Maven) |
| `catalog` | `python-fastapi` | FastAPI |
| `inventory` | `go-gin-gorm` | Gin + GORM |
| `notes` | `python-flask-sqlalchemy` | Flask + SQLAlchemy |
| `tickets` | `node-express-prisma` | Express + Prisma |
| `library` | `java-spring-jpa` | Spring Boot + JPA |
| `portal` | `node-nextjs` | Next.js SSR |
| `blog` | `python-django` | Django |
| `tasks` | `node-nestjs-prisma` | NestJS + Prisma |

Postgres/Redis are **hints only**. litefaas does not start databases; set the env vars printed after detect (or in each pack’s `stack.yml`).
