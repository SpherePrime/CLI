<#
.SYNOPSIS
    Builds release notes for a Prime release from the commits since the last tag.

.DESCRIPTION
    Grouped by conventional-commit prefix, with everything unclaimed collected
    under "Other changes" so no commit is silently dropped.

    This is a PowerShell script rather than a batch one on purpose. The
    release.bat version accumulated subjects into a variable and joined them
    with "`n", which batch does not interpret as a newline, so every section
    collapsed onto one enormous line. Subjects containing & | < or > were then
    parsed as shell operators by echo and quietly corrupted the file.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string] $Repo,
    [string] $From = '',
    [string] $To = '',
    [Parameter(Mandatory = $true)] [string] $Out
)

$ErrorActionPreference = 'Stop'

function Invoke-Git {
    param([string[]] $GitArgs)
    $out = & git -C $Repo @GitArgs 2>$null
    if ($LASTEXITCODE -ne 0) { return @() }
    return @($out)
}

# The range starts at the last release, which the caller already knows: it read
# it from GitHub. Only fall back to searching when there is nothing to start
# from. Walking backwards past $From and using the tag before it would drop
# every commit made since the previous release, which is exactly the set the
# notes are for.
function Get-RangeStart {
    param([string] $Head)

    if ($From) {
        $known = Invoke-Git @('tag', '--list', $From)
        if ($known.Count -gt 0) { return $From }
    }

    $tags = Invoke-Git @('tag', '--merged', $Head, '--sort=-v:refname')
    foreach ($t in $tags) {
        $t = $t.Trim()
        if ($t) { return $t }
    }
    return ''
}

$sha = if ($To) { $To } else { (Invoke-Git @('rev-parse', 'HEAD'))[0] }
$previous = Get-RangeStart -Head $sha

$rangeArgs = if ($previous) { @('log', "$previous..$sha", '--no-merges', '--pretty=format:%s') }
             else { @('log', $sha, '--no-merges', '--pretty=format:%s') }

$subjects = @(Invoke-Git $rangeArgs |
    ForEach-Object { $_.Trim() } |
    Where-Object { $_ })

# Order matters: first match wins, so the specific prefixes come before the
# catch-all.
$categories = [ordered]@{
    'feat:'     = 'Features'
    'fix:'      = 'Fixes'
    'perf:'     = 'Performance'
    'refactor:' = 'Refactoring'
    'add:'      = 'Added'
    'docs:'     = 'Documentation'
    'test:'     = 'Tests'
    'ci:'       = 'CI'
    'build:'    = 'Build'
    'chore:'    = 'Chores'
    'style:'    = 'Style'
}

$buckets = [ordered]@{}
foreach ($key in $categories.Keys) { $buckets[$key] = [System.Collections.Generic.List[string]]::new() }
$other = [System.Collections.Generic.List[string]]::new()

foreach ($subject in $subjects) {
    $placed = $false
    foreach ($prefix in $categories.Keys) {
        if ($subject.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
            $buckets[$prefix].Add($subject.Substring($prefix.Length).Trim())
            $placed = $true
            break
        }
    }
    if (-not $placed) { $other.Add($subject) }
}

$lines = [System.Collections.Generic.List[string]]::new()
$lines.Add('## Changes')
$lines.Add('')
if ($previous) {
    $lines.Add("Everything since ``$previous``.")
} else {
    $lines.Add('The first release this script tracked.')
}
$lines.Add('')

foreach ($prefix in $categories.Keys) {
    $items = $buckets[$prefix]
    if ($items.Count -eq 0) { continue }
    $lines.Add("### $($categories[$prefix])")
    $lines.Add('')
    foreach ($item in $items) { $lines.Add("- $item") }
    $lines.Add('')
}

if ($other.Count -gt 0) {
    $lines.Add('### Other changes')
    $lines.Add('')
    foreach ($item in $other) { $lines.Add("- $item") }
    $lines.Add('')
}

$lines.Add('Download the archive for your platform and verify it against `checksums.txt`.')

# Written with UTF8 and no BOM. A BOM would end up in the middle of the GitHub
# release body as a stray character on the first line.
$dir = Split-Path -Parent $Out
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
[System.IO.File]::WriteAllText($Out, ($lines -join "`r`n"), (New-Object System.Text.UTF8Encoding($false)))

Write-Host "  notes: $($subjects.Count) commits since $(if ($previous) { $previous } else { 'the beginning' })"
