[Home](../README.md) > Auto-Save

# 🍎 Auto-Save with launchd

The `watch` command runs as a macOS service, auto-saving when your terminal multiplexer is active.

> **Note:** The launchd service below uses the cmux socket for activation. Ghostty users can run `crex watch` directly from their shell or `.zprofile`.

## Install the Service

```sh
make install-service    # → ~/Library/LaunchAgents/com.crex.watch.plist
make uninstall-service  # remove it
```

## How It Works

- ⏱️ Runs `crex watch autosave --interval 5m`
- 🔌 Only starts when `/tmp/cmux.sock` exists (cmux backend; see note above for Ghostty)
- 📄 Logs to `/tmp/crex-watch.log`
- 🛡️ Throttles restarts to every 30s
- 🔗 Content-hash deduplication — no duplicate files when layout hasn't changed

## Troubleshooting

Check if the service is running:
```sh
launchctl list | grep crex
```

View logs:
```sh
tail -f /tmp/crex-watch.log
```

---

See also: [Configuration](configuration.md) | [Commands](commands.md)
