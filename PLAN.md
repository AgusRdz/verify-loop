# verify-loop

> PostToolUse/Write hook that runs configured checks on every file Claude writes, then injects a compact error summary into context before the next turn.

---

## Problem

When Claude writes a file with errors, it doesn't know until either:
1. You manually run a linter and paste the output
2. A subsequent tool call happens to reveal the problem
3. The build fails much later

Between the Write and the feedback, Claude often continues making related changes with wrong assumptions. By the time it sees the error, it has 3 turns of context drift to unwind. `verify-loop` closes this feedback loop immediately.

---

## Architecture

```
PostToolUse/Write
      │
      ▼
  verify-loop
      │
      ├── resolve checkers for this file extension
      │       (config-driven; built-ins pre-registered, fully overridable)
      │
      ├── run each checker with timeout enforcement
      │       ├── parse stdout via named parser or generic fallback
      │       └── filter results to written file (for project-wide tools like tsc)
      │
      ├── normalize → compact error format
      │
      └── stdout: compact summary OR "✓ no errors"
```

### Input (stdin)
```json
{
  "tool": "Write",
  "tool_result": {
    "path": "/abs/path/to/file.ts"
  }
}
```

### Output (stdout)
```json
{
  "action": "continue",
  "output": "<compact error summary or clean signal>"
}
```

---

## Compact Error Format

The raw output from any tool is verbose and wastes tokens. `verify-loop` normalizes everything to:

```
VERIFY src/services/auth.ts
✗ TSC   L23  Type 'string' is not assignable to type 'number'
✗ TSC   L45  Property 'userId' does not exist on type 'Session'
✗ LINT  L12  no-unused-vars: 'token' is defined but never used
── 3 errors | TSC: 2 | LINT: 1
```

Clean signal:
```
VERIFY src/services/auth.ts
✓ clean — TSC, LINT (0 errors)
```

**Rules for compact format:**
- One line per error
- Max 80 chars per line (truncate message, never path or line number)
- Tool label comes from the checker's `name` field in config (uppercase)
- Always end with summary line `── N errors | TOOL: N ...`
- Never include warnings unless `include_warnings: true` in config

---

## Checker Interface

Every check — built-in or custom — is a `Checker`:

```go
type Checker interface {
    Name()    string                        // label in compact output (e.g. "TSC")
    Run(file string, cfg Config) ([]Issue, error)
}

type Issue struct {
    File    string
    Line    int
    Message string
}
```

Built-ins (tsc, eslint, go vet, gofmt, ruff, etc.) implement this interface and are registered by name. Config references them by name or provides a shell command — same pipeline either way.

---

## Config-Driven Dispatch

Checkers are mapped to file extensions in config. Built-ins are pre-registered defaults but fully overridable.

**Location:** `~/.config/verify-loop/config.yml` (global) | `.verify-loop.yml` (project, merged on top)

```yaml
enabled: true
timeout_seconds: 30
include_warnings: false

checkers:
  ".ts":
    - name: TSC
      builtin: tsc
      flags: ["--noEmit", "--incremental"]
      scope: project          # runs on whole project, output filtered to written file
    - name: LINT
      builtin: eslint
      flags: ["--max-warnings", "0"]
      fix_on_clean: false     # run --fix only when no errors; re-runs after fix to confirm clean

  ".go":
    - name: VET
      builtin: govet
    - name: FMT
      builtin: gofmt

  ".py":
    - name: LINT
      command: "ruff check {file}"
      parse: generic          # tries common file:line: message patterns

  ".rs":
    - name: CHECK
      command: "cargo check 2>&1"
      parse: rustc

  ".cs":
    - name: BUILD
      command: "dotnet build --no-restore 2>&1"
      parse: msbuild
      scope: project          # filter output to written file

  "*":                        # runs for any extension not otherwise matched
    - name: CUSTOM
      command: "my-linter {file}"
      parse: generic

skip_paths:
  - "*.generated.ts"
  - "dist/**"
  - "node_modules/**"
  - "bin/**"
  - "obj/**"
```

**`scope` field:**
- `file` (default) — tool takes the file path as argument, output is already scoped
- `project` — tool runs on the whole project; verify-loop filters output to lines matching the written file

