//go:build !darwin

package detect

import (
	"os"
	"path/filepath"
)

// readProcessEnv reads a process environment from /proc (Linux). On systems
// without procfs it returns nil and callers fall back to defaults.
func readProcessEnv(pid string) map[string]string {
	if pid == "" {
		return nil
	}
	buf, err := os.ReadFile(filepath.Join("/proc", pid, "environ"))
	if err != nil {
		return nil
	}
	return envFromNULList(buf)
}
