PREFIX ?= $(HOME)/.local

install:
	@mkdir -p $(PREFIX)/bin
	@mkdir -p $(PREFIX)/share/becket
	@mkdir -p $(PREFIX)/share/zsh/site-functions
	@mkdir -p $(PREFIX)/share/bash-completion/completions
	@cp bin/becket $(PREFIX)/bin/becket
	@chmod +x $(PREFIX)/bin/becket
	@cp schema/*.schema.json $(PREFIX)/share/becket/
	@cp completions/_becket $(PREFIX)/share/zsh/site-functions/_becket
	@cp completions/becket.bash $(PREFIX)/share/bash-completion/completions/becket
	@echo "Installed becket to $(PREFIX)/bin/becket"
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

.PHONY: install uninstall
