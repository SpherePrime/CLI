#!/bin/sh
# Build the same local CLI release assets as the Windows release helper.
# This script packages Windows/Linux amd64 and arm64 binaries; publishing is
# intentionally left to CI or an explicit gh release command.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

OUT=dist
VERSION=
RUN_TEST=0

usage() {
    cat <<EOF
Usage: scripts/release.sh --version x.y.z [--out dir] [--test]

Builds and packages: windows/amd64, windows/arm64, linux/amd64, linux/arm64.
The output contains GoReleaser-compatible archive names and checksums.txt.
EOF
    exit 2
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --version)
            VERSION=${2:-}
            VERSION=${VERSION#v}
            [ -n "$VERSION" ] || usage
            shift 2
            ;;
        --out)
            OUT=${2:-}
            [ -n "$OUT" ] || usage
            shift 2
            ;;
        --test) RUN_TEST=1; shift ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1" >&2; usage ;;
    esac
done

[ -n "$VERSION" ] || { echo "--version is required" >&2; usage; }

BUILD_ARGS="--out $OUT --version $VERSION"
[ "$RUN_TEST" -eq 1 ] || BUILD_ARGS="$BUILD_ARGS --no-test"
rm -rf "$OUT/stage" "$OUT/packages"
mkdir -p "$OUT/packages"

# Keep the target list explicit so this stays aligned with .goreleaser.yml.
# shellcheck disable=SC2086
scripts/build.sh --target windows/amd64 --target windows/arm64 \
    --target linux/amd64 --target linux/arm64 $BUILD_ARGS

for target in windows/amd64 windows/arm64 linux/amd64 linux/arm64; do
    os=${target%/*}
    arch=${target#*/}
    archive_arch=$arch
    [ "$arch" = amd64 ] && archive_arch=x86_64
    archive_os=$os
    [ "$os" = windows ] && archive_os=Windows || archive_os=Linux
    ext=tar.gz
    [ "$os" = windows ] && ext=zip
    name="prime_${VERSION}_${archive_os}_${archive_arch}.${ext}"
    stage="$OUT/stage/$os-$arch"
    mkdir -p "$stage"
    if [ "$os" = windows ]; then
        cp "$OUT/prime-$os-$arch.exe" "$stage/prime.exe"
        cp README.md "$stage/README.md" 2>/dev/null || true
        if command -v bsdtar >/dev/null 2>&1; then
            bsdtar -a -cf "$OUT/packages/$name" -C "$stage" .
        elif command -v 7z >/dev/null 2>&1; then
            (cd "$stage" && 7z a -tzip -bd -y "$ROOT/$OUT/packages/$name" . >/dev/null)
        else
            echo "Windows archives require bsdtar or 7z" >&2
            exit 1
        fi
    else
        cp "$OUT/prime-$os-$arch" "$stage/prime"
        cp README.md "$stage/README.md" 2>/dev/null || true
        tar -czf "$OUT/packages/$name" -C "$stage" .
    fi
    [ -s "$OUT/packages/$name" ] || { echo "Failed to create $name" >&2; exit 1; }
done

(cd "$OUT/packages" && sha256sum prime_*.zip prime_*.tar.gz > checksums.txt)
printf '\nRelease assets written to %s/packages:\n' "$OUT"
find "$OUT/packages" -maxdepth 1 -type f -printf '  %f\n' | sort
