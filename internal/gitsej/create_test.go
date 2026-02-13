package gitsej

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferDirectoryName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		repoURL string
		want    string
		wantErr bool
	}{
		{
			name:    "https URL",
			repoURL: "https://github.com/repsejnworb/gitsej.git",
			want:    "gitsej",
		},
		{
			name:    "ssh scp URL",
			repoURL: "git@github.com:repsejnworb/gitsej.git",
			want:    "gitsej",
		},
		{
			name:    "ssh scheme URL",
			repoURL: "ssh://git@github.com/repsejnworb/gitsej.git",
			want:    "gitsej",
		},
		{
			name:    "without dot git suffix",
			repoURL: "https://github.com/repsejnworb/gitsej",
			want:    "gitsej",
		},
		{
			name:    "empty input",
			repoURL: "   ",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := inferDirectoryName(tc.repoURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.repoURL)
				}
				return
			}

			if err != nil {
				t.Fatalf("inferDirectoryName(%q): %v", tc.repoURL, err)
			}
			if got != tc.want {
				t.Fatalf("inferDirectoryName(%q) = %q, want %q", tc.repoURL, got, tc.want)
			}
		})
	}
}

func TestWriteGitsejConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := writeGitsejConfig(tmpDir, "main"); err != nil {
		t.Fatalf("writeGitsejConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".gitsej"))
	if err != nil {
		t.Fatalf("read .gitsej: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "main_worktree=main\n") {
		t.Fatalf("expected main_worktree in config, got:\n%s", content)
	}
	if !strings.Contains(content, "main_branch=main\n") {
		t.Fatalf("expected main_branch in config, got:\n%s", content)
	}
	if !strings.Contains(content, "cooldown=300\n") {
		t.Fatalf("expected cooldown in config, got:\n%s", content)
	}
	if !strings.Contains(content, "auto_update=0\n") {
		t.Fatalf("expected auto_update in config, got:\n%s", content)
	}
}