**`parse` field:**
- Named parsers: `tsc`, `eslint`, `govet`, `gofmt`, `rustc`, `msbuild`, `generic`
- `generic` tries common patterns: `file:line:col: message`, `file(line): message`, `line | message`
- Custom regex: `parse: "(?P<line>\\d+):(?P<message>.+)"`

---

## Project Detection

Walk up from the written file's directory to find the project root and which tools are available:

```
tsconfig.json     → tsc available
.eslintrc*        → eslint available
go.mod            → go vet / gofmt available
package.json      → node project (check for eslint, vitest, etc.)
angular.json      → angular project
Cargo.toml        → cargo available
pyproject.toml    → python project (ruff, mypy, etc.)
*.sln / *.csproj  → dotnet available
```

Detection result cached per session at `~/.cache/verify-loop/session_<id>.json`. Cache key uses `CLAUDE_SESSION_ID` env var — if unset, falls back to a hash of the project root path. Invalidated if any root marker file mtime changes.

**Detection only affects built-ins** — if a checker uses `command:`, it runs regardless of detected project type.

---

## Default Checkers (Built-ins)

These are pre-registered. Config can override flags, disable, or replace entirely.

| Extension | Default checkers |
|---|---|
| `.ts` `.tsx` | `tsc` (project-scoped, incremental), `eslint` |
| `.js` `.jsx` | `eslint` |
| `.go` | `go vet`, `gofmt -l` |
| `.scss` `.css` | `stylelint` (if config present) |
| `.json` | stdlib JSON parse |

Everything else: no built-in defaults. Add checkers via config.

**TSC notes:**
- Runs on the whole project; output filtered to written file only
- Use `--incremental` only if `.tsbuildinfo` already exists — otherwise warn once and skip
- Prefer `tsconfig.build.json` if present

---

## CLI Surface

```bash
verify-loop run <path>        # run manually on a file (debug/testing)
verify-loop config show       # show resolved config + active checkers for CWD
verify-loop version
```

---

## Phases

### Phase 1 — Core protocol + checker interface
- [ ] stdin/stdout JSON protocol
- [ ] `Checker` interface + registry
- [ ] Timeout enforcement (applies to all checkers)
- [ ] Compact format + summary line
- [ ] `generic` parser (covers most tools out of the box)
- [ ] PostToolUse/Write hook wiring
- [ ] Project detection + session cache with fallback

### Phase 2 — Built-in checkers
- [ ] `tsc` (project-scoped, incremental, output filtered to file)
- [ ] `eslint` (file-scoped, `fix_on_clean` support)
- [ ] `govet` + `gofmt`
- [ ] `msbuild` / `dotnet build` parser
- [ ] `rustc` / `cargo check` parser
- [ ] `stylelint`
- [ ] stdlib JSON validation

### Phase 3 — Config + extensibility
- [ ] `.verify-loop.yml` project config (merged with global)
- [ ] Custom `command:` checkers with `parse:` field
- [ ] Custom regex parser
- [ ] `scope: project` filter logic
- [ ] `config show` CLI command

### Phase 4 — Distribution
- [ ] `verify-loop run <path>` CLI command for manual testing
- [ ] GoReleaser + Homebrew tap

---

## Hook Registration

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write",
        "hooks": [{ "type": "command", "command": "verify-loop" }]
      }
    ]
  }
}
```

---

## Interaction with verify-ui skill

`verify-loop` handles **static analysis** (tsc, lint, fmt, build) — immediate, synchronous, zero browser.
`verify-ui` handles **runtime regressions** (console errors, CSS token drift, blocked scripts) — Playwright-based, triggered on demand.

They are complementary, not overlapping. `verify-loop` fires on every Write; `verify-ui` fires manually or on push.

---

## Metrics (target)

| Scenario | Without | With |
|---|---|---|
| Type error after Write | Discovered N turns later | Injected immediately |
| Lint violation | Manual run required | Auto-surfaced |
| Claude re-introduces same error | Common (no signal) | Rare (signal in context) |
| Raw tool output verbosity | 40-80 lines | 3-8 lines compact |
| New language support | Manual paste | Add 3 lines to config |
