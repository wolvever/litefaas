# Generic Node HTTP backend (stdlib http): bind 0.0.0.0:$PORT, /healthz.
FROM node:22-slim
WORKDIR /app
ENV NODE_ENV=production PORT=8080
RUN useradd --system --uid 10001 --create-home app
COPY package.json package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci --omit=dev; else npm install --omit=dev; fi
COPY . .
USER 10001
EXPOSE 8080
CMD ["npm", "start"]
