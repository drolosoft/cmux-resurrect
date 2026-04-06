# crex Launch Plan — April 5, 2026

> Master document: every URL, every target, every thread where crex needs to appear.

---

## The Opportunity

cmux (12.5K stars, 874 forks) launched ~Feb 2026 and has **no session persistence tool**. People are actively asking for one. crex is the first and only tmux-resurrect equivalent for cmux. Timing is perfect.

---

## TIER 1 — Direct Pain Point Threads (Comment with crex as the solution)

These are threads where people are **actively asking** for what crex does. Highest impact.

### GitHub Issues — manaflow-ai/cmux

| # | Issue | Problem | crex solves? | URL |
|---|-------|---------|:---:|-----|
| 1 | #1984 | App update kills ALL terminal sessions — no persistence across Sparkle restart | ✅ YES | https://github.com/manaflow-ai/cmux/issues/1984 |
| 2 | #2125 | Session restore does not recover working directories (all reopen at $HOME) | ✅ YES | https://github.com/manaflow-ai/cmux/issues/2125 |
| 3 | #1664 | First-class SSH workspaces with session persistence | ✅ Partial | https://github.com/manaflow-ai/cmux/issues/1664 |
| 4 | #1781 | Closing window destroys all terminals and sessions | ✅ YES | https://github.com/manaflow-ai/cmux/issues/1781 |
| 5 | #399 | App launches to frozen blank state from session restore race | ✅ Workaround | https://github.com/manaflow-ai/cmux/issues/399 |
| 6 | #2309 | 0.63.0 crashes on launch even after clearing saved state | ✅ Recovery | https://github.com/manaflow-ai/cmux/issues/2309 |
| 7 | #432 | Crash: use-after-free after wake from sleep | ✅ Recovery | https://github.com/manaflow-ai/cmux/issues/432 |
| 8 | #1789 | Terminal surfaces go blank when switching workspaces | 🔶 Related | https://github.com/manaflow-ai/cmux/issues/1789 |

### GitHub Issues — anthropics/claude-code

| # | Issue | Problem | crex solves? | URL |
|---|-------|---------|:---:|-----|
| 9 | #34829 | Expose session/conversation ID to enable terminal session restore | ✅ Enables | https://github.com/anthropics/claude-code/issues/34829 |
| 10 | #27463 | Claude Desktop sessions lost on restart | 🔶 Related | https://github.com/anthropics/claude-code/issues/27463 |
| 11 | #26452 | Session disappeared after logout/restart | 🔶 Related | https://github.com/anthropics/claude-code/issues/26452 |
| 12 | #9581 | All session data in ~/.claude lost after logout/login | 🔶 Related | https://github.com/anthropics/claude-code/issues/9581 |
| 13 | #36320 | Auto-resume after usage limit window resets | 🔶 Related | https://github.com/anthropics/claude-code/issues/36320 |

### Ghostty Discussions

| # | Thread | Problem | crex solves? | URL |
|---|--------|---------|:---:|-----|
| 14 | Ghostty #3358 | Suggestion: Session manager for Ghostty | ✅ Concept | https://github.com/ghostty-org/ghostty/discussions/3358 |
| 15 | gtab | Save and restore Ghostty terminal tab layouts | ✅ Same space | https://github.com/Franvy/gtab |

### Zellij (modern multiplexer — same pain point)

| # | Thread | Problem | URL |
|---|--------|---------|-----|
| 16 | Zellij #1468 | Is there a way to persist session through reboot? | https://github.com/zellij-org/zellij/issues/1468 |
| 17 | Zellij docs | Session Resurrection documentation | https://zellij.dev/documentation/session-resurrection.html |

---

## TIER 2 — Blog Articles & Reviews (Comment or write response posts)

These articles discuss cmux limitations around session persistence. Perfect for commenting with crex.

