[Home](../README.md) > Building from Source

# 🔨 Building from Source

## Prerequisites

- Go 1.26+
- cmux in `$PATH` (for cmux backend) or Ghostty 1.3+ (for Ghostty backend)

## Build Targets

```sh
make build              # → bin/crex (current platform)
make build-all          # → cross-compile for macOS + Linux
make install            # → /usr/local/bin/crex (short name)
make install-long       # → /usr/local/bin/cmux-resurrect (long name)
make install-both       # → both names (crex + cmux-resurrect)
make test               # 🧪 unit tests
make test-integration   # 🧪 integration tests (needs running cmux)
make lint               # 🔍 go vet
make fmt                # ✨ go fmt
make clean              # 🗑️ remove bin/
```

## 🖥️ Platform Compatibility

crex works with both cmux and Ghostty. **If your Mac runs either one, it runs crex** — no extra dependencies, no compatibility surprises. The backend is auto-detected at startup.

The binary is pure Go with zero CGO dependencies.

| Platform | Architecture | Status |
|----------|-------------|--------|
| macOS (Apple Silicon) | M1, M2, M3, M4 | ✅ Tested |
| macOS (Intel) | x86_64 | ✅ Tested |
| Linux | x86_64 | ✅ Builds |
| Linux | ARM64 | ✅ Builds |

`make build-all` produces binaries for all four targets in `bin/`.

> 📐 For architecture details and internal design, see [ARCHITECTURE.md](../ARCHITECTURE.md).

---

See also: [Commands](commands.md) | [Configuration](configuration.md)
