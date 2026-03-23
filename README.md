# verify-loop

Closes the static analysis feedback loop for Claude Code — runs linters automatically on every file Claude writes, injects a compact error summary into the next context turn.

## What it does

Claude Code writes a file. Without verify-loop, you manually run `tsc` or `eslint` to catch errors and paste the output back. With verify-loop, a `PostToolUse/Write` hook fires automatically, runs your configured checkers, and injects a normalized error block before Claude's next turn. One line per error, 80-char truncation, no raw compiler noise.

Works with any tool that produces output. Built-ins for TypeScript, Go, CSS/SCSS, and JSON. Custom checkers via shell command + optional regex parser.

## Install

### macOS / Linux

```sh
curl -fsSL https://raw.githubusercontent.com/agusrdz/verify-loop/main/install.sh | sh
```

Override install directory:

```sh
VERIFY_LOOP_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/agusrdz/verify-loop/main/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/agusrdz/verify-loop/main/install.ps1 | iex
```

### Homebrew

```sh
brew install agusrdz/tap/verify-loop
```

### Build from source

```sh
go install github.com/agusrdz/verify-loop@latest
```

## Quick start

```sh
verify-loop init   # installs the hook into ~/.claude/settings.json
```

That's it. Every subsequent Claude Write triggers checks automatically.

Sample output when errors are found:

```
VERIFY src/services/auth.ts
✗ TSC    L23   Type 'string' is not assignable to type 'number'
✗ TSC    L45   Cannot find name 'userId'
✗ LINT   L12   no-unused-vars: 'token' is defined but never used
── 3 errors | TSC: 2 | LINT: 1
```

Sample output when clean:

```
VERIFY src/services/auth.ts
✓ clean — TSC, LINT (0 errors)
```

## Commands

| Command | Description |
|---|---|
| `verify-loop init` | Install Claude Code hook into `~/.claude/settings.json` |
| `verify-loop init --status` | Check hook installation status |
| `verify-loop uninstall` | Remove hook, config, and cache |
| `verify-loop run <path>` | Run checks on a file manually (bypasses hook) |
| `verify-loop doctor` | Check hook, config, and binary health |
| `verify-loop enable` | Resume checks after disabling |
| `verify-loop disable` | Bypass all checks globally (hook still fires, outputs nothing) |
| `verify-loop config show` | Show resolved config for current directory |
| `verify-loop version` | Show version |
| `verify-loop help` | Show help |

## Configuration

Config is layered: built-in defaults → global (`~/.config/verify-loop/config.yml`) → project (`.verify-loop.yml`, walked up from the written file).

```yaml
# .verify-loop.yml

# Max seconds per checker before emitting a timeout warning. Default: 30.
timeout_seconds: 30

# Include warnings in output (default: errors only).
include_warnings: false

# Glob patterns relative to project root — matched files are silently skipped.
skip_paths:
  - "*.generated.ts"
  - "dist/**"
  - "node_modules/**"

# Checkers keyed by file extension.
checkers:
  .ts:
    # Built-in: uses the registered tsc checker.
    - name: TSC
      builtin: tsc
      flags: ["--noEmit"]
      scope: project    # run once for the project, filter output to the written file

    # Built-in: eslint with fix_on_clean — auto-fixes when there are no errors.
    - name: LINT
      builtin: eslint
      flags: ["--max-warnings", "0"]
      fix_on_clean: true

  .go:
    - name: VET
      builtin: govet
    - name: FMT
      builtin: gofmt

  # Custom checker: Python via pylint.
  .py:
    - name: PYLINT
      command: pylint
      flags: ["--output-format=text"]
      scope: file

  # Custom checker with an explicit regex parser.
  .rb:
    - name: RUBOCOP
      command: rubocop
      flags: ["--format", "emacs"]
      # Regex must capture named groups: file, line, message.
      parse: '(?P<file>[^:]+):(?P<line>\d+):\d+: \w: (?P<message>.+)'
      scope: file

  # Wildcard — applies to any extension not matched above.
  "*":
    - name: CUSTOM
      command: my-linter
      flags: ["--quiet"]
```

### scope

- `file` (default) — passes the written file path as the argument; output is used as-is.
- `project` — runs the tool from the project root without a file argument; output is filtered to lines referencing the written file.

### parse

Omit to use the built-in parser for the checker's output format. Set to a named-capture regex (`file`, `line`, `message` groups) for custom tools. Built-in named parsers: `generic`, `tsc`, `msbuild`, `rustc`, `stylelint`.

## Built-in checkers

| Checker | `builtin` key | Extensions | Scope |
|---|---|---|---|
| TypeScript compiler | `tsc` | `.ts` `.tsx` | project |
| ESLint | `eslint` | `.ts` `.tsx` `.js` `.jsx` | file |
| go vet | `govet` | `.go` | file |
| gofmt | `gofmt` | `.go` | file |
| Stylelint | `stylelint` | `.css` `.scss` `.less` | file |
| JSON validator | `json` | `.json` | file |
| MSBuild / dotnet | (use `command:`) | `.cs` `.csproj` | custom — use `parse: msbuild` |
| Cargo / rustc | (use `command:`) | `.rs` | custom — use `parse: rustc` or `parse: cargo` |

TSC uses `--incremental` detection. If no `.tsbuildinfo` exists on first run, it warns once and suggests running `tsc --incremental` manually to build the cache.

## Custom checkers

Any shell command works. Three practical examples:

**Python (pylint)**

```yaml
checkers:
  .py:
    - name: PYLINT
      command: pylint
      flags: ["--score=no", "--output-format=text"]
      scope: file
```

**Ruby (RuboCop)**

```yaml
checkers:
  .rb:
    - name: RUBOCOP
      command: rubocop
      flags: ["--format", "emacs", "--no-color"]
      parse: '(?P<file>[^:]+):(?P<line>\d+):\d+: \w: (?P<message>.+)'
      scope: file
```

**C# / .NET (dotnet build)**

```yaml
checkers:
  .cs:
    - name: BUILD
      command: dotnet
      flags: ["build", "--no-restore", "-v", "q"]
      parse: msbuild
      scope: project
```

## Uninstall

```sh
verify-loop uninstall
```

Removes the hook from `~/.claude/settings.json`, deletes `~/.config/verify-loop/`, and clears `~/.cache/verify-loop/`. The binary itself is not removed — delete it manually or via your package manager.

## License

MIT
