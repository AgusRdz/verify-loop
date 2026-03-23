package check

import (
	"fmt"
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
	output, timed, runErr := runCmd(fmt.Sprintf("gofmt -l %s", file), "", timeout)
	if timed {
		return Result{Name: "FMT", Timed: true}
	}
	if runErr != nil && output == "" {
		return Result{Name: "FMT", Err: runErr}
	}
	issues := c.parseFunc(output, file)
	return Result{Name: "FMT", Issues: issues}
}
