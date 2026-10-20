@echo off
setlocal EnableExtensions DisableDelayedExpansion
rem ============================================================================
rem  Prime - one launcher for everything: run, build, release.
rem
rem  Replaces Prime-Run.bat and Prime-Release.bat:
rem
rem    run         build (or not) and launch the app. Arguments after "--"
rem                 go to prime itself:  Prime.bat build -- --continue
rem    release     tag the commit and publish a GitHub release. A sub-menu
rem                 picks which platforms ship; the build runs and the
rem                 release is published in one go. Stable tags also ship
rem                 the Prime Desktop setup, built by the Release workflow.
rem    desktop     run Prime Desktop from desktop\ with a CLI built from here
rem    setup       build the Prime Desktop setup locally into desktop\release
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
rem    Prime.bat release               pick platforms, tag, publish
rem    Prime.bat release --targets linux
rem    Prime.bat release --dry-run     print the plan, publish nothing
rem    Prime.bat desktop               run Prime Desktop from sources
rem    Prime.bat setup                 build PrimeSetup-<version>.exe locally
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
if /I "%~1"=="desktop"   ( set "MODE=desktop" & shift & goto parse )
if /I "%~1"=="setup"     ( set "MODE=setup" & shift & goto parse )
if /I "%~1"=="1"         ( set "MODE=run"   & shift & goto parse )
if /I "%~1"=="2"         ( set "MODE=build" & shift & goto parse )
if /I "%~1"=="3"         ( set "MODE=release" & shift & goto parse )
if /I "%~1"=="4"         ( set "MODE=desktop" & shift & goto parse )
if /I "%~1"=="5"         ( set "MODE=setup" & shift & goto parse )
if /I "%~1"=="--no-sync" ( set "SYNC=0" & shift & goto parse )
rem build.bat understands these; thread them through.
if /I "%~1"=="--ratio"   goto opt_ratio
if /I "%~1"=="--jobs"    goto opt_jobs
if /I "%~1"=="--version" goto opt_version
rem Everything the release script understands that the launcher does not
rem parses: forward the flag and its value pair to release.bat as-is.
if /I "%~1"=="--targets"   goto opt_targets_flag
if /I "%~1"=="--notes"     goto opt_pair
if /I "%~1"=="--out"       goto opt_pair
rem Valueless release flags forward as-is.
if /I "%~1"=="--dry-run"      goto opt_bare
if /I "%~1"=="--yes"          goto opt_bare
if /I "%~1"=="--test"        goto opt_bare
if /I "%~1"=="--skip-build"    goto opt_bare
if /I "%~1"=="--upload-local"   goto opt_bare
if /I "%~1"=="--no-auto-commit" goto opt_bare
if /I "%~1"=="--no-verify"      goto opt_bare
if /I "%~1"=="--"          goto opt_prime
if /I "%~1"=="-h"          goto usage
if /I "%~1"=="--help"      goto usage
if /I "%~1"=="-?"          goto usage
if /I "%~1"=="help"        goto usage
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

rem A paired release-only option, value included, e.g. "--notes CHANGELOG.md".
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

rem --targets takes one or more tokens, e.g. "release --targets windows
rem linux/arm64". The list ends at the next flag or the end of the line;
rem all tokens are accumulated and forwarded as ONE --targets pair, so
rem the release script's parser does not reset its list on every
rem occurrence. Naming targets here means release.bat does not show its
rem own pick menu. Quoted tokens arrive without their quotes via %~1,
rem so there is nothing to stop on.
:opt_targets_flag
shift
set "T_LIST="
:targets_loop
if "%~1"=="" goto targets_done
if /I "%~1:~0,1"=="-" goto targets_done
set "T_LIST=%T_LIST% %~1"
shift
goto targets_loop
:targets_done
if not defined T_LIST ( echo --targets needs at least one value & goto usage )
rem Single quotes stay out: a quoted list would break "for %%T in (%TARGETS%)"
rem in the release script.
set "RELEASE_OPTS=%RELEASE_OPTS% --targets %T_LIST%"
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
if /I "%MODE%"=="desktop" goto do_desktop
if /I "%MODE%"=="setup"   goto do_setup
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
echo   3  release    pick platforms, tag this commit, publish to GitHub
echo   4  desktop    run Prime Desktop from sources
echo   5  setup      build the Prime Desktop setup locally
echo   h  help       print this text
echo   q  quit
echo  ------------------------------------------------------------------
choice /c 12345hq /n /m "  Pick: "
if errorlevel 7 goto quit
if errorlevel 6 goto menu_help
if errorlevel 5 ( set "MODE=setup"   & goto parsed )
if errorlevel 4 ( set "MODE=desktop" & goto parsed )
if errorlevel 3 ( set "MODE=release" & goto parsed )
if errorlevel 2 ( set "MODE=build"   & goto parsed )
if errorlevel 1 ( set "MODE=run"     & goto parsed )
goto quit