| # | Article | Platform | URL |
|---|---------|----------|-----|
| 18 | "cmux: The Terminal Built for AI Coding Agents" | Dev.to | https://dev.to/neuraldownload/cmux-the-terminal-built-for-ai-coding-agents-3l7h |
| 19 | "Calyx vs cmux: Choosing the Right Ghostty-Based Terminal" | Dev.to | https://dev.to/yuu1ch13/calyx-vs-cmux-choosing-the-right-ghostty-based-terminal-for-macos-26-28e7 |
| 20 | "Agent Orchestrator vs T3 Code vs cmux: Hands-On Comparison" | Dev.to | https://dev.to/illegalcall/agent-orchestrator-vs-t3-code-vs-openai-symphony-vs-cmux-hands-on-comparison-1ba8 |
| 21 | "Claude Code Lost My 4-Hour Session. Here's the $0 Fix" | Dev.to | https://dev.to/gonewx/claude-code-lost-my-4-hour-session-heres-the-0-fix-that-actually-works-24h6 |
| 22 | "How I Solved Claude Code's Context Loss Problem" | Dev.to | https://dev.to/kaz123/how-i-solved-claude-codes-context-loss-problem-with-a-lightweight-session-manager-265d |
| 23 | "A lot of terminal setups look productive… until you restart" | Dev.to | https://dev.to/ssh_exe/a-lot-of-terminal-setups-look-productive-until-you-restart-your-machine-onh |
| 24 | "I tried to make it automatically run claude /resume when cmux restarts" | DevelopersIO | https://dev.classmethod.jp/en/articles/cmux-auto-resume-claude-code/ |
| 25 | "I Quit tmux. Here's What I Built Instead" | Medium | https://medium.com/@arthurpro/i-quit-tmux-heres-what-i-built-instead-5feda11829de |
| 26 | "Why Your Claude Code Sessions Keep Failing" | Medium | https://0xhagen.medium.com/why-your-claude-code-sessions-keep-failing-and-how-to-fix-it-62d5a4229eaf |
| 27 | "cmux Review (2026): macOS Terminal for AI Agents" | Vibe Coding App | https://vibecoding.app/blog/cmux-review |
| 28 | "cmux Complete Guide" | Gardenee Blog | https://agmazon.com/blog/articles/technology/202603/cmux-terminal-ai-guide-en.html |
| 29 | "cmux vs tmux — Agent Terminal vs Terminal Multiplexer" | SoloTerm | https://soloterm.com/cmux-vs-tmux |
| 30 | "Solo vs cmux" | SoloTerm | https://soloterm.com/solo-vs-cmux |
| 31 | "Replacing tmux with Ghostty" | sterba.dev | https://sterba.dev/posts/replacing-tmux/ |
| 32 | "Why We Built Claude Remote on tmux: Session Persistence" | clauderc.com | https://clauderc.com/blog/2026-02-28-tmux-architecture-and-session-persistence/ |

---

## TIER 3 — High-Visibility Launch Platforms (New posts/submissions)

### A. daily.dev (TODAY — first launch)
- **Format**: New Post with markdown, link to repo
- **Profile**: https://daily.dev/es (Juan's profile)
- **Approach**: Short, value-focused post. Problem → Solution → Install

### B. Hacker News — Show HN
- **Title**: "Show HN: crex – tmux-resurrect for cmux (session persistence)"
- **URL**: https://news.ycombinator.com/submit
- **Context**: cmux hit #2 on HN twice ([thread 1](https://news.ycombinator.com/item?id=45596024), [thread 2](https://news.ycombinator.com/item?id=47079718)). Audience is primed.
- **Also relevant HN threads to reference**:
  - https://news.ycombinator.com/item?id=47008732 (Cmux: Tmux for Claude Code)
  - https://news.ycombinator.com/item?id=47223871 (cmux sessions tied 1-to-1)
  - https://news.ycombinator.com/item?id=47468901 (cmux vs other tools)
  - https://news.ycombinator.com/item?id=10219003 (tmux-resurrect original HN thread)

### C. Reddit
- **r/commandline** — Primary target
- **r/terminal** — Terminal enthusiasts
- **r/golang** — Go community (crex is in Go)
- **r/programming** — General dev audience
- **r/opensource** — Open source community
- **Guide**: https://tereza-tizkova.medium.com/best-subreddits-for-sharing-your-project-517c433442f9

