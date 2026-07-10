package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/agentskill"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

type shellMode int

const (
	modePrompt shellMode = iota
	modeBrowse
	modeConfirm
	modeRestoreAsk
	modeRestoreSkip // second question: skip matching tabs or recreate?
)

const maxHistory = 50

// BannerCycleFn cycles the banner style and returns (newStyle, preview, error).
// preview is the rendered banner in the new style.
type BannerCycleFn func(explicit string) (string, string, error)

// ShellModel is the main Bubble Tea model for the crex interactive shell.
type ShellModel struct {
	mode       shellMode
	prompt     textinput.Model
	browse     BrowseModel
	output     *strings.Builder // per-command buffer
	lastOutput string           // rendered above the prompt
	lastItems  []Item
	history    []string
	histIdx    int
	backend    client.DetectedBackend
	store      persist.Store
	client     client.Backend
	wsFile     string
	quitting   bool
	byeMsg     string // printed to stdout after alt screen closes
	welcome    string // shown as initial lastOutput
	lastCmd    string // last dispatched command, shown as dim header

	// Banner style cycling (injected by cmd layer).
	BannerCycle      BannerCycleFn
	OnSettingChanged func(key, value string)
	bannerStyle      string // current banner style for "banner get"
	restoreMode      string // current restore mode for "settings restore-mode get"

	// Version string (injected by cmd layer for update command).
	version string

	// Tab completion
	completer     completionEngine
	tabCandidates []string         // candidates being cycled
	tabItems      []completionItem // display items for current cycle
	tabIndex      int              // current cycle position (-1 = list shown, not yet cycling)

	// Confirmation state
	confirmMsg string
	confirmFn  func()

	// Restore-ask state (mode picker before restore)
	restoreAskName    string // layout name pending restore
	restoreAskFilter  string // workspace filter pending restore
	restoreAskCursor  int    // 0 = replace, 1 = add
	restoreSkipCursor int    // 0 = skip matching, 1 = recreate all
	restoreAskHint    orchestrate.RestoreHint
}

// NewShellModel creates the interactive shell model.
func NewShellModel(store persist.Store, cl client.Backend, backend client.DetectedBackend, wsFile string, version string) *ShellModel {
	ti := textinput.New()
	ti.Prompt = "  " + shellSuccessStyle.Render("crex") + " " + shellFlameStyle.Render("→") + " "
	ti.Focus()
	ti.CharLimit = 256
	ti.ShowSuggestions = true
	ti.CompletionStyle = shellCompletionStyle
	ti.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("ctrl+n"))
	ti.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("ctrl+p"))
	// Set initial suggestions for level 1 commands.
	var initSuggestions []string
	for _, c := range level1Commands {
		initSuggestions = append(initSuggestions, c.value)
	}
	ti.SetSuggestions(initSuggestions)

	// Build welcome message.
	var w strings.Builder
	w.WriteString("\n")
	w.WriteString(shellDimStyle.Render("  crex interactive shell. Type "))
	w.WriteString(shellSuccessStyle.Render("help"))
	w.WriteString(shellDimStyle.Render(" for commands, "))
	w.WriteString(shellSuccessStyle.Render("exit"))
	w.WriteString(shellDimStyle.Render(" to quit."))
	w.WriteString("\n")

	return &ShellModel{
		mode:       modePrompt,
		prompt:     ti,
		output:     &strings.Builder{},
		lastOutput: w.String(),
		backend:    backend,
		store:      store,
		client:     cl,
		wsFile:     wsFile,
		histIdx:    -1,
		welcome:    w.String(),
		version:    version,
		completer:  completionEngine{store: store, wsFile: wsFile},
	}
}

// SetBannerStyle sets the current banner style name (for "banner get").
func (m *ShellModel) SetBannerStyle(s string) { m.bannerStyle = s }

// SetRestoreMode sets the current restore mode (for "settings restore-mode get").
func (m *ShellModel) SetRestoreMode(mode string) { m.restoreMode = mode }

// ByeMsg returns the farewell message to print after the TUI exits.
func (m *ShellModel) ByeMsg() string { return m.byeMsg }

// Init is the Bubble Tea init function.
func (m *ShellModel) Init() tea.Cmd {
	return textinput.Blink
}

