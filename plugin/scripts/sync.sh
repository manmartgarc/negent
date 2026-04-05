#!/usr/bin/env bash
# Locate and run negent with the given arguments.
# Checks PATH first, then common install locations, then auto-installs
# from GitHub Releases if not found.
set -euo pipefail

REPO="manmartgarc/negent"
INSTALL_DIR="${HOME}/.local/bin"

find_negent() {
  if command -v negent &>/dev/null; then
    echo "negent"
    return
  fi

  # go install / local build
  local go_bin="${GOPATH:-$HOME/go}/bin/negent"
  if [ -x "$go_bin" ]; then
    echo "$go_bin"
    return
  fi

  # auto-installed binary
  if [ -x "${INSTALL_DIR}/negent" ]; then
    echo "${INSTALL_DIR}/negent"
    return
  fi

  return 1
}

auto_install() {
  local platform arch
  case "$(uname -s)" in
    Darwin) platform="darwin" ;;
    Linux)  platform="linux" ;;
    *)      echo "Unsupported OS: $(uname -s)" >&2; return 1 ;;
  esac
  case "$(uname -m)" in
    x86_64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)       echo "Unsupported arch: $(uname -m)" >&2; return 1 ;;
  esac

  local response version
  response=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>&1) || {
    echo "negent: no releases found at https://github.com/${REPO}/releases" >&2
    echo "negent: install manually via 'go install github.com/manmartgarc/negent@latest' or download a release binary" >&2
    return 1
  }
  version=$(echo "$response" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' || true)
  if [ -z "$version" ]; then
    echo "negent: no releases found at https://github.com/${REPO}/releases" >&2
    echo "negent: install manually via 'go install github.com/manmartgarc/negent@latest' or download a release binary" >&2
    return 1
  fi

  local asset="negent-${version}-${platform}-${arch}.tar.gz"
  local url="https://github.com/${REPO}/releases/download/${version}/${asset}"

  echo "Installing negent ${version} (${platform}-${arch})..." >&2
  mkdir -p "${INSTALL_DIR}"
  curl -fsSL "$url" | tar -xz -C "${INSTALL_DIR}" negent
  chmod +x "${INSTALL_DIR}/negent"
  echo "Installed negent to ${INSTALL_DIR}/negent" >&2
}

NEGENT=$(find_negent) || {
  auto_install || {
    echo "negent: binary not found and auto-install failed"
    echo "negent: install manually via 'go install github.com/manmartgarc/negent@latest' or download from https://github.com/${REPO}/releases"
    exit 0  # exit 0 so stdout is added as context on SessionStart
  }
  NEGENT="${INSTALL_DIR}/negent"
}

# Run negent; on failure, echo the error to stdout and exit 0 so
# Claude Code adds it as context on SessionStart.
"$NEGENT" "$@" 2>&1 || true
