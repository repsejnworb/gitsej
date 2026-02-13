package gitsej

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type UpgradeOptions struct {
	Directory  string
	MainBranch string
}

type UpgradeResult struct {
	Directory      string
	CreatedGitFile bool
	CreatedConfig  bool
	AddedKeys      []string
}

func Upgrade(opts UpgradeOptions) (UpgradeResult, error) {
	targetDir := strings.TrimSpace(opts.Directory)
	if targetDir == "" {
		targetDir = "."
	}

	mainBranch := strings.TrimSpace(opts.MainBranch)
	if mainBranch == "" {
		mainBranch = "main"
	}

	info, err := os.Stat(targetDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return UpgradeResult{}, fmt.Errorf("directory does not exist: %s", targetDir)
		}
		return UpgradeResult{}, fmt.Errorf("check directory %s: %w", targetDir, err)
	}
	if !info.IsDir() {
		return UpgradeResult{}, fmt.Errorf("not a directory: %s", targetDir)
	}

	bareDir := filepath.Join(targetDir, ".bare")
	if bareInfo, err := os.Stat(bareDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return UpgradeResult{}, fmt.Errorf("missing .bare directory in %s", targetDir)
		}
		return UpgradeResult{}, fmt.Errorf("check .bare in %s: %w", targetDir, err)
	} else if !bareInfo.IsDir() {
		return UpgradeResult{}, fmt.Errorf(".bare is not a directory in %s", targetDir)
	}

	result := UpgradeResult{Directory: targetDir}

	gitFile := filepath.Join(targetDir, ".git")
	if _, err := os.Stat(gitFile); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return UpgradeResult{}, fmt.Errorf("check .git in %s: %w", targetDir, err)
		}
		if err := os.WriteFile(gitFile, []byte(gitdirFileContent()), 0o644); err != nil {
			return UpgradeResult{}, fmt.Errorf("write .git: %w", err)
		}
		result.CreatedGitFile = true
	}

	configFile := filepath.Join(targetDir, ".gitsej")
	if _, err := os.Stat(configFile); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return UpgradeResult{}, fmt.Errorf("check .gitsej in %s: %w", targetDir, err)
		}
		if err := os.WriteFile(configFile, []byte(gitsejConfigContent(mainBranch)), 0o644); err != nil {
			return UpgradeResult{}, fmt.Errorf("write .gitsej: %w", err)
		}
		result.CreatedConfig = true
		return result, nil
	}

	contentBytes, err := os.ReadFile(configFile)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("read .gitsej: %w", err)
	}
	content := string(contentBytes)

	keys := parseConfigKeys(content)
	additions, addedKeys := missingDefaultConfigAdditions(mainBranch, keys)
	if len(addedKeys) == 0 {
		return result, nil
	}

	updated := content
	if updated != "" && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	if strings.TrimSpace(updated) != "" {
		updated += "\n# Added by gitsej upgrade\n"
	}
	updated += strings.Join(additions, "\n") + "\n"

	if err := os.WriteFile(configFile, []byte(updated), 0o644); err != nil {
		return UpgradeResult{}, fmt.Errorf("write .gitsej: %w", err)
	}

	result.AddedKeys = addedKeys
	return result, nil
}

func parseConfigKeys(content string) map[string]struct{} {
	keys := make(map[string]struct{})
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}
	return keys
}

func missingDefaultConfigAdditions(mainBranch string, existing map[string]struct{}) ([]string, []string) {
	defaultLines := map[string][]string{
		"label":         {"label="},
		"main_worktree": {"main_worktree=main"},
		"main_branch":   {fmt.Sprintf("main_branch=%s", mainBranch)},
		"cooldown":      {"cooldown=300"},
		"auto_update": {
			"# 0 = never auto-pull, 1 = auto-pull when clean and behind.",
			"auto_update=0",
		},
	}
	order := []string{"label", "main_worktree", "main_branch", "cooldown", "auto_update"}

	lines := make([]string, 0, 8)
	keys := make([]string, 0, len(order))
	for _, key := range order {
		if _, ok := existing[key]; ok {
			continue
		}
		keys = append(keys, key)
		lines = append(lines, defaultLines[key]...)
	}
	slices.Clip(lines)
	slices.Clip(keys)
	return lines, keys
}
