package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
)

// restoreResultMsg carries the result of an async restore operation.
type restoreResultMsg struct {
	result *orchestrate.RestoreResult
	err    error
}

// execNow renders the current live workspace/tab tree. Read-only — does not
// populate lastItems or enter browse mode.
func (m *ShellModel) execNow() {
	tree, err := m.client.Tree()
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	label := unitLabel(m.backend, 2)
	// Capitalize first letter without deprecated strings.Title.
	heading := strings.ToUpper(label[:1]) + label[1:]
	m.output.WriteString(shellHeadingStyle.Render(fmt.Sprintf("  Current %s", heading)))
	m.output.WriteString("\n")

	home, _ := os.UserHomeDir()

	total := 0
	for _, win := range tree.Windows {
		for _, ws := range win.Workspaces {
			total++
			// Indicator badges.
			var badges []string
			if ws.Pinned {
				badges = append(badges, "📌")
			}
			if ws.Active || ws.Selected {
				badges = append(badges, "●")
			}
			badgeStr := ""
			if len(badges) > 0 {
				badgeStr = " " + strings.Join(badges, " ")
			}

			// Title.
			fmt.Fprintf(m.output, "  %s%s", shellSuccessStyle.Render(ws.Title), badgeStr)

			// CWD from sidebar-state (best effort).
			if ws.Ref != "" {
				if sidebar, err := m.client.SidebarState(ws.Ref); err == nil && sidebar.CWD != "" {
					cwd := sidebar.CWD
					if home != "" {
						cwd = strings.Replace(cwd, home, "~", 1)
					}
					m.output.WriteString("  ")
					m.output.WriteString(shellDimStyle.Render(cwd))
				}
			}

			// Pane count.
			if len(ws.Panes) > 0 {
				m.output.WriteString("  ")
				m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("%d pane(s)", len(ws.Panes))))
			}

			m.output.WriteString("\n")
		}
	}

	if total == 0 {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  No %s found.", label)))
		m.output.WriteString("\n")
	}
	m.output.WriteString("\n")
}

// execList lists saved layouts and enters browse mode.
func (m *ShellModel) execList() {
	if m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No store configured"))
		m.output.WriteString("\n\n")
		return
	}

	metas, err := m.store.List()
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	if len(metas) == 0 {
		m.output.WriteString(shellDimStyle.Render("  No saved layouts. Try: save my-day"))
		m.output.WriteString("\n\n")
		return
	}

	items := ItemsFromLayouts(metas)
	m.lastItems = items
	m.browse = NewBrowseModel(items, "restore")
	m.mode = modeBrowse
}

// execSave saves the current layout under name.
func (m *ShellModel) execSave(name string) {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return
	}

	saver := &orchestrate.Saver{
		Client: m.client,
		Store:  m.store,
	}

	layout, err := saver.Save(name, "")
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	count := len(layout.Workspaces)
	label := unitLabel(m.backend, count)
	browserCount := 0
	for _, ws := range layout.Workspaces {
		for _, p := range ws.Panes {
			if p.Type == "browser" {
				browserCount++
			}
		}
	}
	msg := fmt.Sprintf("  ✓ Saved %q — %d %s", name, count, label)
	if browserCount > 0 {
		msg += fmt.Sprintf(" (%d browser)", browserCount)
	}
	m.output.WriteString(shellSuccessStyle.Render(msg))
	m.output.WriteString("\n\n")
	m.completer.Invalidate()
}

