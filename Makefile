BINARY := crex
BINARY_LONG := cmux-resurrect
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/drolosoft/cmux-resurrect/cmd.Version=$(VERSION) -X github.com/drolosoft/cmux-resurrect/cmd.Commit=$(COMMIT) -X github.com/drolosoft/cmux-resurrect/cmd.Date=$(DATE)"

.PHONY: build build-all test test-integration validate install install-long install-both clean lint fmt completions

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/crex

# Cross-compile for macOS (Intel + Apple Silicon) and Linux
build-all:
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64  ./cmd/crex
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64  ./cmd/crex
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64   ./cmd/crex
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64   ./cmd/crex
	@echo "✓ Built 4 binaries in bin/"

LAYOUTS_DIR := $(HOME)/.config/crex/layouts

# Install as 'crex' (short name, default)
install: build install-demo
	@echo "Installing as '$(BINARY)' → /usr/local/bin/$(BINARY)"
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "✓ Run with: crex"

# Copy demo layout if it doesn't exist yet
install-demo:
	@mkdir -p $(LAYOUTS_DIR)
	@if [ ! -f $(LAYOUTS_DIR)/demo.toml ]; then \
		cp testdata/layouts/demo.toml $(LAYOUTS_DIR)/demo.toml; \
		echo "✓ Demo layout installed — try: crex restore demo"; \
	fi

# Install as 'cmux-resurrect' (long name)
install-long: build
	@echo "Installing as '$(BINARY_LONG)' → /usr/local/bin/$(BINARY_LONG)"
	cp bin/$(BINARY) /usr/local/bin/$(BINARY_LONG)
	@echo "✓ Run with: cmux-resurrect"

# Install both names (crex + cmux-resurrect symlink)
install-both: install
	@echo "Adding symlink '$(BINARY_LONG)' → $(BINARY)"
	ln -sf /usr/local/bin/$(BINARY) /usr/local/bin/$(BINARY_LONG)
	@echo "✓ Run with: crex  or  cmux-resurrect"

test:
	go test ./... -v -count=1

# Validate v1.3.x features: shortcut, theme, descriptions, branding × both backends
validate:
	@echo "═══ Validation suite: shortcut + theme + descriptions + branding ═══"
	go test ./cmd/ -run TestValidate -v -count=1
	@echo ""
	@echo "═══ Full test suite ═══"
	go test ./... -count=1
	@echo ""
	go vet ./...
	@echo "═══ All checks passed ═══"

test-integration:
	go test ./... -v -count=1 -tags integration

lint:
	go vet ./...
	golangci-lint run

clean:
	rm -rf bin/

fmt:
	go fmt ./...

# Generate shell completion scripts
completions: build
	@mkdir -p completions
	bin/$(BINARY) completion bash > completions/crex.bash
	bin/$(BINARY) completion zsh  > completions/_crex
	bin/$(BINARY) completion fish > completions/crex.fish
	@echo "✓ Generated completions/ (bash, zsh, fish)"

# launchd service management
install-service: install
	@mkdir -p ~/Library/LaunchAgents
	@sed "s|{{BINARY}}|/usr/local/bin/$(BINARY)|g" deploy/launchd/com.cmux-resurrect.watch.plist > ~/Library/LaunchAgents/com.crex.watch.plist
	launchctl load ~/Library/LaunchAgents/com.crex.watch.plist
	@echo "Service installed and started"

uninstall-service:
	launchctl unload ~/Library/LaunchAgents/com.crex.watch.plist 2>/dev/null || true
	rm -f ~/Library/LaunchAgents/com.crex.watch.plist
	@echo "Service removed"

# Show install options
help:
	@echo ""
	@echo "  crex (cmux-resurrect) — Build & Install"
	@echo "  ──────────────────────────────────────────"
	@echo ""
	@echo "  make build          Build the binary (current platform)"
	@echo "  make build-all      Cross-compile for macOS + Linux"
	@echo "  make install        Install as 'crex' (short name)"
	@echo "  make install-long   Install as 'cmux-resurrect' (long name)"
	@echo "  make install-both   Install both names (crex + cmux-resurrect)"
	@echo ""
	@echo "  make test           Run unit tests"
	@echo "  make validate       Run v1.3.x feature validation (shortcut, theme, branding)"
	@echo "  make lint           Run go vet"
	@echo "  make clean          Remove build artifacts"
	@echo ""
