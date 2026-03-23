package check

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// GoVetChecker runs go vet on the package containing the written file.
type GoVetChecker struct {
	parseFunc ParseFunc
}

// NewGoVet creates a GoVetChecker with the given parse function.
// parseFunc is typically parse.GoVet, injected from main to avoid import cycles.
func NewGoVet(parseFunc ParseFunc) *GoVetChecker {
	return &GoVetChecker{parseFunc: parseFunc}
}

func (c *GoVetChecker) Name() string { return "VET" }

func (c *GoVetChecker) Run(file string, timeout time.Duration) Result {
	var shell, shellFlag string
	if runtime.GOOS == "windows" {
		shell, shellFlag = "cmd", "/c"
	} else {
		shell, shellFlag = "sh", "-c"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, shellFlag, "go vet .")
	cmd.Dir = filepath.Dir(file)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Name: "VET", Timed: true}
	}

	output := buf.String()
	if runErr != nil && output == "" {
		return Result{Name: "VET", Err: runErr}
	}

	issues := c.parseFunc(output, file)
	return Result{Name: "VET", Issues: issues}
}
