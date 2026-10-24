#!/bin/sh
set -eu
: "${VERSION:=dev}"
: "${COMMIT:=$(git rev-parse HEAD)}"
: "${BUILD_DATE:=$(git show -s --format=%cI HEAD)}"
: "${SOURCE_DATE_EPOCH:=$(git show -s --format=%ct HEAD)}"
case "$VERSION" in *[!a-zA-Z0-9._-]*) echo 'Unsafe version string' >&2; exit 1;; esac
mkdir -p dist
for arch in amd64 arm64; do
  name="ssync_${VERSION}_linux_${arch}"
  mkdir -p "dist/$name"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false \
    -ldflags "-s -w -buildid= -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$BUILD_DATE" \
    -o "dist/$name/ssync" ./cmd/ssync
  cp LICENSE README.md "dist/$name/"
  tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner \
    -C dist -cf - "$name" | gzip -n > "dist/$name.tar.gz"
done
(cd dist && sha256sum ./*.tar.gz > SHA256SUMS)
