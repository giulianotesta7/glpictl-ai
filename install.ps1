# install.ps1 - Install glpictl-ai on Windows
#
# Usage:
#   Invoke-WebRequest -Uri "https://raw.githubusercontent.com/giulianotesta7/glpictl-ai/main/install.ps1" -UseBasicParsing | Invoke-Expression

$ErrorActionPreference = "Stop"

$Repo = "giulianotesta7/glpictl-ai"
$BinaryName = "glpictl-ai.exe"

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-Err {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default {
            Write-Err "Unsupported architecture: $arch"
            exit 1
        }
    }
}

function Get-LatestVersion {
    Write-Info "Fetching latest release information..."

    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
        return $response.tag_name
    }
    catch {
        Write-Err "Failed to fetch release information from GitHub"
        Write-Err $_.Exception.Message
        exit 1
    }
}

function Download-Binary {
    param(
        [string]$Version,
        [string]$Arch
    )

    $binaryUrl = "https://github.com/$Repo/releases/download/$Version/$BinaryName-$Version-windows-$Arch"
    $fallbackUrl = "https://github.com/$Repo/releases/download/$Version/$BinaryName-windows-$Arch"

    Write-Info "Downloading $BinaryName $Version for windows/$Arch..."

    $tempFile = Join-Path $env:TEMP $BinaryName

    try {
        Invoke-WebRequest -Uri $binaryUrl -OutFile $tempFile -UseBasicParsing
    }
    catch {
        try {
            Invoke-WebRequest -Uri $fallbackUrl -OutFile $tempFile -UseBasicParsing
        }
        catch {
            Write-Err "Failed to download binary from GitHub releases"
            Write-Err "Tried URLs:"
            Write-Err "  $binaryUrl"
            Write-Err "  $fallbackUrl"
            exit 1
        }
    }

    return $tempFile
}

function Install-Binary {
    param([string]$SourceFile)

    $installDir = Join-Path $env:USERPROFILE ".local\bin"

    if (-not (Test-Path $installDir)) {
        Write-Info "Creating installation directory: $installDir"
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    $destFile = Join-Path $installDir $BinaryName

    Write-Info "Installing to $destFile..."
    Copy-Item -Path $SourceFile -Destination $destFile -Force

    # Add to PATH if not already present
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$installDir*") {
        Write-Info "Adding $installDir to user PATH..."
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$installDir", "User")
        $env:Path = "$env:Path;$installDir"
    }

    # Clean up temp file
    Remove-Item $SourceFile -Force -ErrorAction SilentlyContinue

    Write-Info "Installation complete!"
    return $destFile
}

function Run-Configure {
    param([string]$BinaryPath)

    Write-Info "Running configuration..."
    Write-Host ""
    & $BinaryPath configure
}

# Main
Write-Host "========================================="
Write-Host "  glpictl-ai Installer (Windows)"
Write-Host "========================================="
Write-Host ""

$arch = Get-Architecture
Write-Info "Detected architecture: $arch"

$version = Get-LatestVersion
Write-Info "Latest version: $version"

$tempFile = Download-Binary -Version $version -Arch $arch
$binaryPath = Install-Binary -SourceFile $tempFile

Write-Host ""
Run-Configure -BinaryPath $binaryPath
