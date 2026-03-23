package parse

import (
	"testing"
)

const rustcOutput = `error[E0425]: cannot find value ` + "`foo`" + ` in this scope
  --> src/main.rs:10:5
   |
10 |     foo;
   |     ^^^

warning[W0001]: unused variable
  --> src/main.rs:20:9
   |
20 |     let x = 1;
   |         ^

error[E0308]: mismatched types
  --> src/lib.rs:5:1
   |
5  |     42
   |     ^^ should not appear`

func TestRustc(t *testing.T) {
	issues := Rustc(rustcOutput, "src/main.rs")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues for src/main.rs, got %d", len(issues))
	}
	if issues[0].Line != 10 {
		t.Errorf("expected line 10, got %d", issues[0].Line)
	}
	if issues[1].Line != 20 {
		t.Errorf("expected line 20, got %d", issues[1].Line)
	}
}

func TestRustcOtherFile(t *testing.T) {
	issues := Rustc(rustcOutput, "src/lib.rs")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for src/lib.rs, got %d", len(issues))
	}
	if issues[0].Line != 5 {
		t.Errorf("expected line 5, got %d", issues[0].Line)
	}
}

func TestGetRustc(t *testing.T) {
	p := Get("rustc")
	if p == nil {
		t.Fatal("Get(\"rustc\") returned nil")
	}
	_ = p("", "file.rs")
}

func TestGetCargo(t *testing.T) {
	p := Get("cargo")
	if p == nil {
		t.Fatal("Get(\"cargo\") returned nil")
	}
	_ = p("", "file.rs")
}
