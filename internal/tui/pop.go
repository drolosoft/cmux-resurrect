package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/sahilm/fuzzy"
)

// viewMode tracks whether we're in the top-level list or drilled into a layout.
type viewMode int

const (
	modeList  viewMode = iota
	modeDrill          // showing workspaces inside a layout
)

// popStyles holds the lipgloss styles used by PopModel.
var (
	popBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Dark: "#FFB454", Light: "#D4820A"}).
			Padding(1, 2)

	popMatchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "#FFB454", Light: "#D4820A"}).
			Bold(true).
			Underline(true)

	popHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"})
	popCursorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	popNumberStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	popDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	popTitleStyle   = lipgloss.NewStyle().Bold(true)
	popMetaStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	popSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "#FFB454", Light: "#D4820A"})
	popFilterStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
)

// PopItem represents a single entry in the pop picker.
type PopItem struct {
	Kind string // "layout" or "template"
	Name string
	Icon string // template icon (e.g. "⧉"), empty for layouts
	Meta string // "3 tabs  May 28" or "editor+git+term"
}

// DrillItem represents a workspace within a drilled-in layout.
type DrillItem struct {
	LayoutName  string
	Index       int
	Title       string
	CWD         string
	PaneCount   int
	PaneSummary string // "npm | shell"
	Pinned      bool
}

// PopResult represents what the user selected from the picker.
type PopResult struct {
	Kind           string // "layout", "template", "workspace"
	Name           string // layout or template name
	WorkspaceTitle string // only for Kind=="workspace"
}

// PopModel is a centered floating picker for crex pop.
type PopModel struct {
	// Data
	items          []PopItem
	filtered       []PopItem
	matchPositions [][]int // per-item matched char indices from fuzzy

	// List state
	filter   string
	cursor   int
	offset   int // scroll offset
	chosen   *PopItem
	quitting bool

	// Terminal
	width  int
	height int

	// Drill state
	mode            viewMode
	drillLayout     string
	drillItems      []DrillItem
	drillFiltered   []DrillItem
	drillCursor     int
	drillOffset     int
	chosenWorkspace *DrillItem

	// Dependencies
	loadLayout func(string) (*model.Layout, error)
}

// fuzzySource adapts []PopItem for github.com/sahilm/fuzzy.
type fuzzySource []PopItem

func (s fuzzySource) String(i int) string { return s[i].Name + " " + s[i].Meta }
func (s fuzzySource) Len() int            { return len(s) }

// NewPopModel creates a picker with the given items.
func NewPopModel(items []PopItem, width, height int, loadLayout func(string) (*model.Layout, error)) *PopModel {
	m := &PopModel{
		items:      items,
		width:      width,
		height:     height,
		loadLayout: loadLayout,
	}
	m.applyFilter()
	return m
}

// Result returns a PopResult describing what the user selected, or nil if cancelled.
func (m *PopModel) Result() *PopResult {
	if m.chosenWorkspace != nil {
		return &PopResult{
			Kind:           "workspace",
			Name:           m.chosenWorkspace.LayoutName,
			WorkspaceTitle: m.chosenWorkspace.Title,
		}
	}
	if m.chosen != nil {
		return &PopResult{
			Kind: m.chosen.Kind,
			Name: m.chosen.Name,
		}
	}
	return nil
}

// Chosen returns the selected item, or nil if cancelled. (Compat shim.)
func (m *PopModel) Chosen() *PopItem {
	return m.chosen
}

