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

## Daemon Mode (v1.5.0+)

The watch command now supports a built-in daemon mode — no launchd plist needed.

```sh
crex watch --daemon             # start background auto-persistence
crex watch --status             # check if the daemon is running
crex watch --stop               # stop the daemon
```

How it works:
- PID file at `~/.config/crex/crex.pid`
- Logs to `~/.config/crex/watch.log` (1 MB rotation, one `.old` backup)
- Content-hash deduplication (same as foreground mode)
- Set `CREX_NO_WATCH=1` to prevent auto-start from shell hooks

### Shell Hook (auto-start on login)

Generate a shell snippet that starts the daemon automatically:

```sh
crex watch --shell-hook                 # preview the snippet
crex watch --shell-hook >> ~/.zshrc     # install for zsh
crex watch --shell-hook >> ~/.bashrc    # install for bash
```

Fish users: `crex watch --shell-hook | source` or add to `config.fish`.

The hook is idempotent — it checks the PID file before starting a new daemon.

## Troubleshooting

Check if the service is running:
```sh
launchctl list | grep crex
```

View logs:
```sh
tail -f /tmp/crex-watch.log
```

Check daemon status:
```sh
crex watch --status
```

View daemon logs:
```sh
tail -f ~/.config/crex/watch.log
```

Kill a stale daemon:
```sh
crex watch --stop
# If PID file is stale, remove it:
rm ~/.config/crex/crex.pid
```

---

See also: [Configuration](configuration.md) | [Commands](commands.md)
