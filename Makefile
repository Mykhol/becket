PREFIX  ?= $(HOME)/.local
# Stamp the version from git (strip the leading v); override with `make install VERSION=…`.
VERSION ?= $(shell git deservice --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo dev)
# The version the goldens encode (the bash reference), used for the test build so
# bash and Go are exercised at the same version.
BECKET_VERSION := $(shell grep '^VERSION=' bin/becket | cut -d'"' -f2)

# Build the Go binary (becket-go) with the version baked in via ldflags.
build:
	@go build -ldflags "-X main.version=$(VERSION)" -o becket-go .

install: build
	@mkdir -p $(PREFIX)/bin
	@mkdir -p $(PREFIX)/share/zsh/site-functions
	@mkdir -p $(PREFIX)/share/bash-completion/completions
	@cp becket-go $(PREFIX)/bin/becket
	@chmod +x $(PREFIX)/bin/becket
	@cp completions/_becket $(PREFIX)/share/zsh/site-functions/_becket
	@cp completions/becket.bash $(PREFIX)/share/bash-completion/completions/becket
	@echo "Installed becket $(VERSION) to $(PREFIX)/bin/becket"
	@echo ""
	@echo "To enable completions:"
	@echo "  zsh:  Add $(PREFIX)/share/zsh/site-functions to your fpath (before compinit)"
	@echo "  bash: source $(PREFIX)/share/bash-completion/completions/becket"

uninstall:
	@rm -f $(PREFIX)/bin/becket
	@rm -rf $(PREFIX)/share/becket
	@rm -f $(PREFIX)/share/zsh/site-functions/_becket
	@rm -f $(PREFIX)/share/bash-completion/completions/becket
	@echo "Removed becket from $(PREFIX)/bin/becket"

# Run the characterization suite against the Go binary (the source of truth),
# built at the goldens' version so the `becket version` scenario matches.
test test-go:
	@go build -ldflags "-X main.version=$(BECKET_VERSION)" -o becket-go .
	@BECKET_BIN=$(PWD)/becket-go ./tests/run.sh

.PHONY: build install uninstall test test-go
