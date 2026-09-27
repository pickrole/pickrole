<#
.SYNOPSIS
  Builds build\bin\pickrole.exe for Windows.

.DESCRIPTION
  Used by CI and releases (.github/workflows). The .exe is portable: it runs
  without installation or admin rights, using the WebView2 that ships with
  Windows 10 and 11. The version comes from -Version or the VERSION variable.

.EXAMPLE
  scripts\build-windows.ps1 -Version 0.1.0
#>
param(
  [string]$Version = $(if ($env:VERSION) { $env:VERSION } else { 'dev' }),
  [string]$WailsVersion = 'v2.16.0'
)

# Native tools log to stderr; exit codes are checked instead.
$ErrorActionPreference = 'Continue'
Set-Location (Split-Path -Parent $PSScriptRoot)

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
  go install "github.com/wailsapp/wails/v2/cmd/wails@$WailsVersion"
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  $env:Path += ";$(go env GOPATH)\bin"
}

$commit = git rev-parse --short=7 HEAD 2>$null
if (-not $commit -and $env:GITHUB_SHA) { $commit = $env:GITHUB_SHA.Substring(0, 7) }
$date = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')

# The .exe file properties (Explorer → Details) come from wails.json: stamp the
# release version there for this build only, and put the file back afterwards.
# Windows version numbers are numeric only, so a pre-release (0.2.0-beta.1)
# stamps 0.2.0; the app's About dialog shows the full version.
$wailsJson = Join-Path (Get-Location) 'wails.json'
$original = [IO.File]::ReadAllText($wailsJson)
if ($Version -match '^(\d+\.\d+\.\d+)(-[0-9A-Za-z.]+)?$') {
  $stamped = $original -replace '"productVersion":\s*"[^"]*"', "`"productVersion`": `"$($Matches[1])`""
  [IO.File]::WriteAllText($wailsJson, $stamped, (New-Object Text.UTF8Encoding($false)))
}
try {
  wails build -clean -trimpath -ldflags "-s -w -X main.version=$Version -X main.commit=$commit -X main.buildDate=$date"
  $status = $LASTEXITCODE
} finally {
  [IO.File]::WriteAllText($wailsJson, $original, (New-Object Text.UTF8Encoding($false)))
}

# Vite empties frontend/dist, removing the tracked .gitkeep, even when the build fails later.
git checkout -q -- frontend/dist/.gitkeep 2>$null
if ($status -ne 0) { exit $status }

Write-Host "Binary: build\bin\pickrole.exe (version $Version, commit $commit, $date)"
