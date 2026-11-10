#!/usr/bin/env bash
# check-ai-tools.sh — check the installed AI tools against the flags crex types.
#
# crex restores an AI pane by typing the tool's resume command, plus its
# auto-accept flag when auto_accept is on. A new release of a tool can drop
# or rename either one, and the pane then fails to start. For every tool in
# TOOLS below whose binary is on PATH, this script runs its --version and
# --help (and the help of the subcommand that owns the flag) and reports
# whether the words crex types are still listed.
#
# This is the manual check before a release, on whatever tools the machine
# has. The Go test in internal/detect/docs_test.go is the automatic one: it
# keeps docs/ai-tools.md in line with the registry.
#
# It is read-only for this repository and never starts a session: every
# command gets --version or --help, a timeout and stdin from /dev/null. Some
# tools still write their own config or log files when they run.
#
# Usage:
#   scripts/check-ai-tools.sh
#
# Exit codes:
#   0 — every flag that could be checked is still listed
#   1 — an installed tool no longer lists a flag crex types
set -euo pipefail

# One row per tool, in the order of `registry` in internal/detect/sessions.go,
# so the two can be read side by side. Columns, separated by "|":
#   tool         the auto_accept name (registry Name)
#   binary       what to look for on PATH (Cursor is looked up as cursor-agent,
#                since Grok also installs an `agent`)
#   flag         the auto-accept flag crex types (registry AutoAcceptFlag),
#                or "none"; only its first word is searched for
#   flag help    the subcommand whose --help lists the flag, "-" for the top
#   resume       the words of the resume command to look for, comma separated
#   resume help  the subcommand whose --help lists them, "-" for the top
#   hidden       words the tool accepts but leaves out of --help, comma
#                separated; "signin" means its help shows nothing before
#                signing in
TOOLS='
claude      | claude       | --dangerously-skip-permissions             | -           | --resume              | -           | -
opencode    | opencode     | --auto                                     | -           | --session             | -           | -
codex       | codex        | --dangerously-bypass-approvals-and-sandbox | -           | SESSION_ID            | resume      | -
amp         | amp          | --dangerously-allow-all                    | -           | continue              | threads     | --dangerously-allow-all
gemini      | gemini       | --approval-mode=yolo                       | -           | --resume              | -           | -
copilot     | copilot      | --allow-all                                | -           | --resume              | -           | -
grok        | grok         | --always-approve                           | -           | --resume              | -           | -
cursor      | cursor-agent | --force                                    | -           | --resume              | -           | -
aider       | aider        | --yes-always                               | -           | --restore-chat-history| -           | -
pi          | pi           | none                                       | -           | --session             | -           | -
rovo        | acli         | --yolo                                     | rovodev run | --restore             | rovodev run | signin
hermes      | hermes       | --yolo                                     | -           | --resume              | -           | -
codebuddy   | codebuddy    | --dangerously-skip-permissions             | -           | --resume              | -           | -
factory     | droid        | --auto high                                | -           | --resume              | -           | -
qoder       | qoder        | --yolo                                     | -           | --resume              | -           | --yolo
qwen        | qwen         | --yolo                                     | -           | --resume              | -           | -
goose       | goose        | none                                       | -           | --resume,--session-id | session     | -
crush       | crush        | --yolo                                     | -           | --session             | -           | -
kiro        | kiro-cli     | --trust-all-tools                          | chat        | --resume-id           | chat        | -
antigravity | agy          | --dangerously-skip-permissions             | -           | --conversation        | -           | -
cline       | cline        | none                                       | -           | --id                  | -           | -
vibe        | vibe         | --auto-approve                             | -           | --resume              | -           | -
omp         | omp          | --yolo                                     | -           | --resume              | -           | --yolo
kimi        | kimi         | --auto                                     | -           | --session             | -           | -
'

# Seconds any one command may run. A --help that takes longer is waiting
# for something it should not, and is stopped.
TIMEOUT_SECONDS=10

FOUND=0
MISSING_TOOLS=0
REGRESSIONS=0
UNCHECKED=0

# trim prints its argument without the spaces around it.
trim() {
    local value="$1"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s' "$value"
}

# run_limited runs a command with stdin from /dev/null and stops it after
# TIMEOUT_SECONDS. macOS has no timeout(1), so perl's alarm stands in when
# neither timeout nor gtimeout is installed.
run_limited() {
    if command -v timeout >/dev/null 2>&1; then
        timeout "$TIMEOUT_SECONDS" "$@" </dev/null
    elif command -v gtimeout >/dev/null 2>&1; then
        gtimeout "$TIMEOUT_SECONDS" "$@" </dev/null
    else
        perl -e 'alarm shift @ARGV; exec @ARGV or exit 127' "$TIMEOUT_SECONDS" "$@" </dev/null
    fi
}

# help_text prints the --help of a binary, or of one of its subcommands
# ("-" for the top level), with stderr folded in. The exit status of the
# help command goes to the global HELP_STATUS.
help_text() {
    local binary="$1"
    local subcommand="$2"
    local words=()

    if [ "$subcommand" != "-" ]; then
        # The subcommand is split into words on purpose: "rovodev run".
        # shellcheck disable=SC2206
        words=($subcommand)
    fi

    HELP_STATUS=0
    run_limited "$binary" ${words[@]+"${words[@]}"} --help 2>&1 || HELP_STATUS=$?
}

