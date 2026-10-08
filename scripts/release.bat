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
rem    sync      pull the newest sources from origin into this tree
rem    build     build for this machine only (throttled, quick)
rem    run       build for this machine and launch it so you can try it
rem    all       build every Windows and Linux target
rem    check     show the last release, the version that would be used, and
rem               the working tree state. Changes nothing.
rem    release   pick platforms in a sub-menu, auto-commit, build, tag,
rem               publish to GitHub, and verify the assets
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
rem    --skip-build     use whatever is already in the output directory:
rem                     "release" packages it, "run" launches it with no
rem                     compile - and so no Go toolchain needed
rem    --upload-local   publish from this machine without GitHub Actions
rem    --yes            skip the confirmation prompt
rem    --no-auto-commit do not commit the tree before the release build
rem    --no-verify      skip the post-publish asset check
rem    --targets <tok>... which platforms to build and ship. A platform name
rem                     (windows, linux, all) or exact arches
rem                     (windows/amd64, linux/arm64). Skips the pick menu.
rem                     With no --targets, a pick menu is shown before the
rem                     release starts.
rem
rem  Example
rem    scripts\release.bat
rem    scripts\release.bat sync
rem    scripts\release.bat run --ratio 0.25
rem    scripts\release.bat run --skip-build
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
set "STRICT_SYNC=0"
rem Commit the working tree before the release build. Default on; a generic
rem message so the same script is safe for whoever runs it. --no-auto-commit
rem turns it off. Skipped when the tree is already clean or in --dry-run.
set "AUTO_COMMIT=1"
rem Check that the published release carries every selected asset, and say so
rem plainly if one is missing. --no-verify turns it off.
set "VERIFY=1"
rem The targets the release ships. Always a list of os/arch tokens, e.g.
rem "windows/amd64 windows/arm64 linux/amd64 linux/arm64". Platform names are
rem expanded to their arches in the parser, so the build, package and verify
rem steps all work off one representation. Default ships the four-asset set.
set "TARGETS=windows/amd64 windows/arm64 linux/amd64 linux/arm64"
rem 1 when --targets came from the command line: the interactive pick menu
rem stays out of the way when the caller already decided.
set "TARGETS_SET=0"

set "ROOT=%~dp0.."
pushd "%ROOT%" >nul 2>&1 || ( echo Cannot enter %ROOT% & exit /b 1 )

rem ---- argument parsing -------------------------------------------------------
rem Each option jumps to a label so %%~1 is re-expanded after the shift: inside
rem a parenthesised block it is frozen at parse time, which silently shifts the
rem following argument into the wrong variable.
:parse
if "%~1"=="" goto parsed
if /I "%~1"=="sync"    ( set "CMD=sync"    & shift & goto parse )
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
if /I "%~1"=="--strict-sync" ( set "STRICT_SYNC=1" & shift & goto parse )
if /I "%~1"=="--dry-run" ( set "DRY_RUN=1" & shift & goto parse )
if /I "%~1"=="--skip-build" ( set "SKIP_BUILD=1" & shift & goto parse )
if /I "%~1"=="--no-auto-commit" ( set "AUTO_COMMIT=0" & shift & goto parse )
if /I "%~1"=="--no-verify" ( set "VERIFY=0" & shift & goto parse )
if /I "%~1"=="--targets" goto opt_targets
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

:opt_targets
rem A token list follows: "release --targets linux" ships the whole Linux
rem set, "release --targets linux/arm64 windows/amd64" ships exactly two
rem arches. A platform name expands to its arches so the build, package and
rem verify steps share one representation. The list stops at the next flag,
rem so "release --targets linux --dry-run" parses as two options.
rem
rem The one shift here drops "--targets" itself; the loop processes %1
rem before shifting it away, so the first value is not lost. %~1 cannot
rem carry a substring spec itself ("%~1:~0,1" is not valid), so the current
rem argument is copied into a plain variable before the first-character test.
set "TARGETS="
shift
:targets_loop
set "ARG=%~1"
if "!ARG!"=="" goto targets_done
if "!ARG:~0,1!"=="-" goto targets_done
rem Platform names expand to their full arch sets.
if /I "!ARG!"=="windows" ( set "TARGETS=!TARGETS! windows/amd64 windows/arm64" & shift & goto targets_loop )
if /I "!ARG!"=="linux"   ( set "TARGETS=!TARGETS! linux/amd64 linux/arm64"   & shift & goto targets_loop )
if /I "!ARG!"=="all"     ( set "TARGETS=windows/amd64 windows/arm64 linux/amd64 linux/arm64" & shift & goto targets_loop )
rem A single os/arch token is validated against the four the release ships,
rem then taken through unchanged.
if /I "!ARG!"=="windows/amd64"  ( set "TARGETS=!TARGETS! windows/amd64"  & shift & goto targets_loop )
if /I "!ARG!"=="windows/arm64"  ( set "TARGETS=!TARGETS! windows/arm64"  & shift & goto targets_loop )
if /I "!ARG!"=="linux/amd64"    ( set "TARGETS=!TARGETS! linux/amd64"    & shift & goto targets_loop )
if /I "!ARG!"=="linux/arm64"    ( set "TARGETS=!TARGETS! linux/arm64"    & shift & goto targets_loop )
echo Unsupported --targets value: !ARG! ^(use windows, linux, all, or os/arch^)
goto usage
:targets_done
if not defined TARGETS ( echo --targets needs at least one platform & goto usage )
set "TARGETS_SET=1"
goto parse

