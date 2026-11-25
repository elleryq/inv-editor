#!/usr/bin/env bash
# Build inv-editor Linux packages from an already compiled binary.
# Usage: packaging/build.sh <binary> <tag> <arch> <out-dir> [packager...]
#   binary    path to a linux inv-editor binary
#   tag       release tag, e.g. v1.4.0 or v1.4.0-rc1
#   arch      GOARCH-style architecture: amd64 or arm64
#   out-dir   directory to write the packages into
#   packager  deb and/or rpm (default: both)
# Output names follow each format's convention:
#   inv-editor_<version>_<arch>.deb        (1.4.0-rc1 -> 1.4.0~rc1)
#   inv-editor-<version>-1.<rpmarch>.rpm   (amd64 -> x86_64, arm64 -> aarch64)
# Requires nfpm on PATH (see install-nfpm.sh). Run from the repository root.
set -euo pipefail

usage="usage: build.sh <binary> <tag> <arch> <out-dir> [deb|rpm...]"
binary="${1:?$usage}"
tag="${2:?$usage}"
arch="${3:?$usage}"
out_dir="${4:?$usage}"
shift 4
packagers=("$@")
[[ ${#packagers[@]} -gt 0 ]] || packagers=(deb rpm)

version="${tag#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  echo "build.sh: tag '$tag' is not vMAJOR.MINOR.PATCH[-PRERELEASE]" >&2
  exit 1
fi
case "$arch" in
  amd64 | arm64) ;;
  *) echo "build.sh: unsupported arch '$arch'" >&2; exit 1 ;;
esac
for p in "${packagers[@]}"; do
  case "$p" in
    deb | rpm) ;;
    *) echo "build.sh: unsupported packager '$p'" >&2; exit 1 ;;
  esac
done

# Both Debian and RPM sort "~" before anything, so 1.4.0~rc1 < 1.4.0. nfpm
# applies the same mapping to the package version (version_schema: semver).
pkg_version="${version/-/\~}"

stage=build/pkg
rm -rf "$stage"
mkdir -p "$stage" "$out_dir"
install -m 0755 "$binary" "$stage/inv-editor"

maintainer="Ellery Tsai <22297+elleryq@users.noreply.github.com>"
release_url="https://github.com/elleryq/inv-editor/releases/tag/${tag}"
# Changelogs are dated by the release commit so rebuilds are reproducible.
commit_date_rfc2822="$(git log -1 --format=%aD 2>/dev/null || date -R)"
commit_date_iso="$(git log -1 --format=%aI 2>/dev/null || date -Iseconds)"

# Debian: minimal native-package changelog, shipped as
# /usr/share/doc/inv-editor/changelog.gz (see nfpm.yaml).
cat > "$stage/changelog" <<EOF
inv-editor (${pkg_version}) unstable; urgency=medium

  * Release ${tag}.
    ${release_url}

 -- ${maintainer}  ${commit_date_rfc2822}
EOF
gzip -9n "$stage/changelog"

# RPM: nfpm turns a chglog file into the %changelog section. nfpm's
# "changelog" key applies to every packager (for deb it would add a
# changelog.Debian.gz) and cannot be overridden per packager, so it is only
# added to a generated copy of the config used for RPM builds.
# The entry carries the full EVR (version-release); "1" is nfpm's default
# RPM release, and rpmlint expects the changelog to match it.
cat > "$stage/changelog.yml" <<EOF
- semver: ${pkg_version}-1
  date: ${commit_date_iso}
  packager: ${maintainer}
  changes:
    - note: Release ${tag}. ${release_url}
EOF
{
  cat packaging/nfpm.yaml
  echo "changelog: ./$stage/changelog.yml"
} > "$stage/nfpm-rpm.yaml"

for p in "${packagers[@]}"; do
  config=packaging/nfpm.yaml
  [[ "$p" == rpm ]] && config="$stage/nfpm-rpm.yaml"
  PKG_ARCH="$arch" PKG_VERSION="$version" \
    nfpm package --packager "$p" --config "$config" --target "$out_dir/"
done
