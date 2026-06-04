PREFIX  ?= $(HOME)/.local
# Version stamped from git (strip the leading v); override with `make install VERSION=…`.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo dev)

# Build the Go binary (becket-go) with the version baked in via ldflags.
build:
	@go build -ldflags "-X main.version=$(VERSION)" -o becket-go .

install: build
	@mkdir -p $(PREFIX)/bin
	@cp becket-go $(PREFIX)/bin/becket
	@chmod +x $(PREFIX)/bin/becket
	@echo "Installed becket $(VERSION) to $(PREFIX)/bin/becket"
	@echo ""
	@echo "Shell integration (cd via 'becket shell' + completions):"
	@echo "  add to your shell rc:  eval \"\$$(becket shell-init)\""

uninstall:
	@rm -f $(PREFIX)/bin/becket
	@rm -rf $(PREFIX)/share/becket
	@rm -f $(PREFIX)/share/zsh/site-functions/_becket
	@rm -f $(PREFIX)/share/bash-completion/completions/becket
	@echo "Removed becket from $(PREFIX)/bin/becket"

# Characterization regression suite against the Go binary (the source of truth).
test test-go: build
	@BECKET_BIN=$(PWD)/becket-go ./tests/run.sh

.PHONY: build install uninstall test test-go
