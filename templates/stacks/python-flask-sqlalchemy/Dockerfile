# python-flask-sqlalchemy stack pack (RFC-0001 §6 / Phase 7):
# Install deps from requirements.txt or pyproject.toml; gunicorn (preferred) or flask run on 0.0.0.0:$PORT.
FROM python:3.12-slim
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PORT=8080 \
    FLASK_APP=app:app \
    GUNICORN_APP=app:app
COPY . .
RUN set -eux; \
    if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; \
    elif [ -f pyproject.toml ]; then pip install --no-cache-dir .; \
    else echo "missing requirements.txt or pyproject.toml" >&2; exit 1; fi; \
    useradd --system --uid 10001 app
USER 10001
EXPOSE 8080
# Prefer gunicorn when installed (list it in requirements); otherwise flask run.
CMD ["sh", "-c", "if python -c 'import gunicorn' 2>/dev/null; then exec gunicorn --bind 0.0.0.0:${PORT} ${GUNICORN_APP}; else exec flask --app ${FLASK_APP} run --host 0.0.0.0 --port ${PORT}; fi"]
