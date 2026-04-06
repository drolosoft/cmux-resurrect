# crex Launch — Session State & Continuation Guide

> Last updated: 2026-04-06
> Purpose: If the conversation is lost, any new session can pick up exactly where we left off.

---

## STATUS OVERVIEW

| Item | Status | Notes |
|------|--------|-------|
| daily.dev post | DONE | Published at https://app.daily.dev/posts/SQVEaQpSA — live fix applied ("There were workarounds, but nothing that covered the full workflow") |
| GitHub #1781 | DONE | Comment posted with warm closing line |
| GitHub #2125 | SKIPPED | Already closed & fixed natively (PR #2147). Zero comments, no audience — commenting would look like spam |
| Terminal Trove | DONE | Juan filled the form manually. All values listed below |
| Dev.to account | DONE | https://dev.to/drolosoft created Apr 6 |
| Dev.to comment #19 (Calyx vs cmux) | DONE | First comment posted by Juan on Apr 6 |
| DevHunt submission | DONE ✅ | Submitted Apr 6. Account issue RESOLVED — now tied to txeo.msx@gmail.com. Launch Week Apr 7-14 paid ($49) |
| DevHunt Launch Week | ACTIVE | Apr 7-14, 2026. $49 paid. YouTube video: https://youtu.be/TiXPTOv-4oM |
| Show HN | DEFERRED | Juan has NO HN account. Strategy: build karma first by commenting on cmux-related threads, then launch Show HN later |
| GitHub #1984 | DONE | Comment posted by Juan |
| GitHub #1664 | SKIPPED | SSH workspaces — crex only partial solve. Two comments (#1781 + #1984) is enough; more would look spammy |
| GitHub #399 | SKIPPED | Frozen blank state — crex is workaround not fix, avoid repo spam |
| GitHub #2309 | SKIPPED | 0.63.0 crash — crex is recovery not fix, avoid repo spam |
| GitHub #432 | SKIPPED | Crash after sleep — crex is recovery not fix, avoid repo spam |
| Dev.to comments #18,#21,#22,#23 | SCHEDULED | Vikunja tasks #8-#11 in 💻 IT. Spread across Apr 8-16. Check each article for activity before posting. Drafts in devto-comments-drafts.md |
| Dev.to comment #20 | SKIPPED | No session persistence angle, off-topic |
| Reddit posts | PENDING | r/commandline (Apr 11), r/golang (Apr 13), r/terminal (Apr 15) |
| Dev.to full article | PENDING | Apr 19. Tutorial-style "I built tmux-resurrect for cmux" |
| Product Hunt | PENDING | Apr 21 |
| Lobste.rs | PENDING | Apr 22. Invite-only — check access |
| Medium comments | PENDING | Apr 23. #25 and #26 |
| product-launch skill | DONE | Created and packaged in txeo-tools plugin v1.1.0 |

---

## ✅ DEVHUNT ACCOUNT ISSUE — RESOLVED

**Problem:** Tool was submitted under juan.andres@livgolf.com instead of txeo.msx@gmail.com.
**Resolution:** Juan fixed it directly on DevHunt. Account now tied to txeo.msx@gmail.com.
**Vikunja task:** #24 — marked DONE.

---

## DEVHUNT SUBMISSION — DETAILS

**Slogan:** tmux-resurrect for cmux — save and restore your entire terminal layout
**Categories:** CLI, Open Source, Workflow automation, DevOps
**YouTube video:** https://youtu.be/TiXPTOv-4oM
**Screenshots:** 4 uploaded from ~/Git/yo/cmux/assets/demo-screenshots/ (files 1, 2, 3, 6)
**Pricing:** Free
**Profile:** username=txeo, LinkedIn linked, bio filled

**Description HTML:**
```html
Save and restore your entire cmux layout — splits, tabs, working directories, pinned state, and startup commands — in a single command.<br><br><b>Two workflows:</b><br>• <b>Save/Restore</b> — TOML snapshots of your current session. One command to save, one to bring it back.<br>• <b>Import/Export</b> — Markdown Workspace Blueprints you can version in git, share with your team, and reuse across machines.<br><br><b>Highlights:</b> Dry-run mode previews every action before execution. Auto-save via launchd. Idempotent imports. Single Go binary, zero dependencies. MIT licensed.<br><br><code>brew install drolosoft/tap/cmux-resurrect</code>
```

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
   - DevHunt: polished product listing with brew install CTA
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

## SHOW HN — DRAFT READY, WAITING FOR KARMA

**Title:** `Show HN: crex – Session persistence for cmux (like tmux-resurrect)`
**URL:** `https://github.com/drolosoft/cmux-resurrect`
**Target date:** Apr 17 (or earlier if karma is ready)
**Best posting time:** Weekday 9am PT (6pm Spain)

**Strategy update (Apr 6):** Analyzed all 4 HN cmux threads. The lawrencechen thread (47079718, 199pts) discussed session persistence extensively — blorenz mentioned "session persistence" and "session resume" as features. However, the thread is 45 days old and dead. Commenting there wastes impact. A fresh Show HN is the right move.

**Karma-building:** Comment genuinely on CURRENT active threads (Go, CLI, terminals — NOT the old cmux threads). Don't mention crex during karma phase.

**First comment draft (reviewed by 3 experts ✅):**