// execRestore restores a saved layout by name, optionally filtered to a single workspace.
func (m *ShellModel) execRestore(name string, workspaceFilter string) tea.Cmd {
	// Explicit mode from settings — skip detection.
	switch m.restoreMode {
	case "replace":
		return m.startRestore(name, workspaceFilter, orchestrate.RestoreModeReplace, true)
	case "add":
		return m.startRestore(name, workspaceFilter, orchestrate.RestoreModeAdd, true)
	}

	// "ask" or empty — detect state to decide which prompts to show.
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return nil
	}

	layout, err := m.store.Load(name)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return nil
	}
	titles := make([]string, len(layout.Workspaces))
	for i, ws := range layout.Workspaces {
		titles[i] = ws.Title
	}
	detection := orchestrate.DetectRestoreState(m.client, titles)

	switch detection.Hint {
	case orchestrate.HintNoop:
		m.output.WriteString(shellDimStyle.Render("  Layout already matches current tabs. Nothing to do."))
		m.output.WriteString("\n\n")
		return nil
	case orchestrate.HintAutoAdd:
		return m.startRestore(name, workspaceFilter, orchestrate.RestoreModeAdd, true)
	case orchestrate.HintAskFresh:
		// Matching tabs exist but no extras — go straight to Skip/Fresh.
		m.restoreAskName = name
		m.restoreAskFilter = workspaceFilter
		m.restoreSkipCursor = 0
		m.mode = modeRestoreSkip
		return nil
	default:
		m.restoreAskName = name
		m.restoreAskFilter = workspaceFilter
		m.restoreAskCursor = 0
		m.restoreAskHint = detection.Hint
		m.mode = modeRestoreAsk
		return nil
	}
}

// startRestore launches the async restore with a specific mode.
func (m *ShellModel) startRestore(name, workspaceFilter string, mode orchestrate.RestoreMode, skipMatching bool) tea.Cmd {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		m.flushOutput()
		return nil
	}

	if workspaceFilter != "" {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Restoring %q from %q…", workspaceFilter, name)))
	} else {
		action := "Syncing (replace)"
		if mode == orchestrate.RestoreModeAdd {
			action = "Syncing (add)"
		}
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  %s %q…", action, name)))
	}
	m.output.WriteString("\n")
	m.flushOutput()

	cl := m.client
	store := m.store
	filter := workspaceFilter
	restoreMode := mode
	skip := skipMatching
	return func() tea.Msg {
		restorer := &orchestrate.Restorer{
			Client:     cl,
			Store:      store,
			AutoAccept: m.autoAccept,
		}
		result, err := restorer.Restore(name, false, restoreMode, filter, skip)
		return restoreResultMsg{result: result, err: err}
	}
}

// handleRestoreResult formats the result of an async restore.
func (m *ShellModel) handleRestoreResult(msg restoreResultMsg) {
	if msg.err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", msg.err)))
		m.output.WriteString("\n\n")
		return
	}
	r := msg.result
	label := unitLabel(m.backend, r.WorkspacesOK)
	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Restored %d/%d %s", r.WorkspacesOK, r.WorkspacesTotal, label)))
	if len(r.Errors) > 0 {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf(" (%d error(s))", len(r.Errors))))
		for _, e := range r.Errors {
			m.output.WriteString("\n")
			m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("    ⚠ %s", e)))
		}
	}
	m.output.WriteString("\n\n")
}

// execDelete enters confirmation mode for deleting a saved layout.
func (m *ShellModel) execRename(oldName, newName string) {
	if m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No store configured"))
		m.output.WriteString("\n\n")
		return
	}

	if err := m.store.Rename(oldName, newName); err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Renamed %q → %q", oldName, newName)))
	m.output.WriteString("\n\n")
	m.completer.Invalidate()
}

func (m *ShellModel) execDelete(name string) {
	if m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No store configured"))
		m.output.WriteString("\n\n")
		return
	}

	if !m.store.Exists(name) {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Layout %q not found", name)))
		m.output.WriteString("\n\n")
		return
	}

	m.confirmMsg = shellErrorStyle.Render(fmt.Sprintf("  Delete %q? [y/N]", name))
	m.confirmFn = func() {
		if err := m.store.Delete(name); err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
			m.output.WriteString("\n")
		} else {
			m.completer.Invalidate()
		}
	}
	m.mode = modeConfirm
}

