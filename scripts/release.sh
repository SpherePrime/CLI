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
            [ "$#" -ge 2 ] || usage
            VERSION=${2#v}
            [ -n "$VERSION" ] || usage
            shift 2
            ;;
        --out)
            [ "$#" -ge 2 ] || usage
            OUT=$2
            [ -n "$OUT" ] || usage
            shift 2
            ;;
        --test) RUN_TEST=1; shift ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1" >&2; usage ;;
    esac
done

[ -n "$VERSION" ] || { echo "--version is required" >&2; usage; }

case "$OUT" in
    /*) ;;
    *) OUT="$ROOT/$OUT" ;;
esac

rm -rf "$OUT/stage" "$OUT/packages" "$OUT/extras"
mkdir -p "$OUT/packages"

if [ "$RUN_TEST" -eq 1 ]; then
    scripts/build.sh --target windows/amd64 --target windows/arm64 \
        --target linux/amd64 --target linux/arm64 --out "$OUT" --version "$VERSION"
else
    scripts/build.sh --target windows/amd64 --target windows/arm64 \
        --target linux/amd64 --target linux/arm64 --out "$OUT" --version "$VERSION" --no-test
fi

EXTRAS="$OUT/extras"
mkdir -p "$EXTRAS/completions" "$EXTRAS/manpages"
HOST_OS=$(go env GOOS)
HOST_ARCH=$(go env GOARCH)
HOST_BIN="$OUT/prime-$HOST_OS-$HOST_ARCH"
[ "$HOST_OS" = windows ] && HOST_BIN="$HOST_BIN.exe"

generate_extras() {
    for shell in bash zsh fish; do
        if [ -x "$HOST_BIN" ]; then
            "$HOST_BIN" completion "$shell" > "$EXTRAS/completions/prime.$shell" 2>/dev/null || return 1
        else
            go run . completion "$shell" > "$EXTRAS/completions/prime.$shell" 2>/dev/null || return 1
        fi
    done
    if [ -x "$HOST_BIN" ]; then
        "$HOST_BIN" man > "$EXTRAS/manpages/prime.1" 2>/dev/null || return 1
    else
        go run . man > "$EXTRAS/manpages/prime.1" 2>/dev/null || return 1
    fi
    gzip -f "$EXTRAS/manpages/prime.1" || return 1
}

if ! generate_extras; then
    echo "WARNING: could not generate completions and manpage, archives ship without them." >&2
    rm -rf "$EXTRAS"
fi

for target in windows/amd64 windows/arm64 linux/amd64 linux/arm64; do
    os=${target%/*}
    arch=${target#*/}
    archive_arch=$arch
    [ "$arch" = amd64 ] && archive_arch=x86_64
    archive_os=Linux
    [ "$os" = windows ] && archive_os=Windows
    ext=tar.gz
    [ "$os" = windows ] && ext=zip
    base="prime_${VERSION}_${archive_os}_${archive_arch}"
    name="${base}.${ext}"
    wrap="$OUT/stage/$base"
    mkdir -p "$wrap"
    if [ "$os" = windows ]; then
        cp "$OUT/prime-$os-$arch.exe" "$wrap/prime.exe"
    else
        cp "$OUT/prime-$os-$arch" "$wrap/prime"
    fi
    cp README.md "$wrap/README.md" 2>/dev/null || true
    if [ -f "$EXTRAS/completions/prime.bash" ]; then
        cp -R "$EXTRAS/completions" "$wrap/completions"
    fi
    if [ -f "$EXTRAS/manpages/prime.1.gz" ]; then
        cp -R "$EXTRAS/manpages" "$wrap/manpages"
    fi
    if [ "$os" = windows ]; then
        if command -v zip >/dev/null 2>&1; then
            (cd "$OUT/stage" && zip -qr "$OUT/packages/$name" "$base")
        elif command -v bsdtar >/dev/null 2>&1; then
            bsdtar -a -cf "$OUT/packages/$name" -C "$OUT/stage" "$base"
        elif command -v 7z >/dev/null 2>&1; then
            (cd "$OUT/stage" && 7z a -tzip -bd -y "$OUT/packages/$name" "$base" >/dev/null)
        else
            echo "Windows archives require zip, bsdtar or 7z" >&2
            exit 1
        fi
    else
        tar -czf "$OUT/packages/$name" -C "$OUT/stage" "$base"
    fi
    [ -s "$OUT/packages/$name" ] || { echo "Failed to create $name" >&2; exit 1; }
done

if command -v sha256sum >/dev/null 2>&1; then
    (cd "$OUT/packages" && sha256sum prime_*.zip prime_*.tar.gz > checksums.txt)
elif command -v shasum >/dev/null 2>&1; then
    (cd "$OUT/packages" && shasum -a 256 prime_*.zip prime_*.tar.gz > checksums.txt)
else
    echo "sha256sum or shasum is required to generate checksums.txt" >&2
    exit 1
fi
printf '\nRelease assets written to %s/packages:\n' "$OUT"
for f in "$OUT/packages"/*; do
    [ -f "$f" ] && printf '  %s\n' "$(basename "$f")"
done
