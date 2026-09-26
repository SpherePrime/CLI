$ErrorActionPreference = "Stop"

$Repo = if ($env:PRIME_REPO) { $env:PRIME_REPO } else { "SpherePrime/CLI" }
$BinDir = if ($env:PRIME_INSTALL_DIR) { $env:PRIME_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\prime" }

$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "x86_64" }

$Api = "https://api.github.com/repos/$Repo/releases/latest"
Write-Host "prime: querying latest release from $Repo"
$Release = Invoke-RestMethod -Uri $Api -Headers @{ "User-Agent" = "prime-installer" }

$Asset = $Release.assets | Where-Object { $_.name -like "*_Windows_${Arch}.zip" } | Select-Object -First 1
if (-not $Asset) {
  Write-Error "prime: no release asset found for Windows/$Arch, see https://github.com/$Repo/releases"
}

$TmpPath = Join-Path $env:TEMP ([System.IO.Path]::GetRandomFileName())
$Tmp = New-Item -ItemType Directory -Path $TmpPath
try {
  $Zip = Join-Path $Tmp.FullName $Asset.name
  Write-Host "prime: downloading $($Asset.name)"
  Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $Zip

  $SumAsset = $Release.assets | Where-Object { $_.name -eq "checksums.txt" } | Select-Object -First 1
  if ($SumAsset) {
    $Sums = Join-Path $Tmp.FullName "checksums.txt"
    Invoke-WebRequest -Uri $SumAsset.browser_download_url -OutFile $Sums
    $Line = Get-Content $Sums | Where-Object { $_ -like "* $($Asset.name)" } | Select-Object -First 1
    if ($Line) {
      $Expected = ($Line -split "\s+")[0]
      if (Get-Command Get-FileHash -ErrorAction SilentlyContinue) {
        $Actual = (Get-FileHash -Algorithm SHA256 -Path $Zip).Hash.ToLower()
      } else {
        $fs = [System.IO.File]::OpenRead($Zip)
        try {
          $hash = [System.Security.Cryptography.SHA256]::Create().ComputeHash($fs)
        } finally {
          $fs.Dispose()
        }
        $Actual = (-join ($hash | ForEach-Object { $_.ToString("x2") }))
      }
      if ($Expected -ne $Actual) {
        Write-Error "prime: checksum verification failed"
      }
      Write-Host "prime: checksum verified"
    }
  }

  Expand-Archive -Path $Zip -DestinationPath $Tmp.FullName -Force

  $Binary = Get-ChildItem -Path $Tmp.FullName -Recurse -Filter "prime.exe" | Select-Object -First 1
  if (-not $Binary) {
    Write-Error "prime: could not locate prime.exe in archive"
  }

  New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
  $Dest = Join-Path $BinDir "prime.exe"
  if (Test-Path $Dest) { Remove-Item -Force $Dest }
  Move-Item -Force $Binary.FullName $Dest
} finally {
  Remove-Item -Recurse -Force $Tmp.FullName -ErrorAction SilentlyContinue
}

Write-Host "prime: installed to $BinDir\prime.exe"

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$BinDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$UserPath;$BinDir", "User")
  Write-Host "prime: added $BinDir to user PATH (restart your terminal)"
}

if ($env:PRIME_SKIP_VOICE -eq "1") {
  Write-Host "prime: voice setup skipped (PRIME_SKIP_VOICE=1)"
} else {
  Write-Host "prime: setting up voice input (whisper.cpp + Large V3 Turbo Q5, about 580 MB)"
  & $Dest voice setup --yes
  if ($LASTEXITCODE -ne 0) {
    Write-Host "prime: voice setup failed, run 'prime voice setup' later"
  }
}
