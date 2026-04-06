# crex Launch — Session State & Continuation Guide

> Last updated: 2026-04-05
> Purpose: If the conversation is lost, any new session can pick up exactly where we left off.

---

## STATUS OVERVIEW

| Item | Status | Notes |
|------|--------|-------|
| daily.dev post | DONE | Published at https://app.daily.dev/posts/SQVEaQpSA — live fix applied ("There were workarounds, but nothing that covered the full workflow") |
| GitHub #1781 | DONE | Comment posted with warm closing line |
| GitHub #2125 | SKIPPED | Already closed & fixed natively (PR #2147). Zero comments, no audience — commenting would look like spam |
| Terminal Trove | DONE | Juan filled the form manually. All values listed below |
| Show HN | DEFERRED | Juan has NO HN account. Strategy: build karma first by commenting on cmux-related threads, then launch Show HN later |
| GitHub #1984 | DONE | Comment posted by Juan |
| GitHub #1664 | SKIPPED | SSH workspaces — crex only partial solve. Two comments (#1781 + #1984) is enough; more would look spammy |
| GitHub #399 | SKIPPED | Frozen blank state — crex is workaround not fix, avoid repo spam |
| GitHub #2309 | SKIPPED | 0.63.0 crash — crex is recovery not fix, avoid repo spam |
| GitHub #432 | SKIPPED | Crash after sleep — crex is recovery not fix, avoid repo spam |
| Dev.to comment #19 (Calyx vs cmux) | DONE | First comment posted by Juan on Apr 6. Most Dev.to cmux articles are dead (0 comments) — this one was the best fit |
| Dev.to comments #18,#21,#22,#23 | SCHEDULED | Vikunja tasks #7-#11 in 💻 IT. Spread across Apr 8-18. Check each article for activity before posting. Drafts in devto-comments-drafts.md |
| Dev.to comment #20 | SKIPPED | No session persistence angle, off-topic |
| Reddit posts | PENDING | r/commandline, r/golang, r/terminal |
| Dev.to full article | PENDING | Tutorial-style "I built tmux-resurrect for cmux" |
| Product Hunt | PENDING (next week) | |
| Lobste.rs | PENDING (next week) | |

---

## CRITICAL RULES FOR ALL CONTENT

1. **Juan reviews and publishes everything himself** — never auto-publish
2. **All drafts reviewed by 3 experts**: psychologist, UX persuasion expert, devil's advocate
3. **"There wasn't one" is FALSE** — there ARE workarounds (auto-restore PRs, scripts, Cmd+H). Honest framing: "There were workarounds, but nothing that covered the full workflow"
4. **Positioning varies by platform**:
   - GitHub issues: "works today while the native solution lands"
   - daily.dev: standalone "I built this" story
   - Terminal Trove: pure utility, no backstory
   - HN: technical Show HN with tmux-resurrect parallel
5. **Emojis**: use sparingly, only where they serve as visual anchors in scanning contexts. Zero on daily.dev, selective elsewhere
6. **Name**: "cmux-resurrect" is the identity, "crex" is introduced parenthetically as the command shortcut

---

## TERMINAL TROVE — FINAL VALUES (submitted by Juan)

**name:** `cmux-resurrect`

**url:** `github.com/drolosoft/cmux-resurrect`

**tagline:** `🔄 Never lose your cmux workspace again — save, restore, and share layouts in seconds.`

**description (261 chars):**
`Save and restore your entire cmux layout — splits, tabs, working directories, pinned state, and startup commands — in a single command. Export layouts as human-readable Markdown Blueprints you can version in git, share with your team, and reuse across machines.`

**standout features (244 chars):**
`Single-command save/restore of splits, tabs, directories, and startup commands. Markdown Workspace Blueprints for sharing and version control. Dry-run mode previews every action before execution. Auto-save via launchd keeps snapshots current.`

**other notable features (220 chars):**
`Idempotent imports safely merge into existing sessions without duplicates. Works alongside cmux's native layout system — no conflicts. Human-readable TOML snapshots you can edit by hand. Single binary, zero dependencies.`

**who is this for (215 chars):**
`cmux users who want their workspaces to survive crashes, updates, and restarts. Teams who need shareable, version-controlled terminal layouts across machines. Anyone tired of rebuilding splits and tabs from scratch.`

