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
| `tab N:` | 🗂️ Extra sub-tab (surface) inside the preceding pane |
| `"Name"` | 🏷️ Optional label for the pane/tab (see below) |
| `(focused)` | 🎯 This pane gets focus after creation |
| `` `command` `` | ⚡ Send this command to the pane |

Define your own templates by adding `### template-name` sections. Uncheck any pane line to disable that split.

### Naming tabs and panes

Add a quoted name after the descriptor to label a pane or sub-tab. The name is
**optional** — omit it and the line behaves exactly as before (fully backward
compatible):

```markdown
### dev-workflow
- [x] main terminal "Plan": `claude`
- [x] tab 2 "Feature": `claude`
- [x] split right "Diff": `git diff`
- [x] tab 2 "Files": `yazi`
```

Where the name actually shows depends on the backend:

| Unit | cmux | Ghostty |
|------|------|---------|
| Workspace / tab title | ✅ | ✅ (this is the workspace title) |
| Sub-tab (`tab N:`, a surface) | ✅ shown in the pane's tab bar | ❌ Ghostty has no sub-tabs |
| Split (`split right`/`down`) | label stored | ❌ splits have no title slot in Ghostty |

In short: **per-tab/per-surface names render in cmux.** In Ghostty the only
nameable unit is the tab itself (= the crex workspace title); individual splits
and sub-tabs can't carry their own title, so a name on those is stored in the
Blueprint but not displayed. The name is always preserved on save/import either
way, so a layout stays portable between backends.

### Three-Tier Resolution

Templates defined in your Blueprint take priority over built-in gallery templates. When crex resolves a template name, it checks:

1. **Your Blueprint** — templates in your `workspaces.md`
2. **Gallery** — the 16 built-in templates (run `crex template list` to browse)
3. **Fallback** — a single focused terminal pane

This means you can override any built-in template by defining one with the same name. Run `crex template customize <name>` to fork a gallery template as a starting point.

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

See also: [Template Gallery](templates.md) | [Template Authoring](template-authoring.md) | [Workflows](workflows.md) | [Commands](commands.md)