:opt_notes
shift
if "%~1"=="" ( echo --notes needs a file & goto usage )
set "NOTES=%~1"
shift
goto parse

:parsed
if "%STRICT_SYNC%"=="1" set "CMD=sync"

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
if /I "%CMD%"=="sync"   goto do_sync
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
echo   s  sync       pull the newest sources from origin
echo   1  build      build for this machine
echo   2  run        build for this machine and launch it
echo   3  all        build every Windows and Linux target
echo   4  check      show the last release and the proposed version
echo   5  release    build, tag, and publish to GitHub
echo   6  draft      build, tag, and leave a draft release
echo   7  clean      delete %OUT%
echo   q  quit
echo  ------------------------------------------------------------------
choice /c 1234567qs /n /m "  Pick: "
if errorlevel 9 ( set "CMD=sync"   & goto do_sync )
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
rem git remote exits 0 for an empty list - it lists remotes, and an empty list
rem is a successful listing - so the natural "|| error" form never fired and a
rem repository with no remote at all passed this check and failed much later,
rem when something actually tried to push. The check asks about origin by name
rem because that is the remote every publish step below uses.
git remote get-url origin >nul 2>&1 || (
    echo.
    echo ERROR: this repository has no "origin" remote, so nothing could be
    echo        published. Add one with:
    echo          git remote add origin https://github.com/OWNER/REPO.git
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
rem
rem A leading v is stripped so callers can pass either form, and only the first
rem character is taken: deleting every v would turn 1.2.3-v2 into 1.2.3-2, a
rem tag nobody asked for that matches nothing else in the scheme.
rem
rem The strip lives outside a parenthesized block on purpose. cmd reads a block
rem in full before it evaluates "if defined", and expanding a substring of a
rem variable that is not set yet is a syntax error - so the guard meant to make
rem it safe does not protect the expansion written in the same block. Without
rem this shape the whole script died with "command syntax is incorrect" on
rem every run that did not pass --version.
:resolve_version
if not defined VERSION goto resolve_version_auto
if "%VERSION:~0,1%"=="v" set "VERSION=%VERSION:~1%"
exit /b 0

:resolve_version_auto
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
rem  sync
rem
rem  Pull the newest sources from origin, because nothing else here does.
rem
rem  build.bat compiles whatever is in the working tree, and release.bat tags
rem  whatever HEAD happens to point at, so a build made after someone else
rem  pushed was stale by default. The only hint was the dirty-tree line in
rem  "check", which says nothing at all about being behind the remote.
rem
rem  Two rules keep it from ever costing work:
rem    - a dirty tree is never pulled, only reported on;
rem    - the pull is --ff-only, so git either moves HEAD straight forward or
rem      refuses outright. A branch that has diverged is left alone: working
rem      out which side of the split wins is a person's decision, not a build
rem      script's.
rem
rem  A failed fetch is a warning rather than an error. The point of the
rem  command is to end up building the newest sources available, and with no
rem  network the newest sources available are the ones already here - refusing
rem  to build because the internet is down would be worse than saying so.
rem ============================================================================
:do_sync
call :ensure_git || exit /b 1
call :ensure_repo || exit /b 1
call :print_header "Sync sources"

set "DIRTY="
for /f "usebackq tokens=*" %%b in (`git status --porcelain`) do set "DIRTY=1"

echo  Fetching origin ...
git fetch origin --prune >nul 2>&1
if errorlevel 1 (
    echo.
    if "%STRICT_SYNC%"=="1" (
        echo  ERROR: could not fetch from origin. Release stopped.
        exit /b 1
    )
    echo  WARNING: could not fetch from origin - offline, or no remote.
    echo           Continuing with the sources that are already here.
    goto sync_done
)

call :rel_branch
if not defined BRANCH (
    echo  HEAD is detached, so there is no branch to fast-forward to.
    if "%STRICT_SYNC%"=="1" exit /b 1
    goto sync_done
)

git rev-parse --verify --quiet "origin/%BRANCH%" >nul 2>&1
if errorlevel 1 (
    echo  The remote has no "%BRANCH%" branch - nothing to fast-forward to.
    if "%STRICT_SYNC%"=="1" exit /b 1
    goto sync_done
)

set "BEHIND="
set "AHEAD="
for /f "usebackq delims=" %%n in (`git rev-list --count HEAD..origin/%BRANCH%`) do set "BEHIND=%%n"
for /f "usebackq delims=" %%n in (`git rev-list --count origin/%BRANCH%..HEAD`) do set "AHEAD=%%n"

if "%STRICT_SYNC%"=="1" if not defined AHEAD (
    echo  ERROR: could not compare sources with origin/%BRANCH%.
    exit /b 1
)

if not defined BEHIND (
    if "%STRICT_SYNC%"=="1" (
        echo  ERROR: could not compare sources with origin/%BRANCH%.
        exit /b 1
    )
    echo  WARNING: could not read how far behind origin/%BRANCH% this is.
    echo           Not pulling. Continuing with the sources that are here.
    goto sync_done
)

if "%BEHIND%"=="0" (
    echo  Already up to date with origin/%BRANCH%.
    goto sync_done
)

echo  origin/%BRANCH% is ahead of this tree by %BEHIND% commit^(s^).

if defined DIRTY (
    echo.
    if "%STRICT_SYNC%"=="1" (
        echo  ERROR: commit or stash local changes before syncing the release.
        exit /b 1
    )
    echo  WARNING: the working tree has uncommitted changes - not pulling.
    echo           Building HEAD as it stands, %BEHIND% commit^(s^) behind.
    goto sync_done
)

if not "%AHEAD%"=="0" (
    if "%STRICT_SYNC%"=="1" (
        echo  ERROR: local and remote branches have diverged. Release stopped.
        exit /b 1
    )
    echo  This tree also holds %AHEAD% commit^(s^) the remote does not, so a
    echo  fast-forward is impossible. Leaving it alone - compare them with
    echo    git log --oneline --graph --all
    goto sync_done
)

if "%DRY_RUN%"=="1" (
    echo  Dry run - would fast-forward this tree to origin/%BRANCH%.
    goto sync_done
)

echo  Fast-forwarding to origin/%BRANCH% ...
git pull --ff-only origin "%BRANCH%"
if errorlevel 1 (
    echo.
    echo  ERROR: the fast-forward was refused. Nothing was changed.
    echo         Look at it with:  git status
    exit /b 1
)
echo  Now at origin/%BRANCH%.

:sync_done
if defined DIRTY set "DIRTY="
call :print_footer
exit /b 0

rem rel_branch leaves BRANCH holding the checked-out branch name, or empty
rem when HEAD is detached. rev-parse --abbrev-ref reports the literal word
rem "HEAD" for a detached checkout, and without dropping it here every git
rem call above would be pointed at an "origin/HEAD" that means something else
rem entirely.
:rel_branch
set "BRANCH="
for /f "usebackq delims=" %%b in (`git rev-parse --abbrev-ref HEAD 2^>nul`) do set "BRANCH=%%b"
if /I "%BRANCH%"=="HEAD" set "BRANCH="
exit /b 0

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
rem
rem  Builds for this machine and starts the app. With --skip-build it starts
rem  whatever is already in the output directory instead, which is also what
rem  makes a Go toolchain unnecessary for this path.
rem ============================================================================
:do_run
if "%SKIP_BUILD%"=="1" goto do_run_existing
call :ensure_go || exit /b 1
call :print_header "Build and run"
echo  Building the native binary, then launching it.
call :run_build "" %RATIO% || exit /b 1
goto do_run_launch

:do_run_existing
call :print_header "Run the existing build"
echo  mode   : no compile, launching whatever is already in %OUT%

:do_run_launch
echo  Close the app with ctrl+c to come back here.
call :print_footer

if not exist "%OUT%\prime.exe" (
    echo.
    echo ERROR: %OUT%\prime.exe is not in %OUT%.
    echo        Build it first:  scripts\release.bat build
    exit /b 1
)

echo.
echo  prime reports:
"%OUT%\prime.exe" --version
echo.
echo  Starting %OUT%\prime.exe ...
echo  Close it to come back here.
echo.
rem Deliberately no arguments of its own.
rem
rem %* used to be forwarded here, and it could not have worked: %* is the whole
rem command line and shift does not change it, so it still held the "run" that
rem selected this branch - which is prime's own subcommand for a non-interactive
rem prompt, and sits waiting on stdin rather than opening the app. With a ratio
rem option on the command line it did not even get that far: prime answered
rem "Unknown flag: --ratio". Flags for prime itself are passed through the
rem root launcher, Prime.bat: "Prime.bat build -- --continue".
"%OUT%\prime.exe"
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
for %%T in (!TARGETS!) do call :build_one "--target %%T" || ( echo Build failed & exit /b 1 )
goto :eof

rem ---------------------------------------------------------------------------
rem  build_one <build.bat-extra-args>
rem
rem  A throttled build.bat run for one target, e.g. "--target linux/amd64".
rem  The throttle comes from build.bat so it stays in one place.
rem ---------------------------------------------------------------------------
:build_one
call :run_build "%~1" %RATIO%
exit /b %errorlevel%

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
rem Reported before anything reads the repository. Without this the fields
rem below were filled from commands that had all failed: git status printed
rem "fatal: not a git repository" straight to the console, the tree was then
rem called clean because the command had produced no output, and version
rem resolution fell back to v0.0.0 and offered v0.0.1 as the next release.
rem A check that answers questions nobody can use is worse than one that
rem refuses to answer.
call :ensure_repo || exit /b 1
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
  echo NOTE: the working tree has uncommitted changes.
  if "!DRY_RUN!"=="1" (
      echo       A dry run never commits: nothing below will change the tree.
  ) else (
      echo       They will be auto-committed after confirmation unless
      echo       --no-auto-commit was given.
  )
  echo.
)

