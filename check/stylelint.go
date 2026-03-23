package check

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// StylelintChecker runs stylelint with the compact formatter on a single file.
type StylelintChecker struct {
	parseFunc ParseFunc
}

// NewStylelint creates a StylelintChecker with the given parse function.
// parseFunc is typically parse.Stylelint, injected from main to avoid import cycles.
func NewStylelint(parseFunc ParseFunc) *StylelintChecker {
	return &StylelintChecker{parseFunc: parseFunc}
}

func (c *StylelintChecker) Name() string { return "STYLELINT" }

func (c *StylelintChecker) Run(file string, timeout time.Duration) Result {
	cmdStr := fmt.Sprintf("stylelint --formatter=compact %s", file)

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
		return Result{Name: "STYLELINT", Timed: true}
	}

	output := buf.String()
	if runErr != nil && output == "" {
		return Result{Name: "STYLELINT", Err: runErr}
	}

	issues := c.parseFunc(output, file)
	return Result{Name: "STYLELINT", Issues: issues}
}
