# Static frontend (RFC-0001 §12): nginx serving files, SPA fallback.
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY index.html /usr/share/nginx/html/index.html
ENV PORT=8080
EXPOSE 8080