// execTemplates lists gallery templates and enters browse mode.
func (m *ShellModel) execTemplates() {
	templates := gallery.List()
	if len(templates) == 0 {
		m.output.WriteString(shellDimStyle.Render("  No templates available."))
		m.output.WriteString("\n\n")
		return
	}

	items := ItemsFromTemplates(templates)
	m.lastItems = items
	m.browse = NewBrowseModel(items, "use")
	m.mode = modeBrowse
}

// execUse applies a gallery template by name.
func (m *ShellModel) execUse(name string) {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return
	}

	tmpl, ok := gallery.Get(name)
	if !ok {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Template %q not found", name)))
		m.output.WriteString("\n\n")
		return
	}

	panes := gallery.BuildPanes(tmpl)

	user := &orchestrate.TemplateUser{
		Client: m.client,
		OnProgress: func(msg string) {
			m.output.WriteString(shellDimStyle.Render("  " + msg))
			m.output.WriteString("\n")
		},
	}

	opts := orchestrate.TemplateUseOpts{
		Title: tmpl.Name,
		Icon:  tmpl.Icon,
	}

	result, err := user.Use(panes, opts, false)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	label := unitLabel(m.backend, 1)
	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Created %s %q with %d pane(s)", label, result.Title, result.Panes)))
	m.output.WriteString("\n\n")
}

// execWatch handles watch subcommands: status, start, stop.
func (m *ShellModel) execWatch(sub string) {
	pidPath := orchestrate.DefaultPIDPath()

	switch sub {
	case "status", "":
		running, pid := orchestrate.IsDaemonRunning(pidPath)
		if running {
			m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ watch daemon running (pid %d)", pid)))
		} else {
			m.output.WriteString(shellDimStyle.Render("  watch daemon is not running"))
		}
		m.output.WriteString("\n\n")

	case "start":
		m.output.WriteString(shellDimStyle.Render("  Run: crex watch --daemon"))
		m.output.WriteString("\n\n")

	case "stop":
		running, pid := orchestrate.IsDaemonRunning(pidPath)
		if !running {
			m.output.WriteString(shellDimStyle.Render("  watch daemon is not running"))
			m.output.WriteString("\n\n")
			return
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ find process: %v", err)))
			m.output.WriteString("\n\n")
			return
		}
		if err := proc.Signal(syscall.SIGINT); err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ signal: %v", err)))
			m.output.WriteString("\n\n")
			return
		}
		orchestrate.RemovePIDFile(pidPath)
		m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Stopped watch daemon (pid %d)", pid)))
		m.output.WriteString("\n\n")

	default:
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Unknown watch subcommand: %s", sub)))
		m.output.WriteString("\n")
		m.output.WriteString(shellDimStyle.Render("  Usage: watch status|start|stop"))
		m.output.WriteString("\n\n")
	}
}

// execBpAdd adds a project to the workspace Blueprint file.
func (m *ShellModel) execBpAdd(name, path string) {
	if m.wsFile == "" {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No workspace Blueprint file configured"))
		m.output.WriteString("\n\n")
		return
	}

	p := model.Project{
		Enabled:  true,
		Name:     name,
		Path:     path,
		Template: "dev",
		Pin:      true,
	}

	if err := mdfile.AddProject(m.wsFile, p); err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Added %q to Blueprint", name)))
	m.output.WriteString("\n\n")
	m.completer.Invalidate()
}

