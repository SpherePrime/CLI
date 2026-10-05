@echo off
setlocal EnableExtensions EnableDelayedExpansion
rem ============================================================================
rem  Prime release tool
rem
rem  One entry point for the whole shipping loop: build for this machine and
rem  try it, build every Windows and Linux target, work out the next version
rem  from the last published release, and put a release on GitHub.
rem
rem  Commands
rem    build     build for this machine only (throttled, quick)
rem    run       build for this machine and launch it so you can try it
rem    all       build every Windows and Linux target
rem    check     show the last release, the version that would be used, and
rem               the working tree state. Changes nothing.
rem    release   build everything, tag, and publish to GitHub
rem    draft     same as release but leaves a draft for review
rem    clean     delete the output directory
rem    help      this text
rem
rem  With no argument a menu is shown.
rem
rem  Options
rem    --ratio <f>      fraction of the machine to build with (default 0.5)
rem    --jobs <n>       exact job count, overrides --ratio
rem    --out <dir>      output directory (default dist)
rem    --version <x.y.z>  use this version instead of last release + 1
rem    --notes <file>   release notes file (default: auto summary)
rem    --test           run the test suite before packaging (slow)
rem    --dry-run        print what would happen, touch nothing remote
rem    --skip-build     package what is already in the output directory
rem
rem  Example
rem    scripts\release.bat
rem    scripts\release.bat run --ratio 0.25
rem    scripts\release.bat all --no-test
rem    scripts\release.bat release --dry-run
rem ============================================================================

set "CMD="
set "RATIO=0.5"
set "JOBS="
set "OUT=dist"
set "VERSION="
set "NOTES="
set "RUN_TEST=0"
set "DRY_RUN=0"
set "SKIP_BUILD=0"
set "UPLOAD_LOCAL=0"
set "ASSUME_YES=0"

set "ROOT=%~dp0.."
pushd "%ROOT%" >nul 2>&1 || ( echo Cannot enter %ROOT% & exit /b 1 )

rem ---- argument parsing -------------------------------------------------------
rem Each option jumps to a label so %%~1 is re-expanded after the shift: inside
rem a parenthesised block it is frozen at parse time, which silently shifts the
rem following argument into the wrong variable.
:parse
if "%~1"=="" goto parsed
if /I "%~1"=="build"    ( set "CMD=build"    & shift & goto parse )
if /I "%~1"=="run"      ( set "CMD=run"      & shift & goto parse )
if /I "%~1"=="all"      ( set "CMD=all"      & shift & goto parse )
if /I "%~1"=="check"    ( set "CMD=check"    & shift & goto parse )
if /I "%~1"=="release"  ( set "CMD=release"  & shift & goto parse )
if /I "%~1"=="draft"    ( set "CMD=draft"    & shift & goto parse )
if /I "%~1"=="clean"    ( set "CMD=clean"    & shift & goto parse )
if /I "%~1"=="menu"     ( set "CMD=menu"     & shift & goto parse )
if /I "%~1"=="help"     goto usage
if /I "%~1"=="-h"       goto usage
if /I "%~1"=="--help"   goto usage
if /I "%~1"=="--ratio"  goto opt_ratio
if /I "%~1"=="--jobs"   goto opt_jobs
if /I "%~1"=="--out"    goto opt_out
if /I "%~1"=="--version" goto opt_version
if /I "%~1"=="--notes"  goto opt_notes
if /I "%~1"=="--test"   ( set "RUN_TEST=1"  & shift & goto parse )
if /I "%~1"=="--upload-local" ( set "UPLOAD_LOCAL=1" & shift & goto parse )
if /I "%~1"=="--yes"    ( set "ASSUME_YES=1" & shift & goto parse )
if /I "%~1"=="--dry-run" ( set "DRY_RUN=1" & shift & goto parse )
if /I "%~1"=="--skip-build" ( set "SKIP_BUILD=1" & shift & goto parse )
echo Unknown option: %~1
goto usage

:opt_ratio
shift
if "%~1"=="" ( echo --ratio needs a fraction & goto usage )
set "RATIO=%~1"
shift
goto parse

:opt_jobs
shift
if "%~1"=="" ( echo --jobs needs a number & goto usage )
set "JOBS=%~1"
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
set "VERSION=%~1"
shift
goto parse

:opt_notes
shift
if "%~1"=="" ( echo --notes needs a file & goto usage )
set "NOTES=%~1"
shift
goto parse

:parsed

rem build.bat runs the suite unless told otherwise. CI already covers it and the
rem suite takes minutes, so this script defaults to building only.
set "BUILD_TEST_FLAG=--no-test"
if "%RUN_TEST%"=="1" set "BUILD_TEST_FLAG="

