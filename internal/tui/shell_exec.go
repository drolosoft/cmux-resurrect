package tui

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
)

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
	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Saved %q — %d %s", name, count, label)))
	m.output.WriteString("\n\n")
}

// execRestore restores a saved layout by name.
func (m *ShellModel) execRestore(name string) {
	if m.client == nil {
		m.output.WriteString(shellErrorStyle.Render("  ✗ No backend connected"))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  Restoring %q…", name)))
	m.output.WriteString("\n")

	restorer := &orchestrate.Restorer{
		Client: m.client,
		Store:  m.store,
		OnProgress: func(title string, panes int, err error) {
			if err != nil {
				m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  ⚠ %s: %v", title, err)))
			} else {
				m.output.WriteString(shellDimStyle.Render(fmt.Sprintf("  ✓ %s (%d pane(s))", title, panes)))
			}
			m.output.WriteString("\n")
		},
	}

	result, err := restorer.Restore(name, false, orchestrate.RestoreModeAdd)
	if err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	label := unitLabel(m.backend, result.WorkspacesOK)
	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Restored %d/%d %s", result.WorkspacesOK, result.WorkspacesTotal, label)))
	if len(result.Errors) > 0 {
		m.output.WriteString(shellDimStyle.Render(fmt.Sprintf(" (%d error(s))", len(result.Errors))))
	}
	m.output.WriteString("\n\n")
}

// execDelete enters confirmation mode for deleting a saved layout.
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

	if err := mdfile.RemoveProject(m.wsFile, name); err != nil {
		m.output.WriteString(shellErrorStyle.Render(fmt.Sprintf("  ✗ %v", err)))
		m.output.WriteString("\n\n")
		return
	}

	m.output.WriteString(shellSuccessStyle.Render(fmt.Sprintf("  ✓ Removed %q from Blueprint", name)))
	m.output.WriteString("\n\n")
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
}
