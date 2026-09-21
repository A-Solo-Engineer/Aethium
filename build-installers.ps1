#!/usr/bin/env pwsh
# build-installers.ps1


param(
    [string]$Version = "1.3.0"
)

$ErrorActionPreference = "Stop"
$root      = Split-Path -Parent $MyInvocation.MyCommand.Path
$installer = "$root\installer"
$payload   = "$installer\payload"

Write-Host ""
Write-Host "Building Aethium installers v$Version" -ForegroundColor Cyan
Write-Host ""


Write-Host "[1/4] Building Aethium runtime binaries..." -ForegroundColor White

$env:CGO_ENABLED = "0"

$targets = @(
    @{ OS = "windows"; ARCH = "amd64"; Out = "$payload\aethium-windows.exe" },
    @{ OS = "darwin";  ARCH = "amd64"; Out = "$payload\aethium-mac"         },
    @{ OS = "linux";   ARCH = "amd64"; Out = "$payload\aethium-linux"       }
)

New-Item -ItemType Directory -Path $payload -Force | Out-Null

foreach ($t in $targets) {
    $env:GOOS   = $t.OS
    $env:GOARCH = $t.ARCH
    Write-Host "  Building for $($t.OS)/$($t.ARCH)..." -NoNewline
    & go build -o $t.Out ./cmd/aethium
    Write-Host " Done" -ForegroundColor Green
}

$env:GOOS   = ""
$env:GOARCH = ""


Write-Host "[2/4] Packaging extension and examples..." -ForegroundColor White


$vsix = "$root\editors\vscode\aethium-lang-$Version.vsix"
if (-not (Test-Path $vsix)) {
    Write-Host "  Packaging VS Code extension..." -NoNewline
    Push-Location "$root\editors\vscode"
    & vsce package --allow-missing-repository --out "aethium-lang-$Version.vsix" 2>&1 | Out-Null
    Pop-Location
    Write-Host " Done" -ForegroundColor Green
}

New-Item -ItemType Directory -Path "$payload\editors\vscode" -Force | Out-Null
Copy-Item $vsix "$payload\editors\vscode\aethium-lang-$Version.vsix" -Force

if (Test-Path "$root\examples") {
    Copy-Item "$root\examples" "$payload\examples" -Recurse -Force
} else {
    New-Item -ItemType Directory -Path "$payload\examples" -Force | Out-Null
    Set-Content "$payload\examples\hello.aeth" '# Hello, Aethium!
print("Hello, World!")
'
}

Write-Host "  Payload ready at $payload" -ForegroundColor Green


Write-Host "[3/4] Building installer binaries..." -ForegroundColor White

$dist = "$root\dist"
New-Item -ItemType Directory -Path $dist -Force | Out-Null

$installerTargets = @(
    @{ OS = "windows"; ARCH = "amd64"; Out = "$dist\AethiumSetup-$Version-windows.exe" },
    @{ OS = "darwin";  ARCH = "amd64"; Out = "$dist\AethiumSetup-$Version-mac"         },
    @{ OS = "linux";   ARCH = "amd64"; Out = "$dist\AethiumSetup-$Version-linux"       }
)

Push-Location $installer

foreach ($t in $installerTargets) {
    $env:GOOS   = $t.OS
    $env:GOARCH = $t.ARCH
    Write-Host "  Building installer for $($t.OS)/$($t.ARCH)..." -NoNewline
    & go build -o $t.Out .
    Write-Host " Done" -ForegroundColor Green
}

$env:GOOS   = ""
$env:GOARCH = ""
Pop-Location


Write-Host ""
Write-Host "[4/4] Done! Installers ready in $dist :" -ForegroundColor Cyan
Write-Host ""
Get-ChildItem $dist | ForEach-Object {
    $size = [math]::Round($_.Length / 1MB, 1)
    Write-Host ("  {0,-50} {1} MB" -f $_.Name, $size) -ForegroundColor Green
}
Write-Host ""
Write-Host "Ready To Ship" -ForegroundColor White
Write-Host ""