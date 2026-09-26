# Dockerfile escape hatch (RFC-0001 §8 / Phase 5).
# Replace this file with any image that binds 0.0.0.0:$PORT, logs to stdout,
# and prefers GET /healthz → 200 (RFC-0001 §6).
FROM python:3.12-alpine
WORKDIR /app
COPY handler.py .
ENV PORT=8080
EXPOSE 8080
CMD ["python", "handler.py"]
