#!/usr/bin/env bash
set -u

DEFAULT_COOLDOWN="${GITSEJ_TMUX_COOLDOWN:-300}"
DEFAULT_MAIN_WORKTREE="${GITSEJ_MAIN_WORKTREE:-main}"
DEFAULT_MAIN_BRANCH="${GITSEJ_MAIN_BRANCH:-main}"
REQUIRE_MARKER="${GITSEJ_REQUIRE_MARKER:-0}"
SESSION_ID=""
FORCE=0
CYCLE=0
CLEAR_PIN=0

for arg in "$@"; do
  case "$arg" in
    --force|-f)
      FORCE=1
      ;;
    --cycle)
      CYCLE=1
      ;;
    --clear-pin)
      CLEAR_PIN=1
      ;;
    *)
      if [[ -z "$SESSION_ID" ]]; then
        SESSION_ID="$arg"
      fi
      ;;
  esac
done

command -v tmux >/dev/null 2>&1 || exit 0
command -v git >/dev/null 2>&1 || exit 0

if [[ -z "$SESSION_ID" ]]; then
  SESSION_ID="$(tmux display-message -p '#{session_id}' 2>/dev/null || true)"
fi
[[ -n "$SESSION_ID" ]] || exit 0

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

is_valid_gitsej_root() {
  local root="$1"
  [[ -n "$root" ]] || return 1
  [[ -d "$root/.bare" ]] || return 1
  [[ -d "$root/.git" || -f "$root/.git" ]] || return 1
  if [[ "$REQUIRE_MARKER" == "1" && ! -f "$root/.gitsej" ]]; then
    return 1
  fi
  return 0
}

discover_gitsej_root() {
  local path="$1"
  local common_dir root

  [[ -n "$path" && -d "$path" ]] || return 1

  common_dir="$(git -C "$path" rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
  [[ -n "$common_dir" ]] || return 1
  [[ "${common_dir##*/}" == ".bare" ]] || return 1

  root="$(dirname "$common_dir")"
  is_valid_gitsej_root "$root" || return 1

  printf '%s\n' "$root"
}

list_session_pane_paths() {
  local rows sid path
  rows="$(tmux list-panes -a -F '#{session_id}\t#{pane_current_path}' 2>/dev/null || true)"
  [[ -n "$rows" ]] || return 0

  while IFS=$'\t' read -r sid path; do
    [[ "$sid" == "$SESSION_ID" ]] || continue
    [[ -n "$path" ]] || continue
    printf '%s\n' "$path"
  done <<< "$rows"
}

candidates=""
add_candidate() {
  local value="$1"
  local existing
  [[ -n "$value" ]] || return 0
  while IFS= read -r existing; do
    [[ -n "$existing" ]] || continue
    if [[ "$existing" == "$value" ]]; then
      return 0
    fi
  done <<< "$candidates"
  candidates+="$value"$'\n'
}

has_candidate() {
  local needle="$1"
  local existing
  [[ -n "$needle" ]] || return 1
  while IFS= read -r existing; do
    [[ -n "$existing" ]] || continue
    if [[ "$existing" == "$needle" ]]; then
      return 0
    fi
  done <<< "$candidates"
  return 1
}

first_candidate() {
  local existing
  while IFS= read -r existing; do
    [[ -n "$existing" ]] || continue
    printf '%s\n' "$existing"
    return 0
  done <<< "$candidates"
  return 1
}

set_pin() {
  local value="$1"
  tmux set-option -t "$SESSION_ID" -q @gitsej_root "$value" >/dev/null 2>&1 || true
}

clear_pin() {
  tmux set-option -t "$SESSION_ID" -qu @gitsej_root >/dev/null 2>&1 || true
}

active_path="$(tmux display-message -p -t "$SESSION_ID" '#{pane_current_path}' 2>/dev/null || true)"
active_root="$(discover_gitsej_root "$active_path" 2>/dev/null || true)"
add_candidate "$active_root"

pane_paths="$(list_session_pane_paths)"
while IFS= read -r pane_path; do
  [[ -n "$pane_path" ]] || continue
  root="$(discover_gitsej_root "$pane_path" 2>/dev/null || true)"
  add_candidate "$root"
done <<< "$pane_paths"

if (( CLEAR_PIN == 1 )); then
  clear_pin
  exit 0
fi

if (( CYCLE == 1 )); then
  pinned_root="$(tmux show-options -t "$SESSION_ID" -vq @gitsej_root 2>/dev/null || true)"
  if ! has_candidate "$pinned_root"; then
    pinned_root=""
  fi

  next_root=""
  first_root=""
  found=0
  while IFS= read -r root; do
    [[ -n "$root" ]] || continue
    if [[ -z "$first_root" ]]; then
      first_root="$root"
    fi
    if (( found == 1 )); then
      next_root="$root"
      break
    fi
    if [[ -n "$pinned_root" && "$root" == "$pinned_root" ]]; then
      found=1
    fi
  done <<< "$candidates"

  if [[ -z "$next_root" ]]; then
    next_root="$first_root"
  fi

  if [[ -n "$next_root" ]]; then
    set_pin "$next_root"
  else
    clear_pin
  fi
  exit 0
