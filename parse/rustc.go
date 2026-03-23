package parse

import (
	"regexp"
	"strings"

	"github.com/agusrdz/verify-loop/check"
)

// reRustMsg matches: error[E0425]: message  OR  warning[W...]: message
var reRustMsg = regexp.MustCompile(`^(?:error|warning)(?:\[[\w:]+\])?:\s*(?P<msg>.+)$`)

// reRustLocation matches:   --> src/main.rs:10:5
var reRustLocation = regexp.MustCompile(`^\s+-->\s+(?P<file>[^:\n]+):(?P<line>\d+):\d+`)

// Rustc parses human-readable rustc / cargo check output.
func Rustc(output, targetFile string) []check.Issue {
	var issues []check.Issue
	var pendingMsg string

	for _, line := range strings.Split(output, "\n") {
		if g := namedGroups(reRustMsg, line); g != nil {
			pendingMsg = strings.TrimSpace(g["msg"])
			continue
		}
		if g := namedGroups(reRustLocation, line); g != nil {
			if pendingMsg == "" {
				continue
			}
			issueFile := strings.TrimSpace(g["file"])
			if fileMatches(issueFile, targetFile) {
				issues = append(issues, check.Issue{
					File:    issueFile,
					Line:    atoi(g["line"]),
					Message: pendingMsg,
				})
			}
			pendingMsg = ""
			continue
		}
		// Any line that isn't a message or location resets the pending message
		// only if it's non-empty and not continuation context (lines with | or ^).
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "|") && !strings.HasPrefix(trimmed, "^") {
			pendingMsg = ""
		}
	}
	return issues
}

func init() {
	parsers["rustc"] = Rustc
	parsers["cargo"] = Rustc
}