call :pick_targets || exit /b 1

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
if "!DRY_RUN!"=="1" goto confirmed
if "!ASSUME_YES!"=="1" goto confirmed
echo  This will build the selected targets, tag !TAG!, and publish a release.
echo  Uncommitted changes are auto-committed before the build.
echo.
choice /c Yn /n /t 20 /d N /m "  Publish? [Y/n] (20s) "
if errorlevel 2 (
    echo.
    echo  Cancelled. Nothing was built, tagged, or published.
    goto :eof
)
:confirmed

rem The tree is committed only after the user confirmed the publish: a
rem cancelled release leaves the working tree exactly as it was found.
rem Tracked changes are staged; untracked files are the owner's decision.
rem The generic author is a fallback for machines without a git identity -
rem machines with one keep their own.
if defined DIRTY if "!AUTO_COMMIT!"=="1" if "!DRY_RUN!"=="0" (
    echo  Committing local changes ...
    git add -u
    git config user.name >nul 2>&1
    if errorlevel 1 (
        git -c user.name="Prime Release" -c user.email="release@prime.local" ^
            commit -m "Auto commit: prepare release"
    ) else (
        git commit -m "Auto commit: prepare release"
    )
    if errorlevel 1 (
        echo.
        echo ERROR: could not commit the local changes. Resolve them and re-run.
        exit /b 1
    )
    call :current_sha
    echo  Committed !SHA!. The release is built from this commit.
    echo.
)
if defined DIRTY set "DIRTY="