> cmux-resurrect (crex) saves and restores your entire cmux layout — splits, tabs, working directories, pinned state, and startup commands — in a single command.
>
> I built it because a crash wiped my whole workspace and cmux has no native session persistence yet. There were workarounds (auto-restore PRs, scripts, Cmd+H), but nothing that covered the full workflow.
>
> Two modes: Save/Restore uses TOML snapshots for quick recovery. Import/Export uses Markdown Workspace Blueprints you can version in git and share across machines.
>
> Written in Go, single binary, zero dependencies. Dry-run mode previews every action before touching your session. MIT licensed.
>
> brew install drolosoft/tap/cmux-resurrect

**Pre-flight checklist:**
- [ ] HN account created with some karma
- [ ] Post on a weekday morning US time (9am PT = 6pm Spain)
- [ ] Be ready to engage with comments for the first 2-3 hours
- [ ] Do NOT mention DevHunt or Launch Week on HN

---

## KEY ASSETS

- **Demo GIF (372.5KB):** `https://raw.githubusercontent.com/drolosoft/cmux-resurrect/main/assets/demo.gif`
- **Demo MP4 (293KB):** saved locally, also on YouTube: https://youtu.be/TiXPTOv-4oM
- **Import success PNG (209.4KB):** `https://raw.githubusercontent.com/drolosoft/cmux-resurrect/main/assets/import-success.png`
- **Demo screenshots:** `~/Git/yo/cmux/assets/demo-screenshots/` (files 1, 2, 3, 6 used for DevHunt)
- **Repo:** `https://github.com/drolosoft/cmux-resurrect`
- **Homebrew:** `brew install drolosoft/tap/cmux-resurrect`
- **Go install:** `go install github.com/drolosoft/cmux-resurrect@latest`

---

## FULL LAUNCH CALENDAR (all tasks in Vikunja 💻 IT)

| Date | Task | Vikunja ID | Status |
|------|------|-----------|--------|
| Apr 6 | DevHunt submission | #23 | ✅ DONE (account issue — #24 tracks fix) |
| Apr 7 | DevHunt Launch Week begins | — | 🔥 ACTIVE (Apr 7-14) |
| Apr 7 | ~~Resolve DevHunt account migration~~ | #24 | ✅ RESOLVED |
| Apr 8 | Dev.to comment #23 — Terminal setups | #8 | PENDING |
| Apr 9 | Create HN account + build karma | #13 | PENDING |
| Apr 10 | Dev.to comment #18 — cmux for AI Agents | #9 | PENDING |
| Apr 11 | Reddit — r/commandline | #14 | PENDING |
| Apr 12 | Ghostty Discussion #3358 | #22 | PENDING |
| Apr 13 | Reddit — r/golang | #15 | PENDING |
| Apr 14 | Dev.to comment #22 — claunch | #10 | PENDING |
| Apr 15 | Reddit — r/terminal | #16 | PENDING |
| Apr 16 | Dev.to comment #21 — Claude Code (optional) | #11 | PENDING |
| Apr 17 | Show HN (only if karma is ready) | #17 | PENDING |
| Apr 19 | Dev.to full article | #18 | PENDING |
| Apr 21 | Product Hunt launch | #19 | PENDING |
| Apr 22 | Lobste.rs submission | #20 | PENDING |
| Apr 23 | Medium comments (#25, #26) | #21 | PENDING |

All tasks have URLs, suggested text, and instructions in their Vikunja descriptions.

---

## ACCOUNTS & PROFILES

- **daily.dev:** Juan's profile (active, post published)
- **GitHub:** drolosoft (active)
- **Reddit:** personal account (active)
- **Dev.to:** https://dev.to/drolosoft (active, created Apr 6)
- **DevHunt:** txeo.msx@gmail.com (fixed, active)
- **HN:** NO ACCOUNT — needs creation + karma building
- **Product Hunt:** NO ACCOUNT — needs creation
- **Terminal Trove:** no account needed (form submission with email txeo.msx@gmail.com)
- **YouTube:** video uploaded https://youtu.be/TiXPTOv-4oM

---

## TOOLING

- **product-launch skill:** Created and added to txeo-tools plugin v1.1.0
- **Plugin file:** `/sessions/peaceful-youthful-hopper/mnt/cmux/txeo-tools.plugin` (32KB zip)
- **Eval results:** with_skill 100% (21/21) vs without_skill 67% (14/21), delta +33%
- **Eval viewer:** `/sessions/peaceful-youthful-hopper/mnt/cmux/product-launch-eval-review.html`

---

## PULSE DASHBOARD (pulse.txeo.club)

**Prompt file:** `pulse-dashboard-prompt.md` (in workspace folder)
**Update frequency:** Every 3 days (Vikunja #25 recurring, Google Calendar recurring)

**Workflow:**
1. Every 3 days, review this session state doc for changes
2. Update `pulse-dashboard-prompt.md` with: new completed actions, updated metrics (stars, upvotes, comments), new scheduled tasks, any skipped/rescheduled items
3. Feed the updated prompt to pulse.txeo.club

**What changes between updates:**
- Completed vs pending actions table
- GitHub metrics (stars, forks)
- Content performance (DevHunt votes, daily.dev engagement, Reddit upvotes)
- New content drafts or modified schedule
- Account status changes

---

*This document is the single source of truth for the crex launch campaign. Reference `crex-launch-plan.md` for the full URL list of all targets.*
