package parse

import (
	"regexp"
	"strings"

	"github.com/agusrdz/verify-loop/check"
)

// reMSBuild matches lines like:
//
//	src\Services\AuthService.cs(23,5): error CS0103: message [/project/App.csproj]
var reMSBuild = regexp.MustCompile(`^(?P<file>[^(\n]+)\((?P<line>\d+),\d+\):\s*(?:error|warning)\s+\w+:\s*(?P<msg>[^[]+)`)

// MSBuild parses MSBuild / dotnet build output and returns issues for targetFile.
func MSBuild(output, targetFile string) []check.Issue {
	var issues []check.Issue
	for _, line := range strings.Split(output, "\n") {
		g := namedGroups(reMSBuild, line)
		if g == nil {
			continue
		}
		// Normalize backslash paths from Windows MSBuild output.
		// Use strings.ReplaceAll instead of filepath.ToSlash so this works
		// correctly even when running on Linux (where ToSlash is a no-op).
		issueFile := strings.ReplaceAll(strings.TrimSpace(g["file"]), `\`, "/")
		if !fileMatches(issueFile, targetFile) {
			continue
		}
		issues = append(issues, check.Issue{
			File:    issueFile,
			Line:    atoi(g["line"]),
			Message: strings.TrimSpace(g["msg"]),
		})
	}
	return issues
}

func init() {
	parsers["msbuild"] = MSBuild
}