// flushOutput drains the per-command buffer into lastOutput.
func (m *ShellModel) flushOutput() {
	text := m.output.String()
	m.output.Reset()
	if text != "" {
		m.lastOutput += text
	}
}

// Update handles all incoming messages.
func (m *ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case restoreResultMsg:
		m.handleRestoreResult(msg)
		m.flushOutput()
		return m, nil
	case editDoneMsg:
		if msg.err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Editor: %v", msg.err)))
		} else {
			m.output.WriteString(shellSuccessStyle.Render("  ✓ Editor closed"))
		}
		m.output.WriteString("\n\n")
		m.flushOutput()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modePrompt:
			return m.updatePrompt(msg)
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeRestoreAsk:
			return m.updateRestoreAsk(msg)
		case modeRestoreSkip:
			return m.updateRestoreSkip(msg)
		}
	}

	// Pass other messages to the text input
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m *ShellModel) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyUp:
		// When completion list is visible, cycle backwards through options.
		if len(m.tabCandidates) > 0 {
			m.tabIndex = (m.tabIndex - 1 + len(m.tabCandidates)) % len(m.tabCandidates)
			m.prompt.SetValue(m.tabCandidates[m.tabIndex])
			m.prompt.CursorEnd()
			m.lastOutput = RenderItemsHighlighted(m.tabItems, m.tabIndex)
			return m, nil
		}
		if len(m.history) > 0 && m.histIdx < len(m.history)-1 {
			m.histIdx++
			m.prompt.SetValue(m.history[len(m.history)-1-m.histIdx])
			m.prompt.CursorEnd()
		}
		return m, nil

	case tea.KeyDown:
		// When completion list is visible, cycle forwards through options.
		if len(m.tabCandidates) > 0 {
			m.tabIndex = (m.tabIndex + 1) % len(m.tabCandidates)
			m.prompt.SetValue(m.tabCandidates[m.tabIndex])
			m.prompt.CursorEnd()
			m.lastOutput = RenderItemsHighlighted(m.tabItems, m.tabIndex)
			return m, nil
		}
		if m.histIdx > 0 {
			m.histIdx--
			m.prompt.SetValue(m.history[len(m.history)-1-m.histIdx])
			m.prompt.CursorEnd()
		} else if m.histIdx == 0 {
			m.histIdx = -1
			m.prompt.SetValue("")
		}
		return m, nil

	case tea.KeyEnter:
		input := strings.TrimSpace(m.prompt.Value())
		m.histIdx = -1

		if input == "" {
			return m, nil
		}

		// Bare group prefix (bp, blueprint, settings, settings banner) → treat as space+tab.
		normalized := strings.ToLower(input)
		if normalized == "blueprint" {
			normalized = "bp"
		}
		if twoWordPrefixes[normalized] || nestedGroupPrefixes[normalized] {
			m.lastCmd = ""
			m.prompt.SetValue(normalized + " ")
			m.prompt.CursorEnd()
			result := m.completer.Complete(m.prompt.Value())
			if len(result.values) > 0 {
				m.lastOutput = RenderItems(result.items)
				m.tabCandidates = result.values
				m.tabItems = result.items
				m.tabIndex = -1
			}
			return m, nil
		}

		// Record in history
		m.history = append(m.history, input)
		if len(m.history) > maxHistory {
			m.history = m.history[len(m.history)-maxHistory:]
		}

		// Clear screen for new output.
		m.lastCmd = input
		m.lastOutput = ""
		m.output.Reset()

		// Dispatch (exec methods write to m.output via pointer).
		_, dispatchCmd := m.dispatch(input)
		m.flushOutput()

		// Keep command in prompt on usage errors so the user can append args.
		if strings.Contains(m.lastOutput, "Usage:") {
			m.prompt.SetValue(input + " ")
			m.prompt.CursorEnd()
		} else {
			m.prompt.SetValue("")
		}

		return m, dispatchCmd
	}

	// Escape: remove last level from the command (navigate back).
	if msg.Type == tea.KeyEsc {
		m.lastCmd = ""
		val := strings.TrimRight(m.prompt.Value(), " ")
		if val == "" {
			m.tabCandidates = nil
			m.tabItems = nil
			m.tabIndex = -1
			m.lastOutput = m.welcome
			return m, nil
		}
		// Remove last word.
		lastSpace := strings.LastIndex(val, " ")
		if lastSpace >= 0 {
			m.prompt.SetValue(val[:lastSpace+1])
		} else {
			m.prompt.SetValue("")
		}
		m.prompt.CursorEnd()
		// Show completions for the new level, ready for cycling.
		result := m.completer.Complete(m.prompt.Value())
		if len(result.values) > 1 {
			m.lastOutput = RenderItems(result.items)
			m.tabCandidates = result.values
			m.tabItems = result.items
			m.tabIndex = -1
		} else {
			m.tabCandidates = nil
			m.tabItems = nil
			m.tabIndex = -1
			m.lastOutput = m.welcome
		}
		return m, nil
	}

	// Tab / Shift+Tab: completion cycling.
	if msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab {
		forward := msg.Type == tea.KeyTab

		// If already cycling, advance/reverse through candidates.
		if len(m.tabCandidates) > 0 {
			if forward {
				m.tabIndex = (m.tabIndex + 1) % len(m.tabCandidates)
			} else {
				m.tabIndex = (m.tabIndex - 1 + len(m.tabCandidates)) % len(m.tabCandidates)
			}
			m.prompt.SetValue(m.tabCandidates[m.tabIndex])
			m.prompt.CursorEnd()
			m.lastOutput = RenderItemsHighlighted(m.tabItems, m.tabIndex)
			return m, nil
		}

		result := m.completer.Complete(m.prompt.Value())
		switch len(result.values) {
		case 0:
			// No completions — show brief feedback.
			m.lastOutput = shellErrorStyle.Render("  ✗ No completions") + "\n"
		case 1:
			// Single match — fill it in, add space, and try deeper completions.
			filled := strings.TrimSpace(result.values[0])
			m.prompt.SetValue(filled + " ")
			m.prompt.CursorEnd()
			sub := m.completer.Complete(m.prompt.Value())
			if len(sub.values) > 0 {
				m.lastOutput = RenderItems(sub.items)
				m.tabCandidates = sub.values
				m.tabItems = sub.items
				m.tabIndex = -1
			} else {
				// No deeper completions — clear stale display.
				m.lastOutput = ""
				m.tabCandidates = nil
				m.tabItems = nil
				m.tabIndex = -1
			}
		default:
			// Multiple matches — display them and start cycle state.
			m.lastOutput = RenderItems(result.items)
			m.tabCandidates = result.values
			m.tabItems = result.items
			m.tabIndex = -1
			prefix := commonPrefix(result.values)
			if len(prefix) > len(m.prompt.Value()) {
				m.prompt.SetValue(prefix)
				m.prompt.CursorEnd()
			}
		}
		return m, nil
	}

	// Any non-tab/escape key clears the cycling state and stale list.
	if len(m.tabCandidates) > 0 {
		m.lastOutput = ""
	}
	m.tabCandidates = nil
	m.tabItems = nil
	m.tabIndex = -1

	// Update ghost-text suggestions for inline completion.
	result := m.completer.Complete(m.prompt.Value())
	m.prompt.SetSuggestions(result.values)

	// Pass to text input for line editing
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)

	return m, cmd
}

