# User-owned Dockerfile (runtime: dockerfile). Bind 0.0.0.0:$PORT and serve GET /healthz.
FROM python:3.12-alpine
WORKDIR /app
COPY handler.py .
ENV PORT=8080
EXPOSE 8080
CMD ["python", "-u", "handler.py"]
