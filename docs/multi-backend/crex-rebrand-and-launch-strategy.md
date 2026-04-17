# crex Rebrand & Ghostty Launch Strategy

**Date:** 2026-04-17
**Decision:** Option A — keep "crex", drop "cmux-resurrect", multi-backend
**Analysis by:** Post Review Team (8-expert pipeline)

---

## 1. Positioning

**Lead with the category, not the origin.**

- DO: "Terminal workspace manager"
- DON'T: "A cmux tool that now supports Ghostty"
- Ghostty users don't care about cmux. They care about their own gap.

When posting to Ghostty communities, list Ghostty first. When posting to cmux communities, list cmux first. Everywhere else, lead with the category.

**crex vs the competition:**

| | ghostty-workspace | gtab | crex |
|---|---|---|---|
| Language | Python script | Shell script | Go binary |
| Config format | YAML | Generated AppleScript | Markdown (Obsidian-compatible) |
| Save live state | No | Yes (tabs only) | Yes (full layout) |
| Templates | No | No | 16 built-in |
| Splits | Yes | No (tabs only) | Yes |
| Shell completions | No | No | bash/zsh/fish |
| Homebrew | No | Yes | Yes |
| Dry-run preview | No | No | Yes |
| Blueprint (IaC) | No | No | Yes (bidirectional Markdown) |

The killer differentiator is **Workspace Blueprints** — define your workspace in Markdown, version it in git, share it with your team. Nobody else has this.

---

## 2. Tagline

**Primary (everywhere):**
> **crex — terminal workspace manager**

**Subtitle (where you have room):**
> for Ghostty and cmux

**README hero line (recommended):**
> **crex** — save, restore, and template your terminal workspaces.

**Show HN hook:**
> Stop rebuilding your terminal layout every morning.

---

## 3. Launch Strategy

### Phase 0: Pre-launch (1-2 weeks before)
- Comment helpfully on Ghostty workspace discussions (#9825, #8967, #3358)
- Do NOT drop links. Build presence. "I've been working on something like this" is fine.

### Phase 1: Ghostty GitHub Discussions (first)
- Post in Show and Tell category
- Lead with: save/restore (the core pain point)
- Follow with: templates as the "and also" surprise
- Include a short GIF showing save + restore on Ghostty

### Phase 2: Reddit r/commandline (1-2 days later)
- Lead with the Markdown Blueprint angle (Unix philosophy audience)
- Personal story: "I kept losing my terminal layouts after restarts..."

### Phase 3: Reddit r/ghostty (2-3 days later)
- Ghostty-specific angle
- Reference the GitHub discussion if it got traction

### Phase 4: Show HN (next Tue-Thu, 9-10am PT)
- Title: "Show HN: Crex — terminal workspace manager for Ghostty and cmux"
- No AI mentions anywhere. No emojis. No bullet lists.
- Lead with personal story. 150-200 words. Plain text.
- Link to GitHub, nothing else.

### What to lead with per community:

| Community | Lead with | Follow with |
|---|---|---|
| Ghostty Discussions | Save/restore (their pain) | Templates + Blueprints |
| r/commandline | Markdown Blueprints | Template gallery |
| r/ghostty | Ghostty-specific demo | Comparison to alternatives |
| Hacker News | Personal story | Technical choices |

### The Ghostty hook:
> "Ghostty added AppleScript in 1.3 but still has no way to save your workspace and get it back after a restart. crex does that — save your layout, restore it tomorrow, or define reusable templates in Markdown."

---

## 4. Handling the cmux Community

**Frame as growth, not a pivot:**

- BAD: "We're pivoting to Ghostty"
- BAD: "crex is now multi-backend"
- GOOD: "crex now works with Ghostty too — same tool, same commands, more terminals"

**Concrete steps:**
1. GitHub redirect: renaming `cmux-resurrect` to `crex` auto-redirects. All old links work.
2. Changelog: "crex v2.0 adds Ghostty backend support. All existing cmux functionality is unchanged."
3. Homebrew: Update formula name. Add migration note.
4. README: Keep one line: "Born in the cmux ecosystem, crex now supports multiple terminal backends."
5. DO NOT post the rename as its own announcement. Bundle it with the Ghostty launch.

---

## 5. The Corncrake Brand Story

**Verdict: Yes, but with restraint.**

**What works:**
- The name is literally the bird's call ("crex-crex" is onomatopoeia)
- The bird migrates and returns to the same ground — maps to save/restore
- In Celtic symbolism, associated with "death and resurrection"
- Obscure enough to be interesting without being forced

**How to use it:**
1. **Logo**: Minimal, geometric corncrake silhouette. Think Go gopher simplicity.
2. **README**: One sentence. "crex takes its name from the corncrake (Crex crex), a bird known for returning to the same ground year after year — much like your terminal workspaces."
3. **Website**: Expand the story. "The corncrake's binomial name is onomatopoeia for its call. We liked the resonance: a tool that calls your workspaces back."
4. **Merch/stickers**: The bird works great here.

**What NOT to do:**
- Don't put the bird in the tagline
- Don't anthropomorphize with a cartoon mascot in the README
- Don't use bird puns in CLI output

**Precedents:** Go gopher (geometric, minimal), Rust's Ferris (community-driven, never forced), Tux (introduced casually, community ran with it).

---

## 6. Name Risk Assessment

**"crex" alone is crowded in search:**
- CREx Software (commercial real estate SaaS)
- crex.com (equipment marketplace)
- octobanana/crex (regex tester, unmaintained)

**"crex terminal" or "crex cli" is clean.** No competition.

**SEO strategy:** Always pair with a qualifier. GitHub description: "Terminal workspace manager for Ghostty and cmux — save, restore, and template your layouts"

**International safety:** No profanity, slur, or awkward meaning in Spanish, Portuguese, German, French, Japanese, or Chinese. The name is safe globally.

**Trademark:** Don't try to register "CREX" in all-caps (conflicts with railroad mark). Use lowercase "crex" consistently.

---

## 7. Action Plan (Sequence)

1. Rename repo: `drolosoft/cmux-resurrect` → `drolosoft/crex`
2. README hero: "crex — save, restore, and template your terminal workspaces."
3. Tagline everywhere: "Terminal workspace manager for Ghostty and cmux"
4. Logo: Evolve toward minimal corncrake silhouette
5. Pre-launch: 1-2 weeks commenting in Ghostty discussions (no links)
6. Phase 1: Ghostty GitHub Discussions — Show and Tell
7. Phase 2: Reddit r/commandline (2 days later)
8. Phase 3: Show HN (next Tue-Thu, 9am PT)
9. Existing cmux users: Bundle rename with Ghostty announcement
10. SEO: Always "crex terminal" or "crex cli"

---

## Sources

- [Ghostty Discussion #9825](https://github.com/ghostty-org/ghostty/discussions/9825)
- [Ghostty Discussion #8967](https://github.com/ghostty-org/ghostty/discussions/8967)
- [ghostty-workspace](https://github.com/manonstreet/ghostty-workspace)
- [gtab](https://github.com/Franvy/gtab)
- [Ghostty AppleScript Docs](https://ghostty.org/docs/features/applescript)
- [CREx Software](https://crexsoftware.com/)
- [Corncrake - Wikipedia](https://en.wikipedia.org/wiki/Corn_crake)
- [Corncrake Spiritual Symbolism](https://spiritandsymbolism.com/corncrake-spiritual-meaning-symbolism-and-totem/)

*Analysis by Post Review Team — Scout, Brand Strategist, Technical Reviewer, Community Specialist, SEO Analyst, Launch Strategist, Critic, Humanizer.*
