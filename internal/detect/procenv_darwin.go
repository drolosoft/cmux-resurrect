package detect

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// readProcessEnv reads a process environment through the kern.procargs2
// sysctl. Only processes owned by the current user are readable, which
// covers every CLI running in the user's terminals.
func readProcessEnv(pid string) map[string]string {
	n, err := strconv.Atoi(pid)
	if err != nil {
		return nil
	}
	buf, err := unix.SysctlRaw("kern.procargs2", n)
	if err != nil {
		return nil
	}
	return envFromProcArgs(buf)
}
