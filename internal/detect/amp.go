package detect

import (
	"strings"
	"sync"
)

// detectAmp finds the active thread for a running Amp CLI instance.
//
// Each amp process holds an open write handle on its per-thread log file
// (~/.cache/amp/logs/threads/T-<id>.log) for the entire session lifetime.
// During AISessions() we batch-list every AI process's open files with a
// single lsof call (see batchProcessInfo) and seed the pid→thread map as
// a side effect, so by the time detectAmp runs it's a pure map lookup.
//
// The mapping is per-pid precise, so two amp instances sharing a CWD each
// resolve to their own thread.
func detectAmp(cwd, pid string) *Session {
	if pid == "" {
		return nil
	}
	id := ampCache.threadFor(pid)
	if id == "" || !validSessionID.MatchString(id) {
		return nil
	}
	return &Session{
		Tool:    "amp",
		CWD:     cwd,
		Command: "amp threads continue " + id,
	}
}

// ampCache holds the pid→threadID mapping for amp processes, populated as
// a side effect of the AISessions() lsof pass. Reset between invocations
// via resetAmpCache.
var ampCache = &ampThreadCache{}

type ampThreadCache struct {
	mu    sync.Mutex
	byPID map[string]string // pid → threadID
}

// resetAmpCache clears the cached pid→thread map. Called at the top of
// AISessions().
func resetAmpCache() {
	ampCache.mu.Lock()
	defer ampCache.mu.Unlock()
	ampCache.byPID = nil
}

// seedAmpThread records a (pid → threadID) mapping discovered during the
// lsof pass.
func seedAmpThread(pid, threadID string) {
	ampCache.mu.Lock()
	defer ampCache.mu.Unlock()
	if ampCache.byPID == nil {
		ampCache.byPID = make(map[string]string)
	}
	ampCache.byPID[pid] = threadID
}

// threadFor returns the thread ID for pid, or "" if none is known.
func (c *ampThreadCache) threadFor(pid string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.byPID[pid]
}

// ampLogPathPrefix and ampLogPathSuffix bracket the per-thread log path
// `.../logs/threads/T-<id>.log` as it appears in lsof output. We extract
// the thread ID by stripping these — no regex needed.
const (
	ampLogPathPrefix = "/logs/threads/"
	ampLogPathSuffix = ".log"
)

// threadIDFromLogPath returns the T-<id> suffix of a per-thread log path
// or "" if the path isn't one.
func threadIDFromLogPath(path string) string {
	i := strings.LastIndex(path, ampLogPathPrefix)
	if i < 0 {
		return ""
	}
	name := path[i+len(ampLogPathPrefix):]
	if !strings.HasSuffix(name, ampLogPathSuffix) {
		return ""
	}
	id := strings.TrimSuffix(name, ampLogPathSuffix)
	if !strings.HasPrefix(id, "T-") {
		return ""
	}
	return id
}
