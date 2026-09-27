<#
.SYNOPSIS
  Opens PickRole against a local fake AWS (cmd/fakeaws), with no AWS account.

.DESCRIPTION
  Starts the fake AWS in its own window (with a log of the calls) and opens
  build\bin\pickrole.exe with an isolated user folder: USERPROFILE, APPDATA
  and LOCALAPPDATA point to the sandbox, so nothing is written to your real
  ~\.aws, ~\.m2 or PickRole config. Close PickRole to stop everything.

  The sandbox comes with a ~\.aws\config that has an sso-session (PickRole
  detects it and fills in the SSO), a [personal] profile in the credentials
  file and a settings.xml with a mirror, to check that edits are surgical.

  A regular PickRole already open keeps this one from starting (single
  instance): close it first.

.PARAMETER Build
  Runs scripts\build-windows.ps1 first. Needed after changing the code.

.PARAMETER Reset
  Deletes the sandbox and starts from scratch (first-run screen).

.EXAMPLE
  scripts\mock-aws.ps1 -Build -Reset
#>
param(
  [switch]$Build,
  [switch]$Reset,
  [string]$Addr = '127.0.0.1:4599'
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $repo 'build\bin'
$sandbox = Join-Path $env:TEMP 'pickrole-mock'

Push-Location $repo
# Native tools log to stderr; with 'Stop', Windows PowerShell 5.1 turns that
# into a terminating error. Exit codes are checked instead.
$ErrorActionPreference = 'Continue'
try {
  if ($Build -or -not (Test-Path "$bin\pickrole.exe")) {
    & (Join-Path $PSScriptRoot 'build-windows.ps1')
    if ($LASTEXITCODE -ne 0) { throw 'build failed' }
  }
  go build -o "$bin\fakeaws.exe" ./cmd/fakeaws
  if ($LASTEXITCODE -ne 0) { throw 'go build of fakeaws failed' }
} finally {
  Pop-Location
  $ErrorActionPreference = 'Stop'
}

if ($Reset -and (Test-Path $sandbox)) {
  Remove-Item -Recurse -Force $sandbox
}
if (-not (Test-Path $sandbox)) {
  $utf8 = New-Object System.Text.UTF8Encoding($false)
  $seed = @{
    '.aws\config' = "[sso-session pickrole-fake]`nsso_start_url = https://pickrole-fake.awsapps.com/start`nsso_region = us-east-1`nsso_registration_scopes = sso:account:access`n"
    '.aws\credentials' = "[personal]`naws_access_key_id = AKIAPERSONALEXAMPLE`naws_secret_access_key = do-not-touch`n"
    '.m2\settings.xml' = "<settings>`n  <mirrors>`n    <mirror>`n      <id>internal</id>`n      <url>https://maven.example.com/</url>`n      <mirrorOf>*</mirrorOf>`n    </mirror>`n  </mirrors>`n</settings>`n"
  }
  foreach ($rel in $seed.Keys) {
    $path = Join-Path $sandbox $rel
    New-Item -ItemType Directory -Force (Split-Path -Parent $path) | Out-Null
    [System.IO.File]::WriteAllText($path, $seed[$rel], $utf8)
  }
  New-Item -ItemType Directory -Force "$sandbox\AppData\Roaming", "$sandbox\AppData\Local" | Out-Null
}

# Started before the environment changes, so it runs as the real user.
$fake = Start-Process -FilePath "$bin\fakeaws.exe" -ArgumentList "-addr $Addr" -PassThru

$names = 'USERPROFILE', 'HOME', 'APPDATA', 'LOCALAPPDATA', 'AWS_ENDPOINT_URL'
$saved = @{}
foreach ($n in $names) { $saved[$n] = [Environment]::GetEnvironmentVariable($n, 'Process') }
try {
  $env:USERPROFILE = $sandbox
  $env:HOME = $sandbox
  $env:APPDATA = "$sandbox\AppData\Roaming"
  $env:LOCALAPPDATA = "$sandbox\AppData\Local"
  $env:AWS_ENDPOINT_URL = "http://$Addr"

  Write-Host "Fake AWS:   http://$Addr/  (accounts, expire or revoke the session)"
  Write-Host "Sandbox:    $sandbox"
  Write-Host 'CodeArtifact: domain "pickrole", owner account 999999999999; ReadOnly roles have no access.'
  Write-Host 'Close PickRole to stop.'
  Start-Process -FilePath "$bin\pickrole.exe" -Wait
} finally {
  foreach ($n in $names) { [Environment]::SetEnvironmentVariable($n, $saved[$n], 'Process') }
  Stop-Process -Id $fake.Id -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host 'Files written by PickRole:'
foreach ($rel in '.aws\credentials', '.m2\settings.xml') {
  $path = Join-Path $sandbox $rel
  Write-Host "--- $path"
  Get-Content -Encoding UTF8 $path | Write-Host
}
