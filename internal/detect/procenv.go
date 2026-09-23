package detect

import (
	"bytes"
	"encoding/binary"
	"strings"
)

// processEnv returns the environment of a running process, or nil when it
// can't be read. It is a variable so tests can stub it.
var processEnv = readProcessEnv

// envFromProcArgs parses the buffer returned by the macOS kern.procargs2
// sysctl: a native-endian int32 argc, the exec path, NUL padding, argc
// NUL-terminated arguments, then NUL-terminated KEY=VALUE environment
// strings until an empty one.
func envFromProcArgs(buf []byte) map[string]string {
	if len(buf) < 4 {
		return nil
	}
	argc := int(binary.NativeEndian.Uint32(buf[:4]))
	rest := buf[4:]

	// Skip the exec path and the NUL padding that follows it.
	i := bytes.IndexByte(rest, 0)
	if i < 0 {
		return nil
	}
	rest = rest[i:]
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}

	// Skip argv.
	for ; argc > 0; argc-- {
		i := bytes.IndexByte(rest, 0)
		if i < 0 {
			return nil
		}
		rest = rest[i+1:]
	}

	return envFromNULList(rest)
}

// envFromNULList parses NUL-separated KEY=VALUE strings (the format of
// Linux /proc/<pid>/environ and the env block of kern.procargs2), stopping
// at the first empty string.
func envFromNULList(buf []byte) map[string]string {
	env := make(map[string]string)
	for _, kv := range bytes.Split(buf, []byte{0}) {
		if len(kv) == 0 {
			break
		}
		if k, v, ok := strings.Cut(string(kv), "="); ok && k != "" {
			env[k] = v
		}
	}
	return env
}
