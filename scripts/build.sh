#!/bin/sh
# ---------------------------------------------------------------------------
#  Build Prime, throttled so it leaves the machine usable.
#
#  Go otherwise builds with -p equal to the core count, which pins every core.
#  This caps the job count and the Go runtime's scheduler to a fraction of the
#  CPU, and caps heap growth so a large build does not page the box to disk.
#
#  Usage:
#    scripts/build.sh                 current OS/arch only (fast default)
#    scripts/build.sh --all           every release target
#    scripts/build.sh --os linux      cross-compile the linux targets
#    scripts/build.sh --all --jobs 4  override the throttle (slower, cooler)
#    scripts/build.sh --ratio 0.25    use a quarter of the cores instead of half
#
#  Options: --all --os <name> --jobs <n> --ratio <f> --out <dir> --no-test
# ---------------------------------------------------------------------------
set -eu

MODE=current
JOBS=""
RATIO=0.5
OUT=dist
RUN_TEST=1
TARGET_LIST=""

usage() {
	echo "Usage: scripts/build.sh [--all | --os windows|linux|darwin | --target os/arch ...] [--jobs n] [--ratio f] [--out dir] [--no-test] [--version x.y.z]"
	echo "  --target accepts the release arches: windows/amd64 windows/arm64 linux/amd64 linux/arm64"
	exit 2
}

while [ $# -gt 0 ]; do
	case "$1" in
	--all)     MODE=all; shift ;;
	--no-test) RUN_TEST=0; shift ;;
	--os)      MODE=os; OS="${2:-}"; shift 2 ;;
	--target)
		case "${2:-}" in
			windows/amd64|windows/arm64|linux/amd64|linux/arm64)
				MODE=target
				if [ -z "$TARGET_LIST" ]; then TARGET_LIST="$2"; else TARGET_LIST="$TARGET_LIST $2"; fi
				;;
			*) echo "Unsupported --target: ${2:-}"; exit 2 ;;
		esac
		shift 2 ;;
	--jobs)    JOBS="${2:-}"; shift 2 ;;
	--ratio)   RATIO="${2:-}"; shift 2 ;;
	--out)     OUT="${2:-}"; shift 2 ;;
	-h|--help) usage ;;
	*)         echo "Unknown option: $1"; usage ;;
	esac
done

# ---- how many parallel jobs we may use -------------------------------------
CORES=$( (getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 2) )

if [ -z "$JOBS" ]; then
	JOBS=$(awk -v c="$CORES" -v r="$RATIO" 'BEGIN { n = int(c * r); if (n < 1) n = 1; print n }')
fi

# ---- cap heap growth at half of physical RAM -------------------------------
if [ -z "${GOMEMLIMIT:-}" ]; then
	if command -v sysctl >/dev/null 2>&1 && sysctl -n hw.memsize >/dev/null 2>&1; then
		GOMEMLIMIT=$(awk 'BEGIN { printf "%.0fB", $1 * 0.5 }' <<EOF
$(sysctl -n hw.memsize)
EOF
)
	elif command -v free >/dev/null 2>&1; then
		GOMEMLIMIT=$(free -b | awk '/^Mem:/ { printf "%.0fB", $2 * 0.5 }')
	fi
fi

export GOMAXPROCS="$JOBS"
export CGO_ENABLED=0
[ -n "${GOMEMLIMIT:-}" ] && export GOMEMLIMIT

echo
echo " Prime build"
echo " ------------------------------------------------------------------"
echo "  cores       : $CORES, using $JOBS (~$RATIO of the machine)"
echo "  heap limit  : ${GOMEMLIMIT:-unset}"
echo "  mode        : $MODE"
echo "  output      : $OUT"
echo " ------------------------------------------------------------------"
echo

mkdir -p "$OUT"

# ---- pick targets ----------------------------------------------------------
case "$MODE" in
current)
	TARGETS="$(go env GOOS)/$(go env GOARCH)"
	echo " Building for this machine only (no cross-compilation)."
	echo
	;;
os)
	case "$OS" in
	windows) TARGETS="windows/amd64 windows/arm64" ;;
	linux)   TARGETS="linux/amd64 linux/arm64" ;;
	darwin)  TARGETS="darwin/amd64 darwin/arm64" ;;
	*)       echo "Unsupported --os: $OS"; exit 2 ;;
	esac
	echo " Building $OS targets."
	echo
	;;
all)
	TARGETS="windows/amd64 windows/arm64 \
linux/amd64 linux/arm64 \
darwin/amd64 darwin/arm64 \
freebsd/amd64 freebsd/arm64 \
openbsd/amd64 openbsd/arm64 \
netbsd/amd64 netbsd/arm64"
	echo " Building all release targets. This takes a while."
	echo
	;;
target)
	TARGETS="$TARGET_LIST"
	echo " Building explicit targets: $TARGETS"
	echo
	;;
esac

# ---- build -----------------------------------------------------------------
# A multi-target build must not share one output path: each target would
# overwrite the last, leaving a binary for the wrong platform under a
# plausible name. Single native builds keep the plain name.
NTARGETS=$(printf '%s\n' $TARGETS | wc -l | tr -d ' ')
FAILED=0
for t in $TARGETS; do
	GOOS=${t%/*}
	GOARCH=${t#*/}
	if [ "$NTARGETS" -gt 1 ]; then
		BIN="$OUT/prime-$GOOS-$GOARCH"
	else
		BIN="$OUT/prime"
	fi
	echo " [build] $GOOS/$GOARCH -> $BIN"
	GOOS="$GOOS" GOARCH="$GOARCH" GOARM="${GOARM:-}" \
		go build -trimpath -p "$JOBS" -o "$BIN" . || { echo " [fail ] $GOOS/$GOARCH"; FAILED=1; }
	GOARM=""
done

if [ "$FAILED" -ne 0 ]; then
	echo
	echo " Build FAILED."
	exit 1
fi

echo
echo " Build OK -> $OUT"
echo

if [ "$RUN_TEST" -eq 1 ]; then
	echo " Running tests with the same throttle..."
	echo
	go test -p "$JOBS" ./internal/... . || { echo; echo " Tests FAILED."; exit 1; }
	echo
	echo " Tests passed."
fi