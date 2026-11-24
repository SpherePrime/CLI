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

next_version() {
    latest=$(git tag --sort=-v:refname 2>/dev/null | head -n1 || true)
    latest=${latest#v}
    case "$latest" in
        [0-9]*.[0-9]*.[0-9]*)
            major=${latest%%.*}
            rest=${latest#*.}
            minor=${rest%%.*}
            patch=${rest#*.}
            patch=${patch%%-*}
            printf '%s.%s.%s\n' "$major" "$minor" "$((patch + 1))"
            ;;
        *)
            printf '0.0.1\n'
            ;;
    esac
}

ensure_desktop_deps() {
    command -v npm >/dev/null 2>&1 || fail "Node.js/npm is required"
    if [ ! -f desktop/node_modules/.package-lock.json ] || [ -n "$(find desktop/package-lock.json -newer desktop/node_modules/.package-lock.json 2>/dev/null)" ]; then
        npm --prefix desktop ci
    fi
}

run_build() {
    set -- --out "$OUT"
    [ -n "$RATIO" ] && set -- "$@" --ratio "$RATIO"
    [ -n "$JOBS" ] && set -- "$@" --jobs "$JOBS"
    [ "$NO_TEST" -eq 1 ] && set -- "$@" --no-test
    [ -n "$VERSION" ] && set -- "$@" --version "$VERSION"
    scripts/build.sh "$@"
}

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
        3|release)
            MODE=release
            default_ver=$(next_version)
            printf ' Version [%s]: ' "$default_ver"
            IFS= read -r input_ver || exit 0
            VERSION=${input_ver:-$default_ver}
            VERSION=${VERSION#v}
            ;;
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
        --ratio) [ "$#" -ge 2 ] || fail "--ratio needs a value"; RATIO=$2; [ -n "$RATIO" ] || fail "--ratio needs a value"; shift 2 ;;
        --jobs) [ "$#" -ge 2 ] || fail "--jobs needs a value"; JOBS=$2; [ -n "$JOBS" ] || fail "--jobs needs a value"; shift 2 ;;
        --out) [ "$#" -ge 2 ] || fail "--out needs a directory"; OUT=$2; [ -n "$OUT" ] || fail "--out needs a directory"; shift 2 ;;
        --version) [ "$#" -ge 2 ] || fail "--version needs x.y.z"; VERSION=${2#v}; [ -n "$VERSION" ] || fail "--version needs x.y.z"; shift 2 ;;
        --)
            shift
            break
            ;;
        *) fail "unknown option: $1 (use --help)" ;;
    esac
done

[ -n "$MODE" ] || { usage; exit 2; }

case "$MODE" in
    run)
        [ -x "$OUT/prime" ] || fail "$OUT/prime does not exist; run './Prime.sh build' first"
        exec "$OUT/prime" "$@"
        ;;
    build)
        run_build
        exec "$OUT/prime" "$@"
        ;;
    release)
        [ -n "$VERSION" ] || fail "release requires --version x.y.z"
        if [ "$NO_TEST" -eq 0 ]; then
            exec scripts/release.sh --version "$VERSION" --out "$OUT" --test
        else
            exec scripts/release.sh --version "$VERSION" --out "$OUT"
        fi
        ;;
    desktop)
        ensure_desktop_deps
        exec npm --prefix desktop run dev
        ;;
    setup)
        ensure_desktop_deps
        if [ "$(uname -s)" = "Linux" ]; then
            npm --prefix desktop run dist:linux
            echo "Linux packages are in desktop/src-tauri/target/release/bundle"
        else
            npm --prefix desktop run build
            echo "Desktop build completed in desktop/out"
        fi
        ;;
esac
