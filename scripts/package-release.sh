#!/usr/bin/env bash
# Package a tagged release locally or in GitHub Actions. Requires Linux and Go.
set -euo pipefail

tag="${1:?Usage: bash scripts/package-release.sh v1.2.3 [output-directory]}"
output="${2:-dist}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  printf 'Invalid release tag: %s. Expected v1.2.3 or v1.2.3-rc.1.\n' "$tag" >&2
  exit 1
fi
version="${tag#v}"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
mkdir -p "$output"
cp README.md "$stage/README.md"
cp LICENSE "$stage/LICENSE"

archives=()
for arch in amd64 arm64; do
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build \
    -trimpath -buildvcs=false \
    -ldflags "-s -w -X main.version=$version" \
    -o "$stage/sudoku" ./cmd/sudoku
  archive="sudoku_${tag}_linux_${arch}.tar.gz"
  tar -czf "$output/$archive" -C "$stage" sudoku README.md LICENSE
  archives+=("$archive")
done

(
  cd "$output"
  sha256sum "${archives[@]}" > checksums.txt
)
