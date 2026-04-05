#!/usr/bin/env bash
# Locate and run negent with the given arguments.
# Checks PATH first, then common install locations.
set -euo pipefail

find_negent() {
  if command -v negent &>/dev/null; then
    echo "negent"
    return
  fi

  # npm global install
  local npm_bin
  npm_bin="$(npm root -g 2>/dev/null)/negent/bin/negent" || true
  if [ -x "$npm_bin" ]; then
    echo "$npm_bin"
    return
  fi

  # go install / local build
  local go_bin="${GOPATH:-$HOME/go}/bin/negent"
  if [ -x "$go_bin" ]; then
    echo "$go_bin"
    return
  fi

  return 1
}

NEGENT=$(find_negent) || {
  echo "negent not found. Install with: npm install -g negent" >&2
  exit 0  # exit 0 so hook doesn't block the session
}

exec "$NEGENT" "$@"
