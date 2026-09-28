.PHONY: demo build test

build:
	go build -o bin/lf ./cmd/lf
	go build -o bin/litefaasd ./cmd/litefaasd

demo: build
	PATH="$(CURDIR)/bin:$$PATH" ./scripts/demo.sh

test:
	go test ./...
