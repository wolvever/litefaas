# python-django stack pack (RFC-0001 §6 / Phase 7):
# Install deps; gunicorn WSGI on 0.0.0.0:$PORT (fallback manage.py runserver).
FROM python:3.12-slim
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PORT=8080 \
    DJANGO_SETTINGS_MODULE=config.settings \
    GUNICORN_APP=config.wsgi:application
COPY . .
RUN set -eux; \
    if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; \
    elif [ -f pyproject.toml ]; then pip install --no-cache-dir .; \
    else echo "missing requirements.txt or pyproject.toml" >&2; exit 1; fi; \
    useradd --system --uid 10001 app
USER 10001
EXPOSE 8080
CMD ["sh", "-c", "if python -c 'import gunicorn' 2>/dev/null; then exec gunicorn --bind 0.0.0.0:${PORT} ${GUNICORN_APP}; elif [ -f manage.py ]; then exec python manage.py runserver 0.0.0.0:${PORT}; else echo 'need gunicorn or manage.py' >&2; exit 1; fi"]
