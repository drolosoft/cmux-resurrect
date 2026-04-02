[Home](../README.md) > Workspace Blueprints

# 📝 Workspace Blueprints

A Workspace Blueprint is a Markdown document (.md) with two sections: **Projects** and **Templates**. Compatible with Obsidian and any Markdown editor.

## Projects Section

```markdown
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp         | dev      | yes | ~/projects/webapp
- [x] | ⚙️ | api-server     | dev      | yes | ~/projects/api-server
- [x] | 🧪 | testing        | go       | yes | ~/projects/testing
- [ ] | 📓 | notes          | single   | no  | ~/documents/notes
- [x] | 📊 | dashboard      | monitor  | yes | ~/projects/dashboard
```

| Element | Meaning |
|---------|---------|
| `[x]` / `[ ]` | ✅ Enabled / ⬜ Disabled — controls import behavior |
| Pipe columns | 🏷️ Icon, name, template, pin status, filesystem path |
| Unchecked workspace | ⏸️ Excluded from `crex import-from-md` without deleting it |
| Unchecked pane | ⏸️ That split is skipped during import |

## Templates Section

Templates define reusable pane layouts. Reference them by name from any workspace row.

```markdown
## Templates

### dev
- [x] main terminal (focused)
- [x] split right: `npm run dev`
- [x] split right: `lazygit`

### go
- [x] main terminal (focused)
- [x] split right: `go test ./...`

### single
- [x] main terminal (focused)

### monitor
- [x] main terminal: `htop`
- [x] split right: `tail -f /var/log/system.log`
```

| Keyword | What it creates |
|---------|----------------|
| `main terminal` | 🖥️ First pane in the workspace |
| `split right:` | ➡️ Vertical split to the right |
| `split down:` | ⬇️ Horizontal split below |
| `(focused)` | 🎯 This pane gets focus after creation |
| `` `command` `` | ⚡ Send this command to the pane |

Define your own templates by adding `### template-name` sections. Uncheck any pane line to disable that split.

## Managing Blueprint Entries from CLI

```sh
crex workspace add api ~/projects/api -t dev --icon "⚙️"   # add with template + icon
crex workspace add notes ~/docs -t single --disabled       # add disabled by default
crex workspace remove api                                  # remove an entry
crex workspace toggle notes                                # enable/disable
crex workspace list                                        # list all entries
crex workspace list --all                                  # include disabled entries
```

---

See also: [Workflows](workflows.md) | [Commands](commands.md)
