# Dev.to Comments — Drafts for crex Launch

> All drafts need Juan's review before publishing.
> Rule: each comment must feel like a genuine response to THAT article, not a copy-paste promo.

---

## #18 — "cmux: The Terminal Built for AI Coding Agents" (Neural Download)
**URL:** https://dev.to/neuraldownload/cmux-the-terminal-built-for-ai-coding-agents-3l7h
**Relevance:** HIGH — Author explicitly lists "no detach/reattach" as a limitation.

### Draft Comment:

Great writeup — the notification rings breakdown was especially useful. I've been running 4-5 agents simultaneously and that color-coding is what makes cmux usable at scale.

One thing on the "no detach/reattach" limitation you mentioned: I hit that wall hard after a crash wiped an hour of carefully arranged workspaces. So I built [cmux-resurrect](https://github.com/drolosoft/cmux-resurrect) — it saves and restores your entire layout (splits, working directories, pinned tabs, startup commands) in a single command. It also exports layouts as Markdown files you can version in git.

Not a replacement for proper detach/reattach, but it covers the "survive a crash or restart" use case today.

---

## #19 — "Calyx vs cmux: Choosing the Right Ghostty-Based Terminal" (Yuuichi Eguchi)
**URL:** https://dev.to/yuu1ch13/calyx-vs-cmux-choosing-the-right-ghostty-based-terminal-for-macos-26-28e7
**Relevance:** VERY HIGH — Author explicitly calls out cmux's lack of session persistence vs Calyx's auto-restore.

### Draft Comment:

Really thorough comparison — I didn't know Calyx had atomic file writes + crash loop detection for session restore. That's a serious edge.

On the session persistence gap you flagged for cmux: I ran into exactly that problem and built [cmux-resurrect](https://github.com/drolosoft/cmux-resurrect) to close it. `crex save` captures your full layout (splits, tabs, working directories, pinned state, startup commands) and `crex restore` brings it all back. It also has auto-save via launchd so you don't have to remember to save manually.

It doesn't match Calyx's native-level integration, but for anyone who needs cmux for the agent orchestration features and also wants session persistence — it bridges that gap.

---

## #20 — "Agent Orchestrator vs T3 Code vs cmux: Hands-On Comparison" (Dhruv Sharma)
**URL:** https://dev.to/illegalcall/agent-orchestrator-vs-t3-code-vs-openai-symphony-vs-cmux-hands-on-comparison-1ba8
**Relevance:** LOW — No mention of session persistence. The article treats cmux as a terminal layer, not as something needing session recovery.

### RECOMMENDATION: SKIP
The article is about orchestration architecture, not terminal workflows. Dropping a session persistence comment here would feel forced and off-topic. No natural entry point.

---

## #21 — "Claude Code Lost My 4-Hour Session. Here's the $0 Fix" (decker / @gonewx)
**URL:** https://dev.to/gonewx/claude-code-lost-my-4-hour-session-heres-the-0-fix-that-actually-works-24h6
**Relevance:** MEDIUM — About Claude Code context loss (conversation history), not terminal session loss. Different layer of the problem. But readers of this article care about session resilience.

### Draft Comment:

Nice approach — backing up the Claude session files is smart for recovering conversation context.

I hit a related but different angle of the same frustration: losing the *terminal* workspace itself. After a cmux crash, all my splits, directories, and running commands were gone even if the Claude conversation could be restored. So I built [cmux-resurrect](https://github.com/drolosoft/cmux-resurrect) — it snapshots and restores your entire cmux layout. Pairs well with your session backup approach: you recover the Claude context, crex recovers the workspace around it.

---

## #22 — "How I Solved Claude Code's Context Loss Problem" (Kaz / @kaz123)
**URL:** https://dev.to/kaz123/how-i-solved-claude-codes-context-loss-problem-with-a-lightweight-session-manager-265d
**Relevance:** HIGH — Author built "claunch" which uses tmux for persistence. Direct conceptual parallel to crex but for a different tool.

### Draft Comment:

claunch is a clever approach — using tmux as the persistence layer for Claude sessions makes a lot of sense. The project name auto-detection from CWD is a nice touch.

For anyone using cmux instead of tmux for their agent workflows, I built something in the same spirit: [cmux-resurrect](https://github.com/drolosoft/cmux-resurrect). It saves/restores your entire cmux layout — splits, tabs, working directories, startup commands. It also exports layouts as Markdown Blueprints you can version in git, which is handy when you have a standard workspace you want to spin up across machines.

Different terminal, same problem, same philosophy: your workspace should survive restarts.

---

## #23 — "A lot of terminal setups look productive... until you restart your machine" (Oleh Klokov)
**URL:** https://dev.to/ssh_exe/a-lot-of-terminal-setups-look-productive-until-you-restart-your-machine-onh
**Relevance:** VERY HIGH — The entire article IS the problem crex solves, but for tmux+Ghostty. Perfect fit.

### Draft Comment:

This resonated hard — "tabs are a view, they don't encode recovery" is exactly the problem.

Your tmux + Ghostty stack is solid for persistence. For anyone who's gone the cmux route instead (especially for agent orchestration), I built something that solves the same problem there: [cmux-resurrect](https://github.com/drolosoft/cmux-resurrect). One command saves your entire layout — splits, tabs, working directories, startup commands — and one command brings it back. It also exports layouts as human-readable Markdown files, so you can version your workspace setup in git and reproduce it on any machine.

Same philosophy as your approach: make the setup reproducible, not just pretty.

---

## SUMMARY

| # | Article | Action | Why |
|---|---------|--------|-----|
| 18 | cmux for AI Agents | COMMENT | Author explicitly lists "no detach/reattach" as limitation |
| 19 | Calyx vs cmux | COMMENT | Author calls out cmux's session persistence gap vs Calyx |
| 20 | Agent Orchestrator comparison | SKIP | No session persistence angle, would feel forced |
| 21 | Claude Code lost session | COMMENT | Related problem, different layer — but audience cares |
| 22 | Claude Code context loss | COMMENT | Author built tmux-based persistence tool — direct parallel |
| 23 | Terminal setups until restart | COMMENT | Entire article IS the problem crex solves |

**5 comments, 1 skip.**
