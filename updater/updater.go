package updater

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const repo = "AgusRdz/verify-loop"

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// publicKey is the hex-encoded Ed25519 public key used to verify release signatures.
const publicKey = "d21047a328e7332b86dd2535ea4407d1454cd489a7abb0c89be58a2ad0397cbc"

type ghRelease struct {
	TagName string `json:"tag_name"`
}

// Run checks for the latest version and updates the binary if needed.
func Run(currentVersion string) {
	fmt.Println("checking for updates...")

	latest, err := latestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: failed to check for updates: %v\n", err)
		os.Exit(1)
	}

	if latest == currentVersion {
		fmt.Printf("already up to date (%s)\n", currentVersion)
		return
	}

	fmt.Printf("updating %s -> %s\n", currentVersion, latest)

	binaryName := buildBinaryName()
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, binaryName)

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: failed to find current binary: %v\n", err)
		os.Exit(1)
	}

	tmpPath := exe + ".tmp"
	if err := download(url, tmpPath); err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: update failed: %v\n", err)
		os.Remove(tmpPath)
		os.Exit(1)
	}

	if err := verifyChecksum(tmpPath, latest, binaryName); err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: verification failed: %v\n", err)
		os.Remove(tmpPath)
		os.Exit(1)
	}

	if err := replaceBinary(exe, tmpPath); err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: failed to replace binary: %v\n", err)
		os.Remove(tmpPath)
		os.Exit(1)
	}

	fmt.Printf("updated to %s\n", latest)
	clearUpdateAvailable()
}

func latestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("could not reach GitHub (check your internet connection or firewall): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func buildBinaryName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("verify-loop-%s-%s%s", goos, goarch, ext)
}

func download(url, destPath string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d for %s", resp.StatusCode, url)
	}

	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		return fmt.Errorf("failed to verify downloaded file: %w", err)
	}
	if info.Size() < 1024 {
		return fmt.Errorf("downloaded file too small (%d bytes), release may not exist", info.Size())
	}
	return checkBinaryMagic(destPath)
}

// checkBinaryMagic verifies the file starts with the expected magic bytes for the current platform.
func checkBinaryMagic(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open binary for validation: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 4)
	if _, err := io.ReadFull(f, buf); err != nil {
		return fmt.Errorf("binary too small to read magic bytes: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		if buf[0] != 0x7f || buf[1] != 'E' || buf[2] != 'L' || buf[3] != 'F' {
			return fmt.Errorf("downloaded file is not a valid ELF binary")
		}
	case "darwin":
		valid := (buf[0] == 0xca && buf[1] == 0xfe && buf[2] == 0xba && buf[3] == 0xbe) ||
			(buf[0] == 0xcf && buf[1] == 0xfa && buf[2] == 0xed && buf[3] == 0xfe) ||
			(buf[0] == 0xce && buf[1] == 0xfa && buf[2] == 0xed && buf[3] == 0xfe) ||
			(buf[0] == 0xfe && buf[1] == 0xed && buf[2] == 0xfa && buf[3] == 0xcf) ||
			(buf[0] == 0xfe && buf[1] == 0xed && buf[2] == 0xfa && buf[3] == 0xce)
		if !valid {
			return fmt.Errorf("downloaded file is not a valid Mach-O binary")
		}
	case "windows":
		if buf[0] != 'M' || buf[1] != 'Z' {
			return fmt.Errorf("downloaded file is not a valid PE binary")
		}
	}
	return nil
}

// IsDev reports whether the version looks like a dev build.
func IsDev(version string) bool {
	return version == "dev" || strings.Contains(version, "-dirty")
}

// isNewer reports whether version a is strictly newer than version b.
func isNewer(a, b string) bool {
	pa, ok1 := parseSemver(a)
	pb, ok2 := parseSemver(b)
	if !ok1 || !ok2 {
		return false
	}
	if pa[0] != pb[0] {
		return pa[0] > pb[0]
	}
	if pa[1] != pb[1] {
		return pa[1] > pb[1]
	}
	return pa[2] > pb[2]
}

func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return [3]int{}, false
			}
			n = n*10 + int(c-'0')
		}
		nums[i] = n
	}
	return nums, true
}

// verifyChecksum fetches checksums.txt and checksums.txt.sig, verifies the Ed25519
// signature, then verifies the SHA256 hash of the binary.
func verifyChecksum(binaryPath, version, binaryName string) error {
	checksums, err := fetchReleaseFile(version, "checksums.txt")
	if err != nil {
		return fmt.Errorf("failed to fetch checksums.txt: %w", err)
	}

	signature, err := fetchReleaseFile(version, "checksums.txt.sig")
	if err != nil {
		return fmt.Errorf("failed to fetch checksums.txt.sig: %w", err)
	}

	if err := verifySignature(checksums, signature); err != nil {
		return fmt.Errorf("invalid signature for checksums.txt: %w", err)
	}

	expected, err := parseChecksum(string(checksums), binaryName)
	if err != nil {
		return err
	}

	actual, err := hashFile(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to hash downloaded binary: %w", err)
	}

	if actual != expected {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func verifySignature(message, signature []byte) error {
	pub, err := hex.DecodeString(publicKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	sig, err := hex.DecodeString(strings.TrimSpace(string(signature)))
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	if !ed25519.Verify(pub, message, sig) {
		return errors.New("ED25519 signature verification failed")
	}
	return nil
}

func fetchReleaseFile(version, filename string) ([]byte, error) {
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, filename)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("%s not found (404)", filename)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s returned %d", filename, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func parseChecksum(checksums, binaryName string) (string, error) {
	for _, line := range strings.Split(checksums, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimPrefix(parts[1], "*")
		if name == binaryName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", binaryName)
}

func replaceBinary(destPath, srcPath string) error {
	if runtime.GOOS == "windows" {
		oldPath := destPath + ".old"
		os.Remove(oldPath)
		if err := os.Rename(destPath, oldPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(srcPath, destPath); err != nil {
			os.Rename(oldPath, destPath) //nolint
			return err
		}
		os.Remove(oldPath)
		return nil
	}
	return os.Rename(srcPath, destPath)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
