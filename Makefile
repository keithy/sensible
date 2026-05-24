.PHONY: all build clean install test lint fmt vet check install-user install-system

# Binary names - wrapper only goes to bin, rest go to lib
BINARIES = sensible sensible-do sensible-consume sensible-list sensible-status sensible-server sensible-client

# Directories
PREFIX ?= /usr/local
BIN_DIR = $(DESTDIR)$(PREFIX)/bin
LIB_DIR = $(DESTDIR)$(PREFIX)/lib/sensible
CONFIG_DIR = $(DESTDIR)$(PREFIX)/etc/sensible

# User install directories
USER_BIN = $(DESTDIR)$(HOME)/.local/bin
USER_LIB = $(DESTDIR)$(HOME)/.local/lib/sensible
USER_CONFIG = $(DESTDIR)$(HOME)/.config/sensible

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
	go build -o build/sensible-health ./cmd/sensible-health
	chmod +x build/*

clean:
	rm -rf build

# User installation (local)
# Wrapper goes to ~/.local/bin, subcommands to ~/.local/lib/sensible/
install-user: build
	mkdir -p $(USER_BIN)
	mkdir -p $(USER_LIB)
	# Only wrapper to bin
	cp build/sensible $(USER_BIN)/
	chmod +x $(USER_BIN)/sensible
	# Subcommands to lib (wrapper finds them)
	cp build/sensible-* $(USER_LIB)/
	chmod +x $(USER_LIB)/sensible-*
	@echo "Wrapper installed: $(USER_BIN)/sensible"
	@echo "Subcommands: $(USER_LIB)/"
	@echo "Config: $(USER_CONFIG)/config.json"

# System installation (requires sudo)
# Wrapper goes to /usr/local/bin, subcommands to /usr/local/lib/sensible/
install-system: build
	mkdir -p $(BIN_DIR)
	mkdir -p $(LIB_DIR)
	mkdir -p $(CONFIG_DIR)
	# Only wrapper to bin
	cp build/sensible $(BIN_DIR)/
	chmod +x $(BIN_DIR)/sensible
	# Subcommands to lib
	cp build/sensible-* $(LIB_DIR)/
	chmod +x $(LIB_DIR)/sensible-*
	@echo "Wrapper installed: $(BIN_DIR)/sensible"
	@echo "Subcommands: $(LIB_DIR)/"
	@echo "Config: $(CONFIG_DIR)/config.json"

# Alias for backwards compatibility
install: install-user

test:
	go test ./...
	bash tests/config_spec.sh
	bash tests/health_spec.sh

vet:
	go vet ./...

fmt:
	go fmt ./...

check: fmt vet test

uninstall:
	rm -f $(USER_BIN)/sensible
	rm -f $(USER_LIB)/sensible-*

.PHONY: build check clean fmt install install-system install-user lint test vet
