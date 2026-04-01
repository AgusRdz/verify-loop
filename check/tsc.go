package check

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TSCChecker runs the TypeScript compiler on a project and filters results to
// the written file.
type TSCChecker struct {
	projectRoot         string
	parseFunc           ParseFunc
	incremental         bool
	tsbuildInfoGitignore string // "local" (default) or "global"
}

// NewTSC creates a TSCChecker for the given project root and parse function.
// parseFunc is typically parse.TSC, injected from main to avoid import cycles.
func NewTSC(projectRoot string, parseFunc ParseFunc, incremental bool, tsbuildInfoGitignore string) *TSCChecker {
	if tsbuildInfoGitignore == "" {
		tsbuildInfoGitignore = "local"
	}
	return &TSCChecker{
		projectRoot:         projectRoot,
		parseFunc:           parseFunc,
		incremental:         incremental,
		tsbuildInfoGitignore: tsbuildInfoGitignore,
	}
}

func (c *TSCChecker) Name() string { return "TSC" }

// appendToFile appends entry to the given file path if not already present.
func appendToFile(path, entry string) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if string(bytes.TrimSpace(line)) == entry {
			return
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(data) > 0 && data[len(data)-1] != '\n' {
		f.WriteString("\n") //nolint
	}
	f.WriteString(entry + "\n") //nolint
}

// removeFromFile removes all lines matching entry from the given file.
func removeFromFile(path, entry string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := bytes.Split(data, []byte("\n"))
	var kept [][]byte
	for _, line := range lines {
		if string(bytes.TrimSpace(line)) != entry {
			kept = append(kept, line)
		}
	}
	if len(kept) == len(lines) {
		return // nothing to remove
	}
	os.WriteFile(path, bytes.Join(kept, []byte("\n")), 0644) //nolint
}

// GlobalGitignorePath returns the path to the global gitignore file.
// It reads core.excludesFile from git config, falling back to ~/.gitignore_global.
func GlobalGitignorePath() string {
	out, err := exec.Command("git", "config", "--global", "core.excludesFile").Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			// Expand ~ manually since os.ExpandEnv won't handle it.
			if strings.HasPrefix(p, "~/") {
				home, _ := os.UserHomeDir()
				p = filepath.Join(home, p[2:])
			}
			return p
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gitignore_global")
}

// ensureGitignore adds entry to .gitignore in projectRoot if not already present.
func ensureGitignore(projectRoot, entry string) {
	appendToFile(filepath.Join(projectRoot, ".gitignore"), entry)
}

// ApplyGlobalGitignore adds entry to the global gitignore. Call this without a
// project root — it does not touch any local .gitignore.
func ApplyGlobalGitignore(entry string) {
	appendToFile(GlobalGitignorePath(), entry)
}

// ensureGlobalGitignore adds entry to the global gitignore and removes it from
// the local .gitignore if present.
func ensureGlobalGitignore(projectRoot, entry string) {
	appendToFile(GlobalGitignorePath(), entry)
	removeFromFile(filepath.Join(projectRoot, ".gitignore"), entry)
}

// tscBin returns the local tsc binary if present, falling back to the global one.
func tscBin(projectRoot string) string {
	candidates := []string{
		filepath.Join(projectRoot, "node_modules", ".bin", "tsc"),
		filepath.Join(projectRoot, "node_modules", ".bin", "tsc.cmd"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "tsc"
}

func (c *TSCChecker) Run(file string, timeout time.Duration) Result {
	tsc := tscBin(c.projectRoot)

	// Always use --incremental (config opt-in or default behavior).
	// If .tsbuildinfo doesn't exist yet, ensure *.tsbuildinfo is in .gitignore
	// so the generated cache file is never accidentally committed.
	tsbuildinfo := filepath.Join(c.projectRoot, ".tsbuildinfo")
	if _, err := os.Stat(tsbuildinfo); err != nil {
		if c.tsbuildInfoGitignore == "global" {
			ensureGlobalGitignore(c.projectRoot, "*.tsbuildinfo")
		} else {
			ensureGitignore(c.projectRoot, "*.tsbuildinfo")
		}
	}

	cmdStr := tsc + " --noEmit"
	if c.incremental {
		cmdStr += " --incremental"
	} else {
		// Auto-detect: use --incremental if .tsbuildinfo already exists.
		if _, err := os.Stat(tsbuildinfo); err == nil {
			cmdStr += " --incremental"
		}
	}

	output, timed, runErr := runCmd(cmdStr, c.projectRoot, timeout)
	if timed {
		return Result{Name: "TSC", Timed: true}
	}
	if runErr != nil && output == "" {
		return Result{Name: "TSC", Err: runErr}
	}
	issues := c.parseFunc(output, file)
	return Result{Name: "TSC", Issues: issues}
}