rem No argument means the whole thing. The menu is one keystroke away for
rem anything narrower, but the common case should not need a menu at all.
rem
rem This has to enter at the top of do_release and not at the confirmation
rem prompt: the version is resolved and the tag name is built before the prompt
rem is reached, and jumping straight there left the prompt naming an empty tag.
if "%CMD%"=="" (
    set "CMD=release"
    goto do_release
)

if /I "%CMD%"=="help"   goto usage
if /I "%CMD%"=="clean"  goto do_clean
if /I "%CMD%"=="build"  goto do_build
if /I "%CMD%"=="run"    goto do_run
if /I "%CMD%"=="all"    goto do_all
if /I "%CMD%"=="check"  goto do_check
if /I "%CMD%"=="release" goto do_release
if /I "%CMD%"=="draft"  goto do_release
if /I "%CMD%"=="menu"   goto menu
goto usage

rem ============================================================================
rem  Menu
rem ============================================================================
:menu
echo.
echo  Prime release tool
echo  ------------------------------------------------------------------
echo   1  build      build for this machine
echo   2  run        build for this machine and launch it
echo   3  all        build every Windows and Linux target
echo   4  check      show the last release and the proposed version
echo   5  release    build, tag, and publish to GitHub
echo   6  draft      build, tag, and leave a draft release
echo   7  clean      delete %OUT%
echo   q  quit
echo  ------------------------------------------------------------------
choice /c 1234567q /n /m "  Pick: "
if errorlevel 8 goto :eof
if errorlevel 7 ( set "CMD=clean"  & goto do_clean )
if errorlevel 6 ( set "CMD=draft"  & goto do_release )
if errorlevel 5 ( set "CMD=release"& goto do_release )
if errorlevel 4 ( set "CMD=check"  & goto do_check )
if errorlevel 3 ( set "CMD=all"    & goto do_all )
if errorlevel 2 ( set "CMD=run"    & goto do_run )
if errorlevel 1 ( set "CMD=build"  & goto do_build )
goto :eof

rem ============================================================================
rem  Preflight
rem
rem  Missing tools are installed rather than reported. winget is tried first
rem  because it ships with Windows 10 1809 and later; choco is the fallback for
rem  machines without it. Anything neither can provide is explained instead of
rem  guessed at.
rem ============================================================================

rem refresh_path rebuilds PATH from the registry. An installer writes the new
rem location to the environment, but this cmd session was started before that
rem happened, so without this the tool stays invisible for the rest of the run
rem and the next step fails even though the install succeeded.
:refresh_path
set "SAVED_PATH=%PATH%"
set "MACHINE_PATH="
set "USER_PATH="
for /f "tokens=2,*" %%a in ('reg query "HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment" /v Path 2^>nul ^| find "REG_EXPAND_SZ"') do set "MACHINE_PATH=%%b"
for /f "tokens=2,*" %%a in ('reg query "HKCU\Environment" /v Path 2^>nul ^| find "REG_EXPAND_SZ"') do set "USER_PATH=%%b"
if defined MACHINE_PATH if defined USER_PATH (
    set "PATH=%MACHINE_PATH%;%USER_PATH%"
    call :expand_path
    set "PATH=!PATH!;%SAVED_PATH%"
) else (
    set "PATH=%SAVED_PATH%"
)
exit /b 0

rem expand_path turns the %SystemRoot%-style entries the registry stores into
rem real directories. Left unexpanded, Go's directory is never found.
:expand_path
setlocal EnableDelayedExpansion
set "WORK=%PATH%"
set "OUT_PATH="
:expand_loop
if "!WORK!"=="" goto expand_done
for /f "tokens=1,* delims=;" %%a in ("!WORK!") do (
    set "HEAD=%%a"
    set "TAIL=%%b"
)
if defined HEAD (
    call set "HEAD=%%HEAD%%"
    if "!HEAD:~2!"=="~" call set "HEAD=%%HEAD:~2%%"
    if not "!HEAD:~2!"=="~" call set "HEAD=%%PATH:~0,2%%!HEAD:~2!"
    set "OUT_PATH=!OUT_PATH!;!HEAD!"
)
set "WORK=!TAIL!"
goto expand_loop
:expand_done
set "PATH=%OUT_PATH:~1%"
endlocal & set "PATH=%PATH%"
exit /b 0

:have_winget
where winget >nul 2>&1
exit /b %errorlevel%

:have_choco
where choco >nul 2>&1
exit /b %errorlevel%

rem install_tool <winget-id> <choco-package> <human name>
:install_tool
set "WG_ID=%~1"
set "CHOCO_PKG=%~2"
set "HUMAN=%~3"

echo  !HUMAN! is missing. Installing it ...

