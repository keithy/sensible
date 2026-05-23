.PHONY: all build clean install test lint fmt vet check

# Binary names
BINARIES = sensible sensible-do sensible-consume sensible-list sensible-status sensible-server sensible-client

all: build

build:
	mkdir -p build
	go build -o build/sensible ./cmd/sensible
	go build -o build/sensible-do ./cmd/sensible-do
	go build -o build/sensible-consume ./cmd/sensible-consume
	go build -o build/sensible-list ./cmd/sensible-list
	go build -o build/sensible-status ./cmd/sensible-status
	go build -o build/sensible-server ./cmd/sensible-server
	go build -o build/sensible-client ./cmd/sensible-client
	chmod +x build/*

clean:
	rm -rf build

install: build
	mkdir -p ~/.local/bin
	cp build/* ~/.local/bin/

test:
	go test ./...
	bash tests/config_spec.sh

vet:
	go vet ./...

fmt:
	go fmt ./...

check: fmt vet test

# Individual binary targets
build/sensible: cmd/sensible/main.go
	mkdir -p build
	go build -o $@ ./cmd/sensible

build/sensible-do: cmd/sensible-do/main.go
	mkdir -p build
	go build -o $@ ./cmd/sensible-do

build/sensible-consume: cmd/sensible-consume/main.go
	mkdir -p build
	go build -o $@ ./cmd/sensible-consume

build/sensible-list: cmd/sensible-list/main.go
	mkdir -p build
	go build -o $@ ./cmd/sensible-list

build/sensible-status: cmd/sensible-status/main.go
	mkdir -p build
	go build -o $@ ./cmd/sensible-status

build/sensible-server: cmd/sensible-server
	mkdir -p build
	go build -o $@ ./cmd/sensible-server

build/sensible-client: cmd/sensible-client
	mkdir -p build
	go build -o $@ ./cmd/sensible-client