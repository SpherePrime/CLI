#!/bin/sh
# Prime launcher for Unix shells. It mirrors the useful Prime.bat entry points
# without assuming a particular shell beyond POSIX sh.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"

MODE=
OUT=dist
VERSION=
RATIO=
JOBS=
NO_SYNC=0
NO_TEST=1
PRIME_ARGS=
RELEASE_ARGS=

usage() {
    cat <<'EOF'
Usage: ./Prime.sh <run|build|release|desktop|setup|help> [options] [-- prime-args]

  run       run dist/prime without compiling
  build     build the current Linux binary, then run it
  release   build and package Linux/Windows amd64 and arm64 archives
  desktop   start Prime Desktop in development mode
  setup     build the Desktop application (Linux: AppImage, deb, rpm, pacman, tar.gz)

Options:
  --no-sync             accepted for parity with Prime.bat; no sync is performed
  --ratio <fraction>    build CPU fraction
  --jobs <n>            exact build job count
  --out <dir>           output directory (default: dist)
  --version <x.y.z>     release version to stamp and package
  --test                run Go tests before/with a build
  --no-test             skip tests (default)
  --                    pass the remaining arguments to prime
EOF
}

fail() { echo "Prime: $*" >&2; exit 1; }

menu() {
    printf '\n Prime\n'
    printf ' ------------------------------------------------------------------\n'
    printf '  1  run       launch the build already in dist (no compile)\n'
    printf '  2  build     build for this machine, then launch\n'
    printf '  3  release   build and package the release archives\n'
    printf '  4  desktop   run Prime Desktop from sources\n'
    printf '  5  setup     build Prime Desktop locally\n'
    printf '  h  help      print this text\n'
    printf '  q  quit\n'
    printf ' ------------------------------------------------------------------\n'
    printf ' Pick: '
    IFS= read -r choice || exit 0
    case "$choice" in
        1|run) MODE=run ;;
        2|build) MODE=build ;;
        3|release) MODE=release ;;
        4|desktop) MODE=desktop ;;
        5|setup) MODE=setup ;;
        h|H|help|-h|--help) usage; exit 0 ;;
        q|Q|quit) exit 0 ;;
        *) fail "unknown menu choice: $choice" ;;
    esac
}

if [ "$#" -eq 0 ]; then
    menu
fi

while [ "$#" -gt 0 ]; do
    case "$1" in
        1|run) [ -z "$MODE" ] || fail "only one mode may be selected"; MODE=run; shift ;;
        2|build) [ -z "$MODE" ] || fail "only one mode may be selected"; MODE=build; shift ;;
        3|release) [ -z "$MODE" ] || fail "only one mode may be selected"; MODE=release; shift ;;
        4|desktop) [ -z "$MODE" ] || fail "only one mode may be selected"; MODE=desktop; shift ;;
        5|setup) [ -z "$MODE" ] || fail "only one mode may be selected"; MODE=setup; shift ;;
        help|-h|--help) usage; exit 0 ;;
        --no-sync) NO_SYNC=1; shift ;;
        --test) NO_TEST=0; shift ;;
        --no-test) NO_TEST=1; shift ;;
        --ratio) RATIO=${2:-}; [ -n "$RATIO" ] || fail "--ratio needs a value"; shift 2 ;;
        --jobs) JOBS=${2:-}; [ -n "$JOBS" ] || fail "--jobs needs a value"; shift 2 ;;
        --out) OUT=${2:-}; [ -n "$OUT" ] || fail "--out needs a directory"; shift 2 ;;
        --version) VERSION=${2:-}; VERSION=${VERSION#v}; [ -n "$VERSION" ] || fail "--version needs x.y.z"; shift 2 ;;
        --)
            shift
            PRIME_ARGS=$*
            break
            ;;
        *) fail "unknown option: $1 (use --help)" ;;
    esac
done

[ -n "$MODE" ] || { usage; exit 2; }

build_args="--out $OUT"
[ -n "$RATIO" ] && build_args="$build_args --ratio $RATIO"
[ -n "$JOBS" ] && build_args="$build_args --jobs $JOBS"
[ "$NO_TEST" -eq 1 ] && build_args="$build_args --no-test"
[ -n "$VERSION" ] && build_args="$build_args --version $VERSION"

case "$MODE" in
    run)
        [ -x "$OUT/prime" ] || fail "$OUT/prime does not exist; run './Prime.sh build' first"
        exec "$OUT/prime" $PRIME_ARGS
        ;;
    build)
        # shellcheck disable=SC2086
        scripts/build.sh $build_args
        exec "$OUT/prime" $PRIME_ARGS
        ;;
    release)
        [ -n "$VERSION" ] || fail "release requires --version x.y.z"
        # release.sh performs the cross-build, archive creation and checksum.
        release_test_args=
        [ "$NO_TEST" -eq 0 ] && release_test_args=--test
        exec scripts/release.sh --version "$VERSION" --out "$OUT" $release_test_args
        ;;
    desktop)
        command -v npm >/dev/null 2>&1 || fail "Node.js/npm is required"
        [ -d desktop/node_modules ] || npm --prefix desktop ci
        [ -f desktop/runtime/manifest.json ] || node desktop/scripts/acquire-runtime.mjs
        exec npm --prefix desktop run dev
        ;;
    setup)
        command -v npm >/dev/null 2>&1 || fail "Node.js/npm is required"
        [ -d desktop/node_modules ] || npm --prefix desktop ci
        node desktop/scripts/acquire-runtime.mjs
        if [ "$(uname -s)" = "Linux" ]; then
            npm --prefix desktop run dist:linux
            npm --prefix desktop run dist:installer
            echo "Linux installer and packages are in desktop/release"
        else
            npm --prefix desktop run build
            echo "Desktop build completed in desktop/out"
        fi
        ;;
esac
