# CLAUDE.md — verify-loop

## What this is

A Go CLI tool that hooks into `PostToolUse/Write` to run configured checks on every file Claude writes, then injects a compact, normalized error summary into context before the next turn. Language-agnostic: any tool that produces output can be wired in via config. Built-ins exist for common languages (TS, Go, C#, Rust, Python) but the core is a generic checker pipeline. Closes the static-analysis feedback loop without requiring manual runs.

## Critical constraints

- **One output format, always** — every response to Claude Code must be either the compact error block or the clean signal. Never raw compiler output. Claude consuming 60 lines of raw TSC output is the problem this solves.
- **Never block on errors** — always return `"action": "continue"`. The goal is to *inform* Claude, not to stop it. Blocking would interrupt its flow for every trivial error.
- **Timeout is mandatory** — any checker can hang. Always enforce the timeout per checker. On timeout, emit: `VERIFY <path>\n⚠ <TOOL> timed out after 30s — run manually`.
- **Skip generated files silently** — if path matches `skip_paths`, output nothing (empty string). Don't tell Claude it was skipped — just don't add noise.
- **Incremental TSC only** — never run `tsc --noEmit` without `--incremental` on first run detection. Cold TSC on large projects is 10-30 seconds. If no `.tsbuildinfo` exists, warn once and suggest running `tsc --incremental` first.

## Repository structure

```
verify-loop/
├── main.go               # CLI routing, help, init/uninstall/doctor/enable/disable
├── color.go              # TTY detection + ANSI color helpers
├── hooks/
│   └── hooks.go          # Claude Code settings.json install/uninstall/status/disable
├── check/
│   ├── checker.go        # Checker interface, Issue/Result types, registry
│   ├── tsc.go            # built-in: tsc (Phase 1)
│   ├── eslint.go         # built-in: eslint (Phase 1)
│   ├── govet.go          # built-in: go vet (Phase 2)
│   ├── gofmt.go          # built-in: gofmt (Phase 2)
│   └── command.go        # generic command runner for config-defined checkers (Phase 3)
├── parse/
│   ├── generic.go        # generic file:line: message parser (Phase 1)
│   ├── tsc.go            # tsc-specific parser (Phase 1)
│   ├── msbuild.go        # msbuild/dotnet parser (Phase 2)
│   └── rustc.go          # rustc/cargo parser (Phase 2)
├── detect/
│   └── project.go        # walk-up tool availability detection + session cache (Phase 1)
├── format/
│   └── compact.go        # normalize all checker output → compact format (Phase 1)
├── config/
│   └── config.go         # Config struct, Load, Path, Show, merge
├── go.mod
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── .verify-loop.yml      # example project config
├── PLAN.md
└── CLAUDE.md
```

## Protocol

**stdin:**
```json
{ "tool": "Write", "tool_result": { "path": "/abs/path/to/file.ts" } }
```

**stdout (errors found):**
```json
{
  "action": "continue",
  "output": "VERIFY src/services/auth.ts\n✗ TSC   L23  Type 'string' is not assignable to type 'number'\n── 1 error | tsc: 1"
}
```

**stdout (clean):**
```json
{ "action": "continue", "output": "VERIFY src/services/auth.ts\n✓ clean — tsc, eslint (0 errors)" }
```

**stdout (skipped):**
```json
{ "action": "continue", "output": "" }
```

**stderr:** internal errors and debug logs only.

## Compact format spec

```
VERIFY <relative_path_from_project_root>
✗ <TOOL>  L<line>  <message truncated to 80 chars>
── <N> errors | <tool>: <n> [| <tool>: <n>]
```

One line per error. Max message length 80 chars — truncate with `…`. Tool label comes from the checker's `name` field in config (always uppercase). Never include column numbers — line only.

## Session cache

Tool availability detection result cached at `~/.cache/verify-loop/session_<CLAUDE_SESSION_ID>.json` as JSON. Cache key uses `CLAUDE_SESSION_ID` env var — if unset, falls back to a hash of the project root path. Invalidated if any root marker file (`tsconfig.json`, `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`, `*.sln`) mtime changes.

## What NOT to build

- No auto-fix on errors (only `fix_on_clean` for ESLint, and only if explicitly configured)
- No test runner integration in Phase 1-2 (tests are slow, this must be fast)
- No MCP server
- No file watching — hook only, runs on demand per Write
- No IDE integration
- No HTML/markdown reports