call :have_winget
if not errorlevel 1 (
    echo    via winget: !WG_ID!
    winget install --id "!WG_ID!" --exact --silent ^
        --accept-source-agreements --accept-package-agreements
    if not errorlevel 1 goto install_refresh
)

call :have_choco
if not errorlevel 1 (
    echo    via chocolatey: !CHOCO_PKG!
    choco install "!CHOCO_PKG!" -y --no-progress
    if not errorlevel 1 goto install_refresh
)

echo.
echo  ERROR: could not install !HUMAN! automatically.
echo         Install it by hand and re-run this script:
echo           winget install --id !WG_ID! --exact
echo         or download from the official site and open a new terminal.
echo.
exit /b 1

:install_refresh
rem Give the installer a moment to release the file handles before probing.
timeout /t 3 /nobreak >nul
call :refresh_path
exit /b 0

rem ensure_go makes the Go toolchain available, installing it if needed.
:ensure_go
where go >nul 2>&1 && exit /b 0
call :refresh_path
where go >nul 2>&1 && exit /b 0
call :install_tool "GoLang.Go" "golang" "The Go toolchain"
where go >nul 2>&1 || ( echo. & echo ERROR: Go is still not on PATH. & exit /b 1 )
exit /b 0

rem ensure_git makes git available, installing it if needed.
rem
rem Every path here tags, pushes and inspects the repository, so a machine
rem without git produced "not recognized as an internal or external command"
rem from somewhere deep in the flow rather than anything saying which tool was
rem missing. It is checked before the first git call, not before the first
rem failure.
:ensure_git
where git >nul 2>&1 && exit /b 0
call :refresh_path
where git >nul 2>&1 && exit /b 0
call :install_tool "Git.Git" "git" "Git"
where git >nul 2>&1 || ( echo. & echo ERROR: Git is still not on PATH. & exit /b 1 )
exit /b 0

rem ensure_repo checks this is a git working copy with a remote, because
rem everything downstream either tags HEAD or pushes to one.
:ensure_repo
git rev-parse --git-dir >nul 2>&1 || (
    echo.
    echo ERROR: this directory is not a git repository.
    exit /b 1
)
git remote >nul 2>&1 || (
    echo.
    echo ERROR: this repository has no remote, so nothing could be published.
    exit /b 1
)
exit /b 0

rem ensure_gh makes the GitHub CLI available, installing it if needed.
:ensure_gh
where gh >nul 2>&1 && exit /b 0
call :refresh_path
where gh >nul 2>&1 && exit /b 0
call :install_tool "GitHub.cli" "gh" "The GitHub CLI"
where gh >nul 2>&1 || ( echo. & echo ERROR: gh is still not on PATH. & exit /b 1 )
exit /b 0

rem ensure_gh_auth makes sure gh can actually talk to the repository. A token
rem in the environment is honoured first so a machine without a stored login
rem still works unattended.
:ensure_gh_auth
if not "%GITHUB_TOKEN%"=="" exit /b 0
if not "%GH_TOKEN%"=="" exit /b 0
gh auth status >nul 2>&1 && exit /b 0

echo.
echo  GitHub CLI is installed but not signed in.
echo  Signing in now - it opens a browser, approve there, then come back.
echo.
gh auth login --hostname github.com --git-protocol https --web --skip-ssh-key
if errorlevel 1 (
    echo.
    echo ERROR: sign-in did not complete.
    echo         Run "gh auth login" yourself, or set GITHUB_TOKEN in the
    echo         environment, then re-run this script.
    exit /b 1
)
exit /b 0

rem ensure_tar checks the archiver the Linux packages are built with.
rem
rem bsdtar ships with Windows 10 1803 and later, so a miss is rare. It is also
rem not something winget or chocolatey can install: it is an operating system
rem feature, not a package. So this checks, and if it is missing it says which
rem feature to re-enable rather than pretending the script can fix it.
rem
rem There used to be a 7-Zip fallback here for the trimmed-image case. It was
rem removed on purpose: the second implementation of tar.gz creation could not
rem be exercised on the machine that wrote it, and an archive producer that has
rem never run is worse than a clear error - it either fails at the worst moment
rem or produces something subtly different from what was tested.
:ensure_tar
where tar >nul 2>&1 && exit /b 0
call :refresh_path
where tar >nul 2>&1 && exit /b 0
echo.
echo ERROR: tar ^(bsdtar^) was not found. The Linux packages are built with it.
echo        It ships with Windows 10 1803 and later as an operating system
echo        feature, which no package manager installs.
echo.
echo        Re-enable it with, as administrator:
echo          DISM /Online /Enable-Feature /FeatureName:RemoteSigned /All /NoRestart
echo        or turn on the equivalent Windows feature.
exit /b 1

