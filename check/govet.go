package check

import (
	"path/filepath"
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
	output, timed, runErr := runCmd("go vet .", filepath.Dir(file), timeout)
	if timed {
		return Result{Name: "VET", Timed: true}
	}
	if runErr != nil && output == "" {
		return Result{Name: "VET", Err: runErr}
	}
	issues := c.parseFunc(output, file)
	return Result{Name: "VET", Issues: issues}
}
