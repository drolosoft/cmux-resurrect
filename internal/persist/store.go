package persist

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	toml "github.com/pelletier/go-toml/v2"
)

// previousSuffix is appended to a layout file name for its previous version.
// List only picks files ending in ".toml", so these copies never show up as
// layouts of their own.
const previousSuffix = ".prev"

// ErrInvalidName is returned when a layout name contains path separators or traversal sequences.
var ErrInvalidName = errors.New("invalid layout name")

// validateName rejects names that contain path separators or ".." to prevent
// path traversal outside the layouts directory.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name must not be empty", ErrInvalidName)
	}
	if strings.Contains(name, "..") ||
		strings.Contains(name, "/") ||
		strings.Contains(name, string(filepath.Separator)) {
		return fmt.Errorf("%w: %q contains path separator or '..'", ErrInvalidName, name)
	}
	// Control characters (a newline especially) break the TOML header comment,
	// producing a file that saves but can never be loaded.
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: %q contains a control character", ErrInvalidName, name)
		}
	}
	return nil
}

// Store defines the interface for layout persistence.
type Store interface {
	Save(name string, layout *model.Layout) error
	Load(name string) (*model.Layout, error)
	List() ([]model.LayoutMeta, error)
	Delete(name string) error
	Rename(oldName, newName string) error
	Exists(name string) bool
	Path(name string) string
}

// FileStore implements Store using TOML files on disk.
type FileStore struct {
	Dir string
}

// NewFileStore creates a FileStore, ensuring the directory exists.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create layouts dir: %w", err)
	}
	return &FileStore{Dir: dir}, nil
}

func (s *FileStore) Path(name string) string {
	return filepath.Join(s.Dir, name+".toml")
}

// PreviousPath returns where the previous version of a layout is kept: the
// file a changing save or a delete leaves behind, so one mistake can be undone
// with a copy back over the layout file.
func (s *FileStore) PreviousPath(name string) string {
	return s.Path(name) + previousSuffix
}

func (s *FileStore) Exists(name string) bool {
	if validateName(name) != nil {
		return false
	}
	_, err := os.Stat(s.Path(name))
	return err == nil
}

// Save writes a layout to a TOML file atomically (temp + rename).
func (s *FileStore) Save(name string, layout *model.Layout) error {
	if err := validateName(name); err != nil {
		return err
	}

	data, content, err := encodeLayout(name, layout)
	if err != nil {
		return err
	}

	// Atomic write: temp file + rename
	target := s.Path(name)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Keep the version about to be replaced before the rename, and give up
	// the whole save if that copy cannot be made: a save that cannot be
	// undone is exactly what the copy is there to prevent.
	if err := s.keepPrevious(name, data); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("keep previous version of %q: %w", name, err)
	}

	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// encodeLayout returns the TOML of a layout (data) and the full file content
// with the header comment in front of it (content).
func encodeLayout(name string, layout *model.Layout) (data, content []byte, err error) {
	data, err = toml.Marshal(layout)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal layout: %w", err)
	}

	// Add a header comment
	header := fmt.Sprintf("# crex layout: %s\n# Saved at: %s\n\n",
		name, layout.SavedAt.Format("2006-01-02T15:04:05Z07:00"))
	content = append([]byte(header), data...)

	return data, content, nil
}

// writeFileAtomic writes content to path with permissions 0600 through a temp
// file and a rename, so a crash halfway never leaves a truncated file.
func writeFileAtomic(path string, content []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// keepPrevious copies the current file of a layout to its previous path when
// the save about to replace it changes the content. newData is the TOML of the
// new layout without its header. Nothing happens for a new layout or for a
// save that only refreshes the metadata, so an autosave that finds nothing new
// never pushes the good copy out.
func (s *FileStore) keepPrevious(name string, newData []byte) error {
	current, err := os.ReadFile(s.Path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read current file: %w", err)
	}

	if !contentChanged(current, newData) {
		return nil
	}

	return writeFileAtomic(s.PreviousPath(name), current)
}

// contentChanged reports whether two layout files describe different
// layouts. Both sides are parsed and written again without the fields that
// change on every save of the same windows (saved_at, revision and the
// description, which an autosave sets to "autosave"), so the header comment,
// the timestamp and formatting do not count. A file that does not parse
// counts as changed: keeping a copy of it costs nothing and may save it.
func contentChanged(oldData, newData []byte) bool {
	oldContent, err := comparableContent(oldData)
	if err != nil {
		return true
	}

	newContent, err := comparableContent(newData)
	if err != nil {
		return true
	}

	return !bytes.Equal(oldContent, newContent)
}

// comparableContent returns the TOML of a layout with its per-save metadata
// cleared, the form contentChanged compares.
func comparableContent(data []byte) ([]byte, error) {
	var layout model.Layout
	if err := toml.Unmarshal(data, &layout); err != nil {
		return nil, err
	}

	layout.SavedAt = time.Time{}
	layout.Revision = 0
	layout.Description = ""

	return toml.Marshal(&layout)
}

// Load reads and parses a layout TOML file.
func (s *FileStore) Load(name string) (*model.Layout, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.Path(name))
	if err != nil {
		return nil, fmt.Errorf("read layout %q: %w", name, err)
	}
	var layout model.Layout
	if err := toml.Unmarshal(data, &layout); err != nil {
		return nil, fmt.Errorf("parse layout %q: %w", name, err)
	}
	return &layout, nil
}

