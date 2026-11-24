#!/usr/bin/env bash
# Build the inv-editor Debian package from an already compiled binary.
# Usage: packaging/deb/build.sh <binary> <tag> <arch> <out-dir>
#   binary   path to a linux inv-editor binary
#   tag      release tag, e.g. v1.4.0 or v1.4.0-rc1
#   arch     Debian architecture: amd64 or arm64
#   out-dir  directory to write inv-editor_<version>_<arch>.deb into
# Requires nfpm on PATH (see install-nfpm.sh). Run from the repository root.
set -euo pipefail

binary="${1:?usage: build.sh <binary> <tag> <arch> <out-dir>}"
tag="${2:?missing tag}"
arch="${3:?missing arch}"
out_dir="${4:?missing out-dir}"

version="${tag#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  echo "build.sh: tag '$tag' is not vMAJOR.MINOR.PATCH[-PRERELEASE]" >&2
  exit 1
fi
case "$arch" in
  amd64 | arm64) ;;
  *) echo "build.sh: unsupported arch '$arch'" >&2; exit 1 ;;
esac

# Debian sorts "~" before anything, so 1.4.0~rc1 < 1.4.0. nfpm applies the
# same mapping to the package version (version_schema: semver).
deb_version="${version/-/\~}"

stage=build/deb
rm -rf "$stage"
mkdir -p "$stage" "$out_dir"
install -m 0755 "$binary" "$stage/inv-editor"

# Minimal Debian changelog (required by policy for native packages).
# Dated by the release commit so rebuilds are reproducible.
date="$(git log -1 --format=%aD 2>/dev/null || date -R)"
cat > "$stage/changelog" <<EOF
inv-editor (${deb_version}) unstable; urgency=medium

  * Release ${tag}.
    https://github.com/elleryq/inv-editor/releases/tag/${tag}

 -- Ellery Tsai <22297+elleryq@users.noreply.github.com>  ${date}
EOF
gzip -9n "$stage/changelog"

DEB_ARCH="$arch" DEB_VERSION="$version" \
  nfpm package --packager deb --config packaging/deb/nfpm.yaml --target "$out_dir/"
