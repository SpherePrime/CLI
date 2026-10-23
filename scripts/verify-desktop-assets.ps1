param(
    [Parameter(Mandatory = $true)][string]$AssetList,
    [Parameter(Mandatory = $true)][string]$Version,
    [switch]$Report
)
$assets = @(Get-Content -LiteralPath $AssetList -ErrorAction SilentlyContinue)
$legacyAssets = @("PrimeSetup-$Version.exe", "PrimeDesktop-Update-$Version.exe", 'latest.yml')
$legacyComplete = @($legacyAssets | Where-Object { $_ -notin $assets }).Count -eq 0
$escapedVersion = [regex]::Escape($Version)
$windows = @($assets | Where-Object { $_ -match "_${escapedVersion}_x64-setup\.exe$" -and "${_}.sig" -in $assets })
$appimage = @($assets | Where-Object { $_ -match "[_-]${escapedVersion}[_-].*\.AppImage$" -and "${_}.sig" -in $assets })
$deb = @($assets | Where-Object { $_ -match "[_-]${escapedVersion}[_-].*\.deb$" })
$rpm = @($assets | Where-Object { $_ -match "[_-]${escapedVersion}[-_.].*\.rpm$" })
$missing = @()
if (!$legacyComplete) {
    if ($windows.Count -eq 0) { $missing += 'signed native Windows NSIS installer' }
    if ($appimage.Count -eq 0) { $missing += 'signed native Linux AppImage' }
    if ($deb.Count -eq 0) { $missing += 'native Linux Debian package' }
    if ($rpm.Count -eq 0) { $missing += 'native Linux RPM package' }
    if ('latest.json' -notin $assets) { $missing += 'latest.json' }
}
if ($Report) {
    if ($missing.Count -eq 0) { Write-Output '  [ok]      Desktop installer and updater assets' }
    else { $missing | ForEach-Object { Write-Output "  [MISSING] $_" } }
} else {
    Write-Output $missing.Count
}