rem resolve_version sets VERSION to the explicit --version if given, otherwise
rem last release + 1 patch.
:
:resolve_version
if defined VERSION (
    rem strip a leading v so callers can pass either form
    set "VERSION=%VERSION:v=%"
    exit /b 0
)

set "LATEST_TAG="
for /f "usebackq delims=" %%t in (`gh release list --limit 1 --json tagName --jq ".[0].tagName" 2^>nul`) do (
    if not defined LATEST_TAG set "LATEST_TAG=%%t"
)
rem gh may be missing or unauthenticated; the tags are a good enough fallback
rem for working out a number, and :ensure_gh will complain before publishing.
if not defined LATEST_TAG (
    for /f "usebackq delims=" %%t in (`git tag --sort=-v:refname 2^>nul`) do (
        if not defined LATEST_TAG set "LATEST_TAG=%%t"
    )
)
if not defined LATEST_TAG set "LATEST_TAG=v0.0.0"

rem The addition needs its own parentheses: PowerShell binds -f tighter than +,
rem so -f a, b, c+1 formats with c and then appends "1" to the whole string.
rem That turned v0.4.36 into v0.4.361.
for /f "usebackq delims=" %%v in (`powershell -NoProfile -Command ^
  "$t='!LATEST_TAG!'; if($t -match '^v?(\d+)\.(\d+)\.(\d+)'){ '{0}.{1}.{2}' -f [int]$Matches[1], [int]$Matches[2], ([int]$Matches[3]+1) } else { '0.0.1' }"`) do set "VERSION=%%v"
exit /b 0

:current_sha
set "SHA="
for /f "usebackq delims=" %%s in (`git rev-parse HEAD 2^>nul`) do set "SHA=%%s"
exit /b 0

:print_header
echo.
echo  %~1
echo  ------------------------------------------------------------------
exit /b 0

:print_footer
echo  ------------------------------------------------------------------
echo.
exit /b 0

rem ============================================================================
rem  clean
rem ============================================================================
:do_clean
call :print_header "Clean"
if exist "%OUT%" (
    echo  Removing %OUT% ...
    rd /s /q "%OUT%"
)
if exist "%OUT%" ( echo  ERROR: could not remove %OUT% & exit /b 1 )
echo  Nothing left to remove.
goto :eof

rem ============================================================================
rem  build
rem ============================================================================
:do_build
call :ensure_go || exit /b 1
call :print_header "Build for this machine"
echo  mode   : native only
echo  output : %OUT%
call :print_footer
call :run_build "" %RATIO% || exit /b 1
goto :eof

rem ============================================================================
rem  run
rem ============================================================================
:do_run
call :ensure_go || exit /b 1
call :print_header "Build and run"
echo  Building the native binary, then launching it.
echo  Close the app with ctrl+c to come back here.
call :print_footer
call :run_build "" %RATIO% || exit /b 1

if not exist "%OUT%\prime.exe" (
    echo.
    echo ERROR: %OUT%\prime.exe was not produced.
    exit /b 1
)

echo.
echo  prime reports:
"%OUT%\prime.exe" --version
echo.
echo  Starting %OUT%\prime.exe ...
echo  Close it to come back here.
echo.
"%OUT%\prime.exe" %*
goto :eof

rem ============================================================================
rem  all
rem ============================================================================
:do_all
call :ensure_go || exit /b 1
call :print_header "Build every Windows and Linux target"
echo  output : %OUT%
echo  This is several full compiles of a large program.
call :print_footer
call :run_build "--os windows" %RATIO% || exit /b 1
call :run_build "--os linux"   %RATIO% || exit /b 1
goto :eof

rem ---------------------------------------------------------------------------
rem  run_build <build.bat-extra-args> <ratio>
rem
rem  Delegates the actual compiling to build.bat so the throttle lives in one
rem  place. Returns non-zero when anything failed to compile.
rem ---------------------------------------------------------------------------
:run_build
set "EXTRA=%~1"
set "RATIO_ARG=%~2"

rem VERSION is only set for the release path. A release binary that reports a
rem commit hash instead of its tag is worse than useless: prime --version would
rem lie and any update check keyed off it would misbehave.
set "VER_ARG="
if defined VERSION set "VER_ARG=--version %VERSION%"

if defined JOBS (
    call scripts\build.bat !EXTRA! --jobs %JOBS% --out "%OUT%" !VER_ARG! %BUILD_TEST_FLAG%
) else (
    call scripts\build.bat !EXTRA! --ratio %RATIO_ARG% --out "%OUT%" !VER_ARG! %BUILD_TEST_FLAG%
)
exit /b %errorlevel%

