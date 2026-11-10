package detect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// qoderMaxDirName is the longest project folder name Qoder CLI writes as is.
// Longer names are cut to this length and get a hash suffix (Gf in
// @qoder-ai/qodercli 1.1.86, bundle/qodercli.js).
const qoderMaxDirName = 200

// qoderHeaderLines bounds how many transcript lines are read looking for the
// cwd. The first message line carries it; only a few metadata lines (title,
// mode) can come before it, so a transcript without a cwd that far down is
// not one crex can match.
const qoderHeaderLines = 50

// detectQoder finds the session of a Qoder CLI process. Qoder keeps one
// transcript per session in <config>/projects/<encoded cwd>/<sessionId>.jsonl,
// where <config> is ~/.qoder unless the process environment moves it. The
// newest transcript whose recorded cwd is exactly this one is resumed; the
// folder name alone is not enough because the encoding is lossy
// ("/src/my-app" and "/src/my_app" share a folder). Without one, Qoder is
// started fresh, as it was before crex could resume it.
func detectQoder(cwd, pid string) *Session {
	configDir, custom := qoderConfigDir(pid, cwd)
	if configDir == "" {
		return &Session{Tool: "qoder", CWD: cwd, Command: "qoder"}
	}

	projectDir := filepath.Join(configDir, "projects", qoderProjectDirName(cwd))
	sessionID := latestQoderSession(projectDir, cwd)
	if sessionID == "" {
		return &Session{Tool: "qoder", CWD: cwd, Command: "qoder"}
	}

	return &Session{
		Tool:    "qoder",
		CWD:     cwd,
		Command: qoderResumeCommand(configDir, custom, sessionID),
	}
}

// qoderProjectDirName names the project folder of cwd the way Qoder CLI does
// (Gf in bundle/qodercli.js): every character that is not an ASCII letter or
// digit becomes "-". JavaScript strings are UTF-16, so a character outside
// the BMP, such as an emoji, is two units and becomes "--". Names over 200
// units are cut and suffixed with "-" and the base-36 hash of the cwd.
func qoderProjectDirName(cwd string) string {
	units := utf16.Encode([]rune(cwd))

	var name strings.Builder
	for _, unit := range units {
		if isASCIIAlphanumeric(unit) {
			name.WriteByte(byte(unit))
		} else {
			name.WriteByte('-')
		}
	}

	encoded := name.String()
	if len(encoded) <= qoderMaxDirName {
		return encoded
	}
	return encoded[:qoderMaxDirName] + "-" + qoderPathHash(units)
}

// isASCIIAlphanumeric reports whether a UTF-16 unit is in [a-zA-Z0-9], the
// set Qoder keeps when it names a project folder.
func isASCIIAlphanumeric(unit uint16) bool {
	return unit >= 'a' && unit <= 'z' || unit >= 'A' && unit <= 'Z' || unit >= '0' && unit <= '9'
}

// qoderPathHash ports the djb2 variant Qoder appends to long folder names:
// hash = 33*hash ^ unit from 5381 over the UTF-16 units of the cwd. In
// JavaScript ^ truncates to a signed 32-bit integer, which is what int32
// arithmetic does here; the result is Math.abs(hash).toString(36).
func qoderPathHash(units []uint16) string {
	hash := int32(5381)
	for _, unit := range units {
		hash = hash*33 ^ int32(unit)
	}

	// Widen before negating: the absolute value of math.MinInt32 does not
	// fit in an int32, and JavaScript gives 2147483648 for it.
	value := int64(hash)
	if value < 0 {
		value = -value
	}
	return strconv.FormatInt(value, 36)
}

// qoderConfigDir returns the Qoder CLI config directory a process uses,
// resolved as Qoder does it (pi in bundle/qodercli.js): $QODER_CONFIG_DIR
// when set, else $QODER_CLI_HOME (or the home directory) joined with
// $QODER_CONFIG_DIR_NAME (or ".qoder"). Relative values are taken from the
// process's working directory, as Node's path.resolve would. custom reports
// whether it differs from ~/.qoder, in which case the resume command must
// point Qoder at it again. The --config-dir flag is not visible in the
// environment and is not followed.
func qoderConfigDir(pid, cwd string) (dir string, custom bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	defaultDir := filepath.Join(home, ".qoder")
	env := processEnv(pid)

	if configDir := env["QODER_CONFIG_DIR"]; configDir != "" {
		dir = resolveFrom(cwd, configDir)
		return dir, dir != defaultDir
	}

	base := home
	if cliHome := env["QODER_CLI_HOME"]; cliHome != "" {
		base = resolveFrom(cwd, cliHome)
	}

	// Qoder refuses to start with a name that is not a single folder, so
	// such a value cannot belong to a running process; ignore it rather
	// than let it walk out of the base directory.
	name := ".qoder"
	if dirName := env["QODER_CONFIG_DIR_NAME"]; dirName != "" && dirName != "." && dirName != ".." && !strings.ContainsRune(dirName, '/') {
		name = dirName
	}

	dir = filepath.Join(base, name)
	return dir, dir != defaultDir
}

// resolveFrom returns path made absolute against dir and cleaned.
func resolveFrom(dir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(dir, path)
}

// latestQoderSession returns the ID of the most recently modified transcript
// in a Qoder project folder that was recorded in cwd, or "" if none is. The
// ID is the file name, which is what --resume looks up.
func latestQoderSession(projectDir, cwd string) string {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return ""
	}

	type transcript struct {
		sessionID string
		modTime   int64
	}
	var transcripts []transcript
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		if !validSessionID.MatchString(sessionID) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		transcripts = append(transcripts, transcript{sessionID: sessionID, modTime: info.ModTime().UnixNano()})
	}

	sort.Slice(transcripts, func(i, j int) bool {
		return transcripts[i].modTime > transcripts[j].modTime
	})

	// Newest first, so only the headers up to the first match are read.
	for _, candidate := range transcripts {
		path := filepath.Join(projectDir, candidate.sessionID+".jsonl")
		if qoderTranscriptCWD(path) == cwd {
			return candidate.sessionID
		}
	}
	return ""
}

// qoderTranscriptCWD returns the working directory recorded in a Qoder
// transcript: the cwd of its first line that has one. Every message line
// carries sessionId and cwd; metadata lines before them may not. Returns ""
// when the file can't be read or has no cwd in its first lines.
func qoderTranscriptCWD(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(file)
	for range qoderHeaderLines {
		line, err := reader.ReadBytes('\n')

		// Only lines that mention a cwd are worth decoding.
		if bytes.Contains(line, []byte(`"cwd"`)) {
			var entry struct {
				CWD string `json:"cwd"`
			}
			if json.Unmarshal(line, &entry) == nil && entry.CWD != "" {
				return entry.CWD
			}
		}
		if err != nil {
			break
		}
	}
	return ""
}

// qoderResumeCommand builds the resume command, setting QODER_CONFIG_DIR
// when the session lives outside ~/.qoder. That one variable is enough
// whichever variable moved the directory, because Qoder reads it before
// $QODER_CLI_HOME and $QODER_CONFIG_DIR_NAME.
func qoderResumeCommand(configDir string, custom bool, sessionID string) string {
	command := "qoder --resume " + sessionID
	if custom {
		command = "QODER_CONFIG_DIR=" + shellQuote(configDir) + " " + command
	}
	return command
}
