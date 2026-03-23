package check

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// GoFmtChecker runs gofmt -l on a single file to check formatting.
type GoFmtChecker struct {
	parseFunc ParseFunc
}

// NewGoFmt creates a GoFmtChecker with the given parse function.
// parseFunc is typically parse.GoFmt, injected from main to avoid import cycles.
func NewGoFmt(parseFunc ParseFunc) *GoFmtChecker {
	return &GoFmtChecker{parseFunc: parseFunc}
}

func (c *GoFmtChecker) Name() string { return "FMT" }

func (c *GoFmtChecker) Run(file string, timeout time.Duration) Result {
	cmdStr := fmt.Sprintf("gofmt -l %s", file)

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
		return Result{Name: "FMT", Timed: true}
	}

	output := buf.String()
	if runErr != nil && output == "" {
		return Result{Name: "FMT", Err: runErr}
	}

	issues := c.parseFunc(output, file)
	return Result{Name: "FMT", Issues: issues}
}
