#!/usr/bin/env bash
# Download a pinned nfpm release binary, verify its SHA-256, and install it.
# Usage: packaging/install-nfpm.sh <install-dir>
set -euo pipefail

NFPM_VERSION=2.49.0

case "$(uname -m)" in
  x86_64)
    asset="nfpm_${NFPM_VERSION}_Linux_x86_64.tar.gz"
    sha256="2afaa5ffb5e3493402e31e7c67a5a1fb42133e3f66fc315ca0051934125561c7"
    ;;
  aarch64 | arm64)
    asset="nfpm_${NFPM_VERSION}_Linux_arm64.tar.gz"
    sha256="d4eee71839e92c68cbd3b101640bad32d2856fc551acae0635be9118e48c9d9c"
    ;;
  *)
    echo "install-nfpm: unsupported machine $(uname -m)" >&2
    exit 1
    ;;
esac

install_dir="${1:?usage: install-nfpm.sh <install-dir>}"
mkdir -p "$install_dir"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/$asset" \
  "https://github.com/goreleaser/nfpm/releases/download/v${NFPM_VERSION}/${asset}"
echo "${sha256}  $tmp/$asset" | sha256sum -c --quiet -
tar -xzf "$tmp/$asset" -C "$tmp" nfpm
install -m 0755 "$tmp/nfpm" "$install_dir/nfpm"

"$install_dir/nfpm" --version | grep -m1 GitVersion
