#!/usr/bin/env bash
# tools/install-just.sh — hermetic install of `just`.
#
# Pins a specific version of `just` and verifies SHA-256 checksums
# from a table covering the platforms we run on. Idempotent: skips
# the download when the target already reports the pinned version.
#
# Why this exists: replaces the `curl https://just.systems/install.sh
# | bash` pattern, which trusts an arbitrary remote script with no
# version pin and no checksum on the resulting binary. See
# tools/README.md for the threat model.
#
# Bumping the pin (do all three atomically):
#   1. Update JUST_VERSION below.
#   2. Update each SHA in JUST_SHA256 to match the corresponding
#      tarball on https://github.com/casey/just/releases/tag/<version>.
#      Compute hashes locally from the downloaded tarballs:
#        shasum -a 256 just-<version>-<triple>.tar.gz
#      Do NOT lift hashes from third-party install scripts.
#   3. Commit and let CI verify it installs cleanly.
#
# Usage:
#   tools/install-just.sh            # installs to ${INSTALL_DIR:-./bin}
#   tools/install-just.sh ./bin      # explicit
#   INSTALL_DIR=/some/path tools/install-just.sh

set -euo pipefail

JUST_VERSION="1.51.0"

# Per-platform tarball SHA-256, keyed "<arch>-<os>".
# Source: https://github.com/casey/just/releases/tag/${JUST_VERSION}
# Computed via `shasum -a 256 just-${JUST_VERSION}-<triple>.tar.gz`.
# Note: keys are quoted so shfmt doesn't read the `-` as arithmetic
# subtraction and rewrite "x86_64-linux" -> "x86_64 - linux".
declare -A JUST_SHA256=(
    ["x86_64-linux"]="c8f085ca3e885723c341d06243fc291b5abfdc8bbe3b2c076b117de490387b59"
    ["aarch64-linux"]="ed7ec466b77709198fd4afed253dba0270203ba5eb1c006bee2b0139090284f5"
    ["x86_64-darwin"]="d583e45f1f9fcdd26069ad2fe3bb9dea414756d8d0752eb9093974cb5c0246f0"
    ["aarch64-darwin"]="61e3f1b8a545ff064b091eab4b6e14f8cc743ff15549be293b1e92f5b1467002"
)
declare -A JUST_TRIPLE=(
    ["x86_64-linux"]="x86_64-unknown-linux-musl"
    ["aarch64-linux"]="aarch64-unknown-linux-musl"
    ["x86_64-darwin"]="x86_64-apple-darwin"
    ["aarch64-darwin"]="aarch64-apple-darwin"
)

INSTALL_DIR="${1:-${INSTALL_DIR:-./bin}}"

# Resolve to an absolute path BEFORE we cd into the tmp work dir below,
# otherwise a relative INSTALL_DIR resolves against the wrong CWD.
mkdir -p "$INSTALL_DIR"
INSTALL_DIR="$(cd "$INSTALL_DIR" && pwd)"

# Detect arch.
arch_raw="$(uname -m)"
case "$arch_raw" in
    x86_64 | amd64) arch="x86_64" ;;
    aarch64 | arm64) arch="aarch64" ;;
    *)
        echo "tools/install-just: unsupported arch: ${arch_raw}" >&2
        exit 2
        ;;
esac

# Detect OS.
os_raw="$(uname -s)"
case "$os_raw" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    *)
        echo "tools/install-just: unsupported OS: ${os_raw}" >&2
        exit 2
        ;;
esac

key="${arch}-${os}"
sha="${JUST_SHA256[$key]:-}"
triple="${JUST_TRIPLE[$key]:-}"
if [ -z "$sha" ] || [ -z "$triple" ]; then
    echo "tools/install-just: no pin recorded for platform ${key}" >&2
    exit 2
fi

target="${INSTALL_DIR}/just"

# Idempotence: skip if the target already reports the pinned version.
if [ -x "$target" ]; then
    installed="$("$target" --version 2> /dev/null | awk '{print $2}')" || true
    if [ "$installed" = "$JUST_VERSION" ]; then
        echo "tools/install-just: just ${JUST_VERSION} already installed at ${target}"
        exit 0
    fi
fi

# Pick a sha256 verifier — Linux has sha256sum; macOS has shasum.
verify_sha256() {
    local expected="$1" file="$2"
    if command -v sha256sum > /dev/null 2>&1; then
        echo "${expected}  ${file}" | sha256sum -c -
    elif command -v shasum > /dev/null 2>&1; then
        echo "${expected}  ${file}" | shasum -a 256 -c -
    else
        echo "tools/install-just: neither sha256sum nor shasum available" >&2
        exit 2
    fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cd "$tmp"

tarball="just-${JUST_VERSION}-${triple}.tar.gz"
url="https://github.com/casey/just/releases/download/${JUST_VERSION}/${tarball}"

echo "tools/install-just: downloading ${url}"
curl --proto '=https' --tlsv1.2 --fail --silent --show-error --location \
    --output "${tarball}" "${url}"

echo "tools/install-just: verifying sha256"
verify_sha256 "${sha}" "${tarball}"

tar xzf "${tarball}" just
chmod +x just
# Atomic move via rename — a partial failure never leaves a broken
# binary at the target path.
mv -f just "${target}"

echo "tools/install-just: installed $("${target}" --version) to ${target}"
