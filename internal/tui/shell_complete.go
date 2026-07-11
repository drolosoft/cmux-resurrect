package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
)

// completionCacheTTL controls how long cached completion items remain valid.
// Keeps filesystem I/O out of the per-keystroke hot path while staying
// fresh enough that save/delete results appear within a couple of seconds.
const completionCacheTTL = 2 * time.Second

// completionEngine generates tab-completion candidates for the shell.
type completionEngine struct {
	store  persist.Store
	wsFile string

	// Cached data sources (avoid I/O on every keystroke).
	layoutCache   []completionItem
	layoutCacheAt time.Time
	bpCache       []completionItem
	bpCacheAt     time.Time
}

// completionItem pairs a fillable value with display metadata.
type completionItem struct {
	value string // what gets filled into the prompt
	icon  string
	desc  string
}

// --- Level-aware command definitions ----------------------------------------

// level1Commands are the top-level commands (no subcommand expansion).
var level1Commands = []completionItem{
	// Live
	{"now", "🖥", "Show current state"},
	{"watch", "⏱", "Auto-save daemon"},
	// Layouts (alphabetical)
	{"delete", "🗑", "Delete a saved layout"},
	{"edit", "📝", "Edit layout in $EDITOR"},
	{"rename", "✏️", "Rename a saved layout"},
	{"list", "📋", "List saved layouts"},
	{"ls", "📋", "List saved layouts"},
	{"pop", "🐦‍🔥", "Quick workspace launcher (Ctrl+G)"},
	{"restore", "🔄", "Restore a saved layout"},
	{"save", "💾", "Save current layout"},
	{"show", "🔎", "Show layout details"},
	// Templates (alphabetical)
	{"template", "📦", "Template commands…"},
	{"templates", "📦", "Browse template gallery"},
	{"use", "🚀", "Create from template"},
	// Blueprint (alphabetical)
	{"bp", "📐", "Blueprint commands…"},
	{"blueprint", "📐", "Blueprint commands…"},
	{"export", "📤", "Export to Blueprint"},
	{"import", "📥", "Import from Blueprint"},
	// Settings
	{"settings", "⚙️", "Settings & preferences"},
	// Shell
	{"skill", "🤖", "Agent skill…"},
	{"help", "❓", "Show help"},
	{"exit", "👋", "Exit the shell"},
	{"quit", "👋", "Exit the shell"},
}

// level2Subcommands are the subcommands for two-word command groups.
var level2Subcommands = map[string][]completionItem{
	"bp": {
		{"add", "➕", "Add Blueprint entry"},
		{"list", "📋", "List Blueprint entries"},
		{"ls", "📋", "List Blueprint entries"},
		{"remove", "🗑", "Remove Blueprint entry"},
		{"rm", "🗑", "Remove Blueprint entry"},
		{"toggle", "🔀", "Enable/disable entry"},
	},
	"blueprint": {
		{"add", "➕", "Add Blueprint entry"},
		{"list", "📋", "List Blueprint entries"},
		{"remove", "🗑", "Remove Blueprint entry"},
		{"toggle", "🔀", "Enable/disable entry"},
	},
	"template": {
		{"customize", "📝", "Copy template to Blueprint"},
		{"show", "🔎", "Show template details"},
	},
	"settings": {
		{"banner", "🎨", "Banner style"},
		{"restore-mode", "🔧", "Restore mode"},
	},
	"skill": {
		{"install", "🤖", "Install agent skill (Claude Code)"},
		{"show", "📄", "Print the agent skill"},
	},
	"watch": {
		{"start", "▶️", "Start daemon"},
		{"status", "📊", "Show daemon status"},
		{"stop", "⏹", "Stop daemon"},
	},
}

// nestedGroupPrefixes are level-2 subcommands that expand into level-3 commands.
var nestedGroupPrefixes = map[string]bool{
	"settings banner":       true,
	"settings restore-mode": true,
	"settings backend":      true,
}

