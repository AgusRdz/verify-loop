package parse

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/agusrdz/verify-loop/check"
)

// Func is the signature for all parsers.
type Func func(output, targetFile string) []check.Issue

var (
	reGeneric1 = regexp.MustCompile(`^(?P<file>[^:\n]+):(?P<line>\d+):\d+:\s*(?:error|warning|note):\s*(?P<msg>.+)$`)
	reGeneric2 = regexp.MustCompile(`^(?P<file>[^:\n]+):(?P<line>\d+):\s*(?:error|warning):\s*(?P<msg>.+)$`)
	reGeneric3 = regexp.MustCompile(`^(?P<file>[^:\n(]+)\((?P<line>\d+)(?:,\d+)?\):\s*(?:error|warning)\s+\w+:\s*(?P<msg>.+)$`)
	reGeneric4 = regexp.MustCompile(`^(?P<file>[^:\n]+):(?P<line>\d+):(?P<msg>.+)$`)

	reTSC    = regexp.MustCompile(`^(?P<file>[^(]+)\((?P<line>\d+),\d+\):\s*(?:error|warning)\s+TS\d+:\s*(?P<msg>.+)$`)
	reGoVet  = regexp.MustCompile(`^(?P<file>\S+\.go):(?P<line>\d+):\d+:\s*(?P<msg>.+)$`)
	reESLint = regexp.MustCompile(`^\s+(?P<line>\d+):\d+\s+(?:error|warning)\s+\S+\s+(?P<msg>.+)$`)
)

// fileMatches returns true if issueFile refers to targetFile.
// Handles both directions: issueFile may be relative (tsc/govet output) while
// targetFile is absolute, or vice versa. Strips leading ./ from both.
func fileMatches(issueFile, targetFile string) bool {
	norm := func(p string) string {
		p = filepath.ToSlash(strings.TrimSpace(p))
		p = strings.TrimPrefix(p, "./")
		return p
	}
	issue := norm(issueFile)
	target := norm(targetFile)
	if issue == target {
		return true
	}
	// issue relative, target absolute: "/abs/src/foo.ts" ends with "/src/foo.ts"
	if strings.HasSuffix(target, "/"+issue) {
		return true
	}
	// issue absolute, target relative
	if strings.HasSuffix(issue, "/"+target) {
		return true
	}
	return false
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func namedGroups(re *regexp.Regexp, line string) map[string]string {
	match := re.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	result := make(map[string]string, len(re.SubexpNames()))
	for i, name := range re.SubexpNames() {
		if name != "" {
			result[name] = match[i]
		}
	}
	return result
}

// Generic parses output using common file:line patterns.
func Generic(output, targetFile string) []check.Issue {
	var issues []check.Issue
	patterns := []*regexp.Regexp{reGeneric1, reGeneric2, reGeneric3, reGeneric4}

	for _, line := range strings.Split(output, "\n") {
		for _, re := range patterns {
			g := namedGroups(re, line)
			if g == nil {
				continue
			}
			if !fileMatches(g["file"], targetFile) {
				break
			}
			issues = append(issues, check.Issue{
				File:    g["file"],
				Line:    atoi(g["line"]),
				Message: strings.TrimSpace(g["msg"]),
			})
			break
		}
	}
	return issues
}

// TSC parses TypeScript compiler output.
func TSC(output, targetFile string) []check.Issue {
	var issues []check.Issue
	for _, line := range strings.Split(output, "\n") {
		g := namedGroups(reTSC, line)
		if g == nil {
			continue
		}
		if !fileMatches(g["file"], targetFile) {
			continue
		}
		issues = append(issues, check.Issue{
			File:    g["file"],
			Line:    atoi(g["line"]),
			Message: strings.TrimSpace(g["msg"]),
		})
	}
	return issues
}

// GoVet parses go vet output.
func GoVet(output, targetFile string) []check.Issue {
	var issues []check.Issue
	for _, line := range strings.Split(output, "\n") {
		g := namedGroups(reGoVet, line)
		if g == nil {
			continue
		}
		if !fileMatches(g["file"], targetFile) {
			continue
		}
		issues = append(issues, check.Issue{
			File:    g["file"],
			Line:    atoi(g["line"]),
			Message: strings.TrimSpace(g["msg"]),
		})
	}
	return issues
}

// GoFmt parses gofmt -l output (one file path per line, no line numbers).
func GoFmt(output, targetFile string) []check.Issue {
	var issues []check.Issue
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !fileMatches(line, targetFile) {
			continue
		}
		issues = append(issues, check.Issue{
			File:    line,
			Line:    0,
			Message: "not gofmt'd",
		})
	}
	return issues
}

// ESLint parses ESLint default formatter output.
func ESLint(output, targetFile string) []check.Issue {
	var issues []check.Issue
	var currentFile string

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		// Indented lines are diagnostics; non-indented are file paths.
		if line[0] != ' ' && line[0] != '\t' {
			currentFile = strings.TrimSpace(line)
			continue
		}
		if !fileMatches(currentFile, targetFile) {
			continue
		}
		g := namedGroups(reESLint, line)
		if g == nil {
			continue
		}
		issues = append(issues, check.Issue{
			File:    currentFile,
			Line:    atoi(g["line"]),
			Message: strings.TrimSpace(g["msg"]),
		})
	}
	return issues
}

var parsers = map[string]Func{
	"":        Generic,
	"generic": Generic,
	"tsc":     TSC,
	"govet":   GoVet,
	"gofmt":   GoFmt,
	"eslint":  ESLint,
}

// Get returns the parser for the given name, falling back to Generic if not found.
// If name starts with "regex:", the remainder is compiled as a regexp with named
// groups "file", "line", and "msg". Example:
//
//	parse: "regex:^(?P<file>[^:]+):(?P<line>\\d+):(?P<msg>.+)$"
func Get(name string) Func {
	if p, ok := parsers[name]; ok {
		return p
	}
	if strings.HasPrefix(name, "regex:") {
		pattern := strings.TrimPrefix(name, "regex:")
		re, err := regexp.Compile(pattern)
		if err != nil {
			// Bad pattern — fall through to Generic so the tool still runs.
			return Generic
		}
		return func(output, targetFile string) []check.Issue {
			var issues []check.Issue
			for _, line := range strings.Split(output, "\n") {
				g := namedGroups(re, line)
				if g == nil {
					continue
				}
				if !fileMatches(g["file"], targetFile) {
					continue
				}
				issues = append(issues, check.Issue{
					File:    g["file"],
					Line:    atoi(g["line"]),
					Message: strings.TrimSpace(g["msg"]),
				})
			}
			return issues
		}
	}
	return Generic
}
