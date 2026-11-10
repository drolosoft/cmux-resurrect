[Home](../README.md) > AI Tools

# 🤖 AI Tools

When you save a layout, crex looks for AI coding tools running in its panes and records, for each one, the command that resumes its conversation. When you restore the layout, each pane runs that command in its folder, so the tool opens the same session again.

## How far each tool is tested

Each tool has a level that says how it was checked against the real program:

- **A**: a real conversation was saved, the pane closed, the layout restored, and the same conversation came back, on both cmux and Ghostty.
- **B**: the real tool was installed and run without signing in. Its help lists the resume option and the auto-accept flag crex types, in the place crex puts them (or, for a flag the tool hides from its help, the tool accepts it). crex's process scan recognised it running, and the title it sets in the terminal was checked. The format of its session files comes from the tool's source code or published package, not from a real conversation.
- **C**: built from the tool's documentation and test fixtures only.

"Tested against" is the version those checks ran on. A newer release can change a flag or move its session files; [How to check your own machine](#how-to-check-your-own-machine) shows how to look.

## Supported tools

The `auto_accept` name is what layouts store and what the [`auto_accept`](configuration.md#auto-accept-for-ai-agents) setting takes. "Starts with" is what a restore types to start the tool, and the auto-accept flag goes right after it.

| Tool | `auto_accept` name | Starts with | Resumes with | Auto-accept flag | Tested against | Level | Notes |
|------|--------------------|-------------|--------------|------------------|----------------|-------|-------|
| Claude Code | `claude` | `claude` | `claude --resume <id>` | `--dangerously-skip-permissions` | 2.1.x | A | |
| OpenCode | `opencode` | `opencode` | `opencode --session <id>` | `--auto` | 1.14.48 | A | Releases without `--auto` (1.14.48, for example) reject the flag and do not start with it: leave `opencode` out of `auto_accept` on those. |
| Codex | `codex` | `codex` | `codex resume <id>` | `--dangerously-bypass-approvals-and-sandbox` | 0.167.0 | B | No conversation was resumed for real. The session reader was also checked against real Codex 0.128 files. |
| Amp | `amp` | `amp` | `amp threads continue <id>` | `--dangerously-allow-all` | 0.0.1794268864-g13d0b0 | B | The flag is not listed in `amp --help`, but Amp accepts it. |
| Gemini CLI | `gemini` | `gemini` | `gemini --resume <id>` | `--approval-mode=yolo` | 0.67.0 | B | |
| Copilot CLI | `copilot` | `copilot` | `copilot --resume <id>` | `--allow-all` | 1.0.108 | B | |
| Grok | `grok` | `grok` | `grok --resume <id>`, `grok --session <id>` or `grok --continue` | `--always-approve` | 1.0.77 (Grok Build) | B | Grok Build resumes with `--resume`, the community grok-cli with `--session`. When Grok Build is in use but nothing can be read for the folder, `grok --continue` lets Grok choose. |
| Cursor CLI | `cursor` | `agent` | `agent --resume <chat id>` | `--force` | 2026.11.05-86047df | B | An `agent` process counts as Cursor only when its path goes through `cursor-agent`. |
| Aider | `aider` | `aider` | `aider --restore-chat-history` | `--yes-always` | 0.86.2 | B | Aider has no sessions: crex restores its chat history file (`.aider.chat.history.md`) when it is not empty. |
| Pi | `pi` | `pi` | `pi --session <id>` | none | 1.5.1 | B | No flag: Pi does not ask before running tools. |
| Rovo Dev | `rovo` | `acli rovodev run` | `acli rovodev run --restore <id>` | `--yolo` | acli 1.3.40 (launcher only) | C | The Rovo Dev engine downloads only after signing in, so only the `acli` launcher was seen. |
| Hermes Agent | `hermes` | `hermes` | `hermes --resume <id>` | `--yolo` | 0.23.1 | B | The flag is added only on a fresh start: Hermes keeps the approval mode a session was created with. The detector was also run against a real local Hermes database and terminal file. |
| CodeBuddy Code | `codebuddy` | `codebuddy` | `codebuddy --resume <id>` | `--dangerously-skip-permissions` | 2.182.0 | B | Its help says HIGH and CRITICAL actions still ask with this flag. |
| Factory Droid | `factory` | `droid` | `droid --resume <id>` | `--auto high` | 1.7.0 | B | `--auto high` is the highest level of the interactive `droid`; `--skip-permissions-unsafe` exists only on `droid exec`. |
| Qoder CLI | `qoder` | `qoder` | `qoder --resume <id>` | `--yolo` | 1.1.86 | B | The flag is not listed in `qoder --help`, but Qoder accepts it. |
| Qwen Code | `qwen` | `qwen` | `qwen --resume <id>` | `--yolo` | 0.27.1 | B | |
| Goose | `goose` | `goose` | `goose session --resume --session-id <id>` | none | 1.58.0 | B | No flag: Goose approves tool calls by default. |
| Crush | `crush` | `crush` | `crush --session <id>` | `--yolo` | 0.100.0 | B | Crush keeps its sessions in `.crush/` inside the project folder. |
| Kiro CLI | `kiro` | `kiro-cli` | `kiro-cli chat --resume-id <id>` | `--trust-all-tools` | 3.1.0 | B | The flag goes after `chat`: `kiro-cli chat --trust-all-tools --resume-id <id>`. |
| Antigravity CLI | `antigravity` | `agy` | `agy --conversation <id>` | `--dangerously-skip-permissions` | 1.5.3 | B | Antigravity leaves the terminal title alone unless configured, so crex knows it by its process. |
| Cline CLI | `cline` | `cline` | `cline --id <id>` | none | 3.0.74 | B | No flag: Cline approves tool calls by default. Its `--yolo` is a separate mode for unattended runs. |
| Mistral Vibe | `vibe` | `vibe` | `vibe --resume <id>` | `--auto-approve` | 3.1.1 | B | |
| oh-my-pi | `omp` | `omp` | `omp --resume <id>` | `--yolo` | 18.18.6 | B | The flag is not listed in `omp --help`, but oh-my-pi accepts it. |
| Kimi Code | `kimi` | `kimi` | `kimi --session <id>` | `--auto` | 2.5.2 | B | `--auto` never asks; Kimi's `--yolo` still asks before risky actions. |

A few things apply to many rows:

- Copilot CLI, Codex, Gemini CLI and others resume only sessions that contain a conversation. A pane where nothing was asked yet has nothing to resume.
- When crex finds no session for a pane's folder, the pane gets no resume command. From Cursor CLI down the table, the tool is then started again with its bare command.
- When a tool keeps its data in a folder of your choice (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `GROK_HOME` and similar), the saved resume command sets that variable first, so the session opens from the same place.

## How to check your own machine

`scripts/check-ai-tools.sh` checks the tools installed on your machine against what crex types:

```sh
scripts/check-ai-tools.sh
```

For each tool in its table it looks for the binary on `PATH`. When it is there, the script runs `--version` and `--help` (and the help of the subcommand that owns the flag, such as `kiro-cli chat --help` or `amp threads --help`), each with a timeout and no input, and reports whether the auto-accept flag and the resume option crex types are still listed. It never starts a session. Some tools write their own config or log files even for `--help`.

The script exits with 0 when every flag it could check is still there, and with 1 when a flag crex types is missing from an installed tool's help. A flag that a tool hides from its help on purpose (Amp, Qoder and oh-my-pi above) is reported as "not listed in --help (known hidden flag)", not as a failure.

## Adding a tool

Each tool is one entry in `registry` in [`internal/detect/sessions.go`](../internal/detect/sessions.go), with its detector in `internal/detect/<tool>.go`. A new tool also needs a row in this page, in the `wantRegistry` table of `internal/detect/sessions_test.go` and in the table at the top of `scripts/check-ai-tools.sh`. `go test ./internal/detect/` fails when this page and the registry disagree on a tool's name, launch command or auto-accept flag.

---

See also: [Configuration](configuration.md) | [Commands](commands.md)
