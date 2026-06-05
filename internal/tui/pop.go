package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/rivo/uniseg"
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
	popGold = lipgloss.Color("#FFD700")

	popBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderTopForeground(popGold).
			BorderBottomForeground(popGold).
			BorderLeftForeground(popGold).
			BorderRightForeground(popGold).
			Padding(1, 3)

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
	Kind       string // "layout" or "template"
	Name       string
	Icon       string // template icon (e.g. "⧉"), empty for layouts
	Meta       string // "3 tabs  May 28" or "editor+git+term"
	SearchText string // extra searchable text (workspace titles, etc.) — not displayed
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

func (s fuzzySource) String(i int) string {
	text := s[i].Name + " " + s[i].Meta
	if s[i].SearchText != "" {
		text += " " + s[i].SearchText
	}
	return text
}
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
		if m.chosen.Kind == "workspace" {
			// Workspace items carry the layout name in SearchText.
			return &PopResult{
				Kind:           "workspace",
				Name:           m.chosen.SearchText, // layout name
				WorkspaceTitle: m.chosen.Name,        // workspace title
			}
		}
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
				return m, tea.ClearScreen
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
			// Skip "/" — it's a common search-mode trigger, not a filter char.
			if r == '/' {
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
		return m, tea.ClearScreen

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
			if r == '/' {
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
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	boxWidth := clampInt(w-6, 50, 80)
	boxHeight := clampInt(h-4, 14, 26)
	innerWidth := boxWidth - 8   // border(2) + padding(6)
	innerHeight := boxHeight - 4 // border(2) + padding(2)

	// Build the dynamic title centered at top.
	title := m.renderTitle(innerWidth)

	// Build the list or drill content.
	var body string
	if m.mode == modeDrill {
		body = m.viewDrill(innerWidth, innerHeight-3) // reserve title lines
	} else {
		body = m.viewList(innerWidth, innerHeight-3)
	}

	// Build the footer.
	footer := m.renderFooter()

	// Assemble: title + blank + body + blank + footer
	content := title + "\n\n" + body + "\n\n" + footer

	// No fixed Width — let box size from padded content lines.
	// boxWidth-2 absorbs lipgloss's emoji width miscounting in padding.
	box := popBoxStyle.Width(boxWidth).Height(boxHeight).Render(content)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// renderTitle builds the centered dynamic title line.
func (m *PopModel) renderTitle(width int) string {
	var title string
	switch {
	case m.mode == modeDrill && m.filter != "":
		title = popHeaderStyle.Render("🐦‍🔥 crex") + popDimStyle.Render("  ›  ") + popHeaderStyle.Render(m.drillLayout) + popDimStyle.Render("  ›  ") + popFilterStyle.Render("🔍 "+m.filter)
	case m.mode == modeDrill:
		title = popHeaderStyle.Render("🐦‍🔥 crex") + popDimStyle.Render("  ›  ") + popHeaderStyle.Render(m.drillLayout)
	case m.filter != "":
		title = popFilterStyle.Render("🔍 " + m.filter)
	default:
		title = popHeaderStyle.Render("🐦‍🔥 crex")
	}
	// Center the title within the inner width.
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

// renderFooter builds the contextual hint line.
func (m *PopModel) renderFooter() string {
	if m.mode == modeDrill {
		return popDimStyle.Render("↵ restore    esc back    1-9 jump")
	}
	return popDimStyle.Render("↵ launch    tab drill    1-9 jump    esc quit")
}

// viewList renders the list mode content (no header/footer — those are in View).
func (m *PopModel) viewList(innerWidth, listHeight int) string {
	if listHeight < 1 {
		listHeight = 1
	}

	if len(m.filtered) == 0 {
		return popDimStyle.Render("  no results")
	}

	// Build all visible lines first, then apply scroll.
	var lines []string
	idx := 0
	lastKind := ""
	for i, item := range m.filtered {
		// Section header when kind changes (with spacing).
		if item.Kind != lastKind {
			if lastKind != "" {
				lines = append(lines, strings.Repeat(" ", innerWidth))
			}
			section := "LAYOUTS"
			switch item.Kind {
			case "template":
				section = "TEMPLATES"
			case "workspace":
				section = "WORKSPACES"
			}
			lines = append(lines, padLine(popSectionStyle.Render(section), innerWidth))
			lines = append(lines, strings.Repeat(" ", innerWidth))
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

		// Truncate name to prevent wrapping with wide emojis.
		maxName := innerWidth - 40
		if maxName < 12 {
			maxName = 12
		}
		itemName := truncLine(item.Name, maxName)
		nameStr := itemName
		if m.matchPositions != nil && i < len(m.matchPositions) && len(m.matchPositions[i]) > 0 {
			if isCurrent {
				nameStr = highlightMatchesBold(itemName, m.matchPositions[i])
			} else {
				nameStr = highlightMatches(itemName, m.matchPositions[i])
			}
		} else if isCurrent {
			nameStr = popTitleStyle.Render(itemName)
		}
		line.WriteString(nameStr)

		// Show source layout right after name for workspace items.
		if item.Kind == "workspace" && item.SearchText != "" {
			line.WriteString("  ")
			line.WriteString(popDimStyle.Render("‹" + item.SearchText + "›"))
		}

		if item.Meta != "" {
			line.WriteString("   ")
			line.WriteString(popMetaStyle.Render(item.Meta))
		}

		// Drill indicator for layouts under cursor.
		if isCurrent && item.Kind == "layout" {
			line.WriteString("  ")
			line.WriteString(popDimStyle.Render("→"))
		}

		// Pad to exact innerWidth to prevent bubbletea redraw artifacts.
		lines = append(lines, padLine(line.String(), innerWidth))
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
	var b strings.Builder
	end := m.offset + listHeight
	if end > len(lines) {
		end = len(lines)
	}
	for i := m.offset; i < end; i++ {
		b.WriteString(lines[i])
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// termWidth returns the actual terminal cell width of a (possibly ANSI-styled) string.
// It uses rivo/uniseg's grapheme-cluster-aware width, which correctly handles complex
// ZWJ emoji sequences (e.g. 🕵🏼‍♀️) that charmbracelet/x/ansi.StringWidth undercounts.
// ANSI escape codes are stripped before measurement.
func termWidth(s string) int {
	return uniseg.StringWidth(ansi.Strip(s))
}

// sanitizeEmoji replaces complex ZWJ emoji sequences with their base character
// followed by a space. This prevents lipgloss width miscounting that causes
// line wrapping in the terminal. Simple emojis (🧠, 🗿, 🌋) are left as-is.
func sanitizeEmoji(s string) string {
	// Process grapheme clusters; replace any cluster wider than 2 cells
	// or containing ZWJ (U+200D) with a simpler representation.
	var result strings.Builder
	remaining := s
	for len(remaining) > 0 {
		cluster, rest, clusterWidth, _ := uniseg.FirstGraphemeClusterInString(remaining, -1)
		lipW := ansi.StringWidth(cluster)
		if clusterWidth != lipW {
			// Mismatch — lipgloss will miscount this.
			// Replace with a generic marker to keep alignment correct.
			result.WriteString("· ")
		} else {
			result.WriteString(cluster)
		}
		remaining = rest
	}
	return result.String()
}

// truncLine ensures a string doesn't exceed maxLen terminal cells.
// Uses termWidth for accurate ZWJ emoji measurement.
func truncLine(s string, maxLen int) string {
	if termWidth(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "..."
		if termWidth(candidate) <= maxLen {
			return candidate
		}
	}
	return "..."
}

// padLine pads a styled line to exactly width terminal cells with trailing spaces.
// Uses termWidth for accurate ZWJ emoji measurement to prevent line wrapping.
func padLine(s string, width int) string {
	w := termWidth(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	// Line exceeds width (emoji miscount) — truncate runes from end.
	runes := []rune(s)
	for i := len(runes) - 1; i >= 0; i-- {
		candidate := string(runes[:i])
		cw := termWidth(candidate)
		if cw <= width {
			return candidate + strings.Repeat(" ", width-cw)
		}
	}
	return strings.Repeat(" ", width)
}

// viewDrill renders the drill mode content (no header/footer — those are in View).
func (m *PopModel) viewDrill(innerWidth, listHeight int) string {
	if listHeight < 1 {
		listHeight = 1
	}

	sectionHeader := popSectionStyle.Render(fmt.Sprintf("WORKSPACES"))

	if len(m.drillFiltered) == 0 {
		return sectionHeader + "\n\n" + popDimStyle.Render("  no results")
	}

	var lines []string
	lines = append(lines, padLine(sectionHeader, innerWidth))
	lines = append(lines, strings.Repeat(" ", innerWidth)) // breathing space

	for idx, item := range m.drillFiltered {
		num := fmt.Sprintf("[%d]", idx+1)
		isCurrent := idx == m.drillCursor

		// Truncate title to safe width BEFORE styling.
		// Prefix ≈ 8 cells, suffix ≈ 30 cells, emoji safety margin for ZWJ sequences.
		maxTitle := innerWidth - 40
		if maxTitle < 12 {
			maxTitle = 12
		}
		title := truncLine(item.Title, maxTitle)

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

		if isCurrent {
			line.WriteString(popTitleStyle.Render(title))
		} else {
			line.WriteString(title)
		}

		paneInfo := fmt.Sprintf("%d %s", item.PaneCount, pluralPane(item.PaneCount))
		line.WriteString("  ")
		line.WriteString(popMetaStyle.Render(paneInfo))

		if item.PaneSummary != "" {
			summary := truncLine(item.PaneSummary, 20)
			line.WriteString("  ")
			line.WriteString(popDimStyle.Render(summary))
		}

		// Pad to exact innerWidth to prevent bubbletea redraw artifacts.
		lines = append(lines, padLine(line.String(), innerWidth))
	}

	// Scroll: cursor line = drillCursor + 2 (section header + blank)
	cursorLine := m.drillCursor + 2

	if cursorLine < m.drillOffset {
		m.drillOffset = cursorLine
	}
	if cursorLine >= m.drillOffset+listHeight {
		m.drillOffset = cursorLine - listHeight + 1
	}
	if m.drillOffset < 0 {
		m.drillOffset = 0
	}

	var b strings.Builder
	end := m.drillOffset + listHeight
	if end > len(lines) {
		end = len(lines)
	}
	for i := m.drillOffset; i < end; i++ {
		b.WriteString(lines[i])
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// applyFilter rebuilds m.filtered using fuzzy matching on m.filter.
func (m *PopModel) applyFilter() {
	if m.filter == "" {
		// No filter: show layouts + templates only (hide individual workspaces).
		m.filtered = nil
		for _, item := range m.items {
			if item.Kind != "workspace" {
				m.filtered = append(m.filtered, item)
			}
		}
		m.matchPositions = nil
		m.cursor = 0
		m.offset = 0
		return
	}

	matches := fuzzy.FindFrom(m.filter, fuzzySource(m.items))

	// Filter out low-quality fuzzy matches — reject results where the
	// matched characters are too scattered (e.g. "drolos" matching "porra"
	// via scattered d-r-o-l-o-s). Require that the match covers at least
	// half the filter length in a contiguous-ish span.
	filterLen := len([]rune(m.filter))
	var goodMatches []fuzzy.Match
	for _, match := range matches {
		if len(match.MatchedIndexes) == 0 {
			continue
		}
		// Span = distance from first to last matched char.
		span := match.MatchedIndexes[len(match.MatchedIndexes)-1] - match.MatchedIndexes[0] + 1
		// Accept if the matched chars are clustered (span <= 2x filter length).
		if span <= filterLen*2 {
			goodMatches = append(goodMatches, match)
		}
	}

	// Group by kind: layouts first, workspaces second, templates third.
	// Workspaces with the same title in different layouts are shown separately
	// (each displays its source layout name so the user can choose).
	seenWs := make(map[string]bool) // "title|layout" → dedup within same layout
	var layouts, workspaces, templates []PopItem
	var layoutPos, workspacePos, templatePos [][]int

	for _, match := range goodMatches {
		item := m.items[match.Index]
		if item.Kind == "workspace" {
			key := item.Name + "|" + item.SearchText // title + layout
			if seenWs[key] {
				continue
			}
			seenWs[key] = true
			workspaces = append(workspaces, item)
			workspacePos = append(workspacePos, match.MatchedIndexes)
		} else if item.Kind == "template" {
			templates = append(templates, item)
			templatePos = append(templatePos, match.MatchedIndexes)
		} else {
			layouts = append(layouts, item)
			layoutPos = append(layoutPos, match.MatchedIndexes)
		}
	}

	m.filtered = nil
	m.matchPositions = nil
	m.filtered = append(m.filtered, layouts...)
	m.matchPositions = append(m.matchPositions, layoutPos...)
	m.filtered = append(m.filtered, workspaces...)
	m.matchPositions = append(m.matchPositions, workspacePos...)
	m.filtered = append(m.filtered, templates...)
	m.matchPositions = append(m.matchPositions, templatePos...)

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

		// Sanitize title: strip newlines, carriage returns, and trailing whitespace.
		title := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(ws.Title, "\n", ""), "\r", ""))
		if title == "" {
			title = "(untitled)"
		}
		// Sanitize emoji to prevent lipgloss width miscounting.
		title = sanitizeEmoji(title)

		m.drillItems = append(m.drillItems, DrillItem{
			LayoutName:  layoutName,
			Index:       ws.Index,
			Title:       title,
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
