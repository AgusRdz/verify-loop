package parse

import (
	"testing"
)

func TestFileMatches(t *testing.T) {
	cases := []struct {
		issue, target string
		want          bool
	}{
		{"src/foo.ts", "src/foo.ts", true},
		{"./src/foo.ts", "src/foo.ts", true},          // strip leading ./
		{"/abs/src/foo.ts", "src/foo.ts", true},       // absolute issue, relative target
		{"src/foo.ts", "/abs/src/foo.ts", true},       // relative issue, absolute target
		{"foo.ts", "src/foo.ts", true},                // bare filename matches path component
		{"src/bar.ts", "src/foo.ts", false},
		{"src/afoo.ts", "foo.ts", false},              // no false suffix match
	}
	for _, c := range cases {
		got := fileMatches(c.issue, c.target)
		if got != c.want {
			t.Errorf("fileMatches(%q, %q) = %v, want %v", c.issue, c.target, got, c.want)
		}
	}
}

func TestTSC(t *testing.T) {
	output := `src/services/auth.ts(23,5): error TS2345: Argument of type 'string' is not assignable to type 'number'
src/services/auth.ts(45,1): error TS2304: Cannot find name 'foo'
src/other.ts(10,1): error TS2304: Should not appear`

	issues := TSC(output, "src/services/auth.ts")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Line != 23 {
		t.Errorf("expected line 23, got %d", issues[0].Line)
	}
	if issues[1].Line != 45 {
		t.Errorf("expected line 45, got %d", issues[1].Line)
	}
}

func TestTSCAbsoluteTarget(t *testing.T) {
	output := `src/services/auth.ts(23,5): error TS2345: Some error`
	issues := TSC(output, "/project/src/services/auth.ts")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}

func TestGoVet(t *testing.T) {
	output := `./pkg/foo.go:12:3: printf: wrong number of args
./pkg/foo.go:30:1: unreachable code
./pkg/bar.go:5:1: should not appear`

	issues := GoVet(output, "pkg/foo.go")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Line != 12 {
		t.Errorf("expected line 12, got %d", issues[0].Line)
	}
}

func TestGoFmt(t *testing.T) {
	output := "pkg/foo.go\npkg/bar.go\n"
	issues := GoFmt(output, "pkg/foo.go")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Message != "not gofmt'd" {
		t.Errorf("unexpected message: %q", issues[0].Message)
	}
	if issues[0].Line != 0 {
		t.Errorf("expected line 0, got %d", issues[0].Line)
	}
}

func TestESLint(t *testing.T) {
	output := `/project/src/app.ts
  23:5  error  no-unused-vars  'x' is defined but never used
  45:1  warning  no-console   Unexpected console statement
/project/src/other.ts
  10:1  error  no-unused-vars  Should not appear`

	issues := ESLint(output, "/project/src/app.ts")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Line != 23 {
		t.Errorf("expected line 23, got %d", issues[0].Line)
	}
}

func TestGeneric(t *testing.T) {
	output := `src/foo.ts:10:5: error: something went wrong
src/foo.ts:20: error: another error
src/bar.ts:1:1: error: should not appear`

	issues := Generic(output, "src/foo.ts")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
}

func TestGetFallback(t *testing.T) {
	p := Get("unknown-parser")
	if p == nil {
		t.Fatal("expected non-nil fallback parser")
	}
	// Should behave like Generic — just verify it runs without panic
	_ = p("some output", "file.ts")
}

func TestGetKnown(t *testing.T) {
	for _, name := range []string{"", "generic", "tsc", "govet", "gofmt", "eslint", "msbuild", "rustc", "cargo"} {
		p := Get(name)
		if p == nil {
			t.Errorf("Get(%q) returned nil", name)
		}
	}
}

func TestGetRegexParser(t *testing.T) {
	p := Get(`regex:^(?P<file>[^:]+):(?P<line>\d+):(?P<msg>.+)$`)
	if p == nil {
		t.Fatal("expected non-nil regex parser")
	}
	output := "src/foo.py:10:something went wrong\nsrc/bar.py:5:should not appear"
	issues := p(output, "src/foo.py")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Line != 10 {
		t.Errorf("expected line 10, got %d", issues[0].Line)
	}
	if issues[0].Message != "something went wrong" {
		t.Errorf("unexpected message: %q", issues[0].Message)
	}
}

func TestGetRegexBadPattern(t *testing.T) {
	// Invalid regex should fall back to Generic, not panic.
	p := Get("regex:[invalid")
	if p == nil {
		t.Fatal("expected non-nil fallback for bad regex")
	}
	_ = p("some output", "file.py") // must not panic
}

