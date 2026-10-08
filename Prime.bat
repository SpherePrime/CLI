@echo off
setlocal EnableExtensions DisableDelayedExpansion
rem ============================================================================
rem  Prime - one launcher for everything: run, build, release.
rem
rem  Replaces Prime-Run.bat and Prime-Release.bat:
rem
rem    run         build (or not) and launch the app. Arguments after "--"
rem                 go to prime itself:  Prime.bat build -- --continue
rem    release     tag the commit and publish a GitHub release. Pick fewer
rem                 platforms with --targets:  Prime.bat release --targets linux
rem    help        this text
rem
rem  Delayed expansion is off for the whole file, on purpose. The release
rem  script has it on, and a prompt passed to prime may contain "!"; with it
rem  on, cmd eats every "!" while parsing a line. Off, "%VAR%" expansions
rem  are literal and nothing is lost; the cost is that a variable read inside
rem  the block it was set in is not visible, so every branch below jumps to
rem  a label instead.
rem
rem  Everything that is missing is installed when it is needed - the sync asks
rem  for git, the build asks for Go - and the build is throttled, so it leaves
rem  the machine usable rather than pinning every core.
rem
rem  Usage:
rem    Prime.bat                       menu
rem    Prime.bat run                   launch the build already in dist
rem    Prime.bat build                 sync, compile for this machine, launch
rem    Prime.bat build --no-sync       build what is here, do not pull
rem    Prime.bat build -- --continue   everything after -- goes to prime
rem    Prime.bat release               tag HEAD, publish a GitHub release
rem    Prime.bat release --targets linux
rem    Prime.bat release --dry-run     print the plan, publish nothing
rem    Prime.bat release --version 0.9.9 --notes CHANGELOG.md
rem ============================================================================

cd /d "%~dp0"

set "MODE="
set "SYNC=1"
set "RATIO="
rem BUILD_OPTS is what the build script accepts; RELEASE_OPTS is what only
rem the release path understands. Keeping them apart stops "Prime.bat build
rem --targets linux" from feeding a flag into a script that rejects it.
set "BUILD_OPTS="
set "RELEASE_OPTS="
set "PRIME_ARGS="
set "EXITCODE=0"

rem ---- argument parsing ------------------------------------------------------
rem Each option jumps to a label so %1 is re-expanded after the shift: inside a
rem parenthesised block %1 is frozen at parse time, which would silently put
rem the following argument into the wrong variable.
:parse
if "%~1"=="" goto parsed
if /I "%~1"=="run"       ( set "MODE=run"   & shift & goto parse )
if /I "%~1"=="build"     ( set "MODE=build" & shift & goto parse )
if /I "%~1"=="release"   ( set "MODE=release" & shift & goto parse )
if /I "%~1"=="1"         ( set "MODE=run"   & shift & goto parse )
if /I "%~1"=="2"         ( set "MODE=build" & shift & goto parse )
if /I "%~1"=="3"         ( set "MODE=release" & shift & goto parse )
if /I "%~1"=="--no-sync" ( set "SYNC=0" & shift & goto parse )
rem build.bat understands these; thread them through.
if /I "%~1"=="--ratio"   goto opt_ratio
if /I "%~1"=="--jobs"    goto opt_jobs
if /I "%~1"=="--version" goto opt_version
rem Everything the release script understands that the launcher does not
rem parses: forward the flag and its value pair to release.bat as-is.
if /I "%~1"=="--targets"   goto opt_pair
if /I "%~1"=="--notes"     goto opt_pair
if /I "%~1"=="--out"       goto opt_pair
rem Valueless release flags forward as-is.
if /I "%~1"=="--dry-run"     goto opt_bare
if /I "%~1"=="--yes"         goto opt_bare
if /I "%~1"=="--test"        goto opt_bare
if /I "%~1"=="--skip-build"  goto opt_bare
if /I "%~1"=="--upload-local" goto opt_bare
if /I "%~1"=="--"          goto opt_prime
if /I "%~1"=="-h"          goto usage
if /I "%~1"=="--help"      goto usage
if /I "%~1"=="-?"          goto usage
echo Unknown option: %~1
goto usage

:opt_ratio
shift
if "%~1"=="" ( echo --ratio needs a fraction & goto usage )
set "RATIO=%~1"
set "BUILD_OPTS=%BUILD_OPTS% --ratio %~1"
shift
goto parse

:opt_jobs
shift
if "%~1"=="" ( echo --jobs needs a number & goto usage )
set "BUILD_OPTS=%BUILD_OPTS% --jobs %~1"
shift
goto parse

rem --version threads to both scripts: build stamps it into the binary,
rem release uses it as the tag.
:opt_version
shift
if "%~1"=="" ( echo --version needs x.y.z & goto usage )
set "BUILD_OPTS=%BUILD_OPTS% --version %~1"
shift
goto parse

rem A paired release-only option, value included, e.g. "--targets linux".
rem The value belongs to the release script, not to the launcher, so both
rem halves go through uninterpreted. The flag name is captured before the
rem shift: with delayed expansion off the whole line is one statement, and
rem after the shift %1 is the value, not the flag.
:opt_pair
set "PAIR_FLAG=%~1"
shift
if "%~1"=="" (
    echo %PAIR_FLAG% needs a value
    goto usage
)
set "RELEASE_OPTS=%RELEASE_OPTS% %PAIR_FLAG% %~1"
shift
goto parse

rem A bare release flag with no value, e.g. "--dry-run". Only one word is
rem consumed; everything after it keeps parsing in this launcher.
:opt_bare
set "RELEASE_OPTS=%RELEASE_OPTS% %~1"
shift
goto parse