// level3Subcommands are the subcommands for three-word command groups.
var level3Subcommands = map[string][]completionItem{
	"settings banner": {
		{"get", "🔍", "Show current style"},
		{"list", "📋", "List available styles"},
		{"set", "🎨", "Set banner style"},
	},
	"settings restore-mode": {
		{"set", "🔧", "Set restore mode"},
		{"get", "🔍", "Show current mode"},
		{"list", "📋", "List available modes"},
	},
	"settings backend": {
		{"set", "🔧", "Set default backend"},
		{"get", "🔍", "Show default backend"},
		{"list", "📋", "List available backends"},
	},
}

// singleCommands is the set of single-word commands that take arguments.
var singleCommands = map[string]bool{
	"help": true, "ls": true, "list": true, "now": true,
	"save": true, "restore": true, "show": true, "edit": true, "delete": true, "rename": true,
	"templates": true, "use": true, "watch": true,
	"import": true, "import-from-md": true, "export": true, "export-to-md": true,
	"exit": true, "quit": true, "?": true,
}

// twoWordPrefixes maps the first word of two-word commands to valid subcommands.
var twoWordPrefixes = map[string]bool{
	"bp": true, "blueprint": true, "template": true, "settings": true, "watch": true,
}

// --- Candidate generation ---------------------------------------------------

// completionResult holds both the fill values and the display items.
type completionResult struct {
	values []string
	items  []completionItem
}

// Complete returns completion candidates for the current input.
func (ce *completionEngine) Complete(input string) completionResult {
	trimmed := strings.TrimLeft(input, " ")

	// Empty input → level 1 commands.
	if trimmed == "" {
		return ce.level1Completions("")
	}

	parts := strings.Fields(trimmed)
	first := strings.ToLower(parts[0])

	// Check if first word is a two-word command group.
	if twoWordPrefixes[first] {
		trailingSpace := strings.HasSuffix(trimmed, " ")

		if len(parts) == 1 && !trailingSpace {
			// Still typing first word, e.g. "se" → suggest "settings", "save"
			return ce.level1Completions(trimmed)
		}

		// First word complete.
		sub := ""
		if len(parts) >= 2 {
			sub = strings.ToLower(parts[1])
		}

		// Check for nested group (e.g., "settings banner").
		nestedKey := first + " " + sub
		if sub != "" && nestedGroupPrefixes[nestedKey] {
			if len(parts) == 2 && !trailingSpace {
				// Still typing sub-group name: "settings ban"
				return ce.level2Completions(first, sub)
			}
			// Sub-group complete, handle level 3.
			l3partial := ""
			if len(parts) >= 3 {
				l3partial = parts[2]
			}
			if len(parts) >= 3 && trailingSpace {
				// Level 3 command complete → arguments.
				l3cmd := strings.ToLower(parts[2])
				fullCmd := nestedKey + " " + l3cmd
				prefix := nestedKey + " " + l3cmd + " "
				argPartial := ""
				if len(parts) >= 4 {
					argPartial = strings.Join(parts[3:], " ")
				}
				return ce.argCompletions(fullCmd, argPartial, prefix)
			}
			if len(parts) >= 4 {
				// Typing argument after level-3 command.
				l3cmd := strings.ToLower(parts[2])
				fullCmd := nestedKey + " " + l3cmd
				prefix := nestedKey + " " + l3cmd + " "
				argPartial := strings.Join(parts[3:], " ")
				return ce.argCompletions(fullCmd, argPartial, prefix)
			}
			// Show level 3 options.
			return ce.level3Completions(nestedKey, l3partial)
		}

		// Non-nested: original two-level behavior.
		partial := ""
		if len(parts) >= 2 {
			partial = parts[1]
		}
		if len(parts) >= 2 && trailingSpace {
			fullCmd := first + " " + sub
			prefix := first + " " + sub + " "
			argPartial := ""
			if len(parts) >= 3 {
				argPartial = parts[2]
			}
			return ce.argCompletions(fullCmd, argPartial, prefix)
		}
		if len(parts) >= 3 {
			fullCmd := first + " " + sub
			prefix := first + " " + sub + " "
			argPartial := strings.Join(parts[2:], " ")
			return ce.argCompletions(fullCmd, argPartial, prefix)
		}
		return ce.level2Completions(first, partial)
	}

	// Single-word command.
	if singleCommands[first] {
		if !strings.HasSuffix(trimmed, " ") && len(parts) == 1 {
			// Still typing the command name.
			return ce.level1Completions(trimmed)
		}

		// restore: second arg is workspace name within the layout.
		if first == "restore" && len(parts) >= 2 {
			trailingSpace := strings.HasSuffix(trimmed, " ")
			if len(parts) == 2 && trailingSpace {
				// "restore default " → complete workspace names.
				return ce.workspaceArgCompletions(parts[1], "", first+" "+parts[1]+" ")
			}
			if len(parts) >= 3 {
				// "restore default tra" → filter workspace names.
				wsPartial := strings.Join(parts[2:], " ")
				return ce.workspaceArgCompletions(parts[1], wsPartial, first+" "+parts[1]+" ")
			}
		}

		// Command complete, suggest arguments.
		prefix := first + " "
		argPartial := ""
		if len(parts) >= 2 {
			argPartial = strings.Join(parts[1:], " ")
		}
		return ce.argCompletions(first, argPartial, prefix)
	}

	// Unknown prefix — try matching level 1.
	return ce.level1Completions(trimmed)
}