rem ============================================================================
rem  check
rem ============================================================================
:do_check
call :print_header "Check"
echo  repository : %ROOT%
call :ensure_git || exit /b 1
call :current_sha
echo  HEAD       : !SHA!
for /f "usebackq tokens=*" %%b in (`git status --porcelain`) do set "DIRTY=1"
if defined DIRTY (
    echo  tree       : DIRTY - uncommitted changes will NOT be in a release
) else (
    echo  tree       : clean
)
if defined DIRTY set "DIRTY="

echo  output dir : %OUT%

rem check reports, so a missing gh is information rather than a failure: try to
rem install it, but carry on and say what could not be read.
call :ensure_gh
if errorlevel 1 (
    echo  gh         : unavailable, release information cannot be read
    goto :check_finish
)

rem Each field is fetched with its own call. A single jq expression joining
rem them would contain a pipe, and cmd reads that as a command separator even
rem inside a for /f backtick.
set "LAST_TAG="
for /f "usebackq delims=" %%t in (`gh release list --limit 1 --json tagName --jq ".[0].tagName" 2^>nul`) do set "LAST_TAG=%%t"

if not defined LAST_TAG (
    echo  last release: none found
    goto :check_version
)

set "LAST_DRAFT=n/a"
set "LAST_PRE=n/a"
set "LAST_WHEN=n/a"
for /f "usebackq delims=" %%d in (`gh release view "!LAST_TAG!" --json isDraft --jq ".isDraft" 2^>nul`) do set "LAST_DRAFT=%%d"
for /f "usebackq delims=" %%d in (`gh release view "!LAST_TAG!" --json isPrerelease --jq ".isPrerelease" 2^>nul`) do set "LAST_PRE=%%d"
for /f "usebackq delims=" %%d in (`gh release view "!LAST_TAG!" --json publishedAt --jq ".publishedAt" 2^>nul`) do set "LAST_WHEN=%%d"
echo  last release: !LAST_TAG! ^(draft=!LAST_DRAFT! prerelease=!LAST_PRE! published=!LAST_WHEN!^)
set "LATEST_TAG=!LAST_TAG!"

:check_version
call :resolve_version
echo  next version: v%VERSION%
if defined LAST_TAG (
    git ls-remote --exit-code --tags origin "refs/tags/v%VERSION%" >nul 2>&1
    if errorlevel 1 (
        echo  tag v%VERSION% : free
    ) else (
        echo  tag v%VERSION% : ALREADY EXISTS - pick another with --version
    )
)

:check_finish
call :print_footer
goto :eof

rem ============================================================================
rem  release / draft
rem ============================================================================
:do_release
call :ensure_git || exit /b 1
call :ensure_repo || exit /b 1
call :ensure_go  || exit /b 1
call :ensure_gh  || exit /b 1
call :ensure_tar || exit /b 1
call :ensure_gh_auth || exit /b 1

call :current_sha
if not defined SHA ( echo ERROR: not a git repository with a HEAD & exit /b 1 )

for /f "usebackq tokens=*" %%b in (`git status --porcelain`) do set "DIRTY=1"
if defined DIRTY (
    echo.
    echo WARNING: the working tree has uncommitted changes.
    echo          A release is built from HEAD, so they will not be included.
    echo.
)
if defined DIRTY set "DIRTY="

call :resolve_version
set "TAG=v%VERSION%"

rem A tag that already exists on the remote is not automatically fatal. Pushing
rem the tag is the first irreversible step, so a failure after it - a dropped
rem upload, a rate limit - would otherwise leave the script unable to finish
rem the job. If the tag points at the commit being released, carry on and let
rem the release be created or topped up.
set "TAG_EXISTS=0"
set "TAG_SHA="
for /f "usebackq tokens=1" %%t in (`git ls-remote --tags origin "refs/tags/%TAG%" 2^>nul`) do (
    if not defined TAG_SHA set "TAG_SHA=%%t"
)
if defined TAG_SHA (
    set "TAG_EXISTS=1"
    rem An annotated tag resolves to the tag object, so peel it to the commit.
    for /f "usebackq tokens=1" %%t in (`git ls-remote --tags origin "refs/tags/%TAG%^{}" 2^>nul`) do set "TAG_SHA=%%t"
    rem Short SHAs are what git prints locally but the remote sends full ones.
    set "HEAD_SHORT=!SHA:~0,7!"
    if /I "!TAG_SHA:~0,7!"=="!HEAD_SHORT!" (
        echo.
        echo  Tag %TAG% is already pushed for this commit.
        echo  Continuing: the release will be created or topped up.
        echo.
    ) else (
        echo.
        echo ERROR: tag %TAG% exists on the remote and points at !TAG_SHA!,
        echo        not at this commit !SHA!.
        echo        Pass --version x.y.z to choose another version.
        exit /b 1
    )
)

