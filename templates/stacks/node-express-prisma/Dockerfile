# node-express-prisma stack pack (RFC-0001 §6 / Phase 7):
# npm ci/install; prisma generate when schema present; npm start on 0.0.0.0:$PORT.
FROM node:22-slim
WORKDIR /app
ENV NODE_ENV=production \
    PORT=8080
RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home app
COPY package.json package-lock.json* npm-shrinkwrap.json* yarn.lock* pnpm-lock.yaml* ./
# Install all deps first so `prisma` (often a devDependency) is available for generate.
RUN set -eux; \
    if [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; then npm ci; \
    else npm install; fi
COPY . .
RUN set -eux; \
    if [ -f prisma/schema.prisma ]; then npx prisma generate; fi; \
    npm prune --omit=dev; \
    chown -R app:app /app
USER 10001
EXPOSE 8080
CMD ["sh", "-c", "npm start"]
