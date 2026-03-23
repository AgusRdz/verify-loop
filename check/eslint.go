package check

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// ESLintChecker runs ESLint on a single file.
type ESLintChecker struct {
	parseFunc  ParseFunc
	fixOnClean bool // when true, run eslint --fix if no issues are found
}

// NewESLint creates an ESLintChecker with the given parse function.
// parseFunc is typically parse.ESLint, injected from main to avoid import cycles.
func NewESLint(parseFunc ParseFunc) *ESLintChecker {
	return &ESLintChecker{parseFunc: parseFunc}
}

// NewESLintFixOnClean creates an ESLintChecker that auto-fixes when the file is clean.
func NewESLintFixOnClean(parseFunc ParseFunc) *ESLintChecker {
	return &ESLintChecker{parseFunc: parseFunc, fixOnClean: true}
}

func (c *ESLintChecker) Name() string { return "LINT" }

func (c *ESLintChecker) Run(file string, timeout time.Duration) Result {
	cmdStr := fmt.Sprintf("eslint --format=stylish %s", file)

	var shell, shellFlag string
	if runtime.GOOS == "windows" {
		shell, shellFlag = "cmd", "/c"
	} else {
		shell, shellFlag = "sh", "-c"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, shellFlag, cmdStr)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Name: "LINT", Timed: true}
	}

	output := buf.String()
	if runErr != nil && output == "" {
		return Result{Name: "LINT", Err: runErr}
	}

	issues := c.parseFunc(output, file)
	if len(issues) == 0 && c.fixOnClean {
		// Best-effort — ignore errors; the check result is still clean.
		fixCmd := exec.Command(shell, shellFlag, fmt.Sprintf("eslint --fix %s", file))
		_ = fixCmd.Run()
	}
	return Result{Name: "LINT", Issues: issues}
}