rem The choice errorlevels count down: with 7 choices, q is 7, h is 6, then
rem setup, desktop, release, build and run.
:menu_help
call :usage
pause
exit /b 0

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
rem  release - sync, auto-commit, tag and publish, forwarding the options
rem
rem  The platform pick, the pre-release auto-commit, and the post-publish
rem  asset check live in release.bat so both this launcher and anyone who
rem  runs that script directly get the same behavior. Naming --targets on
rem  the command line skips the interactive pick; otherwise release.bat asks.
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
if "%SYNC%"=="1" (
    call scripts\release.bat sync --strict-sync
    set "EXITCODE=%errorlevel%"
    if not "%EXITCODE%"=="0" goto finish
)

call :header "Release"
rem release.bat picks the targets interactively when --targets was not passed,
rem auto-commits the working tree, builds the chosen set, publishes, and then
rem verifies that every chosen asset is present - saying so plainly if not.
call scripts\release.bat release %BUILD_OPTS% %RELEASE_OPTS%
set "EXITCODE=%errorlevel%"
goto finish

rem ============================================================================
rem  desktop / setup - Prime Desktop from desktop\, bundling a CLI built from
rem  this checkout. Node.js 24 and Go are required.
rem ============================================================================
:do_desktop
call :header "Prime Desktop"
call :desktop_prepare
if not "%EXITCODE%"=="0" goto finish
echo  Starting Prime Desktop from sources. Close it to come back here.
echo.
pushd desktop
call npm run dev
set "EXITCODE=%errorlevel%"
popd
goto finish

:do_setup
call :header "Prime Desktop setup"
tasklist /FI "IMAGENAME eq PrimeDesktop.exe" /NH 2>nul | find /I "PrimeDesktop.exe" >nul
if not errorlevel 1 (
    echo  Prime Desktop is running. Close it ^(also from the tray^) and run setup again:
    echo  the installer cannot replace files of a running app.
    set "EXITCODE=1"
    goto finish
)
call :desktop_prepare
if not "%EXITCODE%"=="0" goto setup_prepare_failed
pushd desktop
set "DESKTOP_VERSION="
for /f "usebackq delims=" %%v in (`node scripts\bump-version.mjs`) do set "DESKTOP_VERSION=%%v"
if not defined DESKTOP_VERSION goto setup_failed
echo  Desktop version: %DESKTOP_VERSION%
set "SETUP_FILE=release\%DESKTOP_VERSION%\PrimeSetup-%DESKTOP_VERSION%.exe"
if exist "%SETUP_FILE%" del /f /q "%SETUP_FILE%" >nul 2>&1
if exist "%SETUP_FILE%" (
    echo  %SETUP_FILE% is in use. Close the running installer and try again.
    goto setup_failed
)
if exist "release\%DESKTOP_VERSION%\PrimeDesktop-Update-%DESKTOP_VERSION%.exe" del /f /q "release\%DESKTOP_VERSION%\PrimeDesktop-Update-%DESKTOP_VERSION%.exe" >nul 2>&1
call npm run build
if errorlevel 1 goto setup_failed
call npx electron-builder --config electron-builder.yml --win nsis --x64 --publish never --config.directories.output=release/%DESKTOP_VERSION%
if errorlevel 1 goto setup_failed
call npm run dist:installer
if errorlevel 1 goto setup_failed
if not exist "%SETUP_FILE%" goto setup_failed
popd
echo.
echo  Setup: desktop\%SETUP_FILE%
set "EXITCODE=0"
choice /c YN /n /t 30 /d N /m "  Install it now? [Y/N] (30s, default N) "
if errorlevel 2 goto finish
start "" "%~dp0desktop\%SETUP_FILE%"
goto finish

