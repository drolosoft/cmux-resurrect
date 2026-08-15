#!/usr/bin/env bash
# Tests for check-internal-files.sh. Run: bash scripts/check-internal-files_test.sh
set -u
G="$(dirname "$0")/check-internal-files.sh"
pass=0; fail=0
must_block() { if bash "$G" --files "$1" >/dev/null 2>&1; then echo "  ✗ NOT blocked: $1"; fail=$((fail+1)); else pass=$((pass+1)); fi; }
must_allow() { if bash "$G" --files "$1" >/dev/null 2>&1; then pass=$((pass+1)); else echo "  ✗ wrongly blocked: $1"; fail=$((fail+1)); fi; }

echo "▶ every file that actually leaked into history must be blocked:"
while IFS= read -r f; do must_block "$f"; done << 'LEAKED'
.claude/commands/e2e-tui.md
.claude/commands/release-crex.md
changelog-2026-04-06.md
crex-launch-plan.md
crex-launch-session-state.md
daily-dev-post-draft.md
devto-comments-drafts.md
docs/multi-backend/compatibility-audit.md
docs/multi-backend/crex-prompt-1-backend-abstraction.md
docs/multi-backend/ghostty-live-testing-prompt.md
docs/prompts/v1.6.0-drolosoft-website.md
docs/prompts/v1.6.0-post-review.md
docs/superpowers/plans/2026-05-28-crex-pop.md
docs/superpowers/specs/2026-05-28-crex-pop-design.md
drolosoft-ship-prompt.md
product-launch-eval-review.html
product-launch-workspace/iteration-1/eval-schedule-campaign/without_skill/outputs/schedule.md
pulse-dashboard-prompt.md
reports/daily_report_2026-04-10.md
txeo-tools.plugin
vault-sync-devto-comments.md
CLAUDE.md
claudemap-cache.json
LEAKED

echo "▶ legitimate public files must pass:"
while IFS= read -r f; do must_allow "$f"; done << 'PUBLIC'
README.md
CHANGELOG.md
ARCHITECTURE.md
docs/commands.md
docs/template-authoring.md
docs/tui-testing.md
internal/orchestrate/save.go
scripts/validate-demo.sh
scripts/check-internal-files.sh
testdata/layouts/my-day.toml
deploy/shell/crex.zsh
.github/workflows/ci.yml
PUBLIC

echo "▶ the whole tracked tree must be clean right now:"
if bash "$G" --tree; then pass=$((pass+1)); else echo "  ✗ tracked tree contains internal files"; fail=$((fail+1)); fi

echo "── $pass passed, $fail failed"
[ $fail -eq 0 ]