call :print_header "Release %TAG%"
echo  HEAD        : !SHA!
echo  from commit : !SHA!
echo  output      : %OUT%
echo  mode        : %CMD%
if "%TAG_EXISTS%"=="1" echo  tag        : already on the remote, will not be re-pushed
if "%DRY_RUN%"=="1" echo  DRY RUN - nothing will be pushed
call :print_footer

rem One keystroke to start is the point, but a tag that has been pushed cannot
rem be taken back without deleting it on the remote, so the irreversible step
rem gets one yes or no. --yes skips it for unattended runs.
:confirm_release
if "%DRY_RUN%"=="1" goto confirmed
if "%ASSUME_YES%"=="1" goto confirmed
echo  This will build every target, tag !TAG!, and publish a release.
echo  Anything already committed is fine; uncommitted changes are not included.
echo.
choice /c Yn /n /t 20 /d N /m "  Publish? [Y/n] (20s) "
if errorlevel 2 (
    echo.
    echo  Cancelled. Nothing was built, tagged, or published.
    goto :eof
)
:confirmed

if "%SKIP_BUILD%"=="0" (
    call :ensure_go || exit /b 1
    echo  Building Windows and Linux targets ...
    echo.
    call :run_build "--os windows" %RATIO% || ( echo Build failed & exit /b 1 )
    call :run_build "--os linux"   %RATIO% || ( echo Build failed & exit /b 1 )
) else (
    echo  Skipping the build, packaging whatever is in %OUT%.
    echo.
)

call :package || exit /b 1

rem Notes are written before the dry-run exit so that --dry-run actually shows
rem what would be published. A dry run that hides the release notes is only
rem checking half of what it claims to check.
if not defined NOTES (
    set "NOTES=%OUT%\release-notes.md"
    call :write_notes
)

if "%DRY_RUN%"=="1" (
    echo.
    echo  Release notes that would be published ^(from %NOTES%^):
    echo  ------------------------------------------------------------------
    type "%NOTES%"
    call :print_footer
    echo  Dry run complete. Nothing was tagged or uploaded.
    echo  Run without --dry-run to publish.
    goto :eof
)

call :publish || exit /b 1
goto :eof

rem ---------------------------------------------------------------------------
rem  package - zip the Windows binaries, tar.gz the Linux ones, checksum them
rem ---------------------------------------------------------------------------
:package
set "STAGE=%OUT%\stage"
set "PKG=%OUT%\packages"

if exist "%STAGE%" rd /s /q "%STAGE%"
if exist "%PKG%"   rd /s /q "%PKG%"
mkdir "%STAGE%" >nul 2>&1
mkdir "%PKG%"   >nul 2>&1

set "ASSETS="
set "MISSING="
set "COUNT=0"

rem Windows: the archive holds prime.exe, not the build's platform-stamped name.
rem That is the name a user expects after unzipping, and it is what the
rem install scripts look for.
for %%F in ("%OUT%\prime-windows-*.exe") do (
    if not "%%~zF"=="0" (
        set "ARC=%%~nF"
        del /q "%STAGE%\prime.exe" >nul 2>&1
        copy /y "%%~fF" "%STAGE%\prime.exe" >nul
        call :add_readme "%STAGE%"
        powershell -NoProfile -Command "Compress-Archive -Path '%STAGE%\prime.exe' -DestinationPath '%PKG%\!ARC!.zip' -Force" >nul 2>&1
        if not exist "%PKG%\!ARC!.zip" (
            echo  ERROR: could not zip !ARC!
            exit /b 1
        )
        set "ASSETS=!ASSETS! %PKG%\!ARC!.zip"
        set /a COUNT+=1
    )
)

rem Linux: a static binary called prime inside a .tar.gz, which is what the
rem install scripts and the Arch package expect to unpack.
for %%F in ("%OUT%\prime-linux-*") do (
    if not "%%~zF"=="0" (
        set "ARC=%%~nF"
        del /q "%STAGE%\prime" >nul 2>&1
        copy /y "%%~fF" "%STAGE%\prime" >nul
        tar -czf "%PKG%\!ARC!.tar.gz" -C "%STAGE%" prime >nul 2>&1
        if not exist "%PKG%\!ARC!.tar.gz" (
            echo  ERROR: could not create the archive for !ARC!
            exit /b 1
        )
        set "ASSETS=!ASSETS! %PKG%\!ARC!.tar.gz"
        set /a COUNT+=1
    )
)

if "!COUNT!"=="0" (
    echo.
    echo ERROR: no binaries found in %OUT%.
    echo        Build first, or drop --skip-build.
    exit /b 1
)

