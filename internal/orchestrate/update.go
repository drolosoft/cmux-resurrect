package orchestrate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InstallMethod describes how crex was installed.
type InstallMethod int

const (
	InstallHomebrew  InstallMethod = iota
	InstallGoInstall InstallMethod = iota
	InstallManual    InstallMethod = iota
)

func (m InstallMethod) String() string {
	switch m {
	case InstallHomebrew:
		return "homebrew"
	case InstallGoInstall:
		return "go install"
	default:
		return "manual"
	}
}

const (
	githubReleasesAPI = "https://api.github.com/repos/drolosoft/cmux-resurrect/releases/latest"
	githubReleasesURL = "https://github.com/drolosoft/cmux-resurrect/releases/latest"
	homebrewFormula   = "drolosoft/tap/crex"
	goInstallPath     = "github.com/drolosoft/cmux-resurrect/cmd/crex@latest"
)

// DetectInstallMethod determines how crex was installed by checking the binary location.
func DetectInstallMethod() InstallMethod {
	exe, err := os.Executable()
	if err != nil {
		return InstallManual
	}
	// Resolve symlinks (Homebrew uses symlinks from Cellar).
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}

	// Homebrew: binary is under /opt/homebrew/ or /usr/local/Cellar/
	if strings.Contains(resolved, "/Cellar/") ||
		strings.HasPrefix(resolved, "/opt/homebrew/") ||
		strings.HasPrefix(resolved, "/usr/local/") {
		return InstallHomebrew
	}

	// Go install: binary is in GOPATH/bin or GOBIN
	if gobin := os.Getenv("GOBIN"); gobin != "" && strings.HasPrefix(resolved, gobin) {
		return InstallGoInstall
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		if strings.HasPrefix(resolved, filepath.Join(gopath, "bin")) {
			return InstallGoInstall
		}
	}
	// Default GOPATH is ~/go
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(resolved, filepath.Join(home, "go", "bin")) {
			return InstallGoInstall
		}
	}

	return InstallManual
}

// CheckLatestVersion queries GitHub for the latest release tag.
// Returns the tag name (e.g. "v1.14.0") or an error.
func CheckLatestVersion() (string, error) {
	req, err := http.NewRequest("GET", githubReleasesAPI, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "crex/update")
	req.Header.Set("Accept", "application/vnd.github+json")

	// Use GITHUB_TOKEN or GH_TOKEN if available (5,000 req/hr vs 60 unauthenticated).
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if token := os.Getenv("GH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("GitHub API rate limit exceeded — try again in a few minutes")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name in response")
	}
	return release.TagName, nil
}

// NormalizeVersion strips the "v" prefix for comparison.
func NormalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// UpdateResult holds the outcome of an update attempt.
type UpdateResult struct {
	Method        InstallMethod
	OldVersion    string
	NewVersion    string
	AlreadyLatest bool
	ManualURL     string
	Output        string
	Err           error
}

// RunUpdate detects the install method and runs the appropriate update command.
// currentVersion should be the build-time injected version (e.g. "1.13.1" or "v1.13.1").
func RunUpdate(currentVersion string) *UpdateResult {
	result := &UpdateResult{
		Method:     DetectInstallMethod(),
		OldVersion: currentVersion,
	}

	// Check latest version.
	latest, err := CheckLatestVersion()
	if err != nil {
		result.Err = fmt.Errorf("cannot check for updates: %w", err)
		return result
	}
	result.NewVersion = latest

	// Compare versions.
	if NormalizeVersion(currentVersion) == NormalizeVersion(latest) {
		result.AlreadyLatest = true
		return result
	}

	// Run the update.
	switch result.Method {
	case InstallHomebrew:
		out, err := exec.Command("brew", "upgrade", homebrewFormula).CombinedOutput()
		result.Output = string(out)
		result.Err = err

	case InstallGoInstall:
		out, err := exec.Command("go", "install", goInstallPath).CombinedOutput()
		result.Output = string(out)
		result.Err = err

	case InstallManual:
		result.ManualURL = githubReleasesURL
	}

	return result
}
