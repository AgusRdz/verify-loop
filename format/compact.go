package format

import (
	"fmt"
	"strings"

	"github.com/agusrdz/verify-loop/check"
)

// Compact formats checker results into the compact output string.
// relPath is the file path relative to the project root.
// timeoutSeconds is used in the timeout message.
func Compact(relPath string, results []check.Result, timeoutSeconds int) string {
	if len(results) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("VERIFY ")
	b.WriteString(relPath)

	totalIssues := 0
	hasTimeout := false
	hasErr := false

	// per-tool issue counts for summary
	type toolCount struct {
		name  string
		count int
	}
	var toolCounts []toolCount

	for _, r := range results {
		if r.Timed {
			hasTimeout = true
			b.WriteByte('\n')
			fmt.Fprintf(&b, "⚠ %s timed out after %ds — run manually", r.Name, timeoutSeconds)
			continue
		}
		if r.Err != nil {
			hasErr = true
			b.WriteByte('\n')
			fmt.Fprintf(&b, "⚠ %s failed to run: %s", r.Name, r.Err.Error())
			continue
		}
		for _, issue := range r.Issues {
			msg := issue.Message
			if len([]rune(msg)) > 80 {
				runes := []rune(msg)
				msg = string(runes[:79]) + "…"
			}
			b.WriteByte('\n')
			fmt.Fprintf(&b, "✗ %-6s L%-4d %s", r.Name, issue.Line, msg)
		}
		if len(r.Issues) > 0 {
			toolCounts = append(toolCounts, toolCount{name: r.Name, count: len(r.Issues)})
			totalIssues += len(r.Issues)
		}
	}

	// summary line — only when there are actual issues (not timeouts/errs)
	if totalIssues > 0 {
		errorWord := "errors"
		if totalIssues == 1 {
			errorWord = "error"
		}
		b.WriteByte('\n')
		parts := make([]string, 0, len(toolCounts))
		for _, tc := range toolCounts {
			parts = append(parts, fmt.Sprintf("%s: %d", tc.name, tc.count))
		}
		fmt.Fprintf(&b, "── %d %s | %s", totalIssues, errorWord, strings.Join(parts, " | "))
		return b.String()
	}

	// clean signal — only if no timeouts and no errors
	if !hasTimeout && !hasErr {
		names := make([]string, 0, len(results))
		for _, r := range results {
			names = append(names, r.Name)
		}
		b.WriteByte('\n')
		fmt.Fprintf(&b, "✓ clean — %s (0 errors)", strings.Join(names, ", "))
	}

	return b.String()
}
