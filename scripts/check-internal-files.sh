#!/usr/bin/env bash
# check-internal-files.sh — refuse to commit/publish internal working files.
#
# This repository is public. Working material (plans, specs, prompts,
# launch drafts, daily reports, AI-session commands, marketing notes) must
# never be tracked — once pushed it lives in every fork's history forever.
# .gitignore already excludes these paths, but `git add -f`, a renamed
# folder, or a stray file at the root has slipped past it before. This is
# the hard stop: it runs as a pre-commit hook and in CI.
#
# Usage:
#   scripts/check-internal-files.sh            # check the staged index
#   scripts/check-internal-files.sh --tree     # check everything tracked
#   scripts/check-internal-files.sh --files a b c   # check explicit paths (tests)
set -euo pipefail

# Path patterns (extended regex, matched against the repo-relative path).
# Keep this list boring and explicit; false negatives are worse than a
# rare false positive that a human reviews.
BLOCKED_PATTERNS=(
  '^\.claude/'                       # AI session config, custom commands
  '^\.superpowers/'                  # skill workspace
  '^CLAUDE\.md$'                     # AI project instructions
  '^docs/superpowers/'               # specs & plans
  '^docs/prompts/'                   # prompt drafts
  '^docs/multi-backend/'             # internal research & prompt files
  '^reports/'                        # daily/marketing reports
  '^outputs/'                        # generated artefacts
  '^product-launch'                  # launch workspace / eval reviews
  '^claudemap-.*\.json$'             # ClaudeMap artefacts
  '(^|/)[^/]*-prompt\.md$'           # any *-prompt.md
  '(^|/)[^/]*-drafts?\.md$'          # any *-draft.md / *-drafts.md
  '(^|/)[^/]*-plan\.md$'             # any *-plan.md at any depth
  '(^|/)[^/]*session-state.*\.md$'   # session state notes
  '(^|/)daily[_-]report.*'           # daily reports anywhere
  '(^|/)changelog-[0-9]{4}-[0-9]{2}-[0-9]{2}\.md$'  # dated internal changelogs (public one is CHANGELOG.md)
  '\.plugin$'                        # exported tool bundles
)

# Root-level Markdown allow-list. Working notes tend to be dropped at the
# repo root with ad-hoc names (launch drafts, comment drafts, session
# notes); the public root only ever needs these documents. Anything else
# ending in .md at the root is refused.
ROOT_MD_ALLOWED=(
  README.md CHANGELOG.md ARCHITECTURE.md CONTRIBUTING.md
  SECURITY.md RUNBOOK.md LICENSE.md CODE_OF_CONDUCT.md
)

# Explicit allow-list for paths that would otherwise match but are public
# on purpose. Add here only with a reason.
ALLOWED=(
  'docs/template-authoring.md'       # ends in -authoring, not -plan; listed for clarity
)

mode="${1:---staged}"
case "$mode" in
  --staged) files=$(git diff --cached --name-only --diff-filter=ACR) ;;
  --tree)   files=$(git ls-files) ;;
  --files)  shift; files=$(printf '%s\n' "$@") ;;
  *) echo "usage: $0 [--staged|--tree|--files <paths...>]" >&2; exit 2 ;;
esac

bad=()
while IFS= read -r f; do
  [ -z "$f" ] && continue
  for a in "${ALLOWED[@]}"; do [ "$f" = "$a" ] && continue 2; done
  for p in "${BLOCKED_PATTERNS[@]}"; do
    if [[ "$f" =~ $p ]]; then bad+=("$f  (matches: $p)"); continue 2; fi
  done
  # Root-level .md not in the public allow-list.
  if [[ "$f" != */* && "$f" == *.md ]]; then
    ok=0; for a in "${ROOT_MD_ALLOWED[@]}"; do [ "$f" = "$a" ] && ok=1; done
    [ $ok -eq 1 ] || bad+=("$f  (root .md not in ROOT_MD_ALLOWED)")
  fi
done <<< "$files"

if [ ${#bad[@]} -gt 0 ]; then
  echo "✗ internal working files must not be committed to this public repo:" >&2
  printf '   %s\n' "${bad[@]}" >&2
  echo "   Move them out of the tree (or under an ignored path). If one is" >&2
  echo "   genuinely public, add it to ALLOWED in scripts/check-internal-files.sh" >&2
  echo "   with a reason." >&2
  exit 1
fi
exit 0
