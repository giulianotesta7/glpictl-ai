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

    # Fallback if $env:TEMP is null or empty
    $tempDir = $env:TEMP
    if ([string]::IsNullOrEmpty($tempDir)) {
        $tempDir = [System.IO.Path]::GetTempPath()
    }
    $tempFile = [System.IO.Path]::GetTempFileName()

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

function Verify-Checksum {
    param(
        [string]$FilePath,
        [string]$Version,
        [string]$Arch
    )

    $checksumUrl = "https://github.com/$Repo/releases/download/$Version/$BinaryName-$Version-windows-$Arch.sha256"
    $fallbackUrl = "https://github.com/$Repo/releases/download/$Version/$BinaryName-windows-$Arch.sha256"

    # Use a secure temp file for the checksum to avoid predictable naming
    $script:tempChecksum = [System.IO.Path]::GetTempFileName()

    try {
        Invoke-WebRequest -Uri $checksumUrl -OutFile $script:tempChecksum -UseBasicParsing
    }
    catch {
        try {
            Invoke-WebRequest -Uri $fallbackUrl -OutFile $script:tempChecksum -UseBasicParsing
        }
        catch {
            Write-Warn "No checksum file found at release — skipping verification"
            Write-Warn "For full supply-chain security, publish .sha256 checksums with releases"
            return
        }
    }

    Write-Info "Verifying checksum..."
    $expectedHash = ((Get-Content $script:tempChecksum -Raw).Split(' ')[0]).Trim()
    $actualHash = (Get-FileHash $FilePath -Algorithm SHA256).Hash

    Remove-Item $script:tempChecksum -Force -ErrorAction SilentlyContinue

    if ($expectedHash -ine $actualHash) {
        Write-Err "Checksum verification FAILED — binary may be tampered with"
        exit 1
    }
    Write-Info "Checksum verification passed!"
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

    # Add to PATH if not already present (exact match, not substring)
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = $currentPath -split ';' | ForEach-Object { $_.Trim() }
    $alreadyInPath = $false
    foreach ($entry in $pathEntries) {
        if ($entry -ieq $installDir) {
            $alreadyInPath = $true
            break
        }
    }
    if (-not $alreadyInPath) {
        Write-Info "Adding $installDir to user PATH..."
        if ([string]::IsNullOrEmpty($currentPath)) {
            $newPath = $installDir
        } else {
            $newPath = "$currentPath;$installDir"
        }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
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

$tempFile = $null
$tempChecksum = $null
try {
    $tempFile = Download-Binary -Version $version -Arch $arch
    Verify-Checksum -FilePath $tempFile -Version $version -Arch $arch
    $binaryPath = Install-Binary -SourceFile $tempFile

    Write-Host ""
    Run-Configure -BinaryPath $binaryPath
}
finally {
    # Clean up temp files on any exit path (success, failure, or exception)
    if ($tempFile -and (Test-Path $tempFile)) {
        Remove-Item $tempFile -Force -ErrorAction SilentlyContinue
    }
    if ($tempChecksum -and (Test-Path $tempChecksum)) {
        Remove-Item $tempChecksum -Force -ErrorAction SilentlyContinue
    }
}
