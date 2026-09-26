# Generic Go HTTP function (RFC-0001 §6): bind 0.0.0.0:$PORT, logs to stdout.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fn .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/fn /app/fn
ENV PORT=8080
EXPOSE 8080
USER nobody
CMD ["/app/fn"]
