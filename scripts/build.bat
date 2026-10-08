@echo off
setlocal EnableDelayedExpansion
rem ---------------------------------------------------------------------------
rem  Build Prime, throttled so it leaves the machine usable.
rem
rem  Go otherwise builds with -p equal to the logical core count, which pins
rem  every core and makes the box unusable. This caps the job count and the Go
rem  runtime's scheduler to a fraction of the CPU, and caps heap growth so a
rem  wide build does not page the machine to disk.
rem
rem  Usage:
rem    scripts\build.bat                 current OS only (fast default)
rem    scripts\build.bat --all           every release target
rem    scripts\build.bat --os windows    windows/amd64, windows/arm64
rem    scripts\build.bat --os linux      linux/amd64, linux/arm64
rem    scripts\build.bat --os darwin     darwin/amd64, darwin/arm64
rem    scripts\build.bat --target windows/amd64   one exact os/arch
rem    scripts\build.bat --target linux/arm64     one exact os/arch
rem    scripts\build.bat --all --jobs 4  override the throttle (slower, cooler)
rem    scripts\build.bat --ratio 0.25    use a quarter of the cores instead of half
rem
rem  Options: --all --os <name> --target <os/arch> --jobs <n> --ratio <f>
rem           --out <dir> --no-test --version <x.y.z>
rem           --target stamps the version into the binary only when given
rem ---------------------------------------------------------------------------

set "TARGETS_MODE=current"
set "TARGET_LIST="
set "JOBS="
set "RATIO=0.5"
set "OUT=dist"
set "VERSION="
set "RUN_TEST=1"
set "CORES=%NUMBER_OF_PROCESSORS%"
if "%CORES%"=="" set "CORES=2"

rem ---------------------------------------------------------------------------
rem  Argument parsing. Each option dispatches to a label so that %1 is
rem  re-expanded after the shift: inside a parenthesised block %1 is frozen at
rem  parse time, which silently shifts the following argument into the wrong
rem  variable.
rem ---------------------------------------------------------------------------
:parse
if "%~1"=="" goto parsed
if /I "%~1"=="--all"     goto opt_all
if /I "%~1"=="--no-test" goto opt_notest
if /I "%~1"=="--os"      goto opt_os
if /I "%~1"=="--target"  goto opt_target
if /I "%~1"=="--jobs"    goto opt_jobs
if /I "%~1"=="--ratio"  goto opt_ratio
if /I "%~1"=="--out"     goto opt_out
if /I "%~1"=="--version" goto opt_version
if /I "%~1"=="-h"        goto usage
if /I "%~1"=="--help"   goto usage
echo Unknown option: %~1
goto usage

:opt_all
set "TARGETS_MODE=all"
shift
goto parse

:opt_notest
set "RUN_TEST=0"
shift
goto parse

:opt_os
shift
if "%~1"=="" ( echo --os needs a name & goto usage )
set "OS=%~1"
rem A value with a slash is a single target, e.g. "--os windows/amd64".
if /I "%OS%"=="windows/amd64"  ( set "TARGETS_MODE=target" & set "TARGET_LIST=%OS%" & shift & goto parse )
if /I "%OS%"=="windows/arm64"  ( set "TARGETS_MODE=target" & set "TARGET_LIST=%OS%" & shift & goto parse )
if /I "%OS%"=="linux/amd64"    ( set "TARGETS_MODE=target" & set "TARGET_LIST=%OS%" & shift & goto parse )
if /I "%OS%"=="linux/arm64"    ( set "TARGETS_MODE=target" & set "TARGET_LIST=%OS%" & shift & goto parse )
set "TARGETS_MODE=os"
shift
goto parse

rem A single os/arch pair; repeat the flag to build several. The list is
rem space-separated, so each value is one token and no re-parsing is needed.
rem Only the four arches the release ships are accepted: building an arch
rem that is not on the release set produces a binary nothing would download.
:opt_target
set "TARGETS_MODE=target"
shift
if "%~1"=="" ( echo --target needs os/arch & goto usage )
if /I "%~1"=="windows/amd64" goto target_add
if /I "%~1"=="windows/arm64" goto target_add
if /I "%~1"=="linux/amd64"   goto target_add
if /I "%~1"=="linux/arm64"   goto target_add
echo Unsupported --target: %~1
goto usage
:target_add
if not defined TARGET_LIST (
    set "TARGET_LIST=%~1"
) else (
    set "TARGET_LIST=%TARGET_LIST% %~1"
)
shift
goto parse

:opt_jobs
shift
if "%~1"=="" ( echo --jobs needs a number & goto usage )
set "JOBS=%~1"
shift
goto parse

:opt_ratio
shift
if "%~1"=="" ( echo --ratio needs a fraction & goto usage )
set "RATIO=%~1"
shift
goto parse

:opt_out
shift
if "%~1"=="" ( echo --out needs a directory & goto usage )
set "OUT=%~1"
shift
goto parse

:opt_version
shift
if "%~1"=="" ( echo --version needs x.y.z & goto usage )
rem Accept either form, the linker wants a bare number.
set "VERSION=%~1"
set "VERSION=%VERSION:v=%"
shift
goto parse

:parsed

rem ---- throttle -------------------------------------------------------------
if not defined JOBS (
    for /f %%i in ('powershell -NoProfile -Command "[int][math]::Max(1,[math]::Floor(%CORES% * %RATIO%))"') do set "JOBS=%%i"
)

