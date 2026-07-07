package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/drolosoft/cmux-resurrect/internal/tui"
)

// PickResult holds the user's selection from the restore picker.
type PickResult struct {
	Layout    string
	Workspace string
	Mode      orchestrate.RestoreMode
}

type pickerState int

const (
	stateLayout pickerState = iota
	stateMode
)

// pickerModel combines layout selection and mode selection in one Bubble Tea program.
type pickerModel struct {
	state    pickerState
	browse   tui.BrowseModel
	items    []tui.Item // original items for reset
	skipMode bool       // true when mode is pre-determined (flag/config)

	// Mode picker state.
	modeCursor int // 0 = replace, 1 = add

	// Result.
	cancelled bool
	layout    string
	workspace string
	mode      orchestrate.RestoreMode
}

func newPickerModel(metas []model.LayoutMeta, skipMode bool) pickerModel {
	items := tui.ItemsFromLayouts(metas)
	return pickerModel{
		state:    stateLayout,
		browse:   tui.NewBrowseModel(items, "restore"),
		items:    items,
		skipMode: skipMode,
	}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.Type == tea.KeyCtrlC {
			m.cancelled = true
			return m, tea.Quit
		}
		switch m.state {
		case stateLayout:
			return m.updateLayout(msg)
		case stateMode:
			return m.updateMode(msg)
		}
	}
	return m, nil
}

func (m pickerModel) updateLayout(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.browse, _ = m.browse.Update(msg)
	if m.browse.Done() {
		if !m.browse.Selected() {
			m.cancelled = true
			return m, tea.Quit
		}

		// Extract selection.
		item := m.browse.SelectedItem()
		m.layout = item.Name
		m.workspace = ""
		switch item.Kind {
		case tui.KindWorkspace:
			m.workspace = item.Name
			m.layout = m.browse.LayoutName()
		case tui.KindAllWs:
			m.layout = m.browse.LayoutName()
		}

		if m.skipMode {
			return m, tea.Quit
		}

		// Transition to mode picker.
		m.state = stateMode
		m.modeCursor = 0
	}
	return m, nil
}

func (m pickerModel) updateMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyShiftTab, tea.KeyLeft, tea.KeyBackspace:
		// Go back to layout picker.
		m.state = stateLayout
		m.browse = tui.NewBrowseModel(m.items, "restore")
		return m, nil
	case tea.KeyUp:
		if m.modeCursor > 0 {
			m.modeCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.modeCursor < 1 {
			m.modeCursor++
		}
		return m, nil
	case tea.KeyEnter:
		if m.modeCursor == 0 {
			m.mode = orchestrate.RestoreModeReplace
		} else {
			m.mode = orchestrate.RestoreModeAdd
		}
		return m, tea.Quit
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'r', 'R':
				m.mode = orchestrate.RestoreModeReplace
				return m, tea.Quit
			case 'a', 'A':
				m.mode = orchestrate.RestoreModeAdd
				return m, tea.Quit
			case 'q':
				m.state = stateLayout
				m.browse = tui.NewBrowseModel(m.items, "restore")
				return m, nil
			}
		}
	}
	return m, nil
}

func (m pickerModel) View() string {
	var b strings.Builder

	switch m.state {
	case stateLayout:
		b.WriteString("  📦 Select a layout to restore\n\n")
		b.WriteString(m.browse.View())

	case stateMode:
		b.WriteString(headingStyle.Render("How do you want to restore?"))
		b.WriteString("\n\n")

		labels := []string{
			fmt.Sprintf("Replace — close all current %s, then restore", unitName(2)),
			fmt.Sprintf("Add     — keep current %s, add restored ones", unitName(2)),
		}
		keys := []string{"r", "a"}

		for i, label := range labels {
			if i == m.modeCursor {
				fmt.Fprintf(&b, "  %s %s  %s\n", greenStyle.Render("▸"), cyanStyle.Render("["+keys[i]+"]"), label)
			} else {
				fmt.Fprintf(&b, "    %s  %s\n", cyanStyle.Render("["+keys[i]+"]"), label)
			}
		}

		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  ↑/↓ select · ↵ confirm · esc back"))
		b.WriteString("\n")
	}

	return b.String()
}

// pickLayout runs the combined layout+mode picker.
func pickLayout(metas []model.LayoutMeta, skipMode bool) (*PickResult, error) {
	m := newPickerModel(metas, skipMode)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	fm := finalModel.(pickerModel)
	if fm.cancelled {
		return nil, fmt.Errorf("cancelled")
	}

	return &PickResult{
		Layout:    fm.layout,
		Workspace: fm.workspace,
		Mode:      fm.mode,
	}, nil
}
