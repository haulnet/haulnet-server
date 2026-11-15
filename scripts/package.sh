#!/usr/bin/env bash
# Builds the Linux tarball and the Windows zip: haulnetd + the pinned, unmodified LuxonServer binary.
# Usage: scripts/package.sh <version>    Output: build/*.tar.gz, build/*.zip
set -euo pipefail

version="${1:?usage: scripts/package.sh <version>}"
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
# shellcheck source=../luxon.lock
. ./luxon.lock

build="$root/build"
cache="$build/cache/$LUXON_VERSION"
rm -rf "$build/linux" "$build/windows" "$build/docker" "$build"/*.tar.gz "$build"/*.zip
mkdir -p "$cache"

fetch() { # url file sha256
  if [ ! -f "$2" ]; then
    curl -fsSL --retry 3 -o "$2.part" "$1"
    mv "$2.part" "$2"
  fi
  if ! echo "$3  $2" | sha256sum -c --status -; then
    rm -f "$2"
    echo "SHA-256 mismatch for $1" >&2
    exit 1
  fi
}

base="https://github.com/niansa/LuxonServer/releases/download/$LUXON_VERSION"
fetch "$base/$LUXON_LINUX_ASSET" "$cache/luxon_server" "$LUXON_LINUX_SHA256"
fetch "$base/$LUXON_WINDOWS_ASSET" "$cache/luxon_server.exe" "$LUXON_WINDOWS_SHA256"
fetch "https://raw.githubusercontent.com/niansa/LuxonServer/$LUXON_VERSION/LICENSE" "$cache/LICENSE" "$LUXON_LICENSE_SHA256"

ldflags="-s -w -X main.version=$version -X main.luxonVersion=$LUXON_VERSION"

stage() { # dir goos ext
  mkdir -p "$1"
  GOOS="$2" GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$ldflags" -o "$1/haulnetd$3" ./cmd/haulnetd
  cp "$cache/luxon_server$3" "$1/luxon_server$3"
  cp "$cache/LICENSE" "$1/LuxonServer-LICENSE.txt"
  cp LICENSE THIRD_PARTY_NOTICES.md README.md dist/haulnet-server.env.example "$1/"
}

linux="$build/linux/haulnet-server-$version"
stage "$linux" linux ""
chmod 755 "$linux/haulnetd" "$linux/luxon_server"
cp dist/haulnet-server.service "$linux/"
tar -C "$build/linux" --owner=0 --group=0 -czf "$build/haulnet-server-$version-linux-x86_64.tar.gz" "haulnet-server-$version"

windows="$build/windows/haulnet-server-$version"
stage "$windows" windows ".exe"
cp dist/start.bat "$windows/"
zipfile="$build/haulnet-server-$version-windows-x86_64.zip"
(cd "$build/windows" && if command -v zip >/dev/null; then
  zip -qr "$zipfile" "haulnet-server-$version"
elif python3 -c "" 2>/dev/null; then
  python3 -m zipfile -c "$zipfile" "haulnet-server-$version"
else
  python -m zipfile -c "$zipfile" "haulnet-server-$version"
fi)

# Docker install files, published as release assets so operators download the files of one release.
docker="$build/docker"
mkdir -p "$docker"
cp docker-compose.yml dist/haulnet-update dist/haulnet-server.env.example "$docker/"
chmod 755 "$docker/haulnet-update"

(cd "$build" && sha256sum ./*.tar.gz ./*.zip > SHA256SUMS && (cd docker && sha256sum ./*) >> SHA256SUMS && cat SHA256SUMS)
