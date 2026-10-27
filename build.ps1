# build.ps1 - test, build, and run the image pipeline on Windows.
#
# Usage (PowerShell, from the repository root):
#   .\build.ps1            # run tests, then build bin\imagepipeline.exe
#   .\build.ps1 -Run       # ...and run it (compares all three modes)
#   .\build.ps1 -SkipTests # build only
#
# If scripts are blocked, run once:  Set-ExecutionPolicy -Scope CurrentUser RemoteSigned

param(
    [switch]$Run,
    [switch]$SkipTests
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is not installed or not on PATH. Install Go 1.24 or newer from https://go.dev/dl/ and reopen PowerShell."
}
Write-Host "Using $(go env GOVERSION)"

if (-not $SkipTests) {
    Write-Host "`nRunning unit tests..."
    go test ./...
    if ($LASTEXITCODE -ne 0) { Write-Error "Tests failed; not building." }
}

Write-Host "`nBuilding bin\imagepipeline.exe..."
New-Item -ItemType Directory -Force -Path bin | Out-Null
$env:CGO_ENABLED = "0"   # pure Go: the .exe needs no C libraries or DLLs
go build -trimpath -ldflags "-s -w" -o bin\imagepipeline.exe .
if ($LASTEXITCODE -ne 0) { Write-Error "Build failed." }
Write-Host "Built bin\imagepipeline.exe"

if ($Run) {
    Write-Host "`nRunning..."
    .\bin\imagepipeline.exe -runs 20
}
