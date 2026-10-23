#!/usr/bin/env bash
set -euo pipefail

log() { printf '%s\n' "$*" >&2; }

latest_tag() {
  local repo="$1"
  git -C "$repo" tag --list \
    | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' \
    | sort -V \
    | tail -n 1 || true
}

calculate() {
  local repo="$1"
  local head_tag latest version major minor patch

  head_tag="$(git -C "$repo" describe --tags --exact-match HEAD 2>/dev/null || true)"
  if [[ -n "$head_tag" ]]; then
    log "HEAD already tagged as ${head_tag}; nothing to do"
    return 0
  fi

  latest="$(latest_tag "$repo")"
  if [[ -z "$latest" ]]; then
    printf 'v1.0.0\n'
    return 0
  fi

  if ! git -C "$repo" rev-list --max-count=1 "${latest}..HEAD" | grep -q .; then
    log "no commits since ${latest}; nothing to do"
    return 0
  fi

  if git -C "$repo" log --format='%s%n%b' "${latest}..HEAD" \
    | grep -Eiq '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE:'; then
    version="major"
  elif git -C "$repo" log --format='%s' "${latest}..HEAD" \
    | grep -Eiq '^feat(\([^)]*\))?:'; then
    version="minor"
  else
    version="patch"
  fi

  IFS=. read -r major minor patch <<< "${latest#v}"
  case "$version" in
    major) printf 'v%d.0.0\n' "$((major + 1))" ;;
    minor) printf 'v%d.%d.0\n' "$major" "$((minor + 1))" ;;
    patch) printf 'v%d.%d.%d\n' "$major" "$minor" "$((patch + 1))" ;;
  esac
}

self_test() {
  local tmp repo actual
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  repo="$tmp/repo"
  mkdir -p "$repo"
  git -C "$repo" init -q
  git -C "$repo" config user.name "Test"
  git -C "$repo" config user.email "test@example.com"

  commit() {
    printf '%s\n' "$1" >> "$repo/history"
    git -C "$repo" add history
    git -C "$repo" commit -qm "$1"
  }

  expect() {
    actual="$(calculate "$repo")"
    if [[ "$actual" != "$1" ]]; then
      log "self-test failed: expected '$1', got '$actual'"
      exit 1
    fi
  }

  commit "chore: bootstrap"
  expect "v1.0.0"
  git -C "$repo" tag -a "v1.0.0" -m "v1.0.0"

  commit "fix: correct typo"
  expect "v1.0.1"
  git -C "$repo" tag -a "v1.0.1" -m "v1.0.1"

  commit "feat: add report"
  expect "v1.1.0"
  git -C "$repo" tag -a "v1.1.0" -m "v1.1.0"

  commit "feat!: replace public contract"
  expect "v2.0.0"
  git -C "$repo" tag -a "v2.0.0" -m "v2.0.0"

  commit "docs: update runbook"
  expect "v2.0.1"
  git -C "$repo" tag -a "v2.0.1" -m "v2.0.1"

  expect ""
  commit "chore: prepare"
  git -C "$repo" commit -q --allow-empty -m "chore: breaking body" -m "BREAKING CHANGE: remove legacy endpoint"
  expect "v3.0.0"

  log "self-test passed"
}

if [[ "${1:-}" == "--self-test" ]]; then
  self_test
  exit 0
fi

calculate "$(git rev-parse --show-toplevel)"