// layoutMemberships returns the group membership of every workspace, in the
// same order as the workspaces, so pickers built from metadata can group them.
// It returns nil for a layout without groups, which keeps the JSON of `crex
// list --json` for those layouts exactly as it was.
func layoutMemberships(layout *model.Layout) []model.Membership {
	if len(layout.Groups) == 0 {
		return nil
	}

	return layout.Memberships()
}

// List returns metadata for all saved layouts, sorted by name.
func (s *FileStore) List() ([]model.LayoutMeta, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list layouts dir: %w", err)
	}

	var metas []model.LayoutMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".toml")
		layout, err := s.Load(name)
		if err != nil {
			continue // skip corrupt files
		}
		titles := make([]string, len(layout.Workspaces))
		panes := make([]int, len(layout.Workspaces))
		summaries := make([]string, len(layout.Workspaces))
		for i, ws := range layout.Workspaces {
			titles[i] = ws.Title
			panes[i] = len(ws.Panes)
			summaries[i] = workspacePaneSummary(ws)
		}
		metas = append(metas, model.LayoutMeta{
			Name:               layout.Name,
			Description:        layout.Description,
			SavedAt:            layout.SavedAt,
			WorkspaceCount:     len(layout.Workspaces),
			WorkspaceTitles:    titles,
			WorkspacePanes:     panes,
			WorkspaceSummaries: summaries,
			FilePath:           s.Path(name),
			Groups:             layout.Groups,
			Memberships:        layoutMemberships(layout),
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Name < metas[j].Name
	})
	return metas, nil
}

// workspacePaneSummary builds a short description of a workspace's panes.
// Examples: "shell", "claude", "htop · shell", "claude · 🌐 localhost:3000"
func workspacePaneSummary(ws model.Workspace) string {
	var parts []string
	for _, p := range ws.Panes {
		switch {
		case p.Type == "browser":
			url := p.URL
			if url != "" {
				// Strip protocol for brevity.
				url = strings.TrimPrefix(url, "https://")
				url = strings.TrimPrefix(url, "http://")
				url = strings.TrimSuffix(url, "/")
				if len(url) > 30 {
					url = url[:27] + "..."
				}
			}
			if url != "" {
				parts = append(parts, "🌐 "+url)
			} else {
				parts = append(parts, "🌐 browser")
			}
		case p.Command != "":
			cmd := p.Command
			if len(cmd) > 30 {
				cmd = cmd[:27] + "..."
			}
			parts = append(parts, cmd)
		default:
			parts = append(parts, "shell")
		}
	}
	if len(parts) == 0 {
		return "shell"
	}
	return strings.Join(parts, " · ")
}

// Delete removes a layout from the list by moving its file to the previous
// path, replacing any older copy there, so a delete can be undone like a save.
func (s *FileStore) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if !s.Exists(name) {
		return fmt.Errorf("layout %q not found", name)
	}
	return os.Rename(s.Path(name), s.PreviousPath(name))
}

// Rename moves a layout file and updates the name inside the TOML.
func (s *FileStore) Rename(oldName, newName string) error {
	if err := validateName(oldName); err != nil {
		return err
	}
	if err := validateName(newName); err != nil {
		return err
	}
	if !s.Exists(oldName) {
		return fmt.Errorf("layout %q not found", oldName)
	}
	if s.Exists(newName) {
		return fmt.Errorf("layout %q already exists", newName)
	}

	// Load, update name, save to new path, delete old.
	layout, err := s.Load(oldName)
	if err != nil {
		return fmt.Errorf("load %q: %w", oldName, err)
	}
	layout.Name = newName
	if err := s.Save(newName, layout); err != nil {
		return fmt.Errorf("save %q: %w", newName, err)
	}

	// The previous version travels with the layout. If it cannot move, the
	// new file is taken back so the rename leaves things as they were.
	if err := s.movePrevious(oldName, newName); err != nil {
		_ = os.Remove(s.Path(newName))
		return fmt.Errorf("move previous version of %q: %w", oldName, err)
	}

	return os.Remove(s.Path(oldName))
}

// movePrevious moves the previous version of oldName to that of newName,
// writing the new name inside it so that copying it back gives a layout that
// lists and restores under newName. A copy that does not parse moves as it
// is. A layout without a previous version is not an error. When oldName has
// none, a copy left under newName by an earlier delete stays where it is: it
// is still the only way back to that deleted layout.
func (s *FileStore) movePrevious(oldName, newName string) error {
	oldPrevious := s.PreviousPath(oldName)
	data, err := os.ReadFile(oldPrevious)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	// Only a copy that parses can carry the new name; any other moves as is.
	var layout model.Layout
	if toml.Unmarshal(data, &layout) == nil {
		layout.Name = newName
		_, data, err = encodeLayout(newName, &layout)
		if err != nil {
			return err
		}
	}

	if err := writeFileAtomic(s.PreviousPath(newName), data); err != nil {
		return err
	}
	return os.Remove(oldPrevious)
}
