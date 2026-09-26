# Example dockerfile-backend (RFC-0001 Phase 5). Bind 0.0.0.0:$PORT.
FROM python:3.12-alpine
WORKDIR /app
COPY handler.py .
ENV PORT=8080
EXPOSE 8080
CMD ["python", "handler.py"]
