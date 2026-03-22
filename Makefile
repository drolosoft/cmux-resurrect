BINARY := cmx
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/txeo/cmux-persist/cmd.Version=$(VERSION) -X github.com/txeo/cmux-persist/cmd.Commit=$(COMMIT) -X github.com/txeo/cmux-persist/cmd.Date=$(DATE)"

.PHONY: build test test-integration install clean lint

build:
	go build $(LDFLAGS) -o bin/$(BINARY) .

install: build
	cp bin/$(BINARY) /usr/local/bin/$(BINARY)

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
install-service:
	@mkdir -p ~/Library/LaunchAgents
	@sed "s|{{BINARY}}|$(shell pwd)/bin/$(BINARY)|g" deploy/launchd/com.cmux-persist.watch.plist > ~/Library/LaunchAgents/com.cmux-persist.watch.plist
	launchctl load ~/Library/LaunchAgents/com.cmux-persist.watch.plist
	@echo "Service installed and started"

uninstall-service:
	launchctl unload ~/Library/LaunchAgents/com.cmux-persist.watch.plist 2>/dev/null || true
	rm -f ~/Library/LaunchAgents/com.cmux-persist.watch.plist
	@echo "Service removed"
