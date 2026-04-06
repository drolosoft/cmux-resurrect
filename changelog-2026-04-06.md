---
type: changelog
date: 2026-04-06
tags: [changelog, crex, launch, devhunt, youtube, hackernews, pulse, vikunja, calendar]
---

# Changelog — 2026-04-06

## crex Launch Campaign — Major Session

Massive progress on the crex multi-platform launch campaign. This session covered DevHunt submission fixes, video creation, HN strategy, pulse dashboard, and full system sync.

## DevHunt Submission & Account Fix

- Submitted crex to DevHunt Launch Week (Apr 7-14, $49 paid)
- Discovered demo video field doesn't accept GIF URLs — only YouTube or mp4
- Converted `demo.gif` → `demo.mp4` with ffmpeg (293KB, 58s)
- Juan uploaded to YouTube: https://youtu.be/TiXPTOv-4oM
- Fixed YouTube description validation error (special chars `•` and `—` not accepted — replaced with ASCII)
- **Account crisis**: submission was under `juan.andres@livgolf.com` instead of `txeo.msx@gmail.com`
  - Created Gmail draft to john@marsx.dev requesting migration
  - Juan fixed it himself directly on DevHunt — now tied to txeo.msx@gmail.com
- Vikunja: task #239 (DevHunt submission) marked DONE, task #240 (account migration) created & marked DONE

## Show HN Draft

- Analyzed all 4 HN cmux threads — concluded fresh Show HN is better than commenting on dead threads
- lawrencechen thread (47079718, 199pts) discussed session persistence but is 45 days old and dead
- Drafted full Show HN post with 3-expert review (psychologist, UX persuasion, devil's advocate)
- Strategy: build HN karma first by commenting on active Go/CLI/terminal threads, then Show HN on Apr 17 (weekday 9am PT = 6pm Spain)
- Vikunja: task #233 updated with full draft + pre-flight checklist
- Google Calendar: Apr 17 event updated with draft text

## Pulse Dashboard Prompt

- Created `pulse-dashboard-prompt.md` (375 lines) for pulse.txeo.club
- Contains: all 44 target URLs, campaign timeline, content drafts, 12 dashboard section specs, API integrations, design prefs
- Set up recurring update system: Vikunja task #241 (every 3 days, due Apr 9) + Google Calendar recurring events (Apr 9-24)
- Workflow: review session state → update prompt → feed to pulse.txeo.club

## Vikunja & Calendar Sync

Vikunja tasks modified:

| Task ID | Description | Action |
|---------|-------------|--------|
| #239 | DevHunt submission | Marked DONE, description updated |
| #240 | DevHunt account migration | Created + marked DONE |
| #223 | Dev.to comment #23 | Due date fixed → Apr 8 |
| #224 | Dev.to comment #18 | Due date fixed → Apr 10 |
| #225 | Dev.to comment #22 | Due date fixed → Apr 14 |
| #226 | Dev.to comment #21 | Due date fixed → Apr 16 |
| #233 | Show HN | Description updated with full draft |
| #241 | Pulse dashboard update | Created, recurring every 3 days |

Google Calendar events modified:

| Event | Action |
|-------|--------|
| DevHunt submission (Apr 6) | Moved to correct date, marked DONE |
| DevHunt Launch Week (Apr 7-14) | NEW — red color |
| Show HN (Apr 17) | Updated with full draft + instructions |
| Pulse dashboard update (Apr 9-24) | NEW — recurring every 3 days, peacock color |

## Files Created/Modified

| File | Action |
|------|--------|
| `crex-launch-session-state.md` | Updated: DevHunt resolved, Show HN draft, Pulse section, YouTube in profiles |
| `pulse-dashboard-prompt.md` | NEW — 375 lines, full dashboard prompt |
| `demo.mp4` | NEW — 293KB, converted from GIF |

## Gmail Drafts

- 3 drafts to john@marsx.dev (DevHunt account migration) — final version: draft r-9078015634811589929 (with txeo.msx@gmail.com)

## Pending

- `git push` from Mac Mini (no SSH from Cowork VM)
- Copy `launch-plan-obsidian.md` to Obsidian vault manually (from ~/Git/yo/cmux/docs/)
- Monitor DevHunt Launch Week starting Apr 7
- Next scheduled task: Apr 8 — Dev.to comment #23 (Vikunja #223)
- Next pulse dashboard update: Apr 9 (Vikunja #241)
