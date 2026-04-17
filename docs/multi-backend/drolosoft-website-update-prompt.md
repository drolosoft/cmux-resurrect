# Drolosoft Website Update — crex Multi-Backend Launch

**Give this prompt to the session working on the Drolosoft website.**

**Repo:** `/Users/txeo/Git/mac/go/drolosoft`
**Product page template:** `web/templates/site/pages/products/cmux-resurrect.html`
**i18n English:** `data/site-en.json`
**i18n Spanish:** `data/site-es.json`
**Assets:** `public/assets/`
**Verify at:** `http://localhost:2005/cmux-resurrect.html`

---

## What happened

crex (cmux-resurrect) now supports **Ghostty** as a second backend, alongside cmux. This is the biggest feature since launch. The entire backend was abstracted into a clean interface, a full Ghostty implementation was built via AppleScript, and the CLI now auto-detects which terminal you're running in — zero configuration.

**This is NOT a pivot.** cmux is the origin. Ghostty is additive. The spirit of the project — session persistence, resurrection, the corncrake — stays exactly the same. What changes is reach: crex now serves the Ghostty community (51K+ GitHub stars, zero session tooling until now).

---

## The story to tell

The Ghostty community has been asking for session management since day one. GitHub discussions (#2480, #3358, #9825) — some locked because of too many upvotes. The only existing tool covers maybe 40% of what crex does. crex fills the gap with everything it already had: save, restore, templates, Workspace Blueprints, auto-save — all of it now working in Ghostty.

**Key messaging:**
- crex was born in cmux and is now available for Ghostty
- Zero configuration — crex auto-detects your terminal
- Same features on both backends: save, restore, templates, Blueprint, watch
- The corncrake (crex crex) is a phoenix — it resurrects sessions regardless of which terminal you use
- Inspired by tmux-resurrect, built for the modern terminal ecosystem
- 17/17 feature tests passed on Ghostty live testing
- Open source, MIT licensed, Homebrew installable

**Avoid:**
- Don't frame this as "leaving cmux" or "switching to Ghostty"
- Don't position Ghostty as the "main" backend — they're equal
- Don't mention AI, Claude, or any tooling used in development
- Don't oversell the Ghostty API stability — it's preview (v1.3), expect changes

---

## Changes needed on the website

### 1. Product page hero (cmux-resurrect.html)

Add a **Ghostty badge** next to the existing platform badges. Suggested:

```html
<span class="product-badge product-badge--platform">Ghostty</span>
```

This should be visually prominent — the first thing a Ghostty user sees is that crex supports their terminal.

### 2. i18n strings to update (site-en.json)

**Title** — keep as `cmux-resurrect` (the legacy title). But consider adding a subtitle or the tagline should now reflect both backends.

**Tagline** — update from cmux-only to multi-backend:
```
OLD: "Session persistence for cmux — your terminal workspaces, resurrected."
NEW: "Session persistence for cmux and Ghostty — your terminal workspaces, resurrected."
```

**Meta description** — add Ghostty mention:
```
NEW: "cmux-resurrect - Session persistence for cmux and Ghostty. Save and restore your terminal workspaces with one command. Auto-detects your terminal. Workspace Blueprints, Template Gallery, Homebrew install. Open source, MIT licensed."
```

**Problem section** — reframe to address both audiences:
```
NEW: "Modern terminal multiplexers like cmux and Ghostty handle session restoration well most of the time, but crashes, forced updates, and unexpected reboots can still wipe your workspace."
```

**Solution section** — add Ghostty alongside cmux:
```
NEW: "<strong>crex</strong> is a safety net for those moments. One command saves your entire terminal layout — workspaces, splits, CWDs, pinned state, startup commands. One command brings it all back. Works with <strong>cmux</strong> and <strong>Ghostty</strong> — auto-detected, zero configuration. Inspired by <a href=\"...\">tmux-resurrect</a>, crex goes further with <strong>Workspace Blueprints</strong>: define your ideal terminal setup in Obsidian-compatible Markdown, version it, share it with your team."
```

**Why crex section** — the comparison table currently shows only tmux-resurrect vs crex. Add a new row or column highlighting multi-backend support. Consider adding a note like "Now works with Ghostty — no configuration needed."

**Feature 4 (Dry-Run Preview)** — remove "cmux" specificity:
```
OLD: "See every cmux command that will execute before anything runs."
NEW: "See every command that will execute before anything runs — cmux CLI commands or Ghostty AppleScript, depending on your terminal."
```

**Feature 5 (Auto-Save)** — remove "cmux socket" specificity:
```
OLD: "Periodic saves tied to cmux socket availability."
NEW: "Periodic saves with content-hash deduplication. Zero maintenance."
```

### 3. New "Supported Backends" section

Add a new section after the Problem/Solution block. This is the main visual anchor for multi-backend support. Suggested structure:

```
## Supported Backends

crex auto-detects your terminal — no flags, no configuration.

[cmux icon/card]                    [Ghostty icon/card]
cmux (cmux.dev)                     Ghostty (ghostty.org)
Full support since v1.0             NEW — Full support since v1.3
Save, restore, templates,           Save, restore, templates,
Blueprint, watch, dry-run           Blueprint, watch, dry-run
```

This should be a visual section with two cards side by side. Both backends show the same feature list — emphasizing parity.

### 4. Spanish translations (site-es.json)

Mirror all English changes into Spanish. Key translations:
- "Session persistence for cmux and Ghostty" → "Persistencia de sesiones para cmux y Ghostty"
- "Auto-detects your terminal" → "Detecta tu terminal automáticamente"
- "your terminal workspaces, resurrected" → "tus espacios de trabajo, resucitados"
- "No configuration needed" → "Sin configuración necesaria"

### 5. Homepage app card (index.html)

The app card in the "Our Apps" section should be updated to mention Ghostty. If there's a short description, add "cmux & Ghostty" or "multi-backend" somewhere visible.

### 6. data.json (atom visualization)

If the data.json drives the 3D atom visualization and includes app descriptions, update the cmux-resurrect entry to mention Ghostty.

### 7. Version badge

Update the version badge from `v1.2.0` to whatever the new release version will be (suggest `v1.3.0` to match the Ghostty API version it targets).

---

## Tone and spirit

The Drolosoft website has a specific identity: "Tools we wish existed." The update should feel like a natural evolution, not a big launch event. The tone is:

- **Confident but understated** — "crex now works with Ghostty" not "ANNOUNCING GHOSTTY SUPPORT!!!"
- **Builder's pride** — this was built because the community needed it, not for marketing points
- **Heritage-aware** — cmux is mentioned first, always. Ghostty is the new addition, not the replacement
- **Technical credibility** — mention auto-detection, AppleScript backend, zero-config. Developers trust specifics, not superlatives
- **The corncrake is a phoenix** — the resurrection metaphor works even better now: crex resurrects sessions across terminals

---

## What NOT to change

- The product page URL stays `/cmux-resurrect.html` — this is the canonical name
- The GitHub link stays `github.com/drolosoft/cmux-resurrect` — no repo rename
- The demo GIF will need re-recording to show Ghostty (separate task, not blocking the page update)
- Keep tmux-resurrect credit and links — the inspiration lineage is part of the story
- Do NOT mention AI, Claude, LLM, or any development tooling
