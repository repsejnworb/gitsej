package gitsej

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesMissingFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".bare"), 0o755); err != nil {
		t.Fatalf("mkdir .bare: %v", err)
	}

	result, err := Init(InitOptions{
		Directory:  dir,
		MainBranch: "trunk",
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !result.CreatedGitFile {
		t.Fatalf("expected .git file to be created")
	}
	if !result.CreatedConfig {
		t.Fatalf("expected .gitsej file to be created")
	}

	gitData, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatalf("read .git: %v", err)
	}
	if string(gitData) != "gitdir: ./.bare\n" {
		t.Fatalf("unexpected .git content: %q", string(gitData))
	}

	cfgData, err := os.ReadFile(filepath.Join(dir, ".gitsej"))
	if err != nil {
		t.Fatalf("read .gitsej: %v", err)
	}
	if !strings.Contains(string(cfgData), "main_branch=trunk\n") {
		t.Fatalf("expected branch override in .gitsej, got:\n%s", string(cfgData))
	}
}

func TestInitCreatesOnlyMissingConfigWhenGitExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".bare"), 0o755); err != nil {
		t.Fatalf("mkdir .bare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}

	result, err := Init(InitOptions{Directory: dir})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	if result.CreatedGitFile {
		t.Fatalf("did not expect .git file to be recreated")
	}
	if !result.CreatedConfig {
		t.Fatalf("expected .gitsej file to be created")
	}
}

func TestInitNoopWhenAlreadyInitialized(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".bare"), 0o755); err != nil {
		t.Fatalf("mkdir .bare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitsej"), []byte("main_branch=main\n"), 0o644); err != nil {
		t.Fatalf("write .gitsej: %v", err)
	}

	result, err := Init(InitOptions{Directory: dir})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if result.CreatedGitFile || result.CreatedConfig {
		t.Fatalf("expected no file creation, got %+v", result)
	}
}

func TestInitFailsWithoutBareDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := Init(InitOptions{Directory: dir})
	if err == nil {
		t.Fatalf("expected error when .bare is missing")
	}
}