func (ce *completionEngine) level1Completions(prefix string) completionResult {
	lower := strings.ToLower(strings.TrimSpace(prefix))
	var items []completionItem
	var values []string
	for _, c := range level1Commands {
		if lower == "" || strings.HasPrefix(c.value, lower) {
			items = append(items, c)
			values = append(values, c.value)
		}
	}
	return completionResult{values: values, items: items}
}

func (ce *completionEngine) level2Completions(group, partial string) completionResult {
	subs, ok := level2Subcommands[group]
	if !ok {
		return completionResult{}
	}
	lower := strings.ToLower(strings.TrimSpace(partial))
	var items []completionItem
	var values []string
	for _, s := range subs {
		if lower == "" || strings.HasPrefix(s.value, lower) {
			items = append(items, s)
			values = append(values, group+" "+s.value)
		}
	}
	return completionResult{values: values, items: items}
}

func (ce *completionEngine) level3Completions(nestedKey, partial string) completionResult {
	subs, ok := level3Subcommands[nestedKey]
	if !ok {
		return completionResult{}
	}
	lower := strings.ToLower(strings.TrimSpace(partial))
	var items []completionItem
	var values []string
	for _, s := range subs {
		if lower == "" || strings.HasPrefix(s.value, lower) {
			items = append(items, s)
			values = append(values, nestedKey+" "+s.value)
		}
	}
	return completionResult{values: values, items: items}
}

func (ce *completionEngine) argCompletions(cmd, partial, prefix string) completionResult {
	var argItems []completionItem

	switch cmd {
	case "restore", "delete", "save", "show", "edit", "rename":
		argItems = ce.layoutItems()
	case "use", "template show", "template customize":
		argItems = ce.templateItems()
	case "watch":
		return ce.level2Completions("watch", partial)
	case "settings banner set":
		argItems = []completionItem{
			{"flame", "🔥", "Gradient (ember → gold → green)"},
			{"classic", "🟢", "Solid green"},
			{"plain", "⬜", "Monochrome gray"},
		}
	case "settings restore-mode set":
		argItems = []completionItem{
			{"ask", "❓", "Prompt each time (default)"},
			{"replace", "🔄", "Always replace"},
			{"add", "➕", "Always add"},
		}
	case "bp remove", "bp rm", "bp toggle",
		"blueprint remove", "blueprint toggle":
		argItems = ce.blueprintItems()
	default:
		return completionResult{}
	}

	lowerPartial := strings.ToLower(strings.TrimSpace(partial))
	var items []completionItem
	var values []string
	for _, a := range argItems {
		if lowerPartial == "" || strings.HasPrefix(strings.ToLower(a.value), lowerPartial) {
			items = append(items, a)
			values = append(values, prefix+a.value)
		}
	}
	return completionResult{values: values, items: items}
}

// --- Icon width fix ---------------------------------------------------------

// textPresentationEmoji are emoji with Emoji_Presentation=No that Ghostty
// renders as 1-cell text glyphs. A trailing space aligns them with 2-cell emoji.
var textPresentationEmoji = map[string]bool{
	"🖥": true, // U+1F5A5
	"⏱": true, // U+23F1
	"🗑": true, // U+1F5D1
	"⏹": true, // U+23F9
}

