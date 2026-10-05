@echo off
setlocal EnableExtensions DisableDelayedExpansion
rem ============================================================================
rem  Prime - build and run. Double-click this file.
rem
rem  Two ways in:
rem    run     start the build that is already in dist. No compile, so no Go
rem            toolchain is needed either.
rem    build   pull the newest sources from origin, compile for this machine,
rem            then start it.
rem
rem  Delayed expansion is off for the whole file, on purpose. It is on in
rem  scripts\release.bat, where every value carrying it survives only because
rem  nothing in that script stores a "!" - and a prompt passed to prime can
rem  legitimately contain one. With it on, cmd eats every "!" it finds while
rem  parsing a line, so prime would receive a silently shortened string. Off,
rem  an ordinary "%VAR%" expansion is literal and nothing is lost; the cost is
rem  that no variable may be read back inside the same parenthesised block it
rem  was assigned in, which is why every branch below jumps to a label instead.
rem
rem  Everything that is missing is installed when it is needed - the sync asks
rem  for git, the build asks for Go - and the build is throttled, so it leaves
rem  the machine usable rather than pinning every core.
rem
rem  Usage:
rem    Prime-Run.bat                       menu
rem    Prime-Run.bat run                   launch the existing build
rem    Prime-Run.bat build                 sync, build for this machine, launch
rem    Prime-Run.bat build --ratio 0.25    cooler and slower build
rem    Prime-Run.bat build --no-sync       build what is here, do not pull
rem    Prime-Run.bat run -- --continue     everything after -- goes to prime
rem ============================================================================

cd /d "%~dp0"

set "MODE="
set "SYNC=1"
set "RATIO="
set "BUILD_OPTS="
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
if "%~1"=="1"            ( set "MODE=run"   & shift & goto parse )
if "%~1"=="2"            ( set "MODE=build" & shift & goto parse )
if /I "%~1"=="--no-sync" ( set "SYNC=0"     & shift & goto parse )
if /I "%~1"=="--ratio"   goto opt_ratio
if /I "%~1"=="--jobs"    goto opt_jobs
if "%~1"=="--"           goto opt_prime
if "%~1"=="-h"           goto usage
if /I "%~1"=="--help"    goto usage
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
if /I "%MODE%"=="run" goto do_run
goto do_build

rem ============================================================================
rem  Menu
rem ============================================================================
:menu
echo.
echo  Prime - build and run
echo  ------------------------------------------------------------------
echo   1  run      launch the build already in dist ^(no compile^)
echo   2  build    pull from git, build for this machine, then launch
echo   q  quit
echo  ------------------------------------------------------------------
choice /c 12q /n /m "  Pick: "
if errorlevel 3 goto quit
if errorlevel 2 ( set "MODE=build" & goto parsed )
if errorlevel 1 ( set "MODE=run"   & goto parsed )
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
echo  Build it with:  Prime-Run.bat build
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
rem and endlocal would take it along. Split in two, the expansion would come
rem after the scope was popped and the launcher would always report success.
endlocal & exit /b %EXITCODE%

:usage
echo.
echo Usage: Prime-Run.bat [run^|build] [options] [-- prime-args]
echo.
echo   no mode   show a menu
echo   run       launch the build already in dist - no compile, no Go needed
echo   build     pull from git, compile for this machine, then launch
echo   1  2      the same two choices, as they appear in the menu
echo.
echo Options:
echo   --no-sync      do not pull from origin before building
echo   --ratio ^<f^>   fraction of the machine to build with
echo   --jobs ^<n^>    exact job count, overrides --ratio
echo.
echo Everything after a bare "--" is passed to prime itself:
echo   Prime-Run.bat build -- --continue
echo.
echo Missing tools are installed when they are needed - the sync asks for git,
echo the build asks for Go - and the build is throttled so the machine stays
echo usable while it runs.
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