func (m *ShellModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	bm, _ := m.browse.Update(msg)
	m.browse = bm

	if bm.done {
		m.mode = modePrompt
		if bm.selected {
			_, cmd := m.handleBrowseSelection(bm.SelectedItem())
			m.flushOutput()
			return m, cmd
		}
		if bm.passthrough != 0 {
			m.prompt.SetValue(string(bm.passthrough))
			m.prompt.CursorEnd()
		}
	}
	return m, nil
}

func (m *ShellModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y') {
		if m.confirmFn != nil {
			before := m.output.Len()
			m.confirmFn()
			// Only show success if confirmFn didn't write an error.
			if m.output.Len() == before {
				m.output.WriteString(shellSuccessStyle.Render("  ✓ Done"))
				m.output.WriteString("\n")
			}
		}
	} else {
		m.output.WriteString(shellDimStyle.Render("  Cancelled"))
		m.output.WriteString("\n")
	}
	m.mode = modePrompt
	m.confirmMsg = ""
	m.confirmFn = nil
	m.flushOutput()
	return m, nil
}

func (m *ShellModel) updateRestoreAsk(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	selectReplace := func() (tea.Model, tea.Cmd) {
		if m.restoreAskHint == orchestrate.HintAskBoth {
			m.restoreSkipCursor = 0
			m.mode = modeRestoreSkip
			return m, nil
		}
		// HintAskMode — no matching tabs, auto-Fresh.
		m.mode = modePrompt
		return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, orchestrate.RestoreModeReplace, false)
	}
	selectAdd := func() (tea.Model, tea.Cmd) {
		m.mode = modePrompt
		return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, orchestrate.RestoreModeAdd, true)
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.restoreAskCursor > 0 {
			m.restoreAskCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.restoreAskCursor < 1 {
			m.restoreAskCursor++
		}
		return m, nil
	case tea.KeyEnter:
		if m.restoreAskCursor == 0 {
			return selectReplace()
		}
		return selectAdd()
	case tea.KeyEsc:
		m.mode = modePrompt
		m.output.WriteString(shellDimStyle.Render("  Cancelled"))
		m.output.WriteString("\n\n")
		m.flushOutput()
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'r', 'R':
				return selectReplace()
			case 'a', 'A':
				return selectAdd()
			}
		}
		// Invalid key — ignore.
	}
	return m, nil
}

