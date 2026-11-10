package detect

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// oh-my-pi is a fork of Pi and keeps Pi's variable names for the agent dir
// and the session dir (piAgentDirVariable, piSessionDirVariable), with two
// differences: a named profile ignores the agent dir, and a relative value is
// taken from the directory omp was started in instead of being refused.

// ompConfigDirVariable renames the config root under home, ".omp" by
// default (getConfigDirName in packages/utils/src/dirs.ts).
const ompConfigDirVariable = "PI_CONFIG_DIR"

// ompProfileVariable selects a named profile, whose sessions live in
// ~/.omp/profiles/<name>/agent/sessions. It wins over the legacy one even
// when it is set and empty (resolveProfileEnv in dirs.ts).
const ompProfileVariable = "OMP_PROFILE"

// ompLegacyProfileVariable is the profile variable omp inherited from Pi,
// read only when OMP_PROFILE is not set at all.
const ompLegacyProfileVariable = "PI_PROFILE"

// ompDataHomeVariable moves the sessions to $XDG_DATA_HOME/omp/sessions on
// macOS and Linux, but only once `omp config init-xdg` has created that omp
// folder: omp checks it exists before using it (DirResolver in dirs.ts).
const ompDataHomeVariable = "XDG_DATA_HOME"

// ompTitleSlotType is the type of the fixed-width record current omp files
// open with, before the session header (SESSION_TITLE_SLOT_ENTRY_TYPE in
// src/session/session-entries.ts).
const ompTitleSlotType = "title"

// ompProfileName is the shape omp accepts for a profile name; anything else
// makes it fall back to the default profile (normalizeProfileName in
// dirs.ts). Windows device names are refused on every platform.
var (
	ompProfileName         = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	ompReservedProfileName = regexp.MustCompile(`(?i)^(?:CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])(?:\..*)?$`)
)

// ompHashedNameCharacters are the characters the 17.2.5 to 17.2.8 folder
// names replaced with "-" in the directory's base name.
var ompHashedNameCharacters = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// detectOmp finds the session an oh-my-pi process is running in its
// directory. omp keeps one JSONL file per session in
// <agent dir>/sessions/<encoded cwd>/<timestamp>_<id>.jsonl, and the header
// after the file's title slot names the session and the directory it
// belongs to. The newest file whose header matches cwd is resumed with
// `omp --resume <id>`. When there is none, omp is relaunched empty.
func detectOmp(cwd, pid string) *Session {
	fresh := &Session{Tool: "omp", CWD: cwd, Command: "omp"}

	folders, prefix := ompSessionLocation(cwd, pid)
	for _, folder := range folders {
		sessionID := newestOmpSession(folder, cwd)
		if sessionID != "" {
			return &Session{Tool: "omp", CWD: cwd, Command: prefix + "omp --resume " + sessionID}
		}
	}

	return fresh
}

// ompSessionLocation returns the folders where the omp process may keep the
// sessions of cwd, the current one first, and the variable assignments the
// resume command must carry when they are not the default ones:
// `omp --resume <id>` only finds a session where it is told to look.
func ompSessionLocation(cwd, pid string) (folders []string, prefix string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, ""
	}
	environment := processEnv(pid)

	// A session dir holds the files directly, so cwd adds no folder.
	if sessionDir := piVariable(environment, piSessionDirVariable); sessionDir != "" {
		sessionDir = ompAbsolutePath(sessionDir, cwd)
		return []string{sessionDir}, ompAssignment(piSessionDirVariable, sessionDir)
	}

	sessionsRoot, prefix := ompSessionsRoot(environment, home, cwd)

	// omp names the folder after the directory with its links resolved, and
	// compares it with home and the temp root resolved the same way.
	directory := resolvedPath(cwd)
	tempRoot := resolvedPath(ompTempRoot(environment))
	folder, scope := ompSessionFolder(directory, resolvedPath(home), tempRoot)

	folders = []string{
		filepath.Join(sessionsRoot, folder),
		// Versions 17.2.5 to 17.2.8 named the folder after a hash of the
		// directory. A newer omp moves it back the next time it opens a
		// session there; until then the sessions are only in the old one.
		filepath.Join(sessionsRoot, ompHashedSessionFolder(directory, scope)),
	}
	return folders, prefix
}