// Init is the Bubble Tea init function.
func (m *PopModel) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// Update handles all incoming messages, delegating to list or drill mode.
func (m *PopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeDrill {
			return m.updateDrill(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

// updateList handles key messages in list mode.
func (m *PopModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEnter:
		if len(m.filtered) > 0 {
			item := m.filtered[m.cursor]
			m.chosen = &item
		}
		return m, tea.Quit

	case tea.KeyUp:
		m.cursorUp()
		return m, nil

	case tea.KeyDown:
		m.cursorDown()
		return m, nil

	case tea.KeyTab, tea.KeyRight:
		// Drill into the selected layout.
		if len(m.filtered) > 0 {
			item := m.filtered[m.cursor]
			if item.Kind == "layout" {
				m.enterDrill(item.Name)
			}
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			runes := []rune(m.filter)
			m.filter = string(runes[:len(runes)-1])
			m.applyFilter()
		}
		return m, nil

	default:
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= '1' && r <= '9' {
				n := int(r - '0')
				if item := m.selectByNumber(n); item != nil {
					m.chosen = item
					return m, tea.Quit
				}
				return m, nil
			}
			m.filter += string(r)
			m.applyFilter()
			return m, nil
		}
	}
	return m, nil
}

// updateDrill handles key messages in drill mode.
func (m *PopModel) updateDrill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEsc, tea.KeyTab, tea.KeyLeft:
		m.exitDrill()
		return m, nil

	case tea.KeyEnter:
		if len(m.drillFiltered) > 0 {
			ws := m.drillFiltered[m.drillCursor]
			m.chosenWorkspace = &ws
		}
		return m, tea.Quit

	case tea.KeyUp:
		if m.drillCursor > 0 {
			m.drillCursor--
		}
		return m, nil

	case tea.KeyDown:
		if len(m.drillFiltered) > 0 && m.drillCursor < len(m.drillFiltered)-1 {
			m.drillCursor++
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			runes := []rune(m.filter)
			m.filter = string(runes[:len(runes)-1])
			m.applyDrillFilter()
		}
		return m, nil

	default:
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			r := msg.Runes[0]
			if r >= '1' && r <= '9' {
				n := int(r - '0')
				if n >= 1 && n <= len(m.drillFiltered) {
					ws := m.drillFiltered[n-1]
					m.chosenWorkspace = &ws
					return m, tea.Quit
				}
				return m, nil
			}
			m.filter += string(r)
			m.applyDrillFilter()
			return m, nil
		}
	}
	return m, nil
}

