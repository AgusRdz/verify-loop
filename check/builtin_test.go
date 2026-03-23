package check

import (
	"testing"
)

// noopParse is a minimal ParseFunc for testing — returns no issues.
func noopParse(output, targetFile string) []Issue { return nil }

func TestGetRegistered(t *testing.T) {
	// Register built-ins under their lowercase config aliases (matching main.go's init).
	RegisterAs("eslint", NewESLint(noopParse))
	RegisterAs("govet", NewGoVet(noopParse))
	RegisterAs("gofmt", NewGoFmt(noopParse))
	RegisterAs("stylelint", NewStylelint(noopParse))
	RegisterAs("json", NewJSON())

	for _, name := range []string{"eslint", "govet", "gofmt", "stylelint", "json"} {
		if Get(name) == nil {
			t.Errorf("Get(%q) returned nil — checker not registered", name)
		}
	}
}

func TestTSCName(t *testing.T) {
	c := NewTSC("/tmp", noopParse, false)
	if c.Name() != "TSC" {
		t.Errorf("expected TSC, got %q", c.Name())
	}
}

func TestESLintName(t *testing.T) {
	c := NewESLint(noopParse)
	if c.Name() != "LINT" {
		t.Errorf("expected LINT, got %q", c.Name())
	}
}

func TestGoVetName(t *testing.T) {
	c := NewGoVet(noopParse)
	if c.Name() != "VET" {
		t.Errorf("expected VET, got %q", c.Name())
	}
}

func TestGoFmtName(t *testing.T) {
	c := NewGoFmt(noopParse)
	if c.Name() != "FMT" {
		t.Errorf("expected FMT, got %q", c.Name())
	}
}
