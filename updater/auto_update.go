package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const checkInterval = 24 * time.Hour

func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "verify-loop")
	os.MkdirAll(dir, 0o700)
	return dir, nil
}

func lastCheckPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last-update-check"), nil
}

func pendingUpdatePath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pending-update"), nil
}

func shouldCheck() bool {
	path, err := lastCheckPath()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > checkInterval
}

func touchLastCheck() {
	path, err := lastCheckPath()
	if err != nil {
		return
	}
	os.WriteFile(path, []byte(time.Now().Format(time.RFC3339)), 0o600) //nolint
}

// ApplyPendingUpdate checks for a pending update downloaded in a previous run.
// If found, replaces the current binary. Silent on all errors.
func ApplyPendingUpdate(currentVersion string) {
	if IsDev(currentVersion) {
		return
	}

	pending, err := pendingUpdatePath()
	if err != nil {
		return
	}

	data, err := os.ReadFile(pending)
	if err != nil {
		return
	}

	if !IsAutoUpdateEnabled() {
		parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
		os.Remove(pending)
		if len(parts) == 2 {
			os.Remove(parts[1])
		}
		return
	}

	// Format: "version\ntmpBinaryPath\nsha256hash"
	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 3)
	if len(parts) != 3 {
		os.Remove(pending)
		return
	}

	newVersion := parts[0]
	tmpBinary := parts[1]
	expectedHash := parts[2]

	safeDir, err := dataDir()
	if err != nil {
		os.Remove(pending)
		return
	}
	cleanBinary := filepath.Clean(tmpBinary)
	if !strings.HasPrefix(cleanBinary, safeDir+string(filepath.Separator)) {
		os.Remove(pending)
		return
	}
	tmpBinary = cleanBinary

	info, err := os.Stat(tmpBinary)
	if err != nil || info.Size() < 1024 {
		os.Remove(pending)
		os.Remove(tmpBinary)
		return
	}
	actualHash, err := hashFile(tmpBinary)
	if err != nil || actualHash != expectedHash {
		os.Remove(pending)
		os.Remove(tmpBinary)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		os.Remove(pending)
		return
	}

	if err := replaceBinary(exe, tmpBinary); err != nil {
		os.Remove(pending)
		os.Remove(tmpBinary)
		return
	}

	os.Remove(pending)
	fmt.Fprintf(os.Stderr, "verify-loop: auto-updated %s -> %s\n", currentVersion, newVersion)
}

// BackgroundCheck spawns a detached subprocess to check for updates.
// Silent on all errors.
func BackgroundCheck(currentVersion string) {
	if IsDev(currentVersion) {
		return
	}
	if !shouldCheck() {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe, "--_bg-update", currentVersion)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if cmd.Start() == nil {
		touchLastCheck()
	}
}

// RunBackgroundUpdate performs the version check and optionally downloads.
// Called by the subprocess spawned from BackgroundCheck.
func RunBackgroundUpdate(currentVersion string) {
	latest, err := latestVersion()
	if err != nil || !isNewer(latest, currentVersion) {
		clearUpdateAvailable()
		return
	}

	recordUpdateAvailable(latest)

	if !IsAutoUpdateEnabled() {
		return
	}

	dir, err := dataDir()
	if err != nil {
		return
	}

	tmpPath := filepath.Join(dir, "pending.bin")
	binaryName := buildBinaryName()
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, binaryName)

	if err := download(url, tmpPath); err != nil {
		os.Remove(tmpPath)
		return
	}

	if err := verifyChecksum(tmpPath, latest, binaryName); err != nil {
		os.Remove(tmpPath)
		return
	}

	pending, err := pendingUpdatePath()
	if err != nil {
		os.Remove(tmpPath)
		return
	}

	hash, err := hashFile(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return
	}

	content := fmt.Sprintf("%s\n%s\n%s", latest, tmpPath, hash)
	os.WriteFile(pending, []byte(content), 0o600) //nolint
}
