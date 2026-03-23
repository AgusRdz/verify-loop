package detect

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// ProjectInfo holds the detected project root and available tooling.
type ProjectInfo struct {
	Root           string `json:"root"`
	HasTSConfig    bool   `json:"has_tsconfig"`
	HasGoMod       bool   `json:"has_go_mod"`
	HasPackageJSON bool   `json:"has_package_json"`
	HasESLint      bool   `json:"has_eslint"`
	HasAngular     bool   `json:"has_angular"`
	HasCargo       bool   `json:"has_cargo"`
	HasPyProject   bool   `json:"has_pyproject"`
	HasSolution    bool   `json:"has_solution"`
}

type cacheEntry struct {
	Info   ProjectInfo      `json:"info"`
	Mtimes map[string]int64 `json:"mtimes"` // marker file path → unix mtime
}

var rootMarkers = []string{
	"tsconfig.json",
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
}

var eslintFiles = []string{
	".eslintrc",
	".eslintrc.js",
	".eslintrc.json",
	".eslintrc.yml",
	".eslintrc.yaml",
	"eslint.config.js",
	"eslint.config.mjs",
}

var trackedMarkers = []string{
	"tsconfig.json",
	"go.mod",
	"package.json",
}

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// Find walks up from the directory containing filePath to find the project
// root, then returns a ProjectInfo describing available tooling. Results are
// cached per session to avoid repeated filesystem walks.
func Find(filePath string) (ProjectInfo, error) {
	root := findRoot(filepath.Dir(filePath))
	sessionKey := resolveSessionKey(root)
	cacheFile := cacheFilePath(root, sessionKey)

	if cached, ok := loadCache(cacheFile, root); ok {
		return cached, nil
	}

	info := buildInfo(root)

	saveCache(cacheFile, info, root)

	return info, nil
}

// findRoot walks up the directory tree from start looking for any root marker.
// Returns the first directory that contains a marker, or start if none found.
func findRoot(start string) string {
	dir := start
	for {
		for _, marker := range rootMarkers {
			if fileExists(filepath.Join(dir, marker)) {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// reached filesystem root with no marker
			return start
		}
		dir = parent
	}
}

// buildInfo populates a ProjectInfo for the given root directory.
func buildInfo(root string) ProjectInfo {
	info := ProjectInfo{Root: root}

	info.HasTSConfig = fileExists(filepath.Join(root, "tsconfig.json"))
	info.HasGoMod = fileExists(filepath.Join(root, "go.mod"))
	info.HasPackageJSON = fileExists(filepath.Join(root, "package.json"))
	info.HasAngular = fileExists(filepath.Join(root, "angular.json"))
	info.HasCargo = fileExists(filepath.Join(root, "Cargo.toml"))
	info.HasPyProject = fileExists(filepath.Join(root, "pyproject.toml"))

	for _, name := range eslintFiles {
		if fileExists(filepath.Join(root, name)) {
			info.HasESLint = true
			break
		}
	}

	matches, _ := filepath.Glob(filepath.Join(root, "*.sln"))
	info.HasSolution = len(matches) > 0

	return info
}

// resolveSessionKey returns a sanitized session key derived from CLAUDE_SESSION_ID
// or falls back to a hash of root + "|nosession".
func resolveSessionKey(root string) string {
	raw := os.Getenv("CLAUDE_SESSION_ID")
	if raw == "" {
		sum := md5.Sum([]byte(root + "|nosession"))
		return fmt.Sprintf("%x", sum)[:16]
	}
	sanitized := sanitizeRe.ReplaceAllString(raw, "")
	if len(sanitized) > 32 {
		sanitized = sanitized[:32]
	}
	return sanitized
}

// cacheFilePath returns the path to the cache file for the given root and session.
func cacheFilePath(root, sessionKey string) string {
	sum := md5.Sum([]byte(root))
	rootHash := fmt.Sprintf("%x", sum)[:12]

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	dir := filepath.Join(cacheDir, "verify-loop")
	return filepath.Join(dir, fmt.Sprintf("%s_%s.json", rootHash, sessionKey))
}

// loadCache attempts to read and validate a cache entry. Returns the cached
// ProjectInfo and true on a valid, current cache hit; false otherwise.
func loadCache(cacheFile, root string) (ProjectInfo, bool) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return ProjectInfo{}, false
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return ProjectInfo{}, false
	}

	// Validate mtimes for tracked marker files.
	for path, recorded := range entry.Mtimes {
		fi, err := os.Stat(path)
		if err != nil {
			// File disappeared or can't be read — invalidate.
			return ProjectInfo{}, false
		}
		if fi.ModTime().Unix() != recorded {
			return ProjectInfo{}, false
		}
	}

	return entry.Info, true
}

// saveCache writes the ProjectInfo to the cache file, recording current mtimes
// for the tracked marker files. Silently ignores all errors.
func saveCache(cacheFile string, info ProjectInfo, root string) {
	entry := cacheEntry{
		Info:   info,
		Mtimes: make(map[string]int64, len(trackedMarkers)),
	}

	for _, name := range trackedMarkers {
		path := filepath.Join(root, name)
		fi, err := os.Stat(path)
		if err == nil {
			entry.Mtimes[path] = fi.ModTime().Unix()
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	dir := filepath.Dir(cacheFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	_ = os.WriteFile(cacheFile, data, 0o644)
}

// fileExists reports whether path names an existing file or directory.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
