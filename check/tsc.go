package check

import (
	"bytes"
	"os"
	"path/filepath"
	"time"
)

// TSCChecker runs the TypeScript compiler on a project and filters results to
// the written file.
type TSCChecker struct {
	projectRoot string
	parseFunc   ParseFunc
	incremental bool
}

// NewTSC creates a TSCChecker for the given project root and parse function.
// parseFunc is typically parse.TSC, injected from main to avoid import cycles.
func NewTSC(projectRoot string, parseFunc ParseFunc, incremental bool) *TSCChecker {
	return &TSCChecker{projectRoot: projectRoot, parseFunc: parseFunc, incremental: incremental}
}

func (c *TSCChecker) Name() string { return "TSC" }

// ensureGitignore adds entry to .gitignore in projectRoot if not already present.
func ensureGitignore(projectRoot, entry string) {
	gitignore := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(gitignore)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	// Check if entry already exists.
	for _, line := range bytes.Split(data, []byte("\n")) {
		if string(bytes.TrimSpace(line)) == entry {
			return
		}
	}
	// Append the entry.
	f, err := os.OpenFile(gitignore, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	// Ensure we start on a new line.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		f.WriteString("\n") //nolint
	}
	f.WriteString(entry + "\n") //nolint
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
		ensureGitignore(c.projectRoot, "*.tsbuildinfo")
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
