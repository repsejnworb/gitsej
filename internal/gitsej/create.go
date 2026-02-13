package gitsej

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type CreateOptions struct {
	RepoURL      string
	Directory    string
	MainWorktree bool
	MainBranch   string
}

func Create(ctx context.Context, opts CreateOptions) (string, error) {
	repoURL := strings.TrimSpace(opts.RepoURL)
	if repoURL == "" {
		return "", errors.New("repo URL is required")
	}

	targetDir := strings.TrimSpace(opts.Directory)
	if targetDir == "" {
		var err error
		targetDir, err = inferDirectoryName(repoURL)
		if err != nil {
			return "", err
		}
	}

	mainBranch := strings.TrimSpace(opts.MainBranch)
	if mainBranch == "" {
		mainBranch = "main"
	}

	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("directory already exists: %s", targetDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check directory %s: %w", targetDir, err)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create directory %s: %w", targetDir, err)
	}

	removeOnError := true
	defer func() {
		if removeOnError {
			_ = os.RemoveAll(targetDir)
		}
	}()

	bareDir := filepath.Join(targetDir, ".bare")
	if err := runGit(ctx, "clone", "--bare", repoURL, bareDir); err != nil {
		return "", err
	}

	if err := os.WriteFile(filepath.Join(targetDir, ".git"), []byte(gitdirFileContent()), 0o644); err != nil {
		return "", fmt.Errorf("write .git: %w", err)
	}

	if err := writeGitsejConfig(targetDir, mainBranch); err != nil {
		return "", err
	}

	if opts.MainWorktree {
		if err := createMainWorktree(ctx, targetDir, mainBranch); err != nil {
			return "", err
		}
	}

	removeOnError = false
	return targetDir, nil
}

func inferDirectoryName(repoURL string) (string, error) {
	trimmed := strings.TrimSpace(repoURL)
	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "" {
		return "", errors.New("repo URL is required")
	}

	repoPath := trimmed
	switch {
	case strings.Contains(trimmed, "://"):
		u, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse repo URL: %w", err)
		}
		repoPath = u.Path
	case strings.Contains(trimmed, "@") && strings.Contains(trimmed, ":"):
		parts := strings.SplitN(trimmed, ":", 2)
		repoPath = parts[1]
	}

	repoPath = strings.Trim(repoPath, "/")
	repoPath = strings.TrimSuffix(repoPath, ".git")
	if repoPath == "" {
		return "", fmt.Errorf("cannot infer directory name from %q", repoURL)
	}

	dir := path.Base(repoPath)
	if dir == "" || dir == "." || dir == "/" {
		return "", fmt.Errorf("cannot infer directory name from %q", repoURL)
	}
	return dir, nil
}

func writeGitsejConfig(targetDir, mainBranch string) error {
	content := gitsejConfigContent(mainBranch)
	if err := os.WriteFile(filepath.Join(targetDir, ".gitsej"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write .gitsej: %w", err)
	}
	return nil
}

func gitsejConfigContent(mainBranch string) string {
	return fmt.Sprintf(`# gitsej repo configuration
# Optional label shown in tmux status; defaults to directory name.
label=
main_worktree=main
main_branch=%s
cooldown=300
`, mainBranch)
}

func gitdirFileContent() string {
	return "gitdir: ./.bare\n"
}

func createMainWorktree(ctx context.Context, targetDir, mainBranch string) error {
	mainWorktreePath := filepath.Join(targetDir, "main")
	originRef := "origin/" + mainBranch

	if err := runGit(ctx, "-C", targetDir, "worktree", "add", "-B", mainBranch, mainWorktreePath, originRef); err != nil {
		return fmt.Errorf("create main worktree from %s: %w", originRef, err)
	}

	_ = runGit(
		ctx,
		"-C",
		mainWorktreePath,
		"branch",
		"--set-upstream-to",
		originRef,
		mainBranch,
	)
	return nil
}

func runGit(ctx context.Context, args ...string) error {
	_, err := runGitOutput(ctx, args...)
	return err
}

func runGitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}

	msg := strings.TrimSpace(string(output))
	if msg == "" {
		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, msg)
}