**language:** Go (radio)
**license:** MIT (radio)

**preview PNG:** `https://raw.githubusercontent.com/drolosoft/cmux-resurrect/main/assets/import-success.png`
**preview GIF:** `https://raw.githubusercontent.com/drolosoft/cmux-resurrect/main/assets/demo.gif`

**install platform 1:** macos → homebrew → `brew install drolosoft/tap/cmux-resurrect`
**install platform 2:** other (name: go) → go → `go install github.com/drolosoft/cmux-resurrect@latest`

**categories:** macOS, cli, productivity, utilities

**author:** yes
**email:** txeo.msx@gmail.com

---

## GITHUB #1781 — POSTED COMMENT

Issue: "Closing window destroys all terminals and sessions"
URL: https://github.com/manaflow-ai/cmux/issues/1781

Comment posted by Juan with warm closing: "Hope it saves you from the next accidental close — happy to hear any feedback."

---

## DAILY.DEV POST — PUBLISHED

URL: https://app.daily.dev/posts/SQVEaQpSA

Title: "A crash took my cmux workspaces. So I built cmux-resurrect."

Live version has the corrected line: "There were workarounds, but nothing that covered the full workflow." (the draft file on disk still has the old "There wasn't one" — the fix was applied directly on the live post by Juan).

---

## SHOW HN — READY BUT DEFERRED

**Title:** `Show HN: crex – Session persistence for cmux (like tmux-resurrect)`
**Strategy:** Juan needs to create an HN account and build karma first by commenting on existing cmux-related HN threads:
- https://news.ycombinator.com/item?id=45596024
- https://news.ycombinator.com/item?id=47079718
- https://news.ycombinator.com/item?id=47008732
- https://news.ycombinator.com/item?id=47223871

---

## KEY ASSETS

- **Demo GIF (372.5KB):** `https://raw.githubusercontent.com/drolosoft/cmux-resurrect/main/assets/demo.gif`
- **Import success PNG (209.4KB):** `https://raw.githubusercontent.com/drolosoft/cmux-resurrect/main/assets/import-success.png`
- **Repo:** `https://github.com/drolosoft/cmux-resurrect`
- **Homebrew:** `brew install drolosoft/tap/cmux-resurrect`
- **Go install:** `go install github.com/drolosoft/cmux-resurrect@latest`

---

## FULL LAUNCH CALENDAR (all tasks in Vikunja 💻 IT)

| Date | Task | Vikunja ID |
|------|------|-----------|
| Apr 8 | Dev.to comment #23 — Terminal setups | #8 |
| Apr 9 | Create HN account + build karma | #13 |
| Apr 10 | Dev.to comment #18 — cmux for AI Agents | #9 |
| Apr 11 | Reddit — r/commandline | #14 |
| Apr 12 | Ghostty Discussion #3358 | #22 |
| Apr 13 | Reddit — r/golang | #15 |
| Apr 14 | Dev.to comment #22 — claunch | #10 |
| Apr 15 | Reddit — r/terminal | #16 |
| Apr 16 | Dev.to comment #21 — Claude Code (optional) | #11 |
| Apr 17 | Show HN (only if karma is ready) | #17 |
| Apr 19 | Dev.to full article | #18 |
| Apr 21 | Product Hunt launch | #19 |
| Apr 22 | Lobste.rs submission | #20 |
| Apr 23 | Medium comments (#25, #26) | #21 |
| Apr 24 | DevHunt submission | #23 |

All tasks have URLs, suggested text, and instructions in their Vikunja descriptions.

---

## ACCOUNTS & PROFILES

- **daily.dev:** Juan's profile (active, post published)
- **GitHub:** drolosoft (active)
- **Reddit:** personal account (active)
- **Dev.to:** personal account (active)
- **HN:** NO ACCOUNT — needs creation + karma building
- **Product Hunt:** NO ACCOUNT — needs creation
- **Terminal Trove:** no account needed (form submission with email txeo.msx@gmail.com)

---

*This document is the single source of truth for the crex launch campaign. Reference `crex-launch-plan.md` for the full URL list of all targets.*
