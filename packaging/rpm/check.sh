#!/bin/sh
# Verify an inv-editor RPM inside an RPM-based container (used by CI).
# Usage: check.sh <rpm-file> <rpm-arch> <rpm-version> <reported-version> [lint]
#   rpm-arch          expected %{ARCH}, e.g. x86_64 or aarch64
#   rpm-version       expected %{VERSION}, e.g. 0.0.0~ci
#   reported-version  expected "inv-editor --version" output suffix, e.g. 0.0.0-ci
#   lint              also run rpmlint with packaging/rpm/rpmlint.toml
# Run from the repository root (mounted into the container).
set -eu

rpm_file="$1"
want_arch="$2"
want_version="$3"
want_reported="$4"
lint="${5:-}"

rpm -qip "$rpm_file"
rpm -qlvp "$rpm_file"
test "$(rpm -qp --qf '%{ARCH}' "$rpm_file")" = "$want_arch"
test "$(rpm -qp --qf '%{VERSION}' "$rpm_file")" = "$want_version"
rpm -qp --changelog "$rpm_file" | grep -q -- "- ${want_version}-1\$"

if [ "$lint" = lint ]; then
  dnf install -y -q rpmlint
  rpmlint -c packaging/rpm/rpmlint.toml "$rpm_file"
fi

dnf install -y -q "$rpm_file"
test "$(inv-editor --version)" = "inv-editor ${want_reported}"
inv-editor --help > /dev/null
rpm -q --licensefiles inv-editor | grep -qx /usr/share/licenses/inv-editor/LICENSE
dnf remove -y -q inv-editor
test ! -e /usr/bin/inv-editor
echo "check.sh: ${rpm_file} OK"
