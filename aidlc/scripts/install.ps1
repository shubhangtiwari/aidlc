$ErrorActionPreference = "Stop"

$Repo = if ($env:AIDLC_REPO) { $env:AIDLC_REPO } else { "shubhangtiwari/aidlc" }
$Version = if ($env:AIDLC_VERSION) { $env:AIDLC_VERSION } else { "latest" }
$InstallDir = if ($env:AIDLC_INSTALL_DIR) { $env:AIDLC_INSTALL_DIR } else { Join-Path $HOME "bin" }
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("aidlc-install-" + [System.Guid]::NewGuid())

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

    $Archive = "aidlc_Windows_$Arch.zip"
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
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -Force -Path (Join-Path $TempDir "aidlc.exe") -Destination (Join-Path $InstallDir "aidlc.exe")

    Write-Host "aidlc installed to $(Join-Path $InstallDir "aidlc.exe")"
} finally {
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
}
