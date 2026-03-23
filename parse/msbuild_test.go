package parse

import (
	"testing"
)

func TestMSBuild(t *testing.T) {
	output := `src\Services\AuthService.cs(23,5): error CS0103: The name 'foo' does not exist in the current context [/project/MyApp.csproj]
src\Services\AuthService.cs(45,1): warning CS0168: The variable 'x' is declared but never used [/project/MyApp.csproj]
src\Other\SomeClass.cs(10,1): error CS0001: Should not appear [/project/MyApp.csproj]`

	issues := MSBuild(output, "src/Services/AuthService.cs")
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

func TestMSBuildBackslashNormalization(t *testing.T) {
	// Backslash paths from Windows MSBuild output should match forward-slash targets.
	output := `src\Foo\Bar.cs(1,1): error CS0001: some error [/proj/App.csproj]`
	issues := MSBuild(output, "src/Foo/Bar.cs")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}

func TestMSBuildNoMatch(t *testing.T) {
	output := `src\Other\File.cs(5,1): error CS0001: irrelevant [/proj/App.csproj]`
	issues := MSBuild(output, "src/Services/AuthService.cs")
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestGetMSBuild(t *testing.T) {
	p := Get("msbuild")
	if p == nil {
		t.Fatal("Get(\"msbuild\") returned nil")
	}
	// Verify it runs without panic.
	_ = p("", "file.cs")
}
