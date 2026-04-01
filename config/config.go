package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type CheckerConfig struct {
	Name        string   `yaml:"name"`
	Builtin     string   `yaml:"builtin,omitempty"`
	Command     string   `yaml:"command,omitempty"`
	Flags       []string `yaml:"flags,omitempty"`
	Parse       string   `yaml:"parse,omitempty"`
	Scope       string   `yaml:"scope,omitempty"` // "file" (default) or "project"
	FixOnClean  bool     `yaml:"fix_on_clean,omitempty"`
	Incremental bool     `yaml:"incremental,omitempty"` // tsc: use --incremental flag
}

type Config struct {
	Enabled                bool                       `yaml:"enabled"`
	TimeoutSeconds         int                        `yaml:"timeout_seconds"`
	IncludeWarnings        bool                       `yaml:"include_warnings"`
	Checkers               map[string][]CheckerConfig `yaml:"checkers"`
	SkipPaths              []string                   `yaml:"skip_paths"`
	TsbuildInfoGitignore   string                     `yaml:"tsbuildinfo_gitignore,omitempty"` // "local" (default) or "global"
}

func defaults() *Config {
	return &Config{
		Enabled:        true,
		TimeoutSeconds: 30,
		Checkers: map[string][]CheckerConfig{
			".ts": {
				{Name: "TSC", Builtin: "tsc", Scope: "project", Incremental: true},
				{Name: "LINT", Builtin: "eslint"},
			},
			".tsx": {
				{Name: "TSC", Builtin: "tsc", Scope: "project", Incremental: true},
				{Name: "LINT", Builtin: "eslint"},
			},
			".js": {
				{Name: "LINT", Builtin: "eslint"},
			},
			".jsx": {
				{Name: "LINT", Builtin: "eslint"},
			},
			".go": {
				{Name: "VET", Builtin: "govet"},
				{Name: "FMT", Builtin: "gofmt"},
			},
			".css": {
				{Name: "STYLELINT", Builtin: "stylelint"},
			},
			".scss": {
				{Name: "STYLELINT", Builtin: "stylelint"},
			},
			".less": {
				{Name: "STYLELINT", Builtin: "stylelint"},
			},
			".json": {
				{Name: "JSON", Builtin: "json"},
			},
		},
		SkipPaths: []string{
			"*.generated.ts",
			"dist/**",
			"node_modules/**",
			"bin/**",
			"obj/**",
		},
	}
}

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "verify-loop", "config.yml")
}

func Load(projectDir string) (*Config, error) {
	cfg := defaults()

	// Load global config
	if data, err := os.ReadFile(Path()); err == nil {
		var global Config
		if err := yaml.Unmarshal(data, &global); err != nil {
			return nil, fmt.Errorf("global config: %w", err)
		}
		merge(cfg, &global)
	}

	// Load project config (.verify-loop.yml), walk up from projectDir
	if projectDir != "" {
		dir := projectDir
		for {
			projectPath := filepath.Join(dir, ".verify-loop.yml")
			if data, err := os.ReadFile(projectPath); err == nil {
				var project Config
				if err := yaml.Unmarshal(data, &project); err != nil {
					return nil, fmt.Errorf("project config (%s): %w", projectPath, err)
				}
				merge(cfg, &project)
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return cfg, nil
}

// SaveGlobal writes a partial config to the global config file, merging with
// any existing content. Only non-zero fields in patch are written.
func SaveGlobal(patch *Config) error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	existing := &Config{}
	if data, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(data, existing)
	}
	merge(existing, patch)
	data, err := yaml.Marshal(existing)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func Show(projectDir string) {
	cfg, err := Load(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: failed to load config: %v\n", err)
		return
	}
	fmt.Printf("config: %s\n\n", Path())
	data, _ := yaml.Marshal(cfg)
	fmt.Print(string(data))
}

func merge(base, override *Config) {
	if !override.Enabled {
		base.Enabled = false
	}
	if override.TimeoutSeconds > 0 {
		base.TimeoutSeconds = override.TimeoutSeconds
	}
	if override.IncludeWarnings {
		base.IncludeWarnings = true
	}
	if base.Checkers == nil {
		base.Checkers = map[string][]CheckerConfig{}
	}
	for ext, checkers := range override.Checkers {
		base.Checkers[ext] = checkers
	}
	if len(override.SkipPaths) > 0 {
		base.SkipPaths = append(base.SkipPaths, override.SkipPaths...)
	}
	if override.TsbuildInfoGitignore != "" {
		base.TsbuildInfoGitignore = override.TsbuildInfoGitignore
	}
}