// View renders the centered box with the appropriate mode content.
func (m *PopModel) View() string {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	boxWidth := clampInt(w-4, 40, 60)
	boxHeight := clampInt(h-4, 12, 22)
	innerWidth := boxWidth - 6   // border(2) + padding(4)
	innerHeight := boxHeight - 4 // border(2) + padding(2)

	var content string
	if m.mode == modeDrill {
		content = m.viewDrill(innerWidth, innerHeight)
	} else {
		content = m.viewList(innerWidth, innerHeight)
	}

	box := popBoxStyle.Width(boxWidth).Height(boxHeight).Render(content)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// viewList renders the list mode content.
func (m *PopModel) viewList(innerWidth, innerHeight int) string {
	var b strings.Builder

	// Header: "crex > filter_"
	b.WriteString(popTitleStyle.Render("crex"))
	b.WriteString(" > ")
	if m.filter != "" {
		b.WriteString(popFilterStyle.Render(m.filter))
	}
	b.WriteString(popDimStyle.Render("_"))
	b.WriteString("\n")

	// Footer (will be appended at end).
	footer := popDimStyle.Render("↵ launch  tab drill  1-9 jump  esc")

	headerLines := 2 // prompt + blank line
	footerLines := 2 // blank line + hints
	listHeight := innerHeight - headerLines - footerLines
	if listHeight < 1 {
		listHeight = 1
	}

	if len(m.filtered) == 0 {
		b.WriteString("\n")
		b.WriteString(popDimStyle.Render("no results"))
		b.WriteString("\n")
	} else {
		// Build all visible lines first, then apply scroll.
		var lines []string
		idx := 0
		lastKind := ""
		for i, item := range m.filtered {
			// Section header when kind changes.
			if item.Kind != lastKind {
				if lastKind != "" {
					lines = append(lines, "")
				}
				section := "LAYOUTS"
				if item.Kind == "template" {
					section = "TEMPLATES"
				}
				lines = append(lines, popSectionStyle.Render(section))
				lastKind = item.Kind
			}

			num := fmt.Sprintf("[%d]", idx+1)
			isCurrent := idx == m.cursor

			var line strings.Builder
			if isCurrent {
				line.WriteString(popCursorStyle.Render("▸"))
				line.WriteString(" ")
				line.WriteString(popNumberStyle.Render(num))
			} else {
				line.WriteString("  ")
				line.WriteString(popDimStyle.Render(num))
			}
			line.WriteString("  ")

			// Icon for templates.
			if item.Icon != "" {
				line.WriteString(item.Icon)
				line.WriteString("  ")
			}

			// Name with optional match highlighting.
			nameStr := item.Name
			if m.matchPositions != nil && i < len(m.matchPositions) && len(m.matchPositions[i]) > 0 {
				nameStr = highlightMatches(item.Name, m.matchPositions[i])
				if isCurrent {
					// Wrap non-highlighted parts in bold.
					nameStr = highlightMatchesBold(item.Name, m.matchPositions[i])
				}
			} else if isCurrent {
				nameStr = popTitleStyle.Render(item.Name)
			}
			line.WriteString(nameStr)

			if item.Meta != "" {
				line.WriteString("  ")
				line.WriteString(popMetaStyle.Render(item.Meta))
			}

			// Drill indicator for layouts under cursor.
			if isCurrent && item.Kind == "layout" {
				line.WriteString(" ")
				line.WriteString(popDimStyle.Render("→"))
			}

			lines = append(lines, line.String())
			idx++
		}

		// Find the line index where the cursor item lives.
		cursorLine := m.findCursorLine(lines)

		// Adjust offset to keep cursor visible.
		if cursorLine < m.offset {
			m.offset = cursorLine
		}
		if cursorLine >= m.offset+listHeight {
			m.offset = cursorLine - listHeight + 1
		}
		if m.offset < 0 {
			m.offset = 0
		}

		// Render visible slice.
		b.WriteString("\n")
		end := m.offset + listHeight
		if end > len(lines) {
			end = len(lines)
		}
		for i := m.offset; i < end; i++ {
			b.WriteString(lines[i])
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(footer)

	return b.String()
}

// viewDrill renders the drill mode content.
func (m *PopModel) viewDrill(innerWidth, innerHeight int) string {
	var b strings.Builder

	// Breadcrumb: "crex > layoutName > _"
	b.WriteString(popTitleStyle.Render("crex"))
	b.WriteString(" > ")
	b.WriteString(popHeaderStyle.Render(m.drillLayout))
	b.WriteString(" > ")
	if m.filter != "" {
		b.WriteString(popFilterStyle.Render(m.filter))
	}
	b.WriteString(popDimStyle.Render("_"))
	b.WriteString("\n")

	footer := popDimStyle.Render("↵ restore  esc back  1-9 jump")

	headerLines := 2
	footerLines := 2
	listHeight := innerHeight - headerLines - footerLines
	if listHeight < 1 {
		listHeight = 1
	}

	sectionHeader := fmt.Sprintf("WORKSPACES in %s", m.drillLayout)

	if len(m.drillFiltered) == 0 {
		b.WriteString("\n")
		b.WriteString(popSectionStyle.Render(sectionHeader))
		b.WriteString("\n")
		b.WriteString(popDimStyle.Render("no results"))
		b.WriteString("\n")
	} else {
		var lines []string
		lines = append(lines, popSectionStyle.Render(sectionHeader))

		for idx, item := range m.drillFiltered {
			num := fmt.Sprintf("[%d]", idx+1)
			isCurrent := idx == m.drillCursor

			var line strings.Builder
			if isCurrent {
				line.WriteString(popCursorStyle.Render("▸"))
				line.WriteString(" ")
				line.WriteString(popNumberStyle.Render(num))
			} else {
				line.WriteString("  ")
				line.WriteString(popDimStyle.Render(num))
			}
			line.WriteString("  ")

			titleStr := item.Title
			if isCurrent {
				titleStr = popTitleStyle.Render(item.Title)
			}
			line.WriteString(titleStr)

			paneInfo := fmt.Sprintf("%d %s", item.PaneCount, pluralPane(item.PaneCount))
			line.WriteString("  ")
			line.WriteString(popMetaStyle.Render(paneInfo))

			if item.PaneSummary != "" {
				line.WriteString("  ")
				line.WriteString(popDimStyle.Render(item.PaneSummary))
			}

			lines = append(lines, line.String())
		}

		// Find cursor line in drill view (offset by 1 for section header).
		cursorLine := m.drillCursor + 1 // +1 for section header line

		if cursorLine < m.drillOffset {
			m.drillOffset = cursorLine
		}
		if cursorLine >= m.drillOffset+listHeight {
			m.drillOffset = cursorLine - listHeight + 1
		}
		if m.drillOffset < 0 {
			m.drillOffset = 0
		}

		b.WriteString("\n")
		end := m.drillOffset + listHeight
		if end > len(lines) {
			end = len(lines)
		}
		for i := m.drillOffset; i < end; i++ {
			b.WriteString(lines[i])
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(footer)

	return b.String()
}

// applyFilter rebuilds m.filtered using fuzzy matching on m.filter.
func (m *PopModel) applyFilter() {
	if m.filter == "" {
		m.filtered = make([]PopItem, len(m.items))
		copy(m.filtered, m.items)
		m.matchPositions = nil
		m.cursor = 0
		m.offset = 0
		return
	}

	matches := fuzzy.FindFrom(m.filter, fuzzySource(m.items))
	m.filtered = make([]PopItem, len(matches))
	m.matchPositions = make([][]int, len(matches))
	for i, match := range matches {
		m.filtered[i] = m.items[match.Index]
		m.matchPositions[i] = match.MatchedIndexes
	}
	m.cursor = 0
	m.offset = 0
}

// applyDrillFilter filters drill items using substring match on title and summary.
func (m *PopModel) applyDrillFilter() {
	if m.filter == "" {
		m.drillFiltered = make([]DrillItem, len(m.drillItems))
		copy(m.drillFiltered, m.drillItems)
		m.drillCursor = 0
		m.drillOffset = 0
		return
	}

	lower := strings.ToLower(m.filter)
	m.drillFiltered = nil
	for _, item := range m.drillItems {
		if strings.Contains(strings.ToLower(item.Title), lower) ||
			strings.Contains(strings.ToLower(item.PaneSummary), lower) {
			m.drillFiltered = append(m.drillFiltered, item)
		}
	}
	m.drillCursor = 0
	m.drillOffset = 0
}

// enterDrill switches to drill mode for the named layout.
func (m *PopModel) enterDrill(layoutName string) {
	if m.loadLayout == nil {
		return
	}

	layout, err := m.loadLayout(layoutName)
	if err != nil || layout == nil {
		return // stay in list
	}

	m.mode = modeDrill
	m.drillLayout = layoutName
	m.drillItems = nil

	for _, ws := range layout.Workspaces {
		var parts []string
		for _, p := range ws.Panes {
			if p.Command != "" {
				parts = append(parts, shortCmd(p.Command))
			} else if p.URL != "" {
				parts = append(parts, "browser")
			} else {
				parts = append(parts, "shell")
			}
		}
		summary := strings.Join(parts, " | ")

		m.drillItems = append(m.drillItems, DrillItem{
			LayoutName:  layoutName,
			Index:       ws.Index,
			Title:       ws.Title,
			CWD:         ws.CWD,
			PaneCount:   len(ws.Panes),
			PaneSummary: summary,
			Pinned:      ws.Pinned,
		})
	}

	m.filter = ""
	m.drillCursor = 0
	m.drillOffset = 0
	m.applyDrillFilter()
}

// exitDrill returns to list mode.
func (m *PopModel) exitDrill() {
	m.mode = modeList
	m.filter = ""
	m.drillItems = nil
	m.drillFiltered = nil
	m.drillCursor = 0
	m.drillOffset = 0
}

// cursorUp moves the cursor up by one, clamped to 0.
func (m *PopModel) cursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// cursorDown moves the cursor down by one, clamped to the last index.
func (m *PopModel) cursorDown() {
	if len(m.filtered) == 0 {
		return
	}
	if m.cursor < len(m.filtered)-1 {
		m.cursor++
	}
}

// selectByNumber returns the item at 1-based position n, or nil if out of range.
func (m *PopModel) selectByNumber(n int) *PopItem {
	if n < 1 || n > len(m.filtered) {
		return nil
	}
	item := m.filtered[n-1]
	return &item
}

// findCursorLine finds which rendered line index the cursor item occupies.
// This accounts for section headers and blank separator lines.
func (m *PopModel) findCursorLine(lines []string) int {
	idx := 0
	lastKind := ""
	lineNum := 0
	for _, item := range m.filtered {
		if item.Kind != lastKind {
			if lastKind != "" {
				lineNum++ // blank separator
			}
			lineNum++ // section header
			lastKind = item.Kind
		}
		if idx == m.cursor {
			return lineNum
		}
		lineNum++
		idx++
	}
	return 0
}

// highlightMatches renders matched characters in orange+bold+underline style.
func highlightMatches(s string, positions []int) string {
	posSet := make(map[int]bool, len(positions))
	for _, p := range positions {
		posSet[p] = true
	}

	var result strings.Builder
	for i, r := range s {
		if posSet[i] {
			result.WriteString(popMatchStyle.Render(string(r)))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// highlightMatchesBold renders matched chars in match style, non-matched in bold.
func highlightMatchesBold(s string, positions []int) string {
	posSet := make(map[int]bool, len(positions))
	for _, p := range positions {
		posSet[p] = true
	}

	var result strings.Builder
	for i, r := range s {
		if posSet[i] {
			result.WriteString(popMatchStyle.Render(string(r)))
		} else {
			result.WriteString(popTitleStyle.Render(string(r)))
		}
	}
	return result.String()
}

// shortCmd truncates a command to just the first word.
func shortCmd(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return parts[0]
	}
	return cmd
}

// pluralPane returns "pane" or "panes".
func pluralPane(n int) string {
	if n == 1 {
		return "pane"
	}
	return "panes"
}

// clampInt clamps v to [min, max].
func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
