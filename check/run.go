package check

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// runCmd executes a shell command with a timeout and returns (output, timed out, error).
// On Windows it kills the full process tree when the timeout fires, preventing
// orphaned node/tsc processes from blocking cmd.Run() indefinitely.
func runCmd(cmdStr, workDir string, timeout time.Duration) (output string, timed bool, err error) {
	var shell, shellFlag string
	if runtime.GOOS == "windows" {
		shell, shellFlag = "cmd", "/c"
	} else {
		shell, shellFlag = "sh", "-c"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, shellFlag, cmdStr)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	// Run in a goroutine so we can force-kill on timeout without blocking.
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return "", false, err
	}
	go func() { done <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		// Kill the process tree (important on Windows where cmd /c spawns node).
		if cmd.Process != nil {
			if runtime.GOOS == "windows" {
				exec.Command("taskkill", "/F", "/T", "/PID",
					fmt.Sprintf("%d", cmd.Process.Pid)).Run() //nolint
			} else {
				cmd.Process.Kill() //nolint
			}
		}
		return buf.String(), true, nil
	case runErr := <-done:
		return buf.String(), false, runErr
	}
}
