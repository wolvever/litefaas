# Generic Python HTTP function (RFC-0001 §6): bind 0.0.0.0:$PORT, logs to stdout.
FROM python:3.12-alpine
WORKDIR /app
COPY handler.py .
ENV PORT=8080
EXPOSE 8080
CMD ["python", "handler.py"]
