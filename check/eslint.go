package check

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ESLintChecker runs ESLint on a single file.
type ESLintChecker struct {
	parseFunc   ParseFunc
	fixOnClean  bool   // when true, run eslint --fix if no issues are found
	projectRoot string // used to find local node_modules/.bin/eslint
}

// eslintBin returns the local eslint binary if present, falling back to the global one.
func eslintBin(projectRoot string) string {
	candidates := []string{
		filepath.Join(projectRoot, "node_modules", ".bin", "eslint"),
		filepath.Join(projectRoot, "node_modules", ".bin", "eslint.cmd"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "eslint"
}

// NewESLint creates an ESLintChecker with the given parse function.
// parseFunc is typically parse.ESLint, injected from main to avoid import cycles.
func NewESLint(parseFunc ParseFunc) *ESLintChecker {
	return &ESLintChecker{parseFunc: parseFunc}
}

// NewESLintWithRoot creates an ESLintChecker that uses the local eslint binary.
func NewESLintWithRoot(projectRoot string, parseFunc ParseFunc, fixOnClean bool) *ESLintChecker {
	return &ESLintChecker{parseFunc: parseFunc, fixOnClean: fixOnClean, projectRoot: projectRoot}
}

// NewESLintFixOnClean creates an ESLintChecker that auto-fixes when the file is clean.
func NewESLintFixOnClean(parseFunc ParseFunc) *ESLintChecker {
	return &ESLintChecker{parseFunc: parseFunc, fixOnClean: true}
}

func (c *ESLintChecker) Name() string { return "LINT" }

func (c *ESLintChecker) Run(file string, timeout time.Duration) Result {
	eslint := eslintBin(c.projectRoot)
	output, timed, runErr := runCmd(fmt.Sprintf("%s --format=stylish %s", eslint, file), "", timeout)
	if timed {
		return Result{Name: "LINT", Timed: true}
	}
	if runErr != nil && output == "" {
		return Result{Name: "LINT", Err: runErr}
	}
	issues := c.parseFunc(output, file)
	if len(issues) == 0 && c.fixOnClean {
		runCmd(fmt.Sprintf("%s --fix %s", eslint, file), "", timeout) //nolint
	}
	return Result{Name: "LINT", Issues: issues}
}
