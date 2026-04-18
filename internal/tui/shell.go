package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

type shellMode int

const (
	modePrompt  shellMode = iota
	modeBrowse
	modeConfirm
)

const maxHistory = 50

// ShellModel is the main Bubble Tea model for the crex interactive shell.
type ShellModel struct {
	mode      shellMode
	prompt    textinput.Model
	browse    BrowseModel
	output    *strings.Builder // per-command buffer, flushed via tea.Println
	lastItems []Item
	history   []string
	histIdx   int
	backend   client.DetectedBackend
	store     persist.Store
	client    client.Backend
	wsFile    string
	quitting  bool
	welcome   string // printed once via Init

	// Confirmation state
	confirmMsg string
	confirmFn  func()
}

// NewShellModel creates the interactive shell model.
func NewShellModel(store persist.Store, cl client.Backend, backend client.DetectedBackend, wsFile string) ShellModel {
	ti := textinput.New()
	ti.Prompt = shellPromptStyle.Render("crex❯") + " "
	ti.Focus()
	ti.CharLimit = 256

	// Build welcome message (printed via Init, not accumulated in View).
	var w strings.Builder
	w.WriteString(shellDimStyle.Render("  crex interactive shell. Type "))
	w.WriteString(shellSuccessStyle.Render("help"))
	w.WriteString(shellDimStyle.Render(" for commands, "))
	w.WriteString(shellSuccessStyle.Render("exit"))
	w.WriteString(shellDimStyle.Render(" to quit."))

	return ShellModel{
		mode:    modePrompt,
		prompt:  ti,
		output:  &strings.Builder{},
		backend: backend,
		store:   store,
		client:  cl,
		wsFile:  wsFile,
		histIdx: -1,
		welcome: w.String(),
	}
}

// Init is the Bubble Tea init function.
func (m ShellModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.Println(m.welcome))
}

// flushOutput drains the per-command buffer and returns a tea.Println Cmd.
// Returns nil when the buffer is empty.
func (m ShellModel) flushOutput() tea.Cmd {
	text := m.output.String()
	m.output.Reset()
	if text == "" {
		return nil
	}
	return tea.Println(strings.TrimRight(text, "\n"))
}

// Update handles all incoming messages.
func (m ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modePrompt:
			return m.updatePrompt(msg)
		case modeBrowse:
			return m.updateBrowse(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		}
	}

	// Pass other messages to the text input
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m ShellModel) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyUp:
		if len(m.history) > 0 && m.histIdx < len(m.history)-1 {
			m.histIdx++
			m.prompt.SetValue(m.history[len(m.history)-1-m.histIdx])
			m.prompt.CursorEnd()
		}
		return m, nil

	case tea.KeyDown:
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
		m.prompt.SetValue("")
		m.histIdx = -1

		if input == "" {
			return m, nil
		}

		// Record in history
		m.history = append(m.history, input)
		if len(m.history) > maxHistory {
			m.history = m.history[len(m.history)-maxHistory:]
		}

		// Reset buffer and echo the command
		m.output.Reset()
		m.output.WriteString(shellPromptStyle.Render("crex❯"))
		m.output.WriteString(" ")
		m.output.WriteString(input)
		m.output.WriteString("\n")

		// Dispatch (exec methods write to m.output)
		model, dispatchCmd := m.dispatch(input)

		// Flush buffered output as tea.Println
		sm := model.(ShellModel)
		printCmd := sm.flushOutput()

		return sm, batchNonNil(printCmd, dispatchCmd)
	}

	// Pass to text input for line editing
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m ShellModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	bm, _ := m.browse.Update(msg)
	m.browse = bm

	if bm.done {
		m.mode = modePrompt
		if bm.selected {
			model, cmd := m.handleBrowseSelection(bm.SelectedItem())
			sm := model.(ShellModel)
			printCmd := sm.flushOutput()
			return sm, batchNonNil(printCmd, cmd)
		}
		if bm.passthrough != 0 {
			m.prompt.SetValue(string(bm.passthrough))
			m.prompt.CursorEnd()
		}
	}
	return m, nil
}

func (m ShellModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y') {
		if m.confirmFn != nil {
			m.confirmFn()
		}
		m.output.WriteString(shellSuccessStyle.Render("  ✓ Done"))
		m.output.WriteString("\n")
	} else {
		m.output.WriteString(shellDimStyle.Render("  Cancelled"))
		m.output.WriteString("\n")
	}
	m.mode = modePrompt
	m.confirmMsg = ""
	m.confirmFn = nil
	return m, m.flushOutput()
}

func (m ShellModel) handleBrowseSelection(item Item) (tea.Model, tea.Cmd) {
	switch m.browse.action {
	case "restore":
		m.execRestore(item.Name)
	case "use":
		m.execUse(item.Name)
	case "toggle":
		m.execBpToggle(item.Name)
	}
	return m, nil
}

func (m ShellModel) dispatch(input string) (tea.Model, tea.Cmd) {
	cmd, args := parseCommand(input)

	switch cmd {
	case "exit", "quit":
		m.output.WriteString(shellDimStyle.Render("  👋"))
		m.output.WriteString("\n")
		m.quitting = true
		return m, tea.Quit

	case "help", "?":
		m.output.WriteString(renderHelp(m.backend))
		m.output.WriteString("\n")

	case "ls", "list":
		m.execList()

	case "now":
		m.execNow()

	case "save":
		name := "default"
		if len(args) > 0 {
			name = args[0]
		}
		m.execSave(name)

	case "restore":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: restore <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execRestore(resolved)

	case "delete":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: delete <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execDelete(resolved)

	case "templates":
		m.execTemplates()

	case "use":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: use <template|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execUse(resolved)

	case "watch":
		sub := ""
		if len(args) > 0 {
			sub = args[0]
		}
		m.execWatch(sub)

	case "bp add":
		if len(args) < 2 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp add <name> <path>"))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpAdd(args[0], args[1])

	case "bp list", "bp ls":
		m.execBpList()

	case "bp remove", "bp rm":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp remove <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpRemove(resolved)

	case "bp toggle":
		if len(args) == 0 {
			m.output.WriteString(shellErrorStyle.Render("  ✗ Usage: bp toggle <name|#>"))
			m.output.WriteString("\n\n")
			break
		}
		resolved, err := resolveNameOrNumber(args[0], m.lastItems)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %s", err)))
			m.output.WriteString("\n\n")
			break
		}
		m.execBpToggle(resolved)

	default:
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Unknown command: %s", cmd)))
		m.output.WriteString("\n")
		m.output.WriteString(shellDimStyle.Render("  Type help for available commands."))
		m.output.WriteString("\n\n")
	}

	return m, nil
}

// View renders only the current interactive element.
// Command output is printed above via tea.Println, not accumulated here.
func (m ShellModel) View() string {
	if m.quitting {
		return ""
	}

	switch m.mode {
	case modeBrowse:
		return m.browse.View()
	case modeConfirm:
		return m.confirmMsg + "\n"
	default:
		return m.prompt.View()
	}
}

// batchNonNil batches commands, filtering out nils.
func batchNonNil(cmds ...tea.Cmd) tea.Cmd {
	var live []tea.Cmd
	for _, c := range cmds {
		if c != nil {
			live = append(live, c)
		}
	}
	switch len(live) {
	case 0:
		return nil
	case 1:
		return live[0]
	default:
		return tea.Batch(live...)
	}
}
