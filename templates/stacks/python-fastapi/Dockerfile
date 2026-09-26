# python-fastapi stack pack (RFC-0001 §6 / Phase 7):
# Install deps from requirements.txt or pyproject.toml; uvicorn on 0.0.0.0:$PORT.
FROM python:3.12-slim
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PORT=8080 \
    UVICORN_APP=main:app
COPY . .
RUN set -eux; \
    if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; \
    elif [ -f pyproject.toml ]; then pip install --no-cache-dir .; \
    else echo "missing requirements.txt or pyproject.toml" >&2; exit 1; fi; \
    useradd --system --uid 10001 app
USER 10001
EXPOSE 8080
CMD ["sh", "-c", "uvicorn ${UVICORN_APP} --host 0.0.0.0 --port ${PORT}"]