func (m *ShellModel) updateRestoreSkip(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.restoreSkipCursor > 0 {
			m.restoreSkipCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.restoreSkipCursor < 1 {
			m.restoreSkipCursor++
		}
		return m, nil
	case tea.KeyEnter:
		skipMatching := m.restoreSkipCursor == 0
		m.mode = modePrompt
		return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, orchestrate.RestoreModeReplace, skipMatching)
	case tea.KeyEsc:
		// Go back to mode picker.
		m.mode = modeRestoreAsk
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 's', 'S':
				m.mode = modePrompt
				return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, orchestrate.RestoreModeReplace, true)
			case 'f', 'F':
				m.mode = modePrompt
				return m, m.startRestore(m.restoreAskName, m.restoreAskFilter, orchestrate.RestoreModeReplace, false)
			}
		}
	}
	return m, nil
}

func (m *ShellModel) handleBrowseSelection(item Item) (tea.Model, tea.Cmd) {
	switch m.browse.action {
	case "restore":
		layoutName := m.browse.layoutName
		if layoutName == "" {
			layoutName = item.Name
		}
		wsFilter := ""
		if item.Kind == KindWorkspace {
			wsFilter = item.Name
		}
		return m, m.execRestore(layoutName, wsFilter)
	case "use":
		m.execUse(item.Name)
	case "toggle":
		m.execBpToggle(item.Name)
	}
	return m, nil
}

// writeError writes a styled error line to the output buffer.
func (m *ShellModel) writeError(msg string) {
	m.output.WriteString(shellErrorStyle.Render("  ✗ " + msg))
	m.output.WriteString("\n\n")
}

// requireResolved validates that at least one arg exists and resolves
// a name-or-number reference against the last listing. Returns the
// resolved name and true on success, or writes an error and returns false.
func (m *ShellModel) requireResolved(args []string, usage string) (string, bool) {
	if len(args) == 0 {
		m.writeError("Usage: " + usage)
		return "", false
	}
	resolved, err := resolveNameOrNumber(args[0], m.lastItems)
	if err != nil {
		m.writeError(err.Error())
		return "", false
	}
	return resolved, true
}