echo.
echo  Checksums ...
rem The checksum file lives next to the archives so it is uploaded by bare
rem name and sits alongside them on the release page.
del /q "%PKG%\checksums.txt" >nul 2>&1
for %%A in ("%PKG%\*.zip" "%PKG%\*.tar.gz") do (
    for /f "usebackq delims=" %%h in (`powershell -NoProfile -Command "(Get-FileHash -Algorithm SHA256 -LiteralPath '%%~fA').Hash.ToLower()"`) do (
        >>"%PKG%\checksums.txt" echo %%h  %%~nxA
    )
)
set "ASSETS=!ASSETS! %PKG%\checksums.txt"

call :print_footer
echo  packages
rem for /f, not for-in: the PowerShell command has spaces in it and a plain
rem for-in would iterate over each word separately.
for %%A in ("%PKG%\*") do (
    for /f "usebackq delims=" %%S in (`powershell -NoProfile -Command "[string]::Format([cultureinfo]::InvariantCulture,'{0:N1}',((Get-Item -LiteralPath '%%~fA').Length/1MB))"`) do (
        echo    %%~nxA  %%S MB
    )
)
call :print_footer
exit /b 0

:add_readme
if exist "%ROOT%\README.md" if not exist "%~1\README.md" copy /y "%ROOT%\README.md" "%~1\" >nul
exit /b 0

rem ---------------------------------------------------------------------------
rem  publish - tag on the remote and create the GitHub release
rem ---------------------------------------------------------------------------
:publish
call :print_header "Publish"

where gh >nul 2>&1 || ( echo gh not found & exit /b 1 )
gh auth status >nul 2>&1 || ( echo ERROR: gh is not authenticated. Run: gh auth login & exit /b 1 )

if "%UPLOAD_LOCAL%"=="1" (
    rem Local mode deliberately does NOT push the tag.
    rem
    rem A tag push is a "push" event, and the Release workflow is triggered by
    rem exactly that: push, tags: v*. So pushing here would start a full CI
    rem release build on GitHub, burning Actions minutes, right when the whole
    reason for this mode is that the quota is gone or the release is urgent.
    rem Two builds racing over the same tag is worse: goreleaser would clobber
    rem the assets uploaded below.
    rem
    rem gh release create --target creates the tag through the API instead. An
    rem API-created ref does not emit a push event, so the workflow stays quiet
    rem and the release is entirely this machine's work.
    rem
    rem The local tag is still created so the working copy matches the release
    rem and a later build stamps the right version, but it is never pushed.
    if "%TAG_EXISTS%"=="0" (
        echo  Tagging %TAG% locally, not pushing it ...
        git tag -a "%TAG%" -m "%TAG%" !SHA!
        if errorlevel 1 ( echo ERROR: could not create the tag & exit /b 1 )
    ) else (
        echo  Tag %TAG% is already on the remote.
    )
) else (
    if "%TAG_EXISTS%"=="0" (
        rem The tag has to exist before the release can attach a target to it.
        echo  Tagging %TAG% at !SHA! ...
        git tag -a "%TAG%" -m "%TAG%" !SHA!
        if errorlevel 1 ( echo ERROR: could not create the tag & exit /b 1 )

        echo  Pushing the tag, which starts the Release workflow ...
        git push origin "%TAG%"
        if errorlevel 1 (
            echo ERROR: could not push the tag. Delete it locally with: git tag -d %TAG%
            exit /b 1
        )
    ) else (
        echo  Tag %TAG% is already on the remote.
    )
)


rem Two ways to get a release out. Pick with --upload-local.
rem
rem Default: push the tag and let the Release workflow build the release on
rem GitHub. This is the only path that produces the whole asset set, because
rem goreleaser is what produces the .deb, .rpm, .apk and Arch packages and it
rem cannot run on Windows. Use it when Actions minutes are available.
rem
rem --upload-local: nothing is built on GitHub at all. The tag is created
rem through the API rather than pushed, so no workflow runs, no Actions minutes
rem are spent, and what ships is exactly the binary built and tested on this
rem machine. The cost is the packages: goreleaser cannot run here, so the
rem release carries the zip and tar.gz archives and checksums but not
rem .deb/.rpm/.apk. Use it when the quota is exhausted or a fix has to go out
rem now.
if "%UPLOAD_LOCAL%"=="1" (
    set "GH_ARGS="
    if /I "%CMD%"=="draft" set "GH_ARGS=--draft"

    rem gh release create refuses to touch a release that already exists, and
    rem --clobber only replaces same-named assets. So an existing release gets
    rem an upload instead: that is what makes re-running after a partial
    rem failure finish the job rather than dead-end on the tag it already
    rem pushed.
    gh release view "%TAG%" >nul 2>&1
    if errorlevel 1 (
        echo.
        echo  Creating the release from the local binaries ...
        gh release create "%TAG%" --target !SHA! --title "%TAG%" --notes-file "%NOTES%" !GH_ARGS! !ASSETS!
    ) else (
        echo.
        echo  Release %TAG% exists, uploading the assets again ...
        gh release upload "%TAG%" --clobber !ASSETS!
        if not errorlevel 1 gh release edit "%TAG%" --notes-file "%NOTES%"
    )
    if errorlevel 1 (
        echo.
        echo ERROR: could not publish the release.
        echo        Re-run this script to retry the upload.
        exit /b 1
    )

    call :print_footer
    echo  Release: %TAG%
    for /f "usebackq delims=" %%u in (`gh release view "%TAG%" --json url --jq .url 2^>nul`) do echo           %%u
    echo.
    echo  Published from this machine. No GitHub Actions ran and no minutes
    echo  were spent: the tag was created through the API, which does not
    echo  trigger the Release workflow.
    echo.
    echo  Shipped: the Windows and Linux archives plus checksums.
    echo  Not shipped: .deb, .rpm, .apk and Arch packages. Those come from
    echo  goreleaser, which cannot run on Windows. Publish without
    echo  --upload-local to get them.
    exit /b 0
) else (
    echo.
    echo  Tag pushed. Handing the release to the Release workflow.
)