if "%SKIP_BUILD%"=="0" (
    call :ensure_go || exit /b 1
    echo  Building the selected targets: !TARGETS!
    echo.
    for %%T in (!TARGETS!) do call :build_one "--target %%T" || ( echo Build failed & exit /b 1 )
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

rem The publish step itself says what happened; the asset check then proves
rem the release page actually carries what was selected, and names anything
rem that is absent.
if "!VERIFY!"=="1" call :verify_assets
goto :eof

rem ---------------------------------------------------------------------------
rem  package - one archive per selected target, then checksums
rem
rem  Archive names follow the Release workflow's naming
rem  (prime_<ver>_<OS>_<arch>.<ext>): local uploads and goreleaser assets
rem  are interchangeable, and the install scripts look for exactly this.
rem ---------------------------------------------------------------------------
:package
set "STAGE=%OUT%\stage"
set "PKG=%OUT%\packages"
set "VERSION_BARE=%VERSION%"
set "VERSION_BARE=%VERSION_BARE:v=%"

rem A narrower --targets must not pick up binaries of a platform that was
rem built earlier and is no longer selected: wipe the binary groups that are
rem out of scope before the loops below walk them.
for %%P in (windows linux) do (
    set "IN=0"
    for %%T in (%TARGETS%) do for /f "tokens=1 delims=/" %%Q in ("%%T") do if /I "%%Q"=="%%P" set "IN=1"
    if "!IN!"=="0" del /q "%OUT%\prime-%%P-*" >nul 2>&1
)

rem Binaries the old wide target lists used to build: 32-bit and ARMv7. A
rem release is the slim four-asset set, so leftovers from an earlier wide
rem build must not ride along into the packages.
del /q "%OUT%\prime-windows-386.exe" >nul 2>&1
del /q "%OUT%\prime-linux-386"       >nul 2>&1
del /q "%OUT%\prime-linux-arm"       >nul 2>&1

if exist "%STAGE%" rd /s /q "%STAGE%"
if exist "%PKG%"   rd /s /q "%PKG%"
mkdir "%STAGE%" >nul 2>&1
mkdir "%PKG%"   >nul 2>&1

set "ASSETS="
set "MISSING="
set "COUNT=0"

for %%T in (!TARGETS!) do (
    call :package_one %%T
    if errorlevel 1 exit /b 1
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
rem
rem .NET SHA256 is computed through PowerShell because it is present on every
rem machine, even ones where the Get-FileHash cmdlet has been stripped. A
rem certutil fallback covers a box with no PowerShell at all: certutil is an
rem operating system binary, and its one space-free output line is the hash.
del /q "%PKG%\checksums.txt" >nul 2>&1
for %%A in ("%PKG%\*.zip" "%PKG%\*.tar.gz") do (
    for /f "usebackq delims=" %%h in (`powershell -NoProfile -Command "$b=[System.IO.File]::ReadAllBytes('%%~fA');$s=[System.Security.Cryptography.SHA256]::Create();($s.ComputeHash($b)|ForEach-Object{($_.ToString('x'))}) -join ''"`) do (
        >>"%PKG%\checksums.txt" echo %%h  %%~nxA
    )
    if not exist "%PKG%\checksums.txt" for /f "tokens=*" %%h in ('certutil -hashfile "%%~fA" SHA256 2^>nul ^| findstr /v " "') do >>"%PKG%\checksums.txt" echo %%h  %%~nxA
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

rem ---------------------------------------------------------------------------
rem  package_one <os/arch> - archive one target's binary
rem
rem  The archive is named the way the Release workflow's goreleaser names it,
rem  so an --upload-local release and a CI release carry identical asset sets.
rem  A failure here aborts the whole release: half-packed archives would ship
rem  a release some install script 404s on.
rem ---------------------------------------------------------------------------
:package_one
set "OS_L="
set "ARC_L="
for /f "tokens=1,2 delims=/" %%P in ("%~1") do (
    set "OS_L=%%P"
    set "ARC_L=%%Q"
)
if not defined OS_L (
    echo  ERROR: cannot parse target %~1
    exit /b 1
)

rem goreleaser naming: capitalized OS, x86_64 for amd64, zip on Windows,
rem tar.gz on Linux.
set "OS_UP="
set "ARC_UP=!ARC_L!"
set "EXT=tar.gz"
if /I "!ARC_L!"=="amd64" set "ARC_UP=x86_64"
if /I "!OS_L!"=="windows" (
    set "OS_UP=Windows"
    set "EXT=zip"
) else (
    set "OS_UP=Linux"
)
set "NAME=prime_%VERSION_BARE%_!OS_UP!_!ARC_UP!.!EXT!"
set "BIN=%OUT%\prime-!OS_L!-!ARC_L!"
if /I "!OS_L!"=="windows" set "BIN=!BIN!.exe"
if not exist "!BIN!" (
    echo  ERROR: !BIN! is not in %OUT%.
    echo         Build the targets first, or drop --skip-build.
    exit /b 1
)

rem The archive holds the plain name prime(.exe) plus the README, which is
rem what the install scripts and the Arch package expect to unpack.
del /q "%STAGE%\prime" >nul 2>&1
del /q "%STAGE%\prime.exe" >nul 2>&1

if /I "!OS_L!"=="windows" (
    copy /y "!BIN!" "%STAGE%\prime.exe" >nul
    call :add_readme "%STAGE%"
    call :make_zip "%STAGE%" "%PKG%\!NAME!" || exit /b 1
) else (
    copy /y "!BIN!" "%STAGE%\prime" >nul
    tar -czf "%PKG%\!NAME!" -C "%STAGE%" prime >nul 2>&1
)
if not exist "%PKG%\!NAME!" (
    echo  ERROR: could not create the archive for !NAME!
    exit /b 1
)
set "ASSETS=!ASSETS! %PKG%\!NAME!"
set /a COUNT+=1
exit /b 0

rem ---------------------------------------------------------------------------
rem  make_zip <stagedir> <out.zip> - zip every file in the stage directory
rem
rem  PowerShell's Compress-Archive is the primary producer because it is the
rem  reference zip layout; but the Archive module is an operating system
rem  feature and stripped images load it not at all, so bsdtar stands by.
rem  create_zip_by_extension is its universal mode: .zip out, zip in.
rem ---------------------------------------------------------------------------
:make_zip
set "STG=%~1"
set "ZOUT=%~2"
if exist "!ZOUT!" del /q "!ZOUT!" >nul 2>&1
powershell -NoProfile -Command "Compress-Archive -Path '!STG!\*' -DestinationPath '!ZOUT!' -Force" >nul 2>&1
if not exist "!ZOUT!" (
    rem bsdtar's auto-format mode names the archive by extension, so a .zip
    rem in produces a .zip out. bsdtar does no wildcarding, so the members
    rem are listed explicitly; the stage holds the binary and, when the repo
    rem root has one, the README.
    set "MEMBERS=prime.exe"
    if exist "!STG!\README.md" set "MEMBERS=!MEMBERS! README.md"
    tar -a -cf "!ZOUT!" -C "!STG!" !MEMBERS! 2>nul
)
if not exist "!ZOUT!" exit /b 1
exit /b 0

:add_readme
if exist "%ROOT%\README.md" if not exist "%~1\README.md" copy /y "%ROOT%\README.md" "%~1\" >nul
exit /b 0

rem ---------------------------------------------------------------------------
rem  pick_targets - the interactive target sub-menu.
rem
rem  Shown only when the caller did not pin the set on the command line
rem  (TARGETS_SET stays 0). The pick writes back to TARGETS, the same variable
rem  the build, package and verify steps all read, so one representation flows
rem  through every later step. q cancels out of the whole release.
rem ---------------------------------------------------------------------------
:pick_targets
if "!TARGETS_SET!"=="1" goto pick_targets_done
echo.
echo  Which targets ship in this release?
echo  ------------------------------------------------------------------
echo   1  All        Windows + Linux, amd64 + arm64
echo   2  Windows    Windows only (amd64 + arm64)
echo   3  Linux      Linux only (amd64 + arm64)
echo   4  Selective  tick the exact arches
echo   q  Cancel     stop the release, build nothing
echo  ------------------------------------------------------------------
set "PICK="
choice /c 1234q /n /m "  Pick: "
if errorlevel 5 goto pick_cancel
if errorlevel 4 goto pick_selective
if errorlevel 3 set "TARGETS=linux/amd64 linux/arm64" & set "TARGETS_SET=1" & goto pick_targets_done
if errorlevel 2 set "TARGETS=windows/amd64 windows/arm64" & set "TARGETS_SET=1" & goto pick_targets_done
if errorlevel 1 set "TARGETS=windows/amd64 windows/arm64 linux/amd64 linux/arm64" & set "TARGETS_SET=1" & goto pick_targets_done

:pick_selective
set "SEL_WAMD="
set "SEL_WARM="
set "SEL_LAMD="
set "SEL_LARM="
echo.
echo  Tick the arches to ship. An unticked arch ships nothing.
:selective_loop
echo.
echo   [!SEL_WAMD!]  1  windows/amd64
echo   [!SEL_WARM!]  2  windows/arm64
echo   [!SEL_LAMD!]  3  linux/amd64
echo   [!SEL_LARM!]  4  linux/arm64
echo.
rem choice /c 1234d maps: 1 -^> el 1, 2 -^> el 2, 3 -^> el 3, 4 -^> el 4, d -^> el 5.
rem Every toggle jumps to a label on its own: an if inside a parenthesized
rem block freezes its delayed references at parse time, so the toggle would
rem clear the variable it just set.
choice /c 1234d /n /m "  Toggle an arch, d to finish: "
if errorlevel 5 goto selective_finish
if errorlevel 4 goto sel_toggle_larm
if errorlevel 3 goto sel_toggle_lamd
if errorlevel 2 goto sel_toggle_warm
goto sel_toggle_wamd

:sel_toggle_wamd
if "!SEL_WAMD!"=="" ( set "SEL_WAMD=1" ) else ( set "SEL_WAMD=" )
goto selective_loop
:sel_toggle_warm
if "!SEL_WARM!"=="" ( set "SEL_WARM=1" ) else ( set "SEL_WARM=" )
goto selective_loop
:sel_toggle_lamd
if "!SEL_LAMD!"=="" ( set "SEL_LAMD=1" ) else ( set "SEL_LAMD=" )
goto selective_loop
:sel_toggle_larm
if "!SEL_LARM!"=="" ( set "SEL_LARM=1" ) else ( set "SEL_LARM=" )
goto selective_loop

:selective_finish
set "TARGETS="
if defined SEL_WAMD set "TARGETS=!TARGETS! windows/amd64"
if defined SEL_WARM set "TARGETS=!TARGETS! windows/arm64"
if defined SEL_LAMD set "TARGETS=!TARGETS! linux/amd64"
if defined SEL_LARM set "TARGETS=!TARGETS! linux/arm64"
if not defined TARGETS (
    echo.
    echo  No arch ticked - nothing to ship.
    goto pick_cancel
)
set "TARGETS_SET=1"
goto pick_targets_done

:pick_cancel
echo.
echo  Release cancelled. Nothing was built, tagged, or published.
exit /b 1

:pick_targets_done
exit /b 0

rem ---------------------------------------------------------------------------
rem  verify_assets - prove the published release carries every selected asset.
rem
rem  The default publish path hands the build to the Release workflow, so the
rem  assets are not on the page when the tag is pushed: goreleaser takes about
rem  fifteen minutes. This reads the asset list back and polls it while the
rem  workflow runs, then names anything still absent, so a build that quietly
rem  skipped an arch does not ship a release whose install script 404s. A
rem  missing asset is reported, not fatal.
rem ---------------------------------------------------------------------------
:verify_assets
set "VERSION_BARE=%VERSION%"
set "VERSION_BARE=%VERSION_BARE:v=%"
rem Upload-local assets are on the page the moment :publish returns, so no
rem polling there. The CI path gives the workflow up to 40 x 30s = 20 minutes.
if "!UPLOAD_LOCAL!"=="0" set "POLL=40" else set "POLL=0"
echo.
echo  Verifying the assets on %TAG% ...
:verify_wait
call :verify_fetch
set "MISSING_COUNT=0"
for %%T in (!TARGETS!) do (
    for /f "tokens=1,2 delims=/" %%P in ("%%T") do (
        set "WANT="
        set "FOUND=0"
        call :asset_name %%T
        findstr /i /c:"!WANT!" "%ASSET_LIST%" >nul 2>&1 && set "FOUND=1"
        if not "!FOUND!"=="1" set /a MISSING_COUNT+=1
    )
)
if "!MISSING_COUNT!"=="0" goto verify_report
if "!POLL!"=="0" goto verify_report
set /a POLL-=1
echo  !MISSING_COUNT! asset^(s^) not on the page yet - the workflow is still building.
echo  Checking again in 30 seconds ^(!POLL! checks left^)...
timeout /t 30 /nobreak >nul
goto verify_wait

:verify_report
call :verify_fetch
if not exist "%ASSET_LIST%" (
    echo  WARNING: could not read the asset list for %TAG%.
    echo          Check it by hand:  gh release view %TAG%
    del /q "%ASSET_LIST%" >nul 2>&1
    exit /b 0
)
echo  ------------------------------------------------------------------
set "MISSING_COUNT=0"
for %%T in (!TARGETS!) do (
    for /f "tokens=1,2 delims=/" %%P in ("%%T") do (
        rem call resets errorlevel, so capture the findstr result into a
        rem flag before the next command can clobber it.
        set "WANT="
        set "FOUND=0"
        call :asset_name %%T
        findstr /i /c:"!WANT!" "%ASSET_LIST%" >nul 2>&1 && set "FOUND=1"
        if "!FOUND!"=="1" (
            echo   [ok]      !WANT!
        ) else (
            echo   [MISSING] !WANT!
            set /a MISSING_COUNT+=1
        )
    )
)
del /q "%ASSET_LIST%" >nul 2>&1
echo  ------------------------------------------------------------------
if "!MISSING_COUNT!"=="0" (
    echo  All selected assets are present on %TAG%.
) else (
    echo  !MISSING_COUNT! asset^(s^) still not on the release page.
    echo  Check the workflow:  gh run list --workflow Release
    echo  then the release:    gh release view %TAG%
)
exit /b 0

rem ---------------------------------------------------------------------------
rem  asset_name <os/arch> - leave WANT holding the release asset name for that
rem  target. Mirrors the naming the install scripts and "prime update" look
rem  for: capitalized OS, x86_64 for amd64, zip on Windows, tar.gz on Linux.
rem  A subroutine so the inner for block stays flat: setting several
rem  variables off one token and reading them back only works at statement
rem  level with delayed expansion, and the per-arch mapping is reusable.
rem ---------------------------------------------------------------------------
:asset_name
set "OS_UP="
set "ARC_UP="
set "EXT=tar.gz"
set "WANT="
for /f "tokens=1,2 delims=/" %%P in ("%~1") do (
    set "ARC_UP=%%Q"
    if /I "%%P"=="windows" (
        set "OS_UP=Windows"
        set "EXT=zip"
    ) else (
        set "OS_UP=Linux"
    )
)
if /I "!ARC_UP!"=="amd64" set "ARC_UP=x86_64"
set "WANT=prime_!VERSION_BARE!_!OS_UP!_!ARC_UP!.!EXT!"
exit /b 0

rem ---------------------------------------------------------------------------
rem  verify_fetch - read the asset names of the release into %ASSET_LIST%.
rem  An empty or missing file reads as "nothing uploaded yet", which is the
rem  expected state while the Release workflow is running.
rem ---------------------------------------------------------------------------
:verify_fetch
set "ASSET_LIST=%OUT%\assets.txt"
del /q "%ASSET_LIST%" >nul 2>&1
gh release view "%TAG%" --json assets --jq ".assets[].name" 2^>nul > "%ASSET_LIST%"
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
rem GitHub. goreleaser adds the completions and manpages into each archive
rem and stamps the checksums, which is what a full publish carries. Use it
rem when Actions minutes are available.
rem
rem --upload-local: nothing is built on GitHub at all. The tag is created
rem through the API rather than pushed, so no workflow runs, no Actions
rem minutes are spent, and what ships is exactly the binary built and tested
rem on this machine. The cost: the release carries the zip and tar.gz archives
rem plus checksums, without the completions and manpages that goreleaser
rem bundles in. Use it when the quota is exhausted or a fix has to go out
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
    echo  Shipped: the selected archives plus checksums.
    echo  The Release workflow would also add the completions and manpages
    echo  inside each archive. Publish without --upload-local for the
    echo  full CI set.
    exit /b 0
) else (
    echo.
    echo  Tag pushed. Handing the release to the Release workflow.
)

call :print_footer
echo  Release: %TAG%
for /f "usebackq delims=" %%u in (`gh release view "%TAG%" --json url --jq .url 2^>nul`) do echo           %%u
echo.
echo  The Release workflow builds the selected archive set on GitHub - the
echo  four-asset trim of Windows and Linux, amd64 and arm64 - plus the
echo  completions and manpages bundled into each archive. About fifteen minutes.
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
echo   sync         pull the newest sources from origin into this tree
echo   build        build for this machine only
echo   run          build for this machine and launch it
echo   all          build every Windows and Linux target
echo   check        show the last release and the proposed version
echo   release      pick targets, build, auto-commit, tag, publish to
echo                GitHub, then verify the assets landed
echo   draft        same as release but leaves a draft for review
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
echo   --skip-build       release: package what is already in the output
echo                      directory; run: launch it without compiling, and
echo                      without needing the Go toolchain
echo   --no-auto-commit   do not commit the tree before the release build
echo   --no-verify        skip the post-publish asset check
echo   --targets ^<tok^>... ship a chosen set: a platform name (windows, linux,
echo                      all) or exact arches (windows/amd64, linux/arm64).
echo                      Skips the target pick menu, which is otherwise
echo                      shown before a release starts.
echo   --upload-local   publish from this machine without GitHub Actions at all.
echo                    The tag is created through the API rather than pushed,
echo                    so no workflow runs and no Actions minutes are spent.
echo                    Ships the archives and checksums built here, without
echo                    the completions and manpages the CI build bundles in.
echo.
echo   Publishing, two ways:
echo     default          push the tag, GitHub Actions builds the archive set
echo                      with completions and manpages. Needs Actions minutes.
echo     --upload-local   build and upload here, no Actions. No CI extras.
echo                      For when the quota is gone or a fix is urgent.
echo.
popd
endlocal
exit /b 0
