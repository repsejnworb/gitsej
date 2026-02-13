# gitsej

`gitsej` creates a "gitsej repo": a parent directory backed by a bare repository in `.bare`, with `.git` pointing to that bare repo, plus a `.gitsej` config marker.

## Install

Build from source on your machine:

```sh
go install github.com/repsejnworb/gitsej/cmd/gitsej@latest
```

## Usage

Create a gitsej repo directory from a remote URL:

```sh
gitsej git@github.com:owner/repo.git
```

That creates:

- `repo/.bare` (bare clone)
- `repo/.git` (`gitdir: ./.bare`)
- `repo/.gitsej` (default config)

Create and check out a `main` worktree too:

```sh
gitsej --main-worktree git@github.com:owner/repo.git
```

Override target directory:

```sh
gitsej git@github.com:owner/repo.git my-repo
```

Initialize an existing gitsej directory (must contain `.bare`):

```sh
gitsej init /path/to/repo
```

Initialize current directory:

```sh
gitsej init
```

### Flags

- `--main-worktree`: create `./main` worktree tracking `origin/<main-branch>`
- `--main-branch`: branch name used for `--main-worktree` and `.gitsej` defaults (default: `main`)

`init` command flags:

- `gitsej init --main-branch <branch>`: branch value for newly created `.gitsej` files

### Environment

- `GITSEJ_MAIN_WORKTREE`: default for `--main-worktree` (`true`/`false`)
- `GITSEJ_MAIN_BRANCH`: default for `--main-branch`

## `.gitsej` config

Default file:

```ini
# gitsej repo configuration
label=
main_worktree=main
main_branch=main
cooldown=300
```

## tmux status integration

This repo includes `scripts/tmux/gitsej-main-status.sh`.

Install it (example):

```sh
mkdir -p ~/.config/tmux/scripts
cp scripts/tmux/gitsej-main-status.sh ~/.config/tmux/scripts/
chmod +x ~/.config/tmux/scripts/gitsej-main-status.sh
```

Add to `.tmux.conf`:

```tmux
set -g status-right "#(bash ~/.config/tmux/scripts/gitsej-main-status.sh '#{session_id}')#[fg=#7dcfff] #(whoami) #[fg=#a9b1d6]| #[fg=#9ece6a]󰇅 #H #[fg=#a9b1d6]| #[fg=#c0caf5] %H:%M #[fg=#565f89]| #[fg=#7aa2f7]%Y-%m-%d"

bind u run-shell -b "bash ~/.config/tmux/scripts/gitsej-main-status.sh --force '#{session_id}' >/dev/null 2>&1" \; refresh-client -S \; display-message "status force refresh"
bind g run-shell -b "bash ~/.config/tmux/scripts/gitsej-main-status.sh --cycle '#{session_id}' >/dev/null 2>&1" \; refresh-client -S \; display-message "gitsej root: #{@gitsej_root}"
bind G run-shell -b "bash ~/.config/tmux/scripts/gitsej-main-status.sh --clear-pin '#{session_id}' >/dev/null 2>&1" \; refresh-client -S \; display-message "gitsej root auto"
```

Behavior:

- Auto-detects gitsej roots from pane paths in the current tmux session
- Pins active root per session via `@gitsej_root`
- `Prefix + u`: force refresh now
- `Prefix + g`: cycle pinned root across discovered gitsej repos
- `Prefix + G`: clear pin and return to auto-selection

Optional strict marker mode:

- `GITSEJ_REQUIRE_MARKER=1` to require a `.gitsej` file in the repo root for detection

Cache files live in:

```sh
~/.cache/gitsej-tmux
```
