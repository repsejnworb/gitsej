package gitsej

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type MigrateOptions struct {
	Directory      string
	MainBranch     string
	ForceMainClean bool
}

type MigrateResult struct {
	Directory             string
	MainBranch            string
	CreatedConfig         bool
	MovedWorktrees        []string
	CreatedMainWorktree   string
	RemovedRootEntries    []string
	DetectedDirtyMainPath string
}

type DirtyMainWorktreeError struct {
	Path string
}

func (e *DirtyMainWorktreeError) Error() string {
	return fmt.Sprintf("main worktree has uncommitted changes and will be cleaned: %s", e.Path)
}

type worktreeInfo struct {
	Path string
	Bare bool
}

func Migrate(ctx context.Context, opts MigrateOptions) (MigrateResult, error) {
	targetDir := strings.TrimSpace(opts.Directory)
	if targetDir == "" {
		targetDir = "."
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("resolve path %s: %w", targetDir, err)
	}
	canonicalTarget := canonicalPath(absTarget)

	if info, err := os.Stat(absTarget); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MigrateResult{}, fmt.Errorf("directory does not exist: %s", absTarget)
		}
		return MigrateResult{}, fmt.Errorf("check directory %s: %w", absTarget, err)
	} else if !info.IsDir() {
		return MigrateResult{}, fmt.Errorf("not a directory: %s", absTarget)
	}

	gitPath := filepath.Join(absTarget, ".git")
	gitInfo, err := os.Stat(gitPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MigrateResult{}, fmt.Errorf("missing .git in %s", absTarget)
		}
		return MigrateResult{}, fmt.Errorf("check .git in %s: %w", absTarget, err)
	}
	if !gitInfo.IsDir() {
		return MigrateResult{}, fmt.Errorf(".git is not a directory in %s; expected standard clone", absTarget)
	}

	barePath := filepath.Join(absTarget, ".bare")
	if _, err := os.Stat(barePath); err == nil {
		return MigrateResult{}, fmt.Errorf(".bare already exists in %s; use gitsej init instead", absTarget)
	} else if !errors.Is(err, os.ErrNotExist) {
		return MigrateResult{}, fmt.Errorf("check .bare in %s: %w", absTarget, err)
	}

	worktrees, err := listWorktrees(ctx, absTarget)
	if err != nil {
		return MigrateResult{}, err
	}

	dirty, err := isMainWorktreeDirty(ctx, absTarget)
	if err != nil {
		return MigrateResult{}, err
	}
	if dirty && !opts.ForceMainClean {
		return MigrateResult{
			Directory:             absTarget,
			DetectedDirtyMainPath: absTarget,
		}, &DirtyMainWorktreeError{Path: absTarget}
	}

	mainBranch := strings.TrimSpace(opts.MainBranch)
	if mainBranch == "" {
		mainBranch, err = detectDefaultBranch(ctx, absTarget)
		if err != nil {
			return MigrateResult{}, err
		}
	}

	if err := os.Rename(gitPath, barePath); err != nil {
		return MigrateResult{}, fmt.Errorf("move .git to .bare: %w", err)
	}
	if err := os.WriteFile(gitPath, []byte(gitdirFileContent()), 0o644); err != nil {
		return MigrateResult{}, fmt.Errorf("write .git: %w", err)
	}

	if err := runGit(ctx, "--git-dir", barePath, "config", "core.bare", "true"); err != nil {
		return MigrateResult{}, err
	}
	_ = runGit(ctx, "--git-dir", barePath, "config", "--unset", "core.worktree")

	createdConfig := false
	configPath := filepath.Join(absTarget, ".gitsej")
	if _, err := os.Stat(configPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return MigrateResult{}, fmt.Errorf("check .gitsej in %s: %w", absTarget, err)
		}
		if err := os.WriteFile(configPath, []byte(gitsejConfigContent(mainBranch)), 0o644); err != nil {
			return MigrateResult{}, fmt.Errorf("write .gitsej: %w", err)
		}
		createdConfig = true
	}

	for _, wt := range worktrees {
		wtCanonical := canonicalPath(wt.Path)
		if wt.Bare || wtCanonical == canonicalTarget {
			continue
		}
		if _, err := os.Stat(wt.Path); err != nil {
			continue
		}
		_ = runGit(ctx, "--git-dir", barePath, "worktree", "repair", wt.Path)
	}

	keep := map[string]struct{}{
		".bare":   {},
		".git":    {},
		".gitsej": {},
	}
	removedEntries, err := cleanRootDirectory(absTarget, keep)
	if err != nil {
		return MigrateResult{}, err
	}

	mainWorktreePath := filepath.Join(absTarget, "main")
	if err := runGit(ctx, "--git-dir", barePath, "worktree", "add", "--force", mainWorktreePath, mainBranch); err != nil {
		return MigrateResult{}, fmt.Errorf("create main worktree from %s: %w", mainBranch, err)
	}
	_ = runGit(
		ctx,
		"-C",
		mainWorktreePath,
		"branch",
		"--set-upstream-to",
		"origin/"+mainBranch,
		mainBranch,
	)

	moved := make([]string, 0, len(worktrees))
	usedDestinations := map[string]struct{}{
		filepath.Clean(mainWorktreePath): {},
	}
	for _, wt := range worktrees {
		wtCanonical := canonicalPath(wt.Path)
		if wt.Bare || wtCanonical == canonicalTarget {
			continue
		}
		oldPath := filepath.Clean(wt.Path)
		if strings.HasPrefix(wtCanonical, canonicalTarget+string(os.PathSeparator)) {
			continue
		}
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}

		destPath, err := nextWorktreeDestination(absTarget, filepath.Base(oldPath), usedDestinations)
		if err != nil {
			return MigrateResult{}, err
		}
		if err := runGit(ctx, "--git-dir", barePath, "worktree", "move", oldPath, destPath); err != nil {
			return MigrateResult{}, fmt.Errorf("move worktree %s to %s: %w", oldPath, destPath, err)
		}
		usedDestinations[filepath.Clean(destPath)] = struct{}{}
		moved = append(moved, destPath)
	}

	slices.Sort(moved)
	slices.Sort(removedEntries)

	return MigrateResult{
		Directory:           absTarget,
		MainBranch:          mainBranch,
		CreatedConfig:       createdConfig,
		MovedWorktrees:      moved,
		CreatedMainWorktree: mainWorktreePath,
		RemovedRootEntries:  removedEntries,
	}, nil
}

