# node-nestjs-prisma stack pack (RFC-0001 §6 / Phase 7):
# npm ci/install; prisma generate; build; start:prod on 0.0.0.0:$PORT.
FROM node:22-slim
WORKDIR /app
ENV NODE_ENV=production \
    PORT=8080
RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home app
COPY package.json package-lock.json* npm-shrinkwrap.json* yarn.lock* pnpm-lock.yaml* ./
RUN set -eux; \
    if [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; then npm ci; \
    else npm install; fi
COPY . .
RUN set -eux; \
    if [ -f prisma/schema.prisma ]; then npx prisma generate; fi; \
    if npm run 2>/dev/null | grep -q ' build'; then npm run build; fi; \
    npm prune --omit=dev; \
    chown -R app:app /app
USER 10001
EXPOSE 8080
CMD ["sh", "-c", "if npm run 2>/dev/null | grep -q ' start:prod'; then exec npm run start:prod; elif [ -f dist/main.js ]; then exec node dist/main.js; else exec npm start; fi"]