// execBpList lists all Blueprint projects and enters browse mode.
func (m *ShellModel) execBpList() {
	if m.wsFile == "" {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No workspace Blueprint file configured"))
		m.output.WriteString("\n\n")
		return
	}

	wf, err := mdfile.Parse(m.wsFile)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	if len(wf.Projects) == 0 {
		m.output.WriteString(shellDimStyle.Render("  No Blueprint projects. Try: bp add my-project ~/path"))
		m.output.WriteString("\n\n")
		return
	}

	// Convert projects to Items for browse mode.
	items := make([]Item, len(wf.Projects))
	for i, p := range wf.Projects {
		desc := p.Path
		if p.Enabled {
			desc = "enabled · " + desc
		} else {
			desc = "disabled · " + desc
		}
		items[i] = Item{
			Kind:        KindLayout,
			Name:        p.Name,
			Description: desc,
			Icon:        p.Icon,
		}
	}

	m.lastItems = items
	m.browse = NewBrowseModel(items, "toggle")
	m.mode = modeBrowse
}

// execBpRemove removes a Blueprint project by name.
func (m *ShellModel) execBpRemove(name string) {
	if m.wsFile == "" {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No workspace Blueprint file configured"))
		m.output.WriteString("\n\n")
		return
	}

	m.confirmMsg = shellErrorStyle.Render(fmt.Sprintf("  Remove %q from Blueprint? [y/N]", name))
	m.confirmFn = func() {
		if err := mdfile.RemoveProject(m.wsFile, name); err != nil {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
			m.output.WriteString("\n")
		} else {
			m.completer.Invalidate()
		}
	}
	m.mode = modeConfirm
}

// editDoneMsg signals that the external editor process has exited.
type editDoneMsg struct{ err error }

// execShow renders detailed layout information (workspaces, panes, CWDs).
func (m *ShellModel) execShow(name string) {
	if m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No store configured"))
		m.output.WriteString("\n\n")
		return
	}

	layout, err := m.store.Load(name)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	// Header.
	m.output.WriteString(shellHeadingStyle.Render(fmt.Sprintf("  📦 %s", layout.Name)))
	m.output.WriteString("\n")
	if layout.Description != "" {
		m.output.WriteString(shellDimStyle.Render("  " + layout.Description))
		m.output.WriteString("\n")
	}
	saved := layout.SavedAt.Local().Format("Jan 02, 2006 15:04")
	label := unitLabel(m.backend, len(layout.Workspaces))
	m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Saved %s · %d %s", saved, len(layout.Workspaces), label)))
	m.output.WriteString("\n\n")

	home, _ := os.UserHomeDir()

	for _, ws := range layout.Workspaces {
		title := shellSuccessStyle.Render(ws.Title)
		badges := ""
		if ws.Pinned {
			badges += " 📌"
		}
		if ws.Active {
			badges += " " + shellDimStyle.Render("◀ active")
		}
		fmt.Fprintf(m.output, "  %s%s\n", title, badges)

		// CWD.
		cwd := ws.CWD
		if home != "" {
			cwd = strings.Replace(cwd, home, "~", 1)
		}
		fmt.Fprintf(m.output, "  %s\n", shellDimStyle.Render("cwd "+cwd))

		// Pane tree.
		for i, p := range ws.Panes {
			isLast := i == len(ws.Panes)-1
			prefix := "├──"
			if isLast {
				prefix = "└──"
			}
			prefix = shellDimStyle.Render(prefix)

			var desc string
			if p.Split != "" {
				desc = shellFlameStyle.Render("→"+p.Split) + " "
			}
			if p.Command != "" {
				cmd := p.Command
				if len(cmd) > 50 {
					cmd = cmd[:47] + "..."
				}
				desc += shellSuccessStyle.Render(cmd)
			} else {
				desc += shellDimStyle.Render("shell")
			}
			if p.Focus {
				desc += " " + shellFlameStyle.Render("★")
			}

			fmt.Fprintf(m.output, "  %s %s\n", prefix, desc)
		}
		m.output.WriteString("\n")
	}
}

// execEdit suspends the TUI and opens the layout file in $EDITOR.
func (m *ShellModel) execEdit(name string) tea.Cmd {
	if m.store == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No store configured"))
		m.output.WriteString("\n\n")
		return nil
	}

	if !m.store.Exists(name) {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Layout %q not found", name)))
		m.output.WriteString("\n\n")
		return nil
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	path := m.store.Path(name)
	parts := strings.Fields(editor)
	c := exec.Command(parts[0], append(parts[1:], path)...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editDoneMsg{err: err}
	})
}