// ompSessionsRoot returns the folder that holds omp's per-directory session
// folders, following the order of DirResolver in dirs.ts: a named profile,
// then an agent dir set by hand, then XDG_DATA_HOME/omp when it exists, then
// ~/.omp/agent/sessions. The prefix carries what moved it.
func ompSessionsRoot(environment map[string]string, home, cwd string) (root, prefix string) {
	configDir := ".omp"
	if customDir := piVariable(environment, ompConfigDirVariable); customDir != "" && customDir != configDir {
		configDir = customDir
		prefix = ompAssignment(ompConfigDirVariable, customDir)
	}
	configRoot := filepath.Join(home, configDir)
	dataHome := piVariable(environment, ompDataHomeVariable)
	if dataHome != "" {
		dataHome = ompAbsolutePath(dataHome, cwd)
	}

	if profile := ompProfile(environment); profile != "" {
		prefix += ompAssignment(ompProfileVariable, profile)

		// A profile uses XDG only when its own folder is already there, so a
		// profile never moves once it has sessions.
		profileData := filepath.Join(dataHome, "omp", "profiles", profile)
		if dataHome != "" && directoryExists(profileData) {
			return filepath.Join(profileData, "sessions"), prefix + ompAssignment(ompDataHomeVariable, dataHome)
		}
		return filepath.Join(configRoot, "profiles", profile, "agent", "sessions"), prefix
	}

	defaultAgentDir := filepath.Join(configRoot, "agent")
	if customDir := piVariable(environment, piAgentDirVariable); customDir != "" {
		agentDir := ompAbsolutePath(customDir, cwd)
		if agentDir != defaultAgentDir {
			return filepath.Join(agentDir, "sessions"), prefix + ompAssignment(piAgentDirVariable, agentDir)
		}
	}

	// XDG drops the agent folder: ~/.omp/agent/sessions becomes
	// $XDG_DATA_HOME/omp/sessions.
	appData := filepath.Join(dataHome, "omp")
	if dataHome != "" && directoryExists(appData) {
		return filepath.Join(appData, "sessions"), prefix + ompAssignment(ompDataHomeVariable, dataHome)
	}

	return filepath.Join(defaultAgentDir, "sessions"), prefix
}

// ompProfile returns the named profile the omp process runs under, or ""
// for the default one. "default", an empty value and a name omp would
// refuse all mean the default profile, as when omp starts.
func ompProfile(environment map[string]string) string {
	name, set := ompLookup(environment, ompProfileVariable)
	if !set {
		name, _ = ompLookup(environment, ompLegacyProfileVariable)
	}
	name = strings.TrimSpace(name)

	if name == "default" || strings.HasSuffix(name, ".") {
		return ""
	}
	if !ompProfileName.MatchString(name) || ompReservedProfileName.MatchString(name) {
		return ""
	}
	return name
}

// ompLookup reads one variable from the environment the omp process was
// started with, telling an empty value from a missing one. Like piVariable,
// it falls back to crex's own environment when the process's is unreadable.
func ompLookup(environment map[string]string, name string) (value string, set bool) {
	if environment == nil {
		return os.LookupEnv(name)
	}
	value, set = environment[name]
	return value, set
}

// ompTempRoot returns the temp root omp sees: Bun's os.tmpdir(), which is
// $TMPDIR, $TMP or $TEMP without a trailing "/", or "/tmp". On macOS that is
// the per-user /var/folders/.../T folder, not /tmp.
func ompTempRoot(environment map[string]string) string {
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		path := piVariable(environment, name)
		if path == "" {
			continue
		}
		if len(path) > 1 {
			path = strings.TrimSuffix(path, "/")
		}
		return path
	}

	return "/tmp"
}

// ompSessionFolder names the folder omp keeps the sessions of cwd in, and
// the scope the 17.2.5 to 17.2.8 hashed names used for it. All three paths
// must have their links resolved. Under the temp root, which is checked
// first because it can sit inside home, the folder is "-tmp-<relative>";
// under home it is "-<relative>"; anywhere else it is Pi's
// "--<absolute>--". In all of them "/", "\" and ":" become "-"
// (getDefaultSessionDirName in src/session/session-paths.ts). Different
// directories can share a folder ("~/a/b" and "~/a-b"), so the header still
// has to be checked.
func ompSessionFolder(cwd, home, tempRoot string) (folder, scope string) {
	if relative, inside := ompRelativeWithin(tempRoot, cwd); inside {
		return ompRelativeFolder("-tmp", relative), "tmp"
	}
	if relative, inside := ompRelativeWithin(home, cwd); inside {
		return ompRelativeFolder("-", relative), "home"
	}

	return piSessionFolder(cwd), "abs"
}

