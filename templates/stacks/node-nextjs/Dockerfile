# node-nextjs stack pack (RFC-0001 §6 / Phase 7):
# npm/pnpm/yarn install; next build; prefer standalone server, else next start on 0.0.0.0:$PORT.
FROM node:22-slim AS deps
WORKDIR /app
COPY package.json package-lock.json* npm-shrinkwrap.json* yarn.lock* pnpm-lock.yaml* ./
RUN set -eux; \
    if [ -f pnpm-lock.yaml ]; then corepack enable && pnpm i --frozen-lockfile; \
    elif [ -f yarn.lock ]; then corepack enable && yarn install --frozen-lockfile; \
    elif [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; then npm ci; \
    else npm install; fi

FROM node:22-slim AS builder
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN mkdir -p public && npm run build

FROM node:22-slim AS runner
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=8080 \
    HOSTNAME=0.0.0.0
RUN useradd --system --uid 10001 --create-home app
# Standalone output (next.config output: 'standalone') is preferred.
COPY --from=builder --chown=app:app /app/public ./public
COPY --from=builder --chown=app:app /app/package.json ./package.json
COPY --from=builder --chown=app:app /app/.next/standalone ./
COPY --from=builder --chown=app:app /app/.next/static ./.next/static
USER 10001
EXPOSE 8080
CMD ["sh", "-c", "if [ -f server.js ]; then exec node server.js; else exec npx next start -H 0.0.0.0 -p ${PORT}; fi"]
