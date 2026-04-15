[Home](../README.md) > Template Authoring

# Creating Custom Templates

## Quick Start

Add a `### template-name` section to the Templates area of your `workspaces.md`:

```markdown
## Templates

### my-workflow
- [x] main terminal: `nvim` (focused)
- [x] split right: `lazygit`
- [x] split down: `npm test -- --watch`
```

Reference it from any workspace row:

```markdown
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp | my-workflow | yes | ~/projects/webapp
```

Then run:

```sh
crex import-from-md
```

## Pane Syntax Reference

Each line in a template defines one pane:

```markdown
- [x] main terminal: `command` (focused)
- [x] split right: `command`
- [x] split down: `command`
- [ ] split right: `optional pane`
```

| Element | Meaning |
|---------|---------|
| `- [x]` | Pane is enabled |
| `- [ ]` | Pane is disabled (skipped during import) |
| `main terminal` | First pane in the workspace |
| `split right:` | Vertical split to the right |
| `split down:` | Horizontal split below |
| `` `command` `` | Command sent to the pane after creation |
| `(focused)` | This pane receives focus after all panes are created |

### Split Directions

| Direction | Keyword | Effect |
|-----------|---------|--------|
| Right | `split right:` | New pane appears to the right |
| Down | `split down:` | New pane appears below |

### Commands

Commands are optional. Wrap them in backticks:

```markdown
- [x] main terminal: `htop`
- [x] split right:                    # no command — just a shell
- [x] split down: `tail -f app.log`
```

### Focus

Mark exactly one pane with `(focused)` to set the initial cursor position:

```markdown
- [x] main terminal
- [x] split right: `lazygit` (focused)
- [x] split down:
```

If no pane is marked `(focused)`, the first pane gets focus by default.

### Disabling Panes

Uncheck a pane to skip it during import without deleting the definition:

```markdown
- [x] main terminal (focused)
- [x] split right: `lazygit`
- [ ] split down: `npm test -- --watch`    # temporarily disabled
```

## Starting from a Gallery Template

The fastest way to create a custom template is to fork a built-in one:

```sh
crex template customize claude
```

This copies the `claude` template into your Blueprint. Edit it with:

```sh
crex edit
```

Your copy takes priority over the built-in version. To revert, remove the `### claude` section from your Blueprint.

### Example: Customizing the code template

```sh
# Fork the built-in code template
crex template customize code

# Open your Blueprint in $EDITOR
crex edit
```

Now modify the template in your Blueprint:

```markdown
### code
- [x] main terminal: `nvim` (focused)
- [x] split right: `lazygit`
- [x] split down: `npm run dev`
```

## Template Examples

### Backend development

```markdown
### backend
- [x] main terminal: `nvim` (focused)
- [x] split right: `lazygit`
- [x] split down: `go test ./... -v`
```

### Frontend development

```markdown
### frontend
- [x] main terminal: `nvim` (focused)
- [x] split right: `npm run dev`
- [x] split down: `npm test -- --watch`
```

### Docker monitoring

```markdown
### docker
- [x] main terminal: `docker compose logs -f` (focused)
- [x] split right: `docker stats`
- [x] split down: `watch docker compose ps`
```

### Writing

```markdown
### writing
- [x] main terminal: `nvim` (focused)
- [x] split down:
```

## Contributing Templates

To contribute a template to the built-in gallery:

1. Create a `.md` file in `internal/gallery/templates/` following the naming convention (`layout-<name>.md` or `workflow-<name>.md`)
2. Add YAML frontmatter with `name`, `category`, `icon`, `description`, and `tags`
3. Define the pane layout using the standard syntax
4. Add a diagram variant in `cmd/template_show.go`
5. Submit a PR

### Template file format

```markdown
---
name: my-template
category: workflow
icon: "🔧"
description: Short one-line description
tags: [development, git]
---
### my-template
- [x] main terminal: `command` (focused)
- [x] split right: `command`
```

---

See also: [Template Gallery](templates.md) | [Workspace Blueprints](blueprint.md) | [Commands](commands.md)