# listed reports whether a word appears in a help text as a whole option,
# so --auto is not found inside --auto-approve.
listed() {
    local word="$1"
    local text="$2"
    printf '%s\n' "$text" | grep -Eq -- "(^|[^A-Za-z0-9_-])${word}([^A-Za-z0-9_-]|$)"
}

# in_list reports whether a word is one of the comma-separated items.
in_list() {
    local word="$1"
    local list="$2"
    case ",$list," in
        *",$word,"*) return 0 ;;
    esac
    return 1
}

# missing_hint prints what is known about a flag missing from one tool, so a
# known gap reads as advice instead of a bare failure.
missing_hint() {
    case "$1" in
        opencode) echo "             (releases without --auto, such as 1.14.48, refuse to start with it: leave opencode out of auto_accept)" ;;
    esac
}

# check_word prints one result line for a word crex types, and counts it.
#   $1 tool, $2 label, $3 word, $4 help text, $5 help exit status, $6 hidden
check_word() {
    local tool="$1"
    local label="$2"
    local word="$3"
    local text="$4"
    local status="$5"
    local hidden="$6"

    if listed "$word" "$text"; then
        printf '             %-12s %s: listed\n' "$label" "$word"
        return
    fi

    if [ "$hidden" = "signin" ]; then
        printf '             %-12s %s: not listed (the tool shows its options only after signing in)\n' "$label" "$word"
        UNCHECKED=$((UNCHECKED + 1))
        return
    fi
    if in_list "$word" "$hidden"; then
        printf '             %-12s %s: not listed in --help (known hidden flag)\n' "$label" "$word"
        return
    fi

    # A help that failed or timed out says nothing about the flag.
    if [ "$status" -ne 0 ]; then
        printf '             %-12s %s: not checked, --help exited with %s\n' "$label" "$word" "$status"
        UNCHECKED=$((UNCHECKED + 1))
        return
    fi

    printf '             %-12s %s: MISSING\n' "$label" "$word"
    missing_hint "$tool"
    REGRESSIONS=$((REGRESSIONS + 1))
}

# check_tool reports on one row of TOOLS.
check_tool() {
    local tool="$1"
    local binary="$2"
    local flag="$3"
    local flag_help="$4"
    local resume="$5"
    local resume_help="$6"
    local hidden="$7"

    if ! command -v "$binary" >/dev/null 2>&1; then
        printf '%-12s not installed (%s)\n' "$tool" "$binary"
        MISSING_TOOLS=$((MISSING_TOOLS + 1))
        return
    fi
    FOUND=$((FOUND + 1))

    local version
    local version_status=0
    version=$(run_limited "$binary" --version 2>&1) || version_status=$?
    version=$(trim "$(printf '%s\n' "$version" | grep -m 1 -v '^[[:space:]]*$' || true)")
    if [ "$version_status" -ne 0 ]; then
        version="version unknown (--version exited with $version_status)"
    fi
    printf '%-12s %s\n' "$tool" "${version:0:70}"

    # Most tools list both words in the same help, so it is read once.
    local flag_text
    local flag_status
    flag_text=$(help_text "$binary" "$flag_help"; echo "status=$HELP_STATUS")
    flag_status="${flag_text##*status=}"
    flag_text="${flag_text%status=*}"

    local resume_text="$flag_text"
    local resume_status="$flag_status"
    if [ "$resume_help" != "$flag_help" ]; then
        resume_text=$(help_text "$binary" "$resume_help"; echo "status=$HELP_STATUS")
        resume_status="${resume_text##*status=}"
        resume_text="${resume_text%status=*}"
    fi

    if [ "$flag" = "none" ]; then
        printf '             %-12s none (crex adds no flag)\n' "auto-accept"
    else
        # Only the option itself is searched for: --auto of "--auto high",
        # --approval-mode of "--approval-mode=yolo".
        check_word "$tool" "auto-accept" "${flag%%[ =]*}" "$flag_text" "$flag_status" "$hidden"
    fi

    local word
    local old_ifs="$IFS"
    IFS=','
    for word in $resume; do
        IFS="$old_ifs"
        check_word "$tool" "resume" "$word" "$resume_text" "$resume_status" "$hidden"
    done
    IFS="$old_ifs"
}

while IFS='|' read -r tool binary flag flag_help resume resume_help hidden; do
    tool=$(trim "$tool")
    [ -z "$tool" ] && continue

    check_tool "$tool" "$(trim "$binary")" "$(trim "$flag")" "$(trim "$flag_help")" \
        "$(trim "$resume")" "$(trim "$resume_help")" "$(trim "$hidden")"
done <<< "$TOOLS"

echo
echo "$FOUND installed, $MISSING_TOOLS not installed, $UNCHECKED not checked, $REGRESSIONS missing"
if [ "$REGRESSIONS" -gt 0 ]; then
    echo "✗ an installed tool no longer lists a flag crex types (see MISSING above)" >&2
    exit 1
fi
exit 0
