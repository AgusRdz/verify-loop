package parse

import (
	"testing"
)

func TestStylelint(t *testing.T) {
	output := `src/styles/main.css: line 10, col 3, error - Expected a leading zero (number-leading-zero)
src/styles/main.css: line 15, col 1, warning - Unexpected empty block (block-no-empty)
src/styles/other.css: line 5, col 1, error - Should not appear`

	issues := Stylelint(output, "src/styles/main.css")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Line != 10 {
		t.Errorf("expected line 10, got %d", issues[0].Line)
	}
	if issues[0].Message != "Expected a leading zero (number-leading-zero)" {
		t.Errorf("unexpected message: %q", issues[0].Message)
	}
	if issues[1].Line != 15 {
		t.Errorf("expected line 15, got %d", issues[1].Line)
	}
}

func TestStylelintNoMatch(t *testing.T) {
	output := `src/styles/other.css: line 5, col 1, error - Should not appear`
	issues := Stylelint(output, "src/styles/main.css")
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestStylelintEmpty(t *testing.T) {
	issues := Stylelint("", "src/styles/main.css")
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestGetStylelint(t *testing.T) {
	p := Get("stylelint")
	if p == nil {
		t.Fatal("Get(\"stylelint\") returned nil")
	}
	_ = p("", "file.css")
}
