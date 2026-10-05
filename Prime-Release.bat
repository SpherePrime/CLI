@echo off
rem ============================================================================
rem  Prime release - double click this file.
rem
rem  Builds every Windows and Linux target on this machine, tags the current
rem  commit, and publishes a GitHub release. One Y and it goes.
rem
rem  Everything it needs is installed if missing: the Go toolchain, the GitHub
rem  CLI, and a signed-in gh.
rem
rem  For anything narrower, open a terminal here and use scripts\release.bat:
rem    menu              pick a single step
rem    sync              pull the newest sources from origin
rem    build             this machine only
rem    run               build and launch it
rem    all               every target, no publishing
rem    check             what would be released, changes nothing
rem    release --dry-run print the plan, publish nothing
rem ============================================================================

setlocal
cd /d "%~dp0"
call "%~dp0scripts\release.bat" sync --strict-sync %*
set "EXITCODE=%errorlevel%"
if not "%EXITCODE%"=="0" goto finished
call "%~dp0scripts\release.bat" release %*
set "EXITCODE=%errorlevel%"

:finished
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