// execTemplateShow renders a template card with ASCII diagram and metadata.
func (m *ShellModel) execTemplateShow(name string) {
	tmpl, ok := gallery.Get(name)
	if !ok {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Template %q not found", name)))
		m.output.WriteString("\n\n")
		return
	}

	// Header: icon + name — description.
	fmt.Fprintf(m.output, "\n  %s %s — %s\n\n",
		tmpl.Icon,
		shellSuccessStyle.Render(tmpl.Name),
		shellDimStyle.Render(tmpl.Description))

	// Metadata (pad labels to 10 chars for consistent column alignment).
	label := func(s string) string { return shellDimStyle.Render(fmt.Sprintf("%-10s", s)) }
	fmt.Fprintf(m.output, "  %s %s\n", label("Category:"), shellSuccessStyle.Render(tmpl.Category))
	fmt.Fprintf(m.output, "  %s %s\n", label("Panes:"), shellSuccessStyle.Render(fmt.Sprintf("%d", len(tmpl.Panes))))

	// Split sequence.
	splits := []string{"main"}
	for _, p := range tmpl.Panes {
		if p.Split != "" {
			splits = append(splits, p.Split)
		}
	}
	fmt.Fprintf(m.output, "  %s %s\n", label("Splits:"), shellSuccessStyle.Render(strings.Join(splits, " → ")))

	if len(tmpl.Tags) > 0 {
		fmt.Fprintf(m.output, "  %s %s\n", label("Tags:"), shellSuccessStyle.Render(strings.Join(tmpl.Tags, ", ")))
	}
	m.output.WriteString("\n")
}

// execTemplateCustomize copies a gallery template into the workspace Blueprint.
func (m *ShellModel) execTemplateCustomize(name string) {
	tmpl, ok := gallery.Get(name)
	if !ok {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Template %q not found", name)))
		m.output.WriteString("\n\n")
		return
	}

	if m.wsFile == "" {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No workspace Blueprint file configured"))
		m.output.WriteString("\n\n")
		return
	}

	wf, err := mdfile.Parse(m.wsFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			wf = &model.WorkspaceFile{
				Templates: make(map[string]*model.Template),
			}
		} else {
			m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
			m.output.WriteString("\n\n")
			return
		}
	}

	if wf.Templates == nil {
		wf.Templates = make(map[string]*model.Template)
	}
	if _, exists := wf.Templates[name]; exists {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ Template %q already exists in your Blueprint", name)))
		m.output.WriteString("\n\n")
		return
	}

	userTmpl := &model.Template{Name: tmpl.Name}
	for _, tp := range tmpl.Panes {
		pane := tp
		pane.FocusTarget = -1
		userTmpl.Panes = append(userTmpl.Panes, pane)
	}
	wf.Templates[name] = userTmpl

	if err := mdfile.Write(m.wsFile, wf); err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Copied %q to your Blueprint", name)))
	m.output.WriteString("\n")
	m.output.WriteString(shellDimStyle.Render("  Your copy now takes priority over the built-in."))
	m.output.WriteString("\n\n")
	m.completer.Invalidate()
}

