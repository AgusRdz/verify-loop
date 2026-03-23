package check

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// TSCChecker runs the TypeScript compiler on a project and filters results to
// the written file.
type TSCChecker struct {
	projectRoot string
	parseFunc   ParseFunc
}

// NewTSC creates a TSCChecker for the given project root and parse function.
// parseFunc is typically parse.TSC, injected from main to avoid import cycles.
func NewTSC(projectRoot string, parseFunc ParseFunc) *TSCChecker {
	return &TSCChecker{projectRoot: projectRoot, parseFunc: parseFunc}
}

func (c *TSCChecker) Name() string { return "TSC" }

func (c *TSCChecker) Run(file string, timeout time.Duration) Result {
	// Decide whether to use --incremental based on .tsbuildinfo presence.
	tsbuildinfo := filepath.Join(c.projectRoot, ".tsbuildinfo")
	var cmdStr string
	if _, err := os.Stat(tsbuildinfo); err == nil {
		cmdStr = "tsc --noEmit --incremental"
	} else {
		cmdStr = "tsc --noEmit"
	}

	var shell, shellFlag string
	if runtime.GOOS == "windows" {
		shell, shellFlag = "cmd", "/c"
	} else {
		shell, shellFlag = "sh", "-c"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, shellFlag, cmdStr)
	if c.projectRoot != "" {
		cmd.Dir = c.projectRoot
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Name: "TSC", Timed: true}
	}

	output := buf.String()
	if runErr != nil && output == "" {
		return Result{Name: "TSC", Err: runErr}
	}

	issues := c.parseFunc(output, file)
	return Result{Name: "TSC", Issues: issues}
}