call :print_footer
echo  Release: %TAG%
for /f "usebackq delims=" %%u in (`gh release view "%TAG%" --json url --jq .url 2^>nul`) do echo           %%u
echo.
echo  The Release workflow is building the full asset set on GitHub: the
echo  packages (.deb, .rpm, .apk, Arch) that cannot be produced from Windows,
echo  plus completions and manpages. It takes about fifteen minutes.
echo.
echo  Watch it with:  gh run watch
goto :eof

rem ---------------------------------------------------------------------------
rem ---------------------------------------------------------------------------
rem  write_notes - build the release notes from the commits since the last tag.
rem
rem  Delegated to PowerShell on purpose. Doing this in batch means accumulating
rem  subjects into a variable, and batch has no newline escape: "`n" is three
rem  literal characters, so every section came out as one enormous line. Worse,
rem a commit subject containing & | < > > would be parsed as a shell operator by
rem  echo and silently corrupt the file. One PowerShell pass does the grouping,
rem  the escaping and the writing.
rem ---------------------------------------------------------------------------
:write_notes
powershell -NoProfile -ExecutionPolicy Bypass -File "%ROOT%\scripts\notes.ps1" ^
    -Repo "%ROOT%" -From "%LATEST_TAG%" -To "!SHA!" -Out "%NOTES%"
if errorlevel 1 (
    echo  WARNING: could not generate release notes, writing a placeholder.
    > "%NOTES%" echo ## Changes
    >>"%NOTES%" echo.
    >>"%NOTES%" echo Commit `!SHA!`.
)

if exist "%ROOT%\RELEASE_NOTES.md" (
    >>"%NOTES%" echo.
    type "%ROOT%\RELEASE_NOTES.md" >> "%NOTES%"
)
exit /b 0
rem ============================================================================
:usage
echo.
echo Usage: scripts\release.bat [command] [options]
echo.
echo   no command   the whole thing: build everything, tag, publish
echo   menu         pick a single step from a list
echo   build        build for this machine only
echo   run          build for this machine and launch it
echo   all          build every Windows and Linux target
echo   check        show the last release and the proposed version
echo   release      build, tag, and publish to GitHub
echo   draft        build, tag, and leave a draft release
echo   clean        delete the output directory
echo.
echo   --ratio ^<f^>        fraction of the machine to build with (default 0.35)
echo   --jobs ^<n^>         exact job count, overrides --ratio
echo   --out ^<dir^>        output directory (default dist)
echo   --version ^<x.y.z^>  use this version instead of last release + 1
echo   --notes ^<file^>     release notes file
echo   --test             run the test suite before packaging (slow)
echo   --yes              skip the confirmation prompt
echo   --dry-run          print what would happen, touch nothing remote
echo   --skip-build       package what is already in the output directory
echo   --upload-local     publish from this machine without GitHub Actions at all.
echo                      The tag is created through the API rather than pushed,
echo                      so no workflow runs and no Actions minutes are spent.
echo                      Ships exactly what was built and tested here. The
echo                      .deb / .rpm / .apk / Arch packages are absent, since
echo                      goreleaser cannot run on Windows.
echo.
echo   Publishing, two ways:
echo     default          push the tag, GitHub Actions builds the full set
echo                      including packages. Needs Actions minutes.
echo     --upload-local   build and upload here, no Actions. No packages.
echo                      For when the quota is gone or a fix is urgent.
echo.
popd
endlocal
exit /b 0