// execImport imports workspaces from the Blueprint file.
func (m *ShellModel) execImport() {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return
	}

	if m.wsFile == "" {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No workspace Blueprint file configured"))
		m.output.WriteString("\n\n")
		return
	}

	wf, err := mdfile.Parse(m.wsFile)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	enabled := wf.EnabledProjects()
	if len(enabled) == 0 {
		m.output.WriteString(shellDimStyle.Render("  No enabled entries in Blueprint."))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  📥 Importing %d entries from Blueprint…", len(enabled))))
	m.output.WriteString("\n\n")

	importer := &orchestrate.Importer{
		Client:     m.client,
		AutoAccept: m.autoAccept,
		OnProgress: func(event orchestrate.ImportEvent) {
			switch event.Status {
			case orchestrate.ImportCreated:
				fmt.Fprintf(m.output, "  %s  %s (%d panes)\n",
					shellSuccessStyle.Render("OK"),
					event.Title,
					len(event.Panes))
			case orchestrate.ImportSkipped:
				fmt.Fprintf(m.output, "  %s  %s %s\n",
					shellDimStyle.Render("SKIP"),
					event.Title,
					shellDimStyle.Render("(already exists)"))
			case orchestrate.ImportFailed:
				fmt.Fprintf(m.output, "  %s  %s: %v\n",
					shellErrorStyle.Render("FAIL"),
					event.Title,
					event.Err)
			case orchestrate.ImportWarn:
				fmt.Fprintf(m.output, "  %s  %s\n",
					shellErrorStyle.Render("WARN"),
					event.Warn)
			}
		},
	}

	result, err := importer.ImportFromMD(wf, false)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString("\n")
	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Import complete: %d created, %d skipped", result.Created, result.Skipped)))
	m.output.WriteString("\n\n")
	m.completer.Invalidate()
}

// execExport exports live state to the Blueprint file.
func (m *ShellModel) execExport() {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return
	}

	if m.wsFile == "" {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No workspace Blueprint file configured"))
		m.output.WriteString("\n\n")
		return
	}

	exporter := &orchestrate.Exporter{Client: m.client}
	if err := exporter.ExportToMD(m.wsFile); err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	// Read back for the summary.
	wf, err := mdfile.Parse(m.wsFile)
	if err != nil {
		m.output.WriteString(shellSuccessStyle.Render("  ✓ Exported to Blueprint"))
		m.output.WriteString("\n\n")
		return
	}

	label := unitLabel(m.backend, len(wf.Projects))
	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Exported %d %s to Blueprint", len(wf.Projects), label)))
	m.output.WriteString("\n\n")
	m.completer.Invalidate()
}

// execBpToggle toggles the enabled state of a Blueprint project by name.
func (m *ShellModel) execBpToggle(name string) {
	if m.wsFile == "" {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No workspace Blueprint file configured"))
		m.output.WriteString("\n\n")
		return
	}

	enabled, err := mdfile.ToggleProject(m.wsFile, name)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}
	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ %q is now %s", name, state)))
	m.output.WriteString("\n\n")
	m.completer.Invalidate()
}

// execUpdate checks for a newer crex release and updates if possible.
func (m *ShellModel) execUpdate() {
	m.output.WriteString(shellDimStyle.Render("  Checking for updates..."))
	m.output.WriteString("\n")

	result := orchestrate.RunUpdate(m.version)

	if result.Err != nil && result.NewVersion == "" {
		m.writeError(fmt.Sprintf("%v", result.Err))
		return
	}

	fmt.Fprintf(m.output, "  Current: %s  Latest: %s\n",
		shellDimStyle.Render(result.OldVersion),
		shellSuccessStyle.Render(result.NewVersion))

	if result.AlreadyLatest {
		m.output.WriteString(shellSuccessStyle.Render("  ✓ Already at the latest version."))
		m.output.WriteString("\n\n")
		return
	}

	fmt.Fprintf(m.output, "  Detected install method: %s\n", shellDimStyle.Render(result.Method.String()))

	if result.Method == orchestrate.InstallManual {
		fmt.Fprintf(m.output, "  Download the latest release at:\n  %s\n\n", result.ManualURL)
		return
	}

	m.output.WriteString(shellDimStyle.Render("  Updating..."))
	m.output.WriteString("\n")

	if result.Err != nil {
		m.writeError(fmt.Sprintf("Update failed: %v", result.Err))
		return
	}

	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Updated to %s", result.NewVersion)))
	m.output.WriteString("\n\n")
}
