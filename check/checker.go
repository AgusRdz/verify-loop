package check

import "time"

// Issue is a single diagnostic from a checker.
type Issue struct {
	File    string
	Line    int
	Message string
}

// Result is the output of one checker run.
type Result struct {
	Name   string
	Issues []Issue
	Err    error
	Timed  bool // true if the checker hit the timeout
}

// Checker runs a static analysis tool on a single file and returns issues.
type Checker interface {
	Name() string
	Run(file string, timeout time.Duration) Result
}

// Registry maps checker names to implementations.
var registry = map[string]Checker{}

// Register adds a checker to the registry under its Name().
func Register(c Checker) {
	registry[c.Name()] = c
}

// RegisterAs adds a checker to the registry under an explicit key.
// Use this to register under config-friendly lowercase aliases (e.g. "eslint")
// so that check.Get(ccfg.Builtin) works when Builtin matches the alias.
func RegisterAs(name string, c Checker) {
	registry[name] = c
}

// Get returns a checker by name, or nil if not found.
func Get(name string) Checker {
	return registry[name]
}
