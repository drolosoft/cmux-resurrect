BINARY := crex
BINARY_LONG := cmux-resurrect
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/drolosoft/cmux-resurrect/cmd.Version=$(VERSION) -X github.com/drolosoft/cmux-resurrect/cmd.Commit=$(COMMIT) -X github.com/drolosoft/cmux-resurrect/cmd.Date=$(DATE)"

.PHONY: build test test-integration install install-long install-both clean lint fmt

build:
	go build $(LDFLAGS) -o bin/$(BINARY) .

# Install as 'crex' (short name, default)
install: build
	@echo "Installing as '$(BINARY)' → /usr/local/bin/$(BINARY)"
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "✓ Run with: crex"

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

test-integration:
	go test ./... -v -count=1 -tags integration

lint:
	go vet ./...

clean:
	rm -rf bin/

fmt:
	go fmt ./...

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
	@echo "  cmux-resurrect — Build & Install"
	@echo "  ─────────────────────────────────"
	@echo ""
	@echo "  make build          Build the binary"
	@echo "  make install        Install as 'crex' (short name)"
	@echo "  make install-long   Install as 'cmux-resurrect' (long name)"
	@echo "  make install-both   Install both names (crex + cmux-resurrect)"
	@echo ""
	@echo "  make test           Run unit tests"
	@echo "  make lint           Run go vet"
	@echo "  make clean          Remove build artifacts"
	@echo ""
