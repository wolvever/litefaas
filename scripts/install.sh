#!/bin/sh
# install.sh — fetch lf + litefaasd from GitHub Releases (POSIX sh).
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/wolvever/litefaas/main/scripts/install.sh | sh
#   LITEFAAS_VERSION=v0.1.0-alpha sh install.sh
#   INSTALL_DRY_RUN=1 sh install.sh   # print URLs only
set -eu

REPO="${LITEFAAS_REPO:-wolvever/litefaas}"
INSTALL_DIR="${LITEFAAS_INSTALL_DIR:-/usr/local/bin}"
VERSION="${LITEFAAS_VERSION:-}"
DRY_RUN="${INSTALL_DRY_RUN:-0}"

die() { echo "install.sh: $*" >&2; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$os" in
  linux|darwin) ;;
  msys*|cygwin*|mingw*|windows*)
    die "Windows is not supported by this script; use go install or download release assets manually"
    ;;
  *) die "unsupported OS: $os" ;;
esac
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported arch: $arch" ;;
esac

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "need $1"
}
need_cmd curl
need_cmd uname

api="https://api.github.com/repos/${REPO}/releases"
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "${api}/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
  [ -n "$VERSION" ] || die "could not resolve latest release tag (set LITEFAAS_VERSION=...)"
fi

base="https://github.com/${REPO}/releases/download/${VERSION}"
lf_asset="lf_${os}_${arch}"
daemon_asset="litefaasd_${os}_${arch}"
checksums="checksums.txt"

echo "install.sh: version=${VERSION} os=${os} arch=${arch}"

if [ "$DRY_RUN" = "1" ]; then
  echo "DRY_RUN ${base}/${lf_asset}"
  echo "DRY_RUN ${base}/${daemon_asset}"
  echo "DRY_RUN ${base}/${checksums}"
  exit 0
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT INT HUP

curl -fsSL -o "${tmpdir}/${lf_asset}" "${base}/${lf_asset}"
curl -fsSL -o "${tmpdir}/${daemon_asset}" "${base}/${daemon_asset}"

if curl -fsSL -o "${tmpdir}/${checksums}" "${base}/${checksums}"; then
  # checksums.txt lists all platforms; filter to this host so Darwin shasum
  # (no --ignore-missing) and Linux both verify only downloaded assets.
  grep -E " (lf|litefaasd)_${os}_${arch}$" "${tmpdir}/${checksums}" > "${tmpdir}/checksums.platform.txt" || true
  if [ ! -s "${tmpdir}/checksums.platform.txt" ]; then
    die "no checksum lines for ${lf_asset} / ${daemon_asset}"
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$tmpdir" && sha256sum -c checksums.platform.txt) || die "checksum verification failed"
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$tmpdir" && shasum -a 256 -c checksums.platform.txt) || die "checksum verification failed"
  else
    echo "install.sh: warning: no sha256sum/shasum; skipping checksum verify" >&2
  fi
else
  echo "install.sh: warning: checksums.txt not found for ${VERSION}; skipping verify" >&2
fi

chmod +x "${tmpdir}/${lf_asset}" "${tmpdir}/${daemon_asset}"

if [ ! -d "$INSTALL_DIR" ] || [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  echo "install.sh: using ${INSTALL_DIR} (add to PATH if needed)"
fi

mv "${tmpdir}/${lf_asset}" "${INSTALL_DIR}/lf"
mv "${tmpdir}/${daemon_asset}" "${INSTALL_DIR}/litefaasd"
echo "installed ${INSTALL_DIR}/lf and ${INSTALL_DIR}/litefaasd (${VERSION})"
