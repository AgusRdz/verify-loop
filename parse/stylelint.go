package parse

import (
	"regexp"
	"strings"

	"github.com/agusrdz/verify-loop/check"
)

// reStylelint matches the stylelint compact formatter output:
//
//	src/styles/main.css: line 10, col 3, error - Expected a leading zero (number-leading-zero)
var reStylelint = regexp.MustCompile(`^(?P<file>[^:]+): line (?P<line>\d+), col \d+, (?:error|warning) - (?P<msg>.+)$`)

// Stylelint parses stylelint --formatter=compact output.
func Stylelint(output, targetFile string) []check.Issue {
	var issues []check.Issue
	for _, line := range strings.Split(output, "\n") {
		g := namedGroups(reStylelint, line)
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

func init() {
	parsers["stylelint"] = Stylelint
}