func (m *ShellModel) dispatch(input string) (tea.Model, tea.Cmd) {
	// Silently ignore comment lines (useful for demos).
	if strings.HasPrefix(strings.TrimSpace(input), "#") {
		return m, nil
	}

	cmd, args := parseCommand(input)

	switch cmd {
	case "exit", "quit":
		m.byeMsg = randomBye()
		m.quitting = true
		return m, tea.Quit

	case "help", "?":
		m.output.WriteString(renderHelp(m.backend))
		m.output.WriteString("\n")

	case "ls", "list":
		m.execList()

	case "now":
		m.execNow()

	case "skill show":
		m.output.WriteString(agentskill.Content())
		m.output.WriteString("\n")

	case "skill install":
		codex := len(args) > 0 && args[0] == "codex"
		m.execSkillInstall(codex)

	case "skill":
		m.writeError("Usage: skill install [codex] | skill show")

	case "save":
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}
		m.execSave(name)

	case "restore":
		if resolved, ok := m.requireResolved(args, "restore <name|#>"); ok {
			return m, m.execRestore(resolved, "")
		}

	case "delete":
		if resolved, ok := m.requireResolved(args, "delete <name|#>"); ok {
			m.execDelete(resolved)
		}

	case "rename":
		if len(args) < 2 {
			m.writeError("Usage: rename <old|#> <new>")
			break
		}
		oldName := args[0]
		newName := args[1]
		// Resolve old name if it's a number reference.
		if resolved, err := resolveNameOrNumber(oldName, m.lastItems); err == nil {
			oldName = resolved
		}
		m.execRename(oldName, newName)

	case "show":
		if resolved, ok := m.requireResolved(args, "show <name|#>"); ok {
			m.execShow(resolved)
		}

	case "edit":
		if resolved, ok := m.requireResolved(args, "edit <name|#>"); ok {
			return m, m.execEdit(resolved)
		}

	case "templates":
		m.execTemplates()

	case "use":
		if resolved, ok := m.requireResolved(args, "use <template|#>"); ok {
			m.execUse(resolved)
		}

	case "template show":
		if resolved, ok := m.requireResolved(args, "template show <name|#>"); ok {
			m.execTemplateShow(resolved)
		}

	case "template customize":
		if resolved, ok := m.requireResolved(args, "template customize <name|#>"); ok {
			m.execTemplateCustomize(resolved)
		}

	case "import", "import-from-md":
		m.execImport()

	case "export", "export-to-md":
		m.execExport()

	case "watch":
		sub := ""
		if len(args) > 0 {
			sub = args[0]
		}
		m.execWatch(sub)

	case "bp add":
		if len(args) < 2 {
			m.writeError("Usage: bp add <name> <path>")
			break
		}
		m.execBpAdd(args[0], args[1])

	case "bp list", "bp ls":
		m.execBpList()

	case "bp remove", "bp rm":
		if resolved, ok := m.requireResolved(args, "bp remove <name|#>"); ok {
			m.execBpRemove(resolved)
		}

	case "settings banner set":
		if len(args) == 0 {
			m.writeError("Usage: settings banner set <flame|classic|plain>")
			break
		}
		if m.BannerCycle == nil {
			m.writeError("banner not available")
			break
		}
		newStyle, preview, err := m.BannerCycle(args[0])
		if err != nil {
			m.writeError(err.Error())
			break
		}
		m.bannerStyle = newStyle
		m.output.WriteString(preview)

	case "settings banner get":
		if m.BannerCycle == nil {
			m.writeError("banner not available")
			break
		}
		fmt.Fprintf(m.output, "  Current banner style: %s\n\n",
			shellSuccessStyle.Render(m.bannerStyle))

	case "settings banner list":
		m.output.WriteString("  Available banner styles:\n")
		fmt.Fprintf(m.output, "    %s  gradient (ember → gold → green)\n", shellSuccessStyle.Render("flame  "))
		fmt.Fprintf(m.output, "    %s  solid green\n", shellSuccessStyle.Render("classic"))
		fmt.Fprintf(m.output, "    %s  monochrome gray\n", shellSuccessStyle.Render("plain  "))
		m.output.WriteString("\n")

	case "bp toggle":
		if resolved, ok := m.requireResolved(args, "bp toggle <name|#>"); ok {
			m.execBpToggle(resolved)
		}

	case "settings restore-mode set":
		if len(args) == 0 {
			m.writeError("Usage: settings restore-mode set <ask|replace|add>")
			break
		}
		mode := strings.ToLower(args[0])
		switch mode {
		case "ask", "replace", "add":
			m.restoreMode = mode
			if m.OnSettingChanged != nil {
				m.OnSettingChanged("restore_mode", mode)
			}
			m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Restore mode set to %q", mode)))
			m.output.WriteString("\n\n")
		default:
			m.writeError(fmt.Sprintf("Invalid mode %q — use ask, replace, or add", mode))
		}

	case "settings restore-mode get":
		mode := m.restoreMode
		if mode == "" {
			mode = "ask"
		}
		fmt.Fprintf(m.output, "  Current restore mode: %s\n\n", shellSuccessStyle.Render(mode))

	case "settings restore-mode list":
		m.output.WriteString("  Available restore modes:\n")
		fmt.Fprintf(m.output, "    %s  prompt for replace/add each time (default)\n", shellSuccessStyle.Render("ask    "))
		fmt.Fprintf(m.output, "    %s  always replace existing workspaces\n", shellSuccessStyle.Render("replace"))
		fmt.Fprintf(m.output, "    %s  always add alongside existing workspaces\n", shellSuccessStyle.Render("add    "))
		m.output.WriteString("\n")

	case "update":
		m.execUpdate()

	case "pop":
		m.output.WriteString(shellDimStyle.Render("  pop is a standalone launcher — run it from your terminal:"))
		m.output.WriteString("\n\n")
		m.output.WriteString("  " + shellSuccessStyle.Render("crex pop") + shellDimStyle.Render("          # picker with fuzzy search"))
		m.output.WriteString("\n")
		m.output.WriteString("  " + shellSuccessStyle.Render("crex pop <name>") + shellDimStyle.Render("    # restore layout directly"))
		m.output.WriteString("\n")
		m.output.WriteString("  " + shellSuccessStyle.Render("crex pop --last") + shellDimStyle.Render("    # restore most recent"))
		m.output.WriteString("\n\n")
		m.output.WriteString(shellDimStyle.Render("  Tip: crex setup installs Ctrl+G for instant access."))
		m.output.WriteString("\n\n")

	default:
		// Redirect old "banner" commands to "settings banner".
		if cmd == "banner" || strings.HasPrefix(cmd, "banner ") {
			m.output.WriteString(shellDimStyle.Render("  Banner moved to: settings banner set|get|list"))
			m.output.WriteString("\n\n")
			break
		}
		m.writeError(fmt.Sprintf("Unknown command: %s", cmd))
		m.output.WriteString(shellDimStyle.Render("  Type help for available commands."))
		m.output.WriteString("\n\n")
	}

	return m, nil
}

