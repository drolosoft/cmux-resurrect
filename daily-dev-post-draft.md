# daily.dev Post — DRAFT v4 (do NOT publish without review)

---

## TITLE:

A crash took my cmux workspaces. So I built cmux-resurrect.

---

## BODY:

Last month a [cmux](https://github.com/manaflow-ai/cmux) crash wiped an hour of carefully arranged workspaces — splits, commands, everything. Gone.

I looked for a recovery tool. **There wasn't one.**

If you've used tmux, you might know [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) — it lets you save and restore your entire tmux session across reboots (12K+ stars, essential for any serious tmux workflow). cmux had nothing like it. Inspired by this project, I built one: [**cmux-resurrect**](https://github.com/drolosoft/cmux-resurrect). (The command is `crex` for short.)

`crex save` — captures your entire layout: splits, working directories, pinned tabs, startup commands.

`crex restore` — brings it all back exactly as it was.

But here's what I'm most excited about: **Workspace Blueprints** 📝 Your layouts become human-readable Markdown files — edit them by hand, version them in git, or open them in Obsidian. *Your terminal setup, as code.*

It also has **dry-run mode** (preview before restoring), **reusable templates**, and **auto-save** via launchd.

Built in Go. MIT licensed. Zero dependencies. Homebrew installable. ⚡

Huge thanks to the [cmux](https://github.com/manaflow-ai/cmux) team for building such a clean public API — crex wouldn't exist without it — and to [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) for proving this idea works.

If you've ever lost a cmux session and spent 20 minutes rebuilding it — this is for you.

🔗 [github.com/drolosoft/cmux-resurrect](https://github.com/drolosoft/cmux-resurrect)

---

## PSYCHOLOGY NOTES (v2 changes):

**1. Name recognition**
"cmux-resurrect" is now the protagonist, not "crex". Anyone who knows tmux-resurrect instantly gets what this is — the name IS the pitch. We introduce `crex` parenthetically as the command shortcut ("The command is `crex` for short") so it feels like a practical detail, not the identity.

**2. Closing line — rewritten**
OLD: "If you use cmux — what does your workspace recovery look like today?"
NEW: "If you've ever lost a cmux session and spent 20 minutes rebuilding it — this is for you."

Why this is stronger:
- Questions invite scrolling past. Statements of identification STOP people.
- "20 minutes rebuilding" triggers a specific visceral memory (episodic memory activation). The reader doesn't just think "yeah sessions are annoying" — they FEEL the last time it happened.
- "this is for you" is a direct personal address. It creates belonging, not obligation to answer.
- The naked GitHub link at the end is the final CTA — clean, no noise, just the action.

**3. Emoji placement — why ZERO is the right number here**
After analyzing top-performing daily.dev posts and dev community psychology:
- daily.dev audience skews technical/senior. Heavy emoji use correlates with lower engagement in this audience.
- The post already has strong visual structure: bold, italic, code blocks, link. Adding emojis would compete with those signals.
- tmux-resurrect's own README uses zero emojis. Matching that tone reinforces the "serious tool" positioning.
- The ONE exception where an emoji would work: the title. But daily.dev titles render as plain text in the feed card, so the emoji would be wasted real estate vs a word that communicates more.

**Verdict**: No emojis in this post. Save them for Reddit/Dev.to where the audience expects them.

**4. Structure reinforcement**
- The GitHub link now appears TWICE: once inline on "cmux-resurrect" and once naked at the bottom. This is intentional — the top one catches skimmers, the bottom one catches readers who made it to the end.

## FORMATTING IN THE EDITOR:
- **Bold**: Cmd+B or B button → "cmux-resurrect", "There wasn't one.", "Workspace Blueprints", "dry-run mode", "reusable templates", "auto-save"
- **Italic**: Cmd+I or I button → "Your terminal setup, as code."
- **Code**: backticks → `crex`, `crex save`, `crex restore`
- **Link**: Cmd+K → "cmux-resurrect" (first mention) and final URL