fi

selected_root=""
pinned_root="$(tmux show-options -t "$SESSION_ID" -vq @gitsej_root 2>/dev/null || true)"
if is_valid_gitsej_root "$pinned_root"; then
  selected_root="$pinned_root"
elif has_candidate "$active_root"; then
  selected_root="$active_root"
else
  selected_root="$(first_candidate || true)"
fi

[[ -n "$selected_root" ]] || exit 0
set_pin "$selected_root"

label="$(basename "$selected_root")"
main_worktree_cfg="$DEFAULT_MAIN_WORKTREE"
main_branch="$DEFAULT_MAIN_BRANCH"
COOLDOWN="$DEFAULT_COOLDOWN"

config_file="$selected_root/.gitsej"
if [[ -f "$config_file" ]]; then
  while IFS='=' read -r raw_key raw_value; do
    key="$(trim "$raw_key")"
    value="$(trim "$raw_value")"
    [[ -n "$key" ]] || continue
    [[ "$key" == \#* ]] && continue

    case "$key" in
      label)
        [[ -n "$value" ]] && label="$value"
        ;;
      main_worktree)
        [[ -n "$value" ]] && main_worktree_cfg="$value"
        ;;
      main_branch)
        [[ -n "$value" ]] && main_branch="$value"
        ;;
      cooldown)
        if [[ "$value" =~ ^[0-9]+$ ]]; then
          COOLDOWN="$value"
        fi
        ;;
    esac
  done < "$config_file"
fi

if [[ "$main_worktree_cfg" == /* ]]; then
  main_worktree="$main_worktree_cfg"
else
  main_worktree="$selected_root/$main_worktree_cfg"
fi

[[ -d "$main_worktree/.git" || -f "$main_worktree/.git" ]] || exit 0
[[ "$COOLDOWN" =~ ^[0-9]+$ ]] || COOLDOWN="$DEFAULT_COOLDOWN"

cache_base="${XDG_CACHE_HOME:-$HOME/.cache}/gitsej-tmux"
mkdir -p "$cache_base"

sid_safe="${SESSION_ID//[^a-zA-Z0-9_-]/_}"
root_hash="$(printf '%s' "$selected_root" | cksum | awk '{print $1}')"
state_file="$cache_base/main_status_${sid_safe}_${root_hash}.env"

now="$(date +%s)"
last=0
behind=0
dirty=0

if [[ -f "$state_file" ]]; then
  # shellcheck disable=SC1090
  source "$state_file"
fi

[[ "$last" =~ ^[0-9]+$ ]] || last=0
[[ "$behind" =~ ^[0-9]+$ ]] || behind=0
[[ "$dirty" =~ ^[0-1]$ ]] || dirty=0

if (( FORCE == 1 || now - last >= COOLDOWN )); then
  git -C "$main_worktree" fetch --all --prune >/dev/null 2>&1 || exit 0

  behind="$(git -C "$main_worktree" rev-list --count "${main_branch}..origin/${main_branch}" 2>/dev/null || echo 0)"
  if [[ -n "$(git -C "$main_worktree" status --porcelain 2>/dev/null)" ]]; then
    dirty=1
  else
    dirty=0
  fi

  if (( behind > 0 )) && (( dirty == 0 )); then
    git -C "$main_worktree" pull --ff-only origin "$main_branch" >/dev/null 2>&1 || true
    behind="$(git -C "$main_worktree" rev-list --count "${main_branch}..origin/${main_branch}" 2>/dev/null || echo "$behind")"
    if [[ -n "$(git -C "$main_worktree" status --porcelain 2>/dev/null)" ]]; then
      dirty=1
    else
      dirty=0
    fi
  fi

  {
    printf 'last=%s\n' "$now"
    printf 'behind=%s\n' "$behind"
    printf 'dirty=%s\n' "$dirty"
  } > "$state_file"
fi

status_base=" ${label}: ${main_branch}"
if (( dirty == 1 )) && (( behind > 0 )); then
  printf '#[fg=#f7768e]%s ! +%s#[fg=#a9b1d6] | ' "$status_base" "$behind"
  exit 0
fi

if (( dirty == 1 )); then
  printf '#[fg=#e0af68]%s !#[fg=#a9b1d6] | ' "$status_base"
  exit 0
fi

if (( behind > 0 )); then
  printf '#[fg=#e0af68]%s +%s#[fg=#a9b1d6] | ' "$status_base" "$behind"
  exit 0
fi

printf '#[fg=#9ece6a]%s ✓#[fg=#a9b1d6] | ' "$status_base"
