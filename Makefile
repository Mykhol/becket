PREFIX ?= $(HOME)/.local

install:
	@mkdir -p $(PREFIX)/bin
	@cp bin/becket $(PREFIX)/bin/becket
	@chmod +x $(PREFIX)/bin/becket
	@echo "Installed becket to $(PREFIX)/bin/becket"

uninstall:
	@rm -f $(PREFIX)/bin/becket
	@echo "Removed becket from $(PREFIX)/bin/becket"

.PHONY: install uninstall
