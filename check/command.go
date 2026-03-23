package check

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/agusrdz/verify-loop/config"
)

// ParseFunc parses raw tool output and returns issues scoped to targetFile.
type ParseFunc func(output, targetFile string) []Issue

// CommandChecker implements Checker for config-defined shell commands.
type CommandChecker struct {
	name        string
	command     string // shell command, may contain {file} placeholder
	scope       string // "file" (default) or "project"
	projectRoot string // needed for project-scope runs (set externally)
	parseFunc   ParseFunc
}

// NewCommand creates a CommandChecker from a CheckerConfig.
// parseFunc is looked up by the caller and passed in to avoid circular imports.
func NewCommand(cfg config.CheckerConfig, projectRoot string, parseFunc ParseFunc) *CommandChecker {
	scope := cfg.Scope
	if scope == "" {
		scope = "file"
	}
	return &CommandChecker{
		name:        cfg.Name,
		command:     cfg.Command,
		scope:       scope,
		projectRoot: projectRoot,
		parseFunc:   parseFunc,
	}
}

func (c *CommandChecker) Name() string {
	return c.name
}

func (c *CommandChecker) Run(file string, timeout time.Duration) Result {
	// Build the command string.
	cmdStr := c.command
	if strings.Contains(cmdStr, "{file}") {
		cmdStr = strings.ReplaceAll(cmdStr, "{file}", file)
	} else if c.scope != "project" {
		// file-scope with no placeholder: append the file as an argument
		cmdStr = fmt.Sprintf("%s %s", cmdStr, file)
	}

	// Select shell based on OS.
	var shell, shellFlag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		shellFlag = "/c"
	} else {
		shell = "sh"
		shellFlag = "-c"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, shellFlag, cmdStr)
	if c.scope == "project" && c.projectRoot != "" {
		cmd.Dir = c.projectRoot
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()

	// Timeout takes priority.
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Name: c.name, Timed: true}
	}

	output := buf.String()

	// A non-zero exit code with output is normal for linters.
	// Only treat it as Err if the command failed to produce any output and
	// exec itself reported an error (i.e., binary not found or similar).
	if runErr != nil && output == "" {
		return Result{Name: c.name, Err: runErr}
	}

	issues := c.parseFunc(output, file)
	return Result{Name: c.name, Issues: issues}
}
