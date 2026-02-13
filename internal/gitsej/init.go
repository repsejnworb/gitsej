package gitsej

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InitOptions struct {
	Directory  string
	MainBranch string
}

type InitResult struct {
	Directory      string
	CreatedGitFile bool
	CreatedConfig  bool
}

func Init(opts InitOptions) (InitResult, error) {
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
			return InitResult{}, fmt.Errorf("directory does not exist: %s", targetDir)
		}
		return InitResult{}, fmt.Errorf("check directory %s: %w", targetDir, err)
	}
	if !info.IsDir() {
		return InitResult{}, fmt.Errorf("not a directory: %s", targetDir)
	}

	bareDir := filepath.Join(targetDir, ".bare")
	if bareInfo, err := os.Stat(bareDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InitResult{}, fmt.Errorf("missing .bare directory in %s", targetDir)
		}
		return InitResult{}, fmt.Errorf("check .bare in %s: %w", targetDir, err)
	} else if !bareInfo.IsDir() {
		return InitResult{}, fmt.Errorf(".bare is not a directory in %s", targetDir)
	}

	result := InitResult{Directory: targetDir}

	gitFile := filepath.Join(targetDir, ".git")
	if _, err := os.Stat(gitFile); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return InitResult{}, fmt.Errorf("check .git in %s: %w", targetDir, err)
		}
		if err := os.WriteFile(gitFile, []byte(gitdirFileContent()), 0o644); err != nil {
			return InitResult{}, fmt.Errorf("write .git: %w", err)
		}
		result.CreatedGitFile = true
	}

	configFile := filepath.Join(targetDir, ".gitsej")
	if _, err := os.Stat(configFile); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return InitResult{}, fmt.Errorf("check .gitsej in %s: %w", targetDir, err)
		}
		if err := os.WriteFile(configFile, []byte(gitsejConfigContent(mainBranch)), 0o644); err != nil {
			return InitResult{}, fmt.Errorf("write .gitsej: %w", err)
		}
		result.CreatedConfig = true
	}

	return result, nil
}