rem Everything from a bare "--" onward is for prime, not for this launcher.
rem The accumulation is a plain set on its own line: delayed expansion is off,
rem so "%PRIME_ARGS%" reads the value as it stands rather than as a line the
rem parser has already walked through looking for "!".
:opt_prime
shift
:prime_loop
if "%~1"=="" goto parse
set PRIME_ARGS=%PRIME_ARGS% %1
shift
goto prime_loop

:parsed
if not defined MODE goto menu
if /I "%MODE%"=="run"     goto do_run
if /I "%MODE%"=="release" goto do_release
goto do_build

rem ============================================================================
rem  Menu
rem ============================================================================
:menu
echo.
echo  Prime
echo  ------------------------------------------------------------------
echo   1  run        launch the build already in dist ^(no compile^)
echo   2  build      pull from git, build for this machine, then launch
echo   3  release    tag this commit and publish it to GitHub
echo   h  help       print this text
echo   q  quit
echo  ------------------------------------------------------------------
choice /c 123hq /n /m "  Pick: "
if errorlevel 5 goto quit
if errorlevel 4 goto usage
if errorlevel 3 ( set "MODE=release" & goto parsed )
if errorlevel 2 ( set "MODE=build"   & goto parsed )
if errorlevel 1 ( set "MODE=run"     & goto parsed )
goto quit

:quit
set "EXITCODE=0"
goto finish

rem ============================================================================
rem  run - start what is already built
rem ============================================================================
:do_run
if not exist "dist\prime.exe" goto no_binary
call :header "Run the existing build"
echo  binary : dist\prime.exe  - nothing is compiled in this mode
call :footer
goto launch

:no_binary
call :header "Run the existing build"
echo.
echo  dist\prime.exe does not exist, so there is nothing to run yet.
echo  Build it with:  Prime.bat build
echo.
set "EXITCODE=1"
goto finish

rem ============================================================================
rem  build - sync, compile for this machine, start
rem ============================================================================
:do_build
if "%SYNC%"=="0" goto build_only

call :header "Sync sources"
echo  Pulling the newest sources from origin before building ...
echo.
rem A sync that cannot run must not stop the build. Its own warning has already
rem been printed, and when there is no network or no remote the newest sources
rem available are the ones already here - refusing to build over that would be
rem worse than saying so.
call scripts\release.bat sync
echo.

:build_only
call scripts\release.bat build %BUILD_OPTS%
set "EXITCODE=%errorlevel%"
if not "%EXITCODE%"=="0" goto finish
goto launch

rem ============================================================================
rem  release - sync, then tag and publish, forwarding the release options
rem ============================================================================
:do_release
rem The release is built from whatever HEAD points at, so a tree that is
rem behind origin ships stale code. Sync first - and fail if it cannot
rem fetch, because publishing old code under a new tag is the one mistake
rem this script must not make silently. --no-sync skips it when the caller
rem knows better.
rem
rem The sync call takes only --strict-sync, not the release flags: a
rem --dry-run would quietly make the fast-forward a no-op and the "sync
rem then release" would build from stale sources. Sync and release are
rem separate release.bat invocations with their own scopes, so the version
rem is resolved independently in each - the release step uses the right one.
rem
rem Labels, not a block: a set inside a parenthesized group reads the
rem variable as it was when the group was parsed, so the errorlevel check
rem would test the value from before the call ran.
if "%SYNC%"=="0" goto release_now
call scripts\release.bat sync --strict-sync
set "EXITCODE=%errorlevel%"
if not "%EXITCODE%"=="0" goto finish

:release_now
call :header "Release"
call scripts\release.bat release %BUILD_OPTS% %RELEASE_OPTS%
set "EXITCODE=%errorlevel%"
goto finish

rem ============================================================================
rem  Launch - the single place prime is started, so both modes pass arguments
rem  the same way.
rem ============================================================================
:launch
echo.
echo  prime reports:
"dist\prime.exe" --version
echo.
echo  Starting dist\prime.exe ...
echo  Close it to come back here.
echo.
"dist\prime.exe" %PRIME_ARGS%
set "EXITCODE=%errorlevel%"
goto finish

:finish
echo.
if "%EXITCODE%"=="0" (
    echo  Done.
) else (
    echo  Finished with exit code %EXITCODE%.
)
echo.
pause
rem One line, not two: %EXITCODE% is expanded while the variable still exists,
rem and endlocal would take it along. Split across two lines the expansion came
rem after the scope was popped, yielded nothing, and the script reported success
rem whatever the release had done.
endlocal & exit /b %EXITCODE%

:usage
echo.
echo Usage: Prime.bat [run^|build^|release] [options] [-- prime-args]
echo.
echo   no mode   show a menu
echo   run       launch the build already in dist - no compile, no Go needed
echo   build     pull from git, compile for this machine, then launch
echo   release   tag HEAD and publish a GitHub release
echo   1 2 3     the three choices, as they appear in the menu
echo.
echo Options:
echo   --no-sync      do not pull from origin before building
echo   --ratio ^<f^>   fraction of the machine to build with
echo   --jobs ^<n^>    exact job count, overrides --ratio
echo   --targets ^<os^>     release only, e.g. --targets linux or windows
echo   --version ^<x.y.z^>  release only, use this version number
echo   --notes ^<file^>     release only, release notes file
echo.
echo Everything after a bare "--" is passed to prime itself:
echo   Prime.bat build -- --continue
echo.
endlocal
exit /b 2

:header
echo.
echo  %~1
echo  ------------------------------------------------------------------
exit /b 0

:footer
echo  ------------------------------------------------------------------
echo.
exit /b 0
