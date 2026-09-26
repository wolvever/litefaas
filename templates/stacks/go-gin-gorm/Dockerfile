# go-gin-gorm stack pack (RFC-0001 §6 / Phase 7):
# Static Go binary; listen on 0.0.0.0:$PORT.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
RUN adduser -D -u 10001 app
COPY --from=build /out/app /app/app
ENV PORT=8080
EXPOSE 8080
USER 10001
CMD ["/app/app"]