func padIcon(icon string) string {
	if textPresentationEmoji[icon] {
		return icon + " "
	}
	return icon
}

// --- Rendering --------------------------------------------------------------

// RenderItems formats completion items with icons and descriptions.
func RenderItems(items []completionItem) string {
	return RenderItemsHighlighted(items, -1)
}

// RenderItemsHighlighted formats items with one highlighted by index.
func RenderItemsHighlighted(items []completionItem, highlight int) string {
	var b strings.Builder
	b.WriteString("\n")
	for i, it := range items {
		nameStyle := shellSuccessStyle
		descStyle := shellDimStyle
		marker := "  "
		if i == highlight {
			nameStyle = shellPromptStyle // bright green bold for selected
			marker = shellPromptStyle.Render("▸ ")
		}
		icon := padIcon(it.icon)
		if it.desc != "" {
			fmt.Fprintf(&b, "  %s%s  %s  %s\n",
				marker,
				icon,
				nameStyle.Render(fmt.Sprintf("%-14s", it.value)),
				descStyle.Render(it.desc))
		} else {
			fmt.Fprintf(&b, "  %s%s  %s\n",
				marker,
				icon,
				nameStyle.Render(it.value))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// --- Data source helpers ----------------------------------------------------

// Invalidate clears cached completion data, forcing a refresh on the next
// completion request. Call after save, delete, import, export, or bp mutations.
func (ce *completionEngine) Invalidate() {
	ce.layoutCacheAt = time.Time{}
	ce.bpCacheAt = time.Time{}
}

func (ce *completionEngine) layoutItems() []completionItem {
	if ce.store == nil {
		return nil
	}
	if time.Since(ce.layoutCacheAt) < completionCacheTTL {
		return ce.layoutCache
	}
	metas, err := ce.store.List()
	if err != nil {
		return nil
	}
	items := make([]completionItem, len(metas))
	for i, m := range metas {
		desc := m.Description
		if desc == "" {
			desc = fmt.Sprintf("%d tabs", m.WorkspaceCount)
		}
		items[i] = completionItem{m.Name, "📄", desc}
	}
	ce.layoutCache = items
	ce.layoutCacheAt = time.Now()
	return items
}

func (ce *completionEngine) workspaceItems(layoutName string) []completionItem {
	if ce.store == nil {
		return nil
	}
	layout, err := ce.store.Load(layoutName)
	if err != nil {
		return nil
	}
	items := make([]completionItem, len(layout.Workspaces))
	for i, ws := range layout.Workspaces {
		desc := fmt.Sprintf("%d panes", len(ws.Panes))
		items[i] = completionItem{ws.Title, "📋", desc}
	}
	return items
}

func (ce *completionEngine) workspaceArgCompletions(layoutName, partial, prefix string) completionResult {
	argItems := ce.workspaceItems(layoutName)
	lowerPartial := strings.ToLower(strings.TrimSpace(partial))
	var items []completionItem
	var values []string
	for _, a := range argItems {
		if lowerPartial == "" || strings.Contains(strings.ToLower(a.value), lowerPartial) {
			items = append(items, a)
			values = append(values, prefix+a.value)
		}
	}
	return completionResult{values: values, items: items}
}

func (ce *completionEngine) templateItems() []completionItem {
	templates := gallery.List()
	items := make([]completionItem, len(templates))
	for i, t := range templates {
		items[i] = completionItem{t.Name, t.Icon, t.Description}
	}
	return items
}

func (ce *completionEngine) blueprintItems() []completionItem {
	if ce.wsFile == "" {
		return nil
	}
	if time.Since(ce.bpCacheAt) < completionCacheTTL {
		return ce.bpCache
	}
	wf, err := mdfile.Parse(ce.wsFile)
	if err != nil {
		return nil
	}
	items := make([]completionItem, len(wf.Projects))
	for i, p := range wf.Projects {
		icon := p.Icon
		if icon == "" {
			icon = "📐"
		}
		items[i] = completionItem{p.Name, icon, p.Template}
	}
	ce.bpCache = items
	ce.bpCacheAt = time.Now()
	return items
}

// commonPrefix returns the longest common prefix of all strings.
func commonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	prefix := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}
