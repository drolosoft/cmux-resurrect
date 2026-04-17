# Drolosoft Website Changes — Multi-Backend Release

Checklist of changes needed on [drolosoft.com](https://drolosoft.com) when the multi-backend branch ships.

---

## 1. Homebrew Install — Offer Both Names

The crex page and any install instructions must show both Homebrew formulas:

```sh
brew install drolosoft/tap/cmux-resurrect
# or
brew install drolosoft/tap/crex
```

Both install the same binary (`crex` + `cmux-resurrect` symlink). The second option is a Homebrew tap alias (`Aliases/crex → Formula/cmux-resurrect.rb`).

**Where this appears on the site:**
- crex product page — install section
- Any "getting started" or quickstart copy
- The app card on the homepage (if it shows an install command)

---

## 2. Tagline Update

Current:
> Session persistence for cmux

New:
> Terminal workspace manager for cmux and Ghostty

The subtitle or description should mention both backends. cmux first (origin), Ghostty second (expansion).

---

## 3. Supported Backends Section

Add a section or callout showing:

| Backend | Status |
|---------|--------|
| cmux | Full support (original backend) |
| Ghostty | Full support (v1.3+ macOS, AppleScript) |

Auto-detection: crex detects which terminal you're running in. No configuration needed.

---

## 4. Origin Story — Preserve It

The site copy must keep:
- cmux as the origin ("born in the cmux ecosystem")
- tmux-resurrect as the inspiration
- The corncrake line: "crex takes its name from the corncrake (*Crex crex*) — a migratory bird that returns to the same ground year after year. A phoenix of the grasslands."

Do not rewrite the identity as Ghostty-first. Ghostty is additive.

---

## 5. Feature Comparison Update

If the site has a feature comparison or "why crex" section, add a row:

| Feature | crex |
|---------|------|
| Multi-backend | cmux + Ghostty (auto-detected) |

---

## 6. SEO — Keywords

When the site mentions crex, pair it with qualifiers for discoverability:
- "crex terminal workspace manager"
- "crex cmux ghostty"
- "cmux-resurrect"

The name "crex" alone is crowded in search (CREx Software, crex.com marketplace). Always pair with "terminal" or "workspace".

---

## Notes

- The GitHub repo stays as `drolosoft/cmux-resurrect` for now. All links keep working.
- The `go install` path stays as `github.com/drolosoft/cmux-resurrect/cmd/crex@latest`.
- New screenshots showing Ghostty backend in action should be added when available.
