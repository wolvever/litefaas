#!/usr/bin/env bash
# release-binaries.sh — build lf/litefaasd release assets into dist/ for Judge upload.
# Does NOT create tags or upload. After approval:
#   gh release upload TAG dist/*
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
TAG="${1:-${TAG:-dev}}"
COMMIT=$(git rev-parse --short HEAD)
LDFLAGS="-s -w -X github.com/wolvever/litefaas/internal/version.Version=${TAG} -X github.com/wolvever/litefaas/internal/version.Commit=${COMMIT}"

mkdir -p dist
rm -f dist/lf_* dist/litefaasd_* dist/checksums.txt

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
)

for t in "${targets[@]}"; do
  set -- $t
  GOOS=$1 GOARCH=$2
  echo "building ${GOOS}/${GOARCH}"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags "$LDFLAGS" \
    -o "dist/lf_${GOOS}_${GOARCH}" ./cmd/lf
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -trimpath -ldflags "$LDFLAGS" \
    -o "dist/litefaasd_${GOOS}_${GOARCH}" ./cmd/litefaasd
done

(
  cd dist
  if command -v sha256sum >/dev/null; then
    sha256sum lf_* litefaasd_* > checksums.txt
  else
    shasum -a 256 lf_* litefaasd_* > checksums.txt
  fi
)

echo "artifacts in dist/ — upload with: gh release upload ${TAG} dist/*"
