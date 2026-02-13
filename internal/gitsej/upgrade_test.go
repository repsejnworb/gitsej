package gitsej

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestUpgradeCreatesMissingGitAndConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".bare"), 0o755); err != nil {
		t.Fatalf("mkdir .bare: %v", err)
	}

	result, err := Upgrade(UpgradeOptions{
		Directory:  dir,
		MainBranch: "trunk",
	})
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	if !result.CreatedGitFile {
		t.Fatalf("expected .git creation")
	}
	if !result.CreatedConfig {
		t.Fatalf("expected .gitsej creation")
	}
	if len(result.AddedKeys) != 0 {
		t.Fatalf("expected no added keys when config is newly created, got %v", result.AddedKeys)
	}

	gitData, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatalf("read .git: %v", err)
	}
	if got, want := string(gitData), "gitdir: ./.bare\n"; got != want {
		t.Fatalf(".git content = %q, want %q", got, want)
	}

	cfgData, err := os.ReadFile(filepath.Join(dir, ".gitsej"))
	if err != nil {
		t.Fatalf("read .gitsej: %v", err)
	}
	if !strings.Contains(string(cfgData), "main_branch=trunk\n") {
		t.Fatalf("expected main_branch=trunk, got:\n%s", string(cfgData))
	}
	if !strings.Contains(string(cfgData), "auto_update=0\n") {
		t.Fatalf("expected auto_update=0, got:\n%s", string(cfgData))
	}
}

func TestUpgradeAddsMissingKeysWithoutOverwritingExistingValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".bare"), 0o755); err != nil {
		t.Fatalf("mkdir .bare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}

	initial := `# existing config
main_branch=master
cooldown=42
custom_key=keepme
`
	if err := os.WriteFile(filepath.Join(dir, ".gitsej"), []byte(initial), 0o644); err != nil {
		t.Fatalf("write .gitsej: %v", err)
	}

	result, err := Upgrade(UpgradeOptions{
		Directory:  dir,
		MainBranch: "main",
	})
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	if result.CreatedGitFile {
		t.Fatalf("did not expect .git creation")
	}
	if result.CreatedConfig {
		t.Fatalf("did not expect .gitsej creation")
	}

	wantKeys := []string{"label", "main_worktree", "auto_update"}
	if !slices.Equal(result.AddedKeys, wantKeys) {
		t.Fatalf("added keys = %v, want %v", result.AddedKeys, wantKeys)
	}

	cfgData, err := os.ReadFile(filepath.Join(dir, ".gitsej"))
	if err != nil {
		t.Fatalf("read .gitsej: %v", err)
	}
	content := string(cfgData)

	if !strings.Contains(content, "main_branch=master\n") {
		t.Fatalf("expected existing main_branch preserved, got:\n%s", content)
	}
	if strings.Count(content, "main_branch=") != 1 {
		t.Fatalf("expected single main_branch entry, got:\n%s", content)
	}
	if !strings.Contains(content, "cooldown=42\n") {
		t.Fatalf("expected existing cooldown preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "custom_key=keepme\n") {
		t.Fatalf("expected custom key preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "label=\n") {
		t.Fatalf("expected missing label key added, got:\n%s", content)
	}
	if !strings.Contains(content, "main_worktree=main\n") {
		t.Fatalf("expected missing main_worktree key added, got:\n%s", content)
	}
	if !strings.Contains(content, "auto_update=0\n") {
		t.Fatalf("expected missing auto_update key added, got:\n%s", content)
	}
}

func TestUpgradeNoopWhenConfigIsUpToDate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".bare"), 0o755); err != nil {
		t.Fatalf("mkdir .bare: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644); err != nil {
		t.Fatalf("write .git: %v", err)
	}

	initial := `label=myrepo
main_worktree=main
main_branch=main
cooldown=300
auto_update=1
`
	configPath := filepath.Join(dir, ".gitsej")
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write .gitsej: %v", err)
	}

	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read .gitsej before: %v", err)
	}

	result, err := Upgrade(UpgradeOptions{
		Directory:  dir,
		MainBranch: "trunk",
	})
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if result.CreatedGitFile || result.CreatedConfig || len(result.AddedKeys) != 0 {
		t.Fatalf("expected no-op upgrade result, got %+v", result)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read .gitsej after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected config unchanged; before:\n%s\nafter:\n%s", string(before), string(after))
	}
}
