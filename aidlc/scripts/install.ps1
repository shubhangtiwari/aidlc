$ErrorActionPreference = "Stop"

$Repo = if ($env:AIDLC_REPO) { $env:AIDLC_REPO } else { "shubhangtiwari/aidlc" }
$Version = if ($env:AIDLC_VERSION) { $env:AIDLC_VERSION } else { "latest" }
$DefaultLocalAppData = if ($env:LOCALAPPDATA) {
    $env:LOCALAPPDATA
} else {
    [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
}
if (-not $DefaultLocalAppData) {
    $DefaultLocalAppData = Join-Path $HOME "AppData\Local"
}
$InstallDir = if ($env:AIDLC_INSTALL_DIR) {
    $env:AIDLC_INSTALL_DIR
} else {
    Join-Path (Join-Path $DefaultLocalAppData "Programs") "aidlc\bin"
}
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("aidlc-install-" + [System.Guid]::NewGuid())

function ConvertTo-AidlcFullPath {
    param([string]$Path)

    return [System.IO.Path]::GetFullPath((Expand-Path -Path $Path))
}

function Expand-Path {
    param([string]$Path)

    return $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
}

function Test-AidlcPathEntry {
    param(
        [string]$PathValue,
        [string]$Directory
    )

    if (-not $PathValue) {
        return $false
    }

    $Target = Normalize-AidlcPathEntry -Directory $Directory
    foreach ($Entry in ($PathValue -split [System.IO.Path]::PathSeparator)) {
        if ((Normalize-AidlcPathEntry -Directory $Entry) -eq $Target) {
            return $true
        }
    }
    return $false
}

function Normalize-AidlcPathEntry {
    param([string]$Directory)

    if (-not $Directory) {
        return ""
    }
    try {
        return ([System.IO.Path]::GetFullPath($Directory.Trim())).TrimEnd([char[]]"\/").ToUpperInvariant()
    } catch {
        return $Directory.Trim().TrimEnd([char[]]"\/").ToUpperInvariant()
    }
}

function Format-AidlcPowerShellLiteral {
    param([string]$Value)

    return "'" + ($Value -replace "'", "''") + "'"
}

function Add-AidlcUserPath {
    param([string]$Directory)

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (Test-AidlcPathEntry -PathValue $UserPath -Directory $Directory) {
        return "present"
    }

    $NewUserPath = if ($UserPath) {
        "$Directory$([System.IO.Path]::PathSeparator)$UserPath"
    } else {
        $Directory
    }

    try {
        [Environment]::SetEnvironmentVariable("Path", $NewUserPath, "User")
        return "updated"
    } catch {
        [Console]::Error.WriteLine("aidlc install: failed to update the user PATH: $($_.Exception.Message)")
        return "failed"
    }
}

function Invoke-AidlcDownload {
    param(
        [string]$Uri,
        [string]$OutFile,
        [string]$Label
    )

    try {
        Invoke-WebRequest -Uri $Uri -OutFile $OutFile
    } catch {
        [Console]::Error.WriteLine("aidlc install: failed to download $Label from $Uri")
        [Console]::Error.WriteLine("aidlc install: release assets are required; check AIDLC_REPO=$Repo and AIDLC_VERSION=$Version")
        [Console]::Error.WriteLine("aidlc install: for unreleased source checkouts, run: cd aidlc; go install ./cmd/aidlc")
        throw
    }
}

New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
try {
    $Arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
        "X64" { "x86_64" }
        "Arm64" { "arm64" }
        default { throw "unsupported architecture: $($_)" }
    }

    if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) {
        throw "install.ps1 supports Windows only"
    }

    if ($Version -eq "latest") {
        $BaseUrl = "https://github.com/$Repo/releases/latest/download"
    } else {
        $BaseUrl = "https://github.com/$Repo/releases/download/$Version"
    }

    $Archive = "aidlc_windows_$Arch.zip"
    $Checksums = "checksums.txt"
    $ArchivePath = Join-Path $TempDir $Archive
    $ChecksumsPath = Join-Path $TempDir $Checksums

    Invoke-AidlcDownload -Uri "$BaseUrl/$Archive" -OutFile $ArchivePath -Label $Archive
    Invoke-AidlcDownload -Uri "$BaseUrl/$Checksums" -OutFile $ChecksumsPath -Label $Checksums

    $Expected = Get-Content $ChecksumsPath |
        ForEach-Object { $_.Trim() } |
        Where-Object { $_ -and -not $_.StartsWith("#") } |
        ForEach-Object {
            $Parts = $_ -split "\s+"
            if ($Parts.Count -ge 2 -and ($Parts[1] -eq $Archive -or $Parts[1] -eq "*$Archive")) {
                $Parts[0].ToLowerInvariant()
            }
        } |
        Select-Object -First 1

    if (-not $Expected) {
        throw "checksum for $Archive not found"
    }

    $Actual = (Get-FileHash -Algorithm SHA256 -Path $ArchivePath).Hash.ToLowerInvariant()
    if ($Actual -ne $Expected) {
        throw "checksum mismatch for $Archive"
    }

    Expand-Archive -Path $ArchivePath -DestinationPath $TempDir -Force
    $InstallDir = ConvertTo-AidlcFullPath -Path $InstallDir
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $ExecutablePath = Join-Path $InstallDir "aidlc.exe"
    Copy-Item -Force -Path (Join-Path $TempDir "aidlc.exe") -Destination $ExecutablePath

    Write-Host "aidlc installed to $ExecutablePath"

    $PathStatus = Add-AidlcUserPath -Directory $InstallDir
    if ($PathStatus -eq "updated") {
        Write-Host "aidlc install: added $InstallDir to the user PATH."
        Write-Host "aidlc install: open a new terminal or restart your IDE for PATH changes to be visible."
        Write-Host "aidlc install: verify with: aidlc --version"
    } elseif ($PathStatus -eq "present") {
        Write-Host "aidlc install: $InstallDir is already in the user PATH."
        Write-Host "aidlc install: open a new terminal or restart your IDE if aidlc is not visible in this shell."
        Write-Host "aidlc install: verify with: aidlc --version"
    } else {
        $InstallDirLiteral = Format-AidlcPowerShellLiteral -Value $InstallDir
        $ExecutableLiteral = Format-AidlcPowerShellLiteral -Value $ExecutablePath
        Write-Host "aidlc install: PATH was not changed; installed executable remains available at $ExecutablePath"
        Write-Host "aidlc install: add it manually with:"
        Write-Host "  `$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User'); [Environment]::SetEnvironmentVariable('Path', $InstallDirLiteral + [System.IO.Path]::PathSeparator + `$UserPath, 'User')"
        Write-Host "aidlc install: or run Make helpers with:"
        Write-Host "  `$env:AIDLC_BIN = $ExecutableLiteral"
        Write-Host "aidlc install: open a new terminal or restart your IDE after changing PATH."
    }
} finally {
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
}
