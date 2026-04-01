$ErrorActionPreference = "Stop"

$Repo = "agusrdz/verify-loop"
$InstallDir = if ($env:VERIFY_LOOP_INSTALL_DIR) { $env:VERIFY_LOOP_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\verify-loop" }

$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) {
    "arm64"
} else {
    "amd64"
}

$Binary = "verify-loop-windows-$Arch.exe"

if (-not $env:VERIFY_LOOP_VERSION) {
    $Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $env:VERIFY_LOOP_VERSION = $Release.tag_name
}

$Version = $env:VERIFY_LOOP_VERSION
$Url = "https://github.com/$Repo/releases/download/$Version/$Binary"

Write-Host "Installing verify-loop $Version..."

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

$Destination = "$InstallDir\verify-loop.exe"
Invoke-WebRequest -Uri $Url -OutFile $Destination

Write-Host ""
Write-Host "Installed: $Destination"

# Add to User PATH (persistent across sessions)
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$PathParts = $UserPath -split ";" | ForEach-Object { $_.TrimEnd("\") }
if ($PathParts -notcontains $InstallDir.TrimEnd("\")) {
    [Environment]::SetEnvironmentVariable("Path", "$InstallDir;$UserPath", "User")
    Write-Host "Added $InstallDir to User PATH"
}

# Make available in the current session immediately
$env:PATH = "$InstallDir;$env:PATH"

Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Run: verify-loop init"
Write-Host "  2. That's it — checks run automatically on every Claude Write"
Write-Host ""
Write-Host "Quick start:"
Write-Host "  verify-loop run src/app.ts     # manually check a file"
Write-Host "  verify-loop doctor             # diagnose any issues"
Write-Host "  verify-loop disable            # temporarily silence checks"
