#!/bin/sh
set -eu

repo="ajbeck/scut"
version="latest"
bin_dir="${HOME}/.local/bin"

usage() {
  cat <<'EOF'
Install scut from GitHub Releases.

Usage:
  install.sh [--version VERSION] [--bin-dir DIR]

Options:
  --version VERSION  Install a specific release, for example v0.1.0.
  --bin-dir DIR      Install directory. Defaults to $HOME/.local/bin.
  -h, --help         Show this help.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      version="${2:?missing value for --version}"
      shift 2
      ;;
    --bin-dir)
      bin_dir="${2:?missing value for --bin-dir}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

need curl
need grep
need awk
need sed
need tar
need shasum

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    echo "unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

if [ "$version" = "latest" ]; then
  version="$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  if [ -z "$version" ]; then
    echo "could not resolve latest scut release" >&2
    exit 1
  fi
fi

case "$version" in
  v*) ;;
  *) version="v${version}" ;;
esac

asset="scut-${version}-${os}-${arch}.tar.gz"
base_url="https://github.com/${repo}/releases/download/${version}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

curl -fsSL "${base_url}/${asset}" -o "${tmp_dir}/${asset}"
curl -fsSL "${base_url}/checksums.txt" -o "${tmp_dir}/checksums.txt"

expected="$(grep "  ${asset}\$" "${tmp_dir}/checksums.txt" | awk '{print $1}')"
if [ -z "$expected" ]; then
  echo "checksum for ${asset} not found" >&2
  exit 1
fi

actual="$(shasum -a 256 "${tmp_dir}/${asset}" | awk '{print $1}')"
if [ "$actual" != "$expected" ]; then
  echo "checksum verification failed for ${asset}" >&2
  exit 1
fi

mkdir -p "$bin_dir"
tar -xzf "${tmp_dir}/${asset}" -C "$tmp_dir" scut
install -m 0755 "${tmp_dir}/scut" "${bin_dir}/scut"

echo "installed scut ${version} to ${bin_dir}/scut"
