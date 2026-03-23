package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func disabledFlagPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "verify-loop", ".disabled")
}

func loadSettings() map[string]interface{} {
	data, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func saveSettings(m map[string]interface{}) error {
	path := claudeSettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func Install(version string) {
	exe, err := exePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	settings := loadSettings()

	hooksMap := getOrCreateMap(settings, "hooks")
	postToolUse := getOrCreateSlice(hooksMap, "PostToolUse")
	postToolUse = removeOurEntries(postToolUse)

	// Use forward slashes and quote the path for Claude Code compatibility on Windows.
	cmd := filepath.ToSlash(exe)
	cmd = `"` + cmd + `"`

	postToolUse = append(postToolUse, map[string]interface{}{
		"matcher": "Write",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": cmd,
			},
		},
	})

	hooksMap["PostToolUse"] = postToolUse
	settings["hooks"] = hooksMap

	if err := saveSettings(settings); err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: failed to write settings: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("verify-loop %s hook installed\n", version)
	fmt.Printf("  binary: %s\n", exe)
	fmt.Printf("  config: %s\n", claudeSettingsPath())
}

func Uninstall() {
	settings := loadSettings()

	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		fmt.Println("no hook found")
		return
	}
	postToolUse, ok := hooksMap["PostToolUse"].([]interface{})
	if !ok {
		fmt.Println("no hook found")
		return
	}

	before := len(postToolUse)
	postToolUse = removeOurEntries(postToolUse)

	if len(postToolUse) == before {
		fmt.Println("no hook found")
		return
	}

	// If empty after removal, set to empty array (not null) or delete the key.
	if len(postToolUse) == 0 {
		hooksMap["PostToolUse"] = []interface{}{}
	} else {
		hooksMap["PostToolUse"] = postToolUse
	}
	settings["hooks"] = hooksMap

	if err := saveSettings(settings); err != nil {
		fmt.Fprintf(os.Stderr, "verify-loop: failed to write settings: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("hook removed from ~/.claude/settings.json")
}

func IsInstalled() (bool, string) {
	cmd := GetHookCommand()
	return cmd != "", cmd
}

func GetHookCommand() string {
	settings := loadSettings()
	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		return ""
	}
	postToolUse, ok := hooksMap["PostToolUse"].([]interface{})
	if !ok {
		return ""
	}
	for _, entry := range postToolUse {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		if m["matcher"] != "Write" {
			continue
		}
		hooksList, ok := m["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range hooksList {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			if strings.Contains(cmd, "verify-loop") {
				return cmd
			}
		}
	}
	return ""
}

func IsDisabledGlobally() bool {
	_, err := os.Stat(disabledFlagPath())
	return err == nil
}

func Disable() error {
	path := disabledFlagPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func Enable() error {
	return os.Remove(disabledFlagPath())
}

// helpers

func getOrCreateMap(m map[string]interface{}, key string) map[string]interface{} {
	v, ok := m[key].(map[string]interface{})
	if !ok {
		v = map[string]interface{}{}
	}
	return v
}

func getOrCreateSlice(m map[string]interface{}, key string) []interface{} {
	v, ok := m[key].([]interface{})
	if !ok {
		v = []interface{}{}
	}
	return v
}

func removeOurEntries(entries []interface{}) []interface{} {
	var result []interface{}
	for _, entry := range entries {
		m, ok := entry.(map[string]interface{})
		if !ok {
			result = append(result, entry)
			continue
		}
		hooksList, ok := m["hooks"].([]interface{})
		if !ok {
			result = append(result, entry)
			continue
		}
		isOurs := false
		for _, h := range hooksList {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, _ := hm["command"].(string)
			if strings.Contains(cmd, "verify-loop") {
				isOurs = true
				break
			}
		}
		if !isOurs {
			result = append(result, entry)
		}
	}
	return result
}
