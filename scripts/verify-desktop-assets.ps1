param(
    [Parameter(Mandatory = $true)][string]$AssetList,
    [Parameter(Mandatory = $true)][string]$Version,
    [switch]$Report
)
$assets = @(Get-Content -LiteralPath $AssetList -ErrorAction SilentlyContinue)
$escapedVersion = [regex]::Escape($Version)
$windows = @($assets | Where-Object { $_ -match "_${escapedVersion}_x64-setup\.exe$" -and "${_}.sig" -in $assets })
$appimage = @($assets | Where-Object { $_ -match "[_-]${escapedVersion}[_-].*\.AppImage$" -and "${_}.sig" -in $assets })
$deb = @($assets | Where-Object { $_ -match "[_-]${escapedVersion}[_-].*\.deb$" })
$rpm = @($assets | Where-Object { $_ -match "[_-]${escapedVersion}[-_.].*\.rpm$" })
$missing = @()
if ("PrimeDesktop-Setup-$Version.exe" -notin $assets) { $missing += 'Prime Desktop Windows setup' }
if ($windows.Count -eq 0) { $missing += 'signed Windows NSIS update package' }
if ($appimage.Count -eq 0) { $missing += 'signed Linux AppImage' }
if ($deb.Count -eq 0) { $missing += 'Linux Debian package' }
if ($rpm.Count -eq 0) { $missing += 'Linux RPM package' }
if ('latest.json' -notin $assets) { $missing += 'latest.json' }
if ($Report) {
    if ($missing.Count -eq 0) { Write-Output '  [ok]      Desktop setup and updater assets' }
    else { $missing | ForEach-Object { Write-Output "  [MISSING] $_" } }
} else {
    Write-Output $missing.Count
}
