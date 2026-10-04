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
rem    build             this machine only
rem    run               build and launch it
rem    all               every target, no publishing
rem    check             what would be released, changes nothing
rem    release --dry-run print the plan, publish nothing
rem ============================================================================

setlocal
cd /d "%~dp0"
call "%~dp0scripts\release.bat" release %*
set "EXITCODE=%errorlevel%"

echo.
if "%EXITCODE%"=="0" (
    echo  Done.
) else (
    echo  Finished with exit code %EXITCODE%.
)
echo.
pause
endlocal
exit /b %EXITCODE%
