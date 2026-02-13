package gitsej

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateConvertsStandardCloneAndMovesWorktrees(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := t.TempDir()
	repoDir := filepath.Join(base, "orc")
	featureWorktree := filepath.Join(base, "orc-feature")

	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGitTest(t, ctx, "init", "-b", "master", repoDir)
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitTest(t, ctx, "-C", repoDir, "add", "README.md")
	runGitTest(t, ctx, "-C", repoDir, "commit", "-m", "init")
	runGitTest(t, ctx, "-C", repoDir, "branch", "feature")
	runGitTest(t, ctx, "-C", repoDir, "worktree", "add", featureWorktree, "feature")

	result, err := Migrate(ctx, MigrateOptions{
		Directory: repoDir,
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if got, want := result.MainBranch, "master"; got != want {
		t.Fatalf("result.MainBranch = %q, want %q", got, want)
	}

	if _, err := os.Stat(filepath.Join(repoDir, ".bare")); err != nil {
		t.Fatalf("expected .bare after migration: %v", err)
	}
	gitFile, err := os.ReadFile(filepath.Join(repoDir, ".git"))
	if err != nil {
		t.Fatalf("read .git: %v", err)
	}
	if string(gitFile) != "gitdir: ./.bare\n" {
		t.Fatalf("unexpected .git file content: %q", string(gitFile))
	}

	mainBranch, err := runGitTestOutput(ctx, "-C", filepath.Join(repoDir, "main"), "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("main worktree branch: %v", err)
	}
	if strings.TrimSpace(mainBranch) != "master" {
		t.Fatalf("main worktree HEAD branch = %q, want master", strings.TrimSpace(mainBranch))
	}

	newFeaturePath := filepath.Join(repoDir, "orc-feature")
	if _, err := os.Stat(newFeaturePath); err != nil {
		t.Fatalf("expected moved feature worktree at %s: %v", newFeaturePath, err)
	}
	if _, err := os.Stat(featureWorktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected old feature worktree path to be moved away, stat err=%v", err)
	}

	if _, err := runGitTestOutput(ctx, "--git-dir", filepath.Join(repoDir, ".bare"), "show-ref", "--verify", "refs/heads/feature"); err != nil {
		t.Fatalf("expected feature branch to exist after migration: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "README.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected root working tree files to be removed; stat err=%v", err)
	}
}

func TestMigrateRequiresConfirmationWhenMainIsDirty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := t.TempDir()
	repoDir := filepath.Join(base, "dirty")

	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGitTest(t, ctx, "init", "-b", "main", repoDir)
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitTest(t, ctx, "-C", repoDir, "add", "file.txt")
	runGitTest(t, ctx, "-C", repoDir, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := Migrate(ctx, MigrateOptions{Directory: repoDir})
	if err == nil {
		t.Fatalf("expected dirty main error")
	}
	var dirtyErr *DirtyMainWorktreeError
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("expected DirtyMainWorktreeError, got %T (%v)", err, err)
	}
}

func TestMigrateForceCleansDirtyMain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	base := t.TempDir()
	repoDir := filepath.Join(base, "force")

	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGitTest(t, ctx, "init", "-b", "main", repoDir)
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitTest(t, ctx, "-C", repoDir, "add", "file.txt")
	runGitTest(t, ctx, "-C", repoDir, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := Migrate(ctx, MigrateOptions{
		Directory:      repoDir,
		ForceMainClean: true,
	}); err != nil {
		t.Fatalf("Migrate(force): %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dirty root file to be removed; stat err=%v", err)
	}
}

func runGitTest(t *testing.T, ctx context.Context, args ...string) {
	t.Helper()
	if _, err := runGitTestOutput(ctx, args...); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}

func runGitTestOutput(ctx context.Context, args ...string) (string, error) {
	prefix := []string{
		"-c", "user.name=gitsej-test",
		"-c", "user.email=gitsej-test@example.invalid",
		"-c", "commit.gpgsign=false",
	}
	allArgs := append(prefix, args...)
	return runGitOutput(ctx, allArgs...)
}
