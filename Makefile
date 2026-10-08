PLUGIN_ID := io.github.raybarrera.gh-actions-watch
BIN_NAME  := omarchy-gh-actions-watch
PREFIX    ?= $(HOME)/.local
BINDIR    := $(PREFIX)/bin
PLUGIN_DIR:= $(HOME)/.config/omarchy/plugins/$(PLUGIN_ID)
OMARCHY_PATH ?= /usr/share/omarchy

PLUGIN_FILES := manifest.json BarWidget.qml Panel.qml Model.js

.PHONY: all build test install uninstall validate lint clean

all: build test

build:
	go build -o bin/$(BIN_NAME) ./cmd/$(BIN_NAME)

test:
	go test ./...
	node --test test/Model.test.cjs

lint:
	go vet ./...
	qmllint -I "$(OMARCHY_PATH)/shell" plugin/BarWidget.qml plugin/Panel.qml

validate:
	omarchy plugin validate plugin

install: build
	install -d "$(BINDIR)"
	install -m 0755 bin/$(BIN_NAME) "$(BINDIR)/$(BIN_NAME)"
	install -d "$(PLUGIN_DIR)"
	$(foreach f,$(PLUGIN_FILES),install -m 0644 plugin/$(f) "$(PLUGIN_DIR)/$(f)";)
	@echo "Installed $(BIN_NAME) to $(BINDIR) and plugin to $(PLUGIN_DIR)"
	@echo "Reload the shell with: omarchy-shell shell rescanPlugins"

uninstall:
	rm -f "$(BINDIR)/$(BIN_NAME)"
	rm -rf "$(PLUGIN_DIR)"
	@echo "Removed $(BIN_NAME) and $(PLUGIN_DIR)"

clean:
	rm -rf bin
