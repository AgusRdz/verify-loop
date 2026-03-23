# Changelog

All notable changes to verify-loop are documented here.

## [0.1.0] - 2026-03-23

### Features

- Initial implementation of verify-loop
- PostToolUse/Write hook protocol (stdin JSON → stdout compact output)
- Built-in checkers: TSC, ESLint, go vet, gofmt, Stylelint, JSON validation
- Parsers: generic, tsc, govet, gofmt, eslint, stylelint, msbuild, rustc/cargo
- Custom `command:` checkers with `parse:` field and regex parser support
- Project-scoped vs file-scoped checker modes
- Config merging: global (`~/.config/verify-loop/config.yml`) + project (`.verify-loop.yml`)
- Session cache for project detection (`~/.cache/verify-loop/`)
- `fix_on_clean` support for ESLint
- CLI commands: `init`, `uninstall`, `enable`, `disable`, `doctor`, `run`, `config show`, `version`, `help`
- GoReleaser + Homebrew tap distribution
- Install scripts for Unix (`install.sh`) and Windows (`install.ps1`)
