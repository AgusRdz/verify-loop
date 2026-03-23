package format

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agusrdz/verify-loop/check"
)

func TestCompactEmpty(t *testing.T) {
	out := Compact("src/foo.ts", nil, 30)
	if out != "" {
		t.Errorf("expected empty string for nil results, got %q", out)
	}
}

func TestCompactClean(t *testing.T) {
	results := []check.Result{
		{Name: "TSC", Issues: nil},
		{Name: "LINT", Issues: nil},
	}
	out := Compact("src/foo.ts", results, 30)
	if !strings.HasPrefix(out, "VERIFY src/foo.ts") {
		t.Errorf("missing VERIFY header: %q", out)
	}
	if !strings.Contains(out, "✓ clean") {
		t.Errorf("expected clean signal: %q", out)
	}
	if !strings.Contains(out, "TSC") || !strings.Contains(out, "LINT") {
		t.Errorf("expected tool names in clean signal: %q", out)
	}
}

func TestCompactErrors(t *testing.T) {
	results := []check.Result{
		{Name: "TSC", Issues: []check.Issue{
			{Line: 23, Message: "Type 'string' is not assignable to type 'number'"},
			{Line: 45, Message: "Property 'userId' does not exist on type 'Session'"},
		}},
		{Name: "LINT", Issues: []check.Issue{
			{Line: 12, Message: "no-unused-vars: 'token' is defined but never used"},
		}},
	}
	out := Compact("src/services/auth.ts", results, 30)

	if !strings.HasPrefix(out, "VERIFY src/services/auth.ts") {
		t.Errorf("missing VERIFY header: %q", out)
	}
	if !strings.Contains(out, "✗ TSC") {
		t.Errorf("expected TSC error lines: %q", out)
	}
	if !strings.Contains(out, "✗ LINT") {
		t.Errorf("expected LINT error lines: %q", out)
	}
	if !strings.Contains(out, "── 3 errors") {
		t.Errorf("expected summary with 3 errors: %q", out)
	}
	if !strings.Contains(out, "TSC: 2") {
		t.Errorf("expected TSC: 2 in summary: %q", out)
	}
	if !strings.Contains(out, "LINT: 1") {
		t.Errorf("expected LINT: 1 in summary: %q", out)
	}
}

func TestCompactSingularError(t *testing.T) {
	results := []check.Result{
		{Name: "TSC", Issues: []check.Issue{{Line: 1, Message: "error"}}},
	}
	out := Compact("foo.ts", results, 30)
	if !strings.Contains(out, "── 1 error |") {
		t.Errorf("expected singular 'error': %q", out)
	}
}

func TestCompactTimeout(t *testing.T) {
	results := []check.Result{
		{Name: "TSC", Timed: true},
	}
	out := Compact("foo.ts", results, 30)
	if !strings.Contains(out, "⚠ TSC timed out after 30s") {
		t.Errorf("expected timeout message: %q", out)
	}
	if strings.Contains(out, "✓ clean") {
		t.Errorf("should not emit clean on timeout: %q", out)
	}
}

func TestCompactCheckerError(t *testing.T) {
	results := []check.Result{
		{Name: "VET", Err: fmt.Errorf("go: command not found")},
	}
	out := Compact("foo.go", results, 30)
	if !strings.Contains(out, "⚠ VET failed to run") {
		t.Errorf("expected error message: %q", out)
	}
}

func TestCompactMessageTruncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	results := []check.Result{
		{Name: "TSC", Issues: []check.Issue{{Line: 1, Message: long}}},
	}
	out := Compact("foo.ts", results, 30)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "✗") {
			runes := []rune(line)
			// message part should be truncated
			if !strings.HasSuffix(line, "…") {
				t.Errorf("expected truncation with …: %q", line)
			}
			_ = runes
		}
	}
}