// ompRelativeWithin returns path relative to root, and whether path is root
// itself or inside it. Like omp's isRelativeWithin, any relative path that
// starts with ".." counts as outside, so a folder named "..cache" in home
// gets the absolute form.
func ompRelativeWithin(root, path string) (relative string, inside bool) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	if relative == "." {
		return "", true
	}

	if strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return "", false
	}
	return relative, true
}

// ompRelativeFolder joins a folder prefix and a relative path with its
// separators turned into "-": "-" and "src/proj" give "-src-proj", "-tmp"
// and "build" give "-tmp-build". An empty path leaves the prefix alone
// (encodeRelativeSessionDirName in session-paths.ts).
func ompRelativeFolder(prefix, relative string) string {
	encoded := strings.NewReplacer("/", "-", `\`, "-", ":", "-").Replace(relative)
	if encoded == "" {
		return prefix
	}

	if strings.HasSuffix(prefix, "-") {
		return prefix + encoded
	}
	return prefix + "-" + encoded
}

// ompHashedSessionFolder names the folder omp 17.2.5 to 17.2.8 kept the
// sessions of cwd in: <scope>-<base name>-<hex SHA-256 of the path>, with
// the base name reduced to letters, digits, ".", "_" and "-" and cut to its
// last 80 characters (encodeHashedSessionDirName in session-paths.ts). The
// hash is omp's naming, not a security measure.
func ompHashedSessionFolder(cwd, scope string) string {
	readable := ompHashedNameCharacters.ReplaceAllString(filepath.Base(cwd), "-")
	readable = strings.Trim(readable, "-")
	if len(readable) > 80 {
		readable = readable[len(readable)-80:]
	}
	if readable == "" {
		readable = "project"
	}

	digest := sha256.Sum256([]byte(strings.ReplaceAll(cwd, `\`, "/")))

	return scope + "-" + readable + "-" + hex.EncodeToString(digest[:])
}

// newestOmpSession returns the id of the most recently written session file
// in folder whose header belongs to cwd, or "" when there is none. Like
// `omp -c`, it goes by modification time. The header may spell cwd through
// a link, so the two are compared with their links resolved.
func newestOmpSession(folder, cwd string) string {
	for _, path := range sessionFilesNewestFirst(folder) {
		sessionID, sessionCWD := readOmpSessionHeader(path)
		if sessionCWD == "" || !sameFile(sessionCWD, cwd) {
			continue
		}
		// The id is typed into a terminal on restore, so a file that
		// names something else than a plain id is passed over.
		if !validSessionID.MatchString(sessionID) {
			continue
		}
		return sessionID
	}

	return ""
}

// readOmpSessionHeader returns the session id and directory from the header
// of an omp session file. Current files open with a title slot, a
// {"type":"title",...} line padded to 256 bytes, and the header follows it;
// older files open with the header itself (docs/session.md, "File
// Format"). Pi's readSessionHeader stops at the title slot, hence this one.
// Anything but a "session" header with a string id gives "".
func readOmpSessionHeader(path string) (sessionID, cwd string) {
	file, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer func() { _ = file.Close() }()

	titleSkipped := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var record struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			CWD  string `json:"cwd"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			return "", ""
		}

		// Only the first record may be the title slot.
		if record.Type == ompTitleSlotType && !titleSkipped {
			titleSkipped = true
			continue
		}
		if record.Type != "session" {
			return "", ""
		}
		return record.ID, record.CWD
	}

	return "", ""
}

// ompAbsolutePath makes a directory taken from an environment variable
// absolute the way omp does, with path.resolve: a relative value is read
// from the directory omp was started in, and "~" is not expanded.
func ompAbsolutePath(path, cwd string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(cwd, path)
}

// ompAssignment is the shell assignment a resume command starts with to give
// omp the same variable it ran with.
func ompAssignment(name, value string) string {
	return name + "=" + shellQuote(value) + " "
}

// directoryExists reports whether path is a directory crex can see.
func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
