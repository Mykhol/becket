PREFIX  ?= $(HOME)/.local
# Version stamped from git (strip the leading v); override with `make install VERSION=…`.
# Fall back to "dev" if empty (e.g. building outside a git checkout) — the
# pipeline can't use `|| echo dev` because the trailing `sed` always succeeds.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
VERSION := $(or $(strip $(VERSION)),dev)

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

# Go-native test suite: per-package unit tests plus the testscript (.txtar)
# end-to-end scenarios under tests/testscripts/ (driven by main_test.go).
test test-go:
	@go test ./...

.PHONY: build install uninstall test test-go