### D. Dev.to — Full article
- **Format**: Tutorial-style article with demo GIF
- **Angle**: "I built tmux-resurrect for cmux — here's why and how"

### E. Product Hunt
- **Category**: Engineering & Development Tools / Open Source
- **Guide**: https://www.producthunt.com/launch
- **Tips**: https://syntaxhut.tech/blog/best-product-hunt-launch-tips-2026
- **Reference**: https://github.com/fmerian/awesome-product-hunt

### F. Lobste.rs
- **Relevant thread already**: https://lobste.rs/s/fvdh2d/zmx_session_persistence_for_terminal
- **Submit**: New story with tag `terminal`

### G. DevHunt
- **Specialized for dev tools**: Requires advance submission
- **GitHub verification for authentic feedback**

### H. Terminal Trove ⭐
- **URL**: https://terminaltrove.com/post/
- **Perfect fit**: Curated directory of TUI/CLI tools — crex is exactly this
- **Form requires**: name, tagline (~100 chars), description (250-300 chars), standout features (150-300 chars), who is it for (150-250 chars), language (Go), license (MIT), preview image (PNG/GIF/MP4), categories, install instructions
- **Assets ready**: demo.gif, import-success.png, brew install command

---

## TIER 4 — Related Tools & Repos (Create awareness)

| # | Repo/Tool | Relationship | URL |
|---|-----------|-------------|-----|
| 33 | claude-auto-resume | Workaround crex replaces | https://github.com/terryso/claude-auto-resume |
| 34 | hopchouinard/cmux-plugin | Only other cmux plugin | https://github.com/hopchouinard/cmux-plugin |
| 35 | tmux-resurrect | Spiritual predecessor | https://github.com/tmux-plugins/tmux-resurrect |
| 36 | tmux-continuum | Auto-save inspiration | https://github.com/tmux-plugins/tmux-continuum |
| 37 | tmuxp (has resurrect alternative request) | Cross-community | https://github.com/tmux-python/tmuxp/issues/522 |
| 38 | crabmux | tmux helper with session features | https://github.com/madhavajay/crabmux |
| 39 | cmux-windows | Windows variant | https://github.com/mkurman/cmux-windows |
| 40 | cmux official community | Discord | https://cmux.com/community |

---

## TIER 5 — Comparison & Review Sites

| # | Site | URL |
|---|------|-----|
| 41 | Product Hunt alternatives page | https://www.producthunt.com/products/cmux/alternatives |
| 42 | LobHub skills marketplace | https://lobehub.com/skills/neversight-learn-skills.dev-cmux |
| 43 | DeepWiki workspace lifecycle docs | https://deepwiki.com/coder/cmux/4.1-workspace-lifecycle |
| 44 | Better Stack cmux guide | https://betterstack.com/community/guides/ai/cmux-terminal/ |

---

## Launch Execution Order

1. ✅ **TODAY** — daily.dev post (Juan's profile)
2. 📝 **TODAY** — Comment on cmux GitHub issues #1984, #2125, #1781
3. 📝 **TODAY/TOMORROW** — Show HN submission
4. 📝 **THIS WEEK** — Reddit posts (r/commandline, r/golang, r/terminal)
5. 📝 **THIS WEEK** — Dev.to full article
6. 📝 **THIS WEEK** — Comment on Dev.to articles (#18-#23)
7. 📝 **NEXT WEEK** — Product Hunt launch
8. 📝 **NEXT WEEK** — Lobste.rs submission
9. 📝 **ONGOING** — Comment on Medium articles, comparison sites

---

## Key Messaging Points

- **"tmux-resurrect proved session persistence is essential. crex brings it to cmux."**
- **One command saves. One command restores.** Workspaces, splits, CWDs, pinned state, startup commands.
- **Workspace Blueprints**: Define your ideal setup in Markdown (Obsidian-compatible), version it, share it with your team.
- **Born from a real crash**: A crashed cmux session took an hour of carefully arranged workspaces with it.
- First and only session persistence tool for the cmux ecosystem.
- MIT licensed, pure Go, zero dependencies, Homebrew installable.

---

*Generated: 2026-04-05 | Forged by [Drolosoft](https://drolosoft.com)*