:setup_prepare_failed
echo.
echo  Setup was not built: the bundled CLI could not be prepared.
echo  If Prime Desktop runs from sources ^(menu 4^), close it: it locks desktop\runtime\prime.exe.
goto finish

:setup_failed
set "EXITCODE=%errorlevel%"
if "%EXITCODE%"=="0" set "EXITCODE=1"
popd
echo.
echo  Setup was not built. The messages above show why; no old installer was left behind.
goto finish

:desktop_prepare
set "EXITCODE=0"
if not exist "desktop\package.json" (
    echo  desktop\package.json not found.
    set "EXITCODE=1"
    exit /b 1
)
where node >nul 2>&1 || goto desktop_no_node
where go >nul 2>&1 || goto desktop_no_go
pushd desktop
if not exist "node_modules" (
    echo  Installing Desktop dependencies ...
    call npm ci
    if errorlevel 1 goto desktop_prepare_failed
)
echo  Building the bundled CLI runtime ...
node scripts\acquire-runtime.mjs
if errorlevel 1 goto desktop_prepare_failed
popd
exit /b 0

:desktop_prepare_failed
popd
set "EXITCODE=1"
exit /b 1

:desktop_no_node
echo  Node.js 24 is required: https://nodejs.org
set "EXITCODE=1"
exit /b 1

:desktop_no_go
echo  Go is required to build the bundled CLI. Run "Prime.bat build" once to install it.
set "EXITCODE=1"
exit /b 1

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
rem A pause makes sense when a person double-clicks the launcher; when it is
rem called from another script the caller decides when to stop.
if "%PRIME_CALLED%"=="1" endlocal & exit /b %EXITCODE%
pause
rem One line, not two: %EXITCODE% is expanded while the variable still exists,
rem and endlocal would take it along. Split across two lines the expansion came
rem after the scope was popped, yielded nothing, and the script reported success
rem whatever the release had done.
endlocal & exit /b %EXITCODE%

rem ---------------------------------------------------------------------------
rem  usage - the help text. A plain label so "h", "help", "-h" and "--help"
rem  all reach it; it exits without the final pause-and-exit of :finish so
rem  it is quiet enough to pipe.
rem ---------------------------------------------------------------------------
:usage
echo.
echo Usage: Prime.bat [run^|build^|release^|desktop^|setup] [options] [-- prime-args]
echo.
echo   no mode   show a menu
echo   run       launch the build already in dist - no compile, no Go needed
echo   build     pull from git, compile for this machine, then launch
echo   release   pick platforms in a menu, tag HEAD, publish a GitHub release;
echo             stable tags also ship PrimeSetup-^<version^>.exe for Desktop
echo   desktop   run Prime Desktop from desktop\ ^(Node.js 24 and Go^)
echo   setup     build PrimeSetup-^<version^>.exe into desktop\release
echo   1-5       the choices, as they appear in the menu
echo   help      print this text
echo.
echo Options:
echo   --no-sync      do not pull from origin before building
echo   --ratio ^<f^>   fraction of the machine to build with
echo   --jobs ^<n^>    exact job count, overrides --ratio
echo   --targets ^<tok^>... release only; a platform (windows, linux, all) or
echo                  an arch (linux/arm64). Skips the target menu.
echo   --version ^<x.y.z^>  release only, use this version number
echo   --notes ^<file^>     release only, release notes file
echo   --no-auto-commit release only, do not commit the tree before building
echo   --no-verify      release only, skip the post-publish asset check
echo.
echo Everything after a bare "--" is passed to prime itself:
echo   Prime.bat build -- --continue
echo.
exit /b 0

:header
echo.
echo  %~1
echo  ------------------------------------------------------------------
exit /b 0

:footer
echo  ------------------------------------------------------------------
echo.
exit /b 0
