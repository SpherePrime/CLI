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
rem    scripts\build.bat --os windows    windows/amd64, windows/arm64, windows/386
rem    scripts\build.bat --os linux      linux/amd64, linux/arm64, linux/386, linux/arm
rem    scripts\build.bat --os darwin     darwin/amd64, darwin/arm64
rem    scripts\build.bat --all --jobs 4  override the throttle (slower, cooler)
rem    scripts\build.bat --ratio 0.25    use a quarter of the cores instead of half
rem
rem  Options: --all --os <name> --jobs <n> --ratio <f> --out <dir> --no-test
rem ---------------------------------------------------------------------------

set "TARGETS_MODE=current"
set "JOBS="
set "RATIO=0.5"
set "OUT=dist"
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
if /I "%~1"=="--jobs"    goto opt_jobs
if /I "%~1"=="--ratio"  goto opt_ratio
if /I "%~1"=="--out"     goto opt_out
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
set "TARGETS_MODE=os"
shift
if "%~1"=="" ( echo --os needs a name & goto usage )
set "OS=%~1"
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

:parsed

rem ---- throttle -------------------------------------------------------------
if not defined JOBS (
    for /f %%i in ('powershell -NoProfile -Command "[int][math]::Max(1,[math]::Floor(%CORES% * %RATIO%))"') do set "JOBS=%%i"
)

rem Cap heap at half of physical RAM, in whole MB, so a wide build cannot
rem push the machine into swap.
if not defined GOMEMLIMIT (
    for /f %%i in ('powershell -NoProfile -Command "[int]((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1MB * 0.5)"') do set "MEMLIMIT_MB=%%i"
    rem Go's suffix set is B/KiB/MiB/GiB; "MB" is malformed and the runtime
    rem aborts at startup.
    if defined MEMLIMIT_MB set "GOMEMLIMIT=!MEMLIMIT_MB!MiB"
)

set "GOMAXPROCS=%JOBS%"
set "CGO_ENABLED=0"

echo.
echo  Prime build
echo  ------------------------------------------------------------------
echo   cores      : %CORES% logical, using %JOBS% (~%RATIO%% of the machine)
echo   heap limit : %GOMEMLIMIT%
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
    if /I "%OS%"=="windows" set "TARGETS=windows/amd64 windows/arm64 windows/386"
    if /I "%OS%"=="linux"   set "TARGETS=linux/amd64 linux/arm64 linux/386 linux/arm"
    if /I "%OS%"=="darwin"  set "TARGETS=darwin/amd64 darwin/arm64"
    if not defined TARGETS ( echo Unsupported --os: %OS% & goto usage )
    set "MULTI=1"
    echo  %OS% targets.
    echo.
)
if /I "%TARGETS_MODE%"=="all" (
    set "TARGETS=windows/amd64 windows/arm64 windows/386 linux/amd64 linux/arm64 linux/386 linux/arm darwin/amd64 darwin/arm64 freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64 netbsd/amd64 netbsd/arm64"
    set "MULTI=1"
    echo  All release targets. This takes a while.
    echo.
)

rem ---- build ----------------------------------------------------------------
rem A multi-target build must not share one output path: each target would
rem overwrite the last, leaving a binary for the wrong platform under a
rem plausible name. A single native build keeps the plain name.
set "MULTI=0"

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
    if "!MULTI!"=="1" (
        set "BIN=%OUT%\prime-!GOOS!-!GOARCH!.exe"
    ) else (
        set "BIN=%OUT%\prime.exe"
    )
    rem Delayed expansion is required here: the variables were just set
    rem inside this block, and %VAR% inside a block is frozen at parse time.
    echo  [build] !GOOS!/!GOARCH! -^> !BIN!
    go build -trimpath -p %JOBS% -o "!BIN!" .
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
echo Usage: scripts\build.bat [--all ^| --os windows^|linux^|darwin] [--jobs n] [--ratio f] [--out dir] [--no-test]
endlocal
exit /b 2