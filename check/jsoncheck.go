package check

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"time"
)

// JSONChecker validates JSON files using the Go standard library.
// No external tool required.
type JSONChecker struct{}

// NewJSON creates a JSONChecker.
func NewJSON() *JSONChecker {
	return &JSONChecker{}
}

func (c *JSONChecker) Name() string { return "JSON" }

func (c *JSONChecker) Run(file string, timeout time.Duration) Result {
	data, err := os.ReadFile(file)
	if err != nil {
		// Don't error on missing/unreadable files — just return clean.
		return Result{Name: "JSON"}
	}

	if json.Valid(data) {
		return Result{Name: "JSON", Issues: nil}
	}

	// Use a decoder to get a byte offset and convert to line number.
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		if err := dec.Decode(&json.RawMessage{}); err != nil {
			if err == io.EOF {
				break
			}
			if synErr, ok := err.(*json.SyntaxError); ok {
				line := bytes.Count(data[:synErr.Offset], []byte("\n")) + 1
				return Result{Name: "JSON", Issues: []Issue{{File: file, Line: line, Message: err.Error()}}}
			}
			return Result{Name: "JSON", Issues: []Issue{{File: file, Line: 0, Message: err.Error()}}}
		}
	}

	return Result{Name: "JSON"}
}