// View renders the full screen: prompt always at top, blank line, then content.
func (m *ShellModel) View() string {
	if m.quitting {
		return ""
	}

	prompt := m.prompt.View()
	header := ""
	if m.lastCmd != "" {
		header = "\n" + shellDimStyle.Render("  "+m.lastCmd)
	}

	switch m.mode {
	case modeBrowse:
		return prompt + header + "\n\n" + m.browse.View()
	case modeConfirm:
		return prompt + header + "\n\n" + m.confirmMsg + "\n"
	case modeRestoreAsk:
		var b strings.Builder
		b.WriteString("  How do you want to restore?\n\n")
		labels := []string{
			"Replace — close non-matching, keep matching",
			"Add     — keep all existing, add missing",
		}
		keys := []string{"r", "a"}
		for i, label := range labels {
			if i == m.restoreAskCursor {
				fmt.Fprintf(&b, "  %s %s  %s\n", shellSuccessStyle.Render("▸"), shellSuccessStyle.Render("["+keys[i]+"]"), label)
			} else {
				fmt.Fprintf(&b, "    %s  %s\n", shellDimStyle.Render("["+keys[i]+"]"), label)
			}
		}
		b.WriteString("\n")
		b.WriteString(shellDimStyle.Render("  ↑/↓ select · ↵ confirm · r/a shortcut · esc cancel"))
		b.WriteString("\n")
		return prompt + header + "\n\n" + b.String()
	case modeRestoreSkip:
		var b strings.Builder
		b.WriteString("  Tabs already open match the layout.\n\n")
		labels := []string{
			"Skip  — leave matching tabs as they are",
			"Fresh — close and recreate from layout",
		}
		keys := []string{"s", "f"}
		for i, label := range labels {
			if i == m.restoreSkipCursor {
				fmt.Fprintf(&b, "  %s %s  %s\n", shellSuccessStyle.Render("▸"), shellSuccessStyle.Render("["+keys[i]+"]"), label)
			} else {
				fmt.Fprintf(&b, "    %s  %s\n", shellDimStyle.Render("["+keys[i]+"]"), label)
			}
		}
		b.WriteString("\n")
		b.WriteString(shellDimStyle.Render("  ↑/↓ select · ↵ confirm · s/f shortcut · esc back"))
		b.WriteString("\n")
		return prompt + header + "\n\n" + b.String()
	default:
		return prompt + header + "\n" + m.lastOutput
	}
}