func cleanRootDirectory(dir string, keep map[string]struct{}) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	removed := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := keep[name]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return nil, fmt.Errorf("remove %s: %w", filepath.Join(dir, name), err)
		}
		removed = append(removed, name)
	}
	return removed, nil
}

func nextWorktreeDestination(root, base string, used map[string]struct{}) (string, error) {
	candidateBase := strings.TrimSpace(base)
	if candidateBase == "" || candidateBase == "." || candidateBase == "/" {
		candidateBase = "worktree"
	}

	candidate := filepath.Join(root, candidateBase)
	if _, taken := used[filepath.Clean(candidate)]; !taken {
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}

	for i := 1; i <= 9999; i++ {
		candidate = filepath.Join(root, fmt.Sprintf("%s-%d", candidateBase, i))
		if _, taken := used[filepath.Clean(candidate)]; taken {
			continue
		}
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to find destination for worktree %q under %s", base, root)
}

func detectDefaultBranch(ctx context.Context, repoDir string) (string, error) {
	originHead, err := runGitOutput(ctx, "-C", repoDir, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		originHead = strings.TrimSpace(originHead)
		if strings.HasPrefix(originHead, "origin/") && len(originHead) > len("origin/") {
			return strings.TrimPrefix(originHead, "origin/"), nil
		}
	}

	current, err := runGitOutput(ctx, "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		current = strings.TrimSpace(current)
		if current != "" && current != "HEAD" {
			return current, nil
		}
	}

	if err := runGit(ctx, "-C", repoDir, "show-ref", "--verify", "--quiet", "refs/heads/main"); err == nil {
		return "main", nil
	}
	if err := runGit(ctx, "-C", repoDir, "show-ref", "--verify", "--quiet", "refs/heads/master"); err == nil {
		return "master", nil
	}

	return "main", nil
}

func isMainWorktreeDirty(ctx context.Context, repoDir string) (bool, error) {
	out, err := runGitOutput(ctx, "-C", repoDir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func listWorktrees(ctx context.Context, repoDir string) ([]worktreeInfo, error) {
	out, err := runGitOutput(ctx, "-C", repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	worktrees := make([]worktreeInfo, 0, 4)
	var current worktreeInfo
	haveCurrent := false

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if haveCurrent {
				worktrees = append(worktrees, current)
				current = worktreeInfo{}
				haveCurrent = false
			}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			if haveCurrent {
				worktrees = append(worktrees, current)
			}
			current = worktreeInfo{Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree "))}
			haveCurrent = true
			continue
		}
		if line == "bare" {
			current.Bare = true
		}
	}
	if haveCurrent {
		worktrees = append(worktrees, current)
	}
	return worktrees, nil
}

func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(abs)
}