rem Cap total heap across all concurrent compiler processes at half of physical
rem RAM.
rem
rem GOMEMLIMIT is a per-process soft limit and every compiler inherits it from
rem here, so setting it to the whole budget would let N jobs hold N times that
rem between them. On a 16GB box with 14 jobs that is a 112GB ceiling, and the
rem machine swaps long before any single process notices. The budget has to be
rem divided by the job count, with a floor so a very wide build still has room
rem to compile the large packages.
if not defined GOMEMLIMIT (
    for /f %%i in ('powershell -NoProfile -Command "[int]((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1MB * 0.5)"') do set "MEM_BUDGET_MB=%%i"
    if defined MEM_BUDGET_MB (
        set /a PER_PROC_MB=MEM_BUDGET_MB / JOBS 2>nul
        if !PER_PROC_MB! LSS 512 set "PER_PROC_MB=512"
        rem Go's suffix set is B/KiB/MiB/GiB; "MB" is malformed and the runtime
        rem aborts at startup.
        set "GOMEMLIMIT=!PER_PROC_MB!MiB"
    )
)

set "GOMAXPROCS=%JOBS%"
set "CGO_ENABLED=0"

echo.
echo  Prime build
echo  ------------------------------------------------------------------
echo   cores      : %CORES% logical, using %JOBS% (~%RATIO%% of the machine)
echo   heap limit : %GOMEMLIMIT% per process, %MEM_BUDGET_MB%MB total across %JOBS% jobs
echo   mode       : %TARGETS_MODE%
echo   output     : %OUT%
echo  ------------------------------------------------------------------
echo.

if not exist "%OUT%" mkdir "%OUT%"

rem ---- targets --------------------------------------------------------------
if /I "%TARGETS_MODE%"=="current" (
    if /I "%OS%"=="Windows_NT" (
        set "TARGETS=windows/amd64"
    ) else (
        set "TARGETS=linux/amd64"
    )
    set "MULTI=0"
    echo  Native target only, nothing cross-compiled.
    echo.
)
if /I "%TARGETS_MODE%"=="os" (
    rem The same set the release ships: one binary per real device class,
    rem not every CPU Go can cross-compile for.
    if /I "%OS%"=="windows" set "TARGETS=windows/amd64 windows/arm64"
    if /I "%OS%"=="linux"   set "TARGETS=linux/amd64 linux/arm64"
    if /I "%OS%"=="darwin"  set "TARGETS=darwin/amd64 darwin/arm64"
    if not defined TARGETS ( echo Unsupported --os: %OS% & goto usage )
    set "MULTI=1"
    echo  %OS% targets.
    echo.
)
if /I "%TARGETS_MODE%"=="all" (
    set "TARGETS=windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64 netbsd/amd64 netbsd/arm64"
    set "MULTI=1"
    echo  All release targets. This takes a while.
    echo.
)
if /I "%TARGETS_MODE%"=="target" (
    set "TARGETS=%TARGET_LIST%"
    set "MULTI=1"
    if not defined TARGETS ( echo No --target given & goto usage )
    echo  Explicit targets: %TARGET_LIST%
    echo.
)

rem ---- build ----------------------------------------------------------------
rem A multi-target build must not share one output path: each target would
rem overwrite the last, leaving a binary for the wrong platform under a
rem plausible name. A single native build keeps the plain name. MULTI was set
rem by the mode blocks above; it must not be reset here.

set "FAILED="
for %%T in (%TARGETS%) do (
    rem Split "os/arch" on the slash. set+%%~T would leave the arch inside GOOS.
    for /f "tokens=1,2 delims=/" %%a in ("%%T") do (
        set "GOOS=%%a"
        set "GOARCH=%%b"
    )
    if /I "%%GOARCH%%"=="arm" (
        set "GOARM=7"
    ) else (
        set "GOARM="
    )
    rem Only Windows binaries carry the .exe suffix. Appending it to a Linux
    rem build produced prime-linux-amd64.exe, which is neither executable on
    rem Linux nor obviously wrong on Windows.
    if "!MULTI!"=="1" (
        set "EXT="
        if /I "!GOOS!"=="windows" set "EXT=.exe"
        set "BIN=%OUT%\prime-!GOOS!-!GOARCH!!EXT!"
    ) else (
        set "BIN=%OUT%\prime.exe"
    )
    rem Delayed expansion is required here: the variables were just set
    rem inside this block, and %VAR% inside a block is frozen at parse time.
    echo  [build] !GOOS!/!GOARCH! -^> !BIN!
    rem -buildvcs=false keeps the pseudo-version out of the embedded build
    rem info. Without --version the binary keeps the "devel" default, which is
    rem what a development build should say. A release build must pass it, or
    rem the binary reports a commit hash instead of its own tag.
    if defined VERSION (
        go build -trimpath -buildvcs=false -p %JOBS% -ldflags "-s -w -X github.com/SpherePrime/CLI/internal/version.Version=%VERSION%" -o "!BIN!" .
    ) else (
        go build -trimpath -buildvcs=false -p %JOBS% -o "!BIN!" .
    )
    if errorlevel 1 (
        set "FAILED=1"
        echo  [fail ] !GOOS!/!GOARCH!
    )
)

if defined FAILED (
    echo.
    echo  Build FAILED.
    exit /b 1
)

echo.
echo  Build OK -^> %OUT%
dir /b "%OUT%" 2>nul | findstr /i prime >nul && echo   (binaries listed in %OUT%)
echo.

if "%RUN_TEST%"=="1" (
    echo  Running tests with the same throttle...
    echo.
    go test -p %JOBS% ./internal/... .
    if errorlevel 1 (
        echo.
        echo  Tests FAILED.
        exit /b 1
    )
    echo.
    echo  Tests passed.
)

endlocal
exit /b 0

:usage
echo Usage: scripts\build.bat [--all ^| --os windows^|linux^|darwin] [--jobs n] [--ratio f] [--out dir] [--no-test] [--version x.y.z]
endlocal
exit /b 2