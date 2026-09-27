# Generic Node HTTP function (RFC-0001 §6): bind 0.0.0.0:$PORT, logs to stdout.
FROM node:22-alpine
WORKDIR /app
COPY package.json server.js ./
ENV PORT=8080
EXPOSE 8080
USER node
CMD ["node", "server.js"]
