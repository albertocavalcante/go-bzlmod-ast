#!/usr/bin/env bash
# tools/install-shellcheck.sh — hermetic install of `shellcheck`.
#
# Mirrors install-just.sh's pattern: pinned version + SHA-256 per
# platform, idempotent, atomic. See tools/README.md for the threat
# model.
#
# Bumping the pin (do all three atomically):
#   1. Update SHELLCHECK_VERSION below.
#   2. Update each SHA in SHELLCHECK_SHA256 to match the corresponding
#      tarball on https://github.com/koalaman/shellcheck/releases.
#      Compute hashes locally:
#        shasum -a 256 shellcheck-<version>.<plat>.tar.xz
#   3. Commit and let CI verify it installs cleanly.
#
# Usage:
#   tools/install-shellcheck.sh           # installs to ${INSTALL_DIR:-./bin}
#   tools/install-shellcheck.sh ./bin
#   INSTALL_DIR=/some/path tools/install-shellcheck.sh

set -euo pipefail

SHELLCHECK_VERSION="v0.11.0"

# Per-platform tarball SHA-256, keyed "<arch>-<os>".
# Source: https://github.com/koalaman/shellcheck/releases/tag/${SHELLCHECK_VERSION}
# Note: keys are quoted so shfmt doesn't read the `-` as arithmetic.
declare -A SHELLCHECK_SHA256=(
    ["x86_64-linux"]="8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198"
    ["aarch64-linux"]="12b331c1d2db6b9eb13cfca64306b1b157a86eb69db83023e261eaa7e7c14588"
    ["x86_64-darwin"]="3c89db4edcab7cf1c27bff178882e0f6f27f7afdf54e859fa041fca10febe4c6"
    ["aarch64-darwin"]="56affdd8de5527894dca6dc3d7e0a99a873b0f004d7aabc30ae407d3f48b0a79"
)
# Tarball naming on upstream is "linux.x86_64" / "darwin.aarch64".
declare -A SHELLCHECK_PLAT=(
    ["x86_64-linux"]="linux.x86_64"
    ["aarch64-linux"]="linux.aarch64"
    ["x86_64-darwin"]="darwin.x86_64"
    ["aarch64-darwin"]="darwin.aarch64"
)

INSTALL_DIR="${1:-${INSTALL_DIR:-./bin}}"
mkdir -p "$INSTALL_DIR"
INSTALL_DIR="$(cd "$INSTALL_DIR" && pwd)"

arch_raw="$(uname -m)"
case "$arch_raw" in
    x86_64 | amd64) arch="x86_64" ;;
    aarch64 | arm64) arch="aarch64" ;;
    *)
        echo "tools/install-shellcheck: unsupported arch: ${arch_raw}" >&2
        exit 2
        ;;
esac
os_raw="$(uname -s)"
case "$os_raw" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *)
        echo "tools/install-shellcheck: unsupported OS: ${os_raw}" >&2
        exit 2
        ;;
esac

key="${arch}-${os}"
sha="${SHELLCHECK_SHA256[$key]:-}"
plat="${SHELLCHECK_PLAT[$key]:-}"
if [ -z "$sha" ] || [ -z "$plat" ]; then
    echo "tools/install-shellcheck: no pin recorded for ${key}" >&2
    exit 2
fi

target="${INSTALL_DIR}/shellcheck"
# The binary reports the version on a "version: X.Y.Z" line (no leading "v").
expected_version="${SHELLCHECK_VERSION#v}"
if [ -x "$target" ]; then
    installed="$("$target" --version 2> /dev/null | awk '/^version:/ {print $2}')" || true
    if [ "$installed" = "$expected_version" ]; then
        echo "tools/install-shellcheck: shellcheck ${SHELLCHECK_VERSION} already installed at ${target}"
        exit 0
    fi
fi

verify_sha256() {
    local expected="$1" file="$2"
    if command -v sha256sum > /dev/null 2>&1; then
        echo "${expected}  ${file}" | sha256sum -c -
    elif command -v shasum > /dev/null 2>&1; then
        echo "${expected}  ${file}" | shasum -a 256 -c -
    else
        echo "tools/install-shellcheck: neither sha256sum nor shasum available" >&2
        exit 2
    fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

tarball="shellcheck-${SHELLCHECK_VERSION}.${plat}.tar.xz"
url="https://github.com/koalaman/shellcheck/releases/download/${SHELLCHECK_VERSION}/${tarball}"

echo "tools/install-shellcheck: downloading ${url}"
curl --proto '=https' --tlsv1.2 --fail --silent --show-error --location \
    --output "${tarball}" "${url}"

echo "tools/install-shellcheck: verifying sha256"
verify_sha256 "${sha}" "${tarball}"

tar xJf "${tarball}"
# The tarball extracts to shellcheck-<version>/shellcheck.
chmod +x "shellcheck-${SHELLCHECK_VERSION}/shellcheck"
mv -f "shellcheck-${SHELLCHECK_VERSION}/shellcheck" "${target}"

echo "tools/install-shellcheck: installed shellcheck ${SHELLCHECK_VERSION} to ${target}"
