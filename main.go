package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/agusrdz/verify-loop/check"
	"github.com/agusrdz/verify-loop/config"
	"github.com/agusrdz/verify-loop/detect"
	"github.com/agusrdz/verify-loop/format"
	"github.com/agusrdz/verify-loop/hooks"
	"github.com/agusrdz/verify-loop/parse"
	"github.com/mattn/go-isatty"
)

var version = "dev"

func init() {
	// Register built-in checkers with their parse functions.
	// Done here (not in check package) to avoid a check→parse→check import cycle.
	// RegisterAs uses the lowercase config alias so check.Get(ccfg.Builtin) works
	// when users write e.g. `builtin: eslint` in their config.
	check.RegisterAs("eslint", check.NewESLint(check.ParseFunc(parse.ESLint)))
	check.RegisterAs("govet", check.NewGoVet(check.ParseFunc(parse.GoVet)))
	check.RegisterAs("gofmt", check.NewGoFmt(check.ParseFunc(parse.GoFmt)))
	check.RegisterAs("stylelint", check.NewStylelint(check.ParseFunc(parse.Stylelint)))
	check.RegisterAs("json", check.NewJSON())
}

func main() {
	if len(os.Args) < 2 {
		// No args — hook mode if stdin is a pipe, otherwise help.
		if isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			printHelp()
			return
		}
		runHook()
		return
	}

	switch os.Args[1] {
	case "--help", "help", "-h":
		printHelp()

	case "--version", "version":
		fmt.Printf("verify-loop %s\n", version)

	case "init", "setup":
		runInit()

	case "uninstall":
		runUninstall()

	case "enable":
		if hooks.IsDisabledGlobally() {
			if err := hooks.Enable(); err != nil {
				fmt.Fprintf(os.Stderr, "verify-loop: failed to enable: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("verify-loop enabled — checks will run on every Write")
		} else {
			fmt.Println("verify-loop is already enabled")
		}

	case "disable":
		if hooks.IsDisabledGlobally() {
			fmt.Println("verify-loop is already disabled")
		} else {
			if err := hooks.Disable(); err != nil {
				fmt.Fprintf(os.Stderr, "verify-loop: failed to disable: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("verify-loop disabled — hook will pass through all Writes")
			fmt.Println("run 'verify-loop enable' to resume")
		}

	case "doctor":
		runDoctor()

	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: verify-loop run <path>")
			os.Exit(1)
		}
		fmt.Print(runFile(os.Args[2]))

	case "config":
		if len(os.Args) < 3 || os.Args[2] == "show" {
			cwd, _ := os.Getwd()
			config.Show(cwd)
		} else {
			fmt.Fprintf(os.Stderr, "unknown config subcommand %q\nusage: verify-loop config show\n", os.Args[2])
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\nrun 'verify-loop help' for usage\n", os.Args[1])
		os.Exit(1)
	}
}

// runHook reads the PostToolUse JSON from stdin and runs checks on the written file.
func runHook() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		respond(fmt.Sprintf("verify-loop: failed to read stdin: %v", err))
		return
	}

	var input struct {
		Tool       string `json:"tool"`
		ToolResult struct {
			Path string `json:"path"`
		} `json:"tool_result"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		respond(fmt.Sprintf("verify-loop: invalid input JSON: %v", err))
		return
	}

	if hooks.IsDisabledGlobally() {
		respond("")
		return
	}

	respond(runFile(input.ToolResult.Path))
}

// runFile runs all configured checks on a single file and returns the compact output string.
func runFile(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	proj, err := detect.Find(absPath)
	if err != nil {
		return fmt.Sprintf("VERIFY %s\n⚠ project detection failed: %v", path, err)
	}

	cfg, err := config.Load(proj.Root)
	if err != nil {
		return fmt.Sprintf("VERIFY %s\n⚠ config error: %v", path, err)
	}

	relPath, err := filepath.Rel(proj.Root, absPath)
	if err != nil {
		relPath = absPath
	}
	relPath = filepath.ToSlash(relPath)

	if matchesSkipPath(relPath, cfg.SkipPaths) {
		return ""
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	checkerCfgs, ok := cfg.Checkers[ext]
	if !ok {
		checkerCfgs = cfg.Checkers["*"]
	}
	if len(checkerCfgs) == 0 {
		return ""
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	var results []check.Result

	for _, ccfg := range checkerCfgs {
		var c check.Checker
		switch {
		case ccfg.Command != "":
			parseFunc := check.ParseFunc(parse.Get(ccfg.Parse))
			c = check.NewCommand(ccfg, proj.Root, parseFunc)
		case ccfg.Builtin != "":
			if ccfg.Builtin == "TSC" {
				c = check.NewTSC(proj.Root, check.ParseFunc(parse.TSC))
			} else if ccfg.Builtin == "eslint" && ccfg.FixOnClean {
				c = check.NewESLintFixOnClean(check.ParseFunc(parse.ESLint))
			} else {
				c = check.Get(ccfg.Builtin)
			}
		}
		if c == nil {
			continue
		}
		// Project-scope tools get the relative path so the parser can match
		// against tool output (which also uses relative paths).
		fileArg := absPath
		if ccfg.Scope == "project" {
			fileArg = relPath
		}
		results = append(results, c.Run(fileArg, timeout))
	}

	return format.Compact(relPath, results, cfg.TimeoutSeconds)
}

// matchesSkipPath reports whether relPath matches any skip pattern.
func matchesSkipPath(relPath string, patterns []string) bool {
	base := filepath.Base(relPath)
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		// dir/** prefix match
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(relPath, prefix+"/") || relPath == prefix {
				return true
			}
			continue
		}
		// **/name anywhere in path
		if strings.HasPrefix(pattern, "**/") {
			name := strings.TrimPrefix(pattern, "**/")
			if base == name || strings.Contains(relPath, "/"+name+"/") {
				return true
			}
			continue
		}
		// *.ext or simple glob — match against filename only
		if !strings.Contains(pattern, "/") {
			if matched, _ := filepath.Match(pattern, base); matched {
				return true
			}
			continue
		}
		// full path glob
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
	}
	return false
}

// respond writes the PostToolUse JSON response to stdout.
func respond(output string) {
	resp := map[string]string{
		"action": "continue",
		"output": output,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func runInit() {
	if len(os.Args) < 3 {
		hooks.Install(version)
		return
	}
	switch os.Args[2] {
	case "--status":
		installed, path := hooks.IsInstalled()
		if installed {
			fmt.Printf("verify-loop hook is installed (%s)\n", path)
		} else {
			fmt.Println("verify-loop hook is NOT installed")
			fmt.Println("run 'verify-loop init' to install")
		}
	case "--uninstall":
		hooks.Uninstall()
	default:
		fmt.Fprintf(os.Stderr, "unknown flag %q\nusage: verify-loop init [--status|--uninstall]\n", os.Args[2])
		os.Exit(1)
	}
}

func runUninstall() {
	hooks.Uninstall()
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "verify-loop")
	cacheDir := filepath.Join(home, ".cache", "verify-loop")
	os.RemoveAll(configDir)
	os.RemoveAll(cacheDir)
	fmt.Println("verify-loop uninstalled")
	fmt.Printf("  hook removed from ~/.claude/settings.json\n")
	fmt.Printf("  config removed:  %s\n", configDir)
	fmt.Printf("  cache removed:   %s\n", cacheDir)
	fmt.Println("\nbinary not removed — delete manually or via your package manager")
}

func runDoctor() {
	issues := 0

	// 1. Hook installed?
	installed, _ := hooks.IsInstalled()
	if !installed {
		fmt.Println("[!] hook is not installed")
		fmt.Println("    fix: verify-loop init")
		issues++
	} else {
		hookCmd := hooks.GetHookCommand()
		exe, err := os.Executable()
		if err == nil {
			exe, _ = filepath.EvalSymlinks(exe)
		}
		if err == nil && hookCmd != exe {
			fmt.Println("[!] hook points to wrong binary")
			fmt.Printf("    current:  %s\n", hookCmd)
			fmt.Printf("    expected: %s\n", exe)
			fmt.Println("    fix: verify-loop init")
			issues++
		} else {
			fmt.Println("[ok] hook is installed and path is correct")
		}
	}

	// 2. Disabled?
	if hooks.IsDisabledGlobally() {
		fmt.Println("[!] verify-loop is disabled — hook passes through all Writes")
		fmt.Println("    fix: verify-loop enable")
		issues++
	}

	// 3. Config valid?
	cfgPath := config.Path()
	if _, err := os.Stat(cfgPath); err == nil {
		if _, err := config.Load(""); err != nil {
			fmt.Printf("[!] config file has errors: %s\n", cfgPath)
			fmt.Printf("    %v\n", err)
			issues++
		} else {
			fmt.Printf("[ok] config is valid (%s)\n", cfgPath)
		}
	} else {
		fmt.Println("[ok] no global config (using defaults)")
	}

	// 4. Legacy binary location warning (Windows)
	if runtime.GOOS == "windows" {
		if exe, err := os.Executable(); err == nil {
			if exe, err = filepath.EvalSymlinks(exe); err == nil {
				home, _ := os.UserHomeDir()
				if strings.HasPrefix(exe, filepath.Join(home, "bin")) {
					fmt.Println("[!] binary is in ~/bin — consider moving to %LOCALAPPDATA%/Programs/verify-loop")
					issues++
				}
			}
		}
	}

	if issues == 0 {
		fmt.Println("\nall good!")
	} else {
		fmt.Printf("\n%d issue(s) found\n", issues)
	}
}

func printHelp() {
	const colW = 38
	section := func(name string) string { return bold(cyan(name)) + "\n" }
	row := func(cmd, desc string) string {
		return fmt.Sprintf("  %-*s%s\n", colW, cmd, dim(desc))
	}
	flag := func(f string) string { return yellow(f) }

	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s %s — PostToolUse/Write hook for static analysis feedback\n\n", bold("verify-loop"), version))

	b.WriteString(bold("Usage") + "\n")
	b.WriteString(row("verify-loop", "Hook mode — reads PostToolUse JSON from stdin"))
	b.WriteString(row("verify-loop <subcommand>", "Run a management subcommand"))
	b.WriteString("\n")

	b.WriteString(section("Setup"))
	b.WriteString(row("init", "Install Claude Code hook (~/.claude/settings.json)"))
	b.WriteString(row("init "+flag("--status"), "Check hook installation status"))
	b.WriteString(row("init "+flag("--uninstall"), "Remove the hook from settings.json"))
	b.WriteString(row("uninstall", "Remove hook, config, and cache"))
	b.WriteString("\n")

	b.WriteString(section("Maintenance"))
	b.WriteString(row("doctor", "Check hook, config, and binary health"))
	b.WriteString(row("enable / disable", "Resume or bypass verify-loop globally"))
	b.WriteString(row("config show", "Show resolved config for current directory"))
	b.WriteString("\n")

	b.WriteString(section("Debug"))
	b.WriteString(row("run <path>", "Run checks on a file manually (bypass hook)"))
	b.WriteString("\n")

	b.WriteString(section("Other"))
	b.WriteString(row("version", "Show version"))
	b.WriteString(row("help", "Show this help"))
	b.WriteString("\n")

	b.WriteString(bold("Config") + "\n")
	b.WriteString(dim(fmt.Sprintf("  global:  %s\n", config.Path())))
	b.WriteString(dim("  project: .verify-loop.yml (walk-up from written file)\n"))
	b.WriteString(dim("  Run 'verify-loop config show' to see the resolved config.\n"))
	b.WriteString("\n")

	b.WriteString(bold("Checkers") + dim(" (extension → checker list, config-driven)") + "\n")
	b.WriteString(row(".ts .tsx", "TSC (project-scoped), LINT (eslint)"))
	b.WriteString(row(".js .jsx", "LINT (eslint)"))
	b.WriteString(row(".go", "VET (go vet), FMT (gofmt)"))
	b.WriteString(row("<any>", "custom via "+yellow("checkers:")+" in config"))
	b.WriteString("\n")

	b.WriteString(bold("Examples") + "\n")
	b.WriteString(row("verify-loop init", "Install and start getting feedback"))
	b.WriteString(row("verify-loop run src/app.ts", "Manually check a file"))
	b.WriteString(row("verify-loop doctor", "Diagnose any setup issues"))
	b.WriteString(row("verify-loop disable", "Temporarily silence all checks"))

	fmt.Print(b.String())
}
