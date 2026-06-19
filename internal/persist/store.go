package persist

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	toml "github.com/pelletier/go-toml/v2"
)

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

	data, err := toml.Marshal(layout)
	if err != nil {
		return fmt.Errorf("marshal layout: %w", err)
	}

	// Add a header comment
	header := fmt.Sprintf("# crex layout: %s\n# Saved at: %s\n\n",
		name, layout.SavedAt.Format("2006-01-02T15:04:05Z07:00"))
	content := header + string(data)

	// Atomic write: temp file + rename
	target := s.Path(name)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
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
		if p.Type == "browser" {
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
		} else if p.Command != "" {
			cmd := p.Command
			if len(cmd) > 30 {
				cmd = cmd[:27] + "..."
			}
			parts = append(parts, cmd)
		} else {
			parts = append(parts, "shell")
		}
	}
	if len(parts) == 0 {
		return "shell"
	}
	return strings.Join(parts, " · ")
}

// Delete removes a layout file.
func (s *FileStore) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if !s.Exists(name) {
		return fmt.Errorf("layout %q not found", name)
	}
	return os.Remove(s.Path(name))
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
	return os.Remove(s.Path(oldName))
}
