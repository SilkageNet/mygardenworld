$ErrorActionPreference = "Stop"

$AppName = "mygardenworld"
$Repo = "SilkageNet/mygardenworld"
$ExpectedOS = "__MYGARDENWORLD_OS__"
$ExpectedArch = "__MYGARDENWORLD_ARCH__"
$TempDir = $null

if ($PSVersionTable.PSEdition -eq "Core" -and -not $IsWindows) {
    throw "install.ps1 supports Windows only. Use install.sh on Linux or macOS."
}

$CurrentOS = "windows"
$MachineArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
switch -Regex ($MachineArch) {
    "^(AMD64|x64)$" { $CurrentArch = "amd64"; break }
    "^ARM64$" { $CurrentArch = "arm64"; break }
    default { $CurrentArch = $MachineArch.ToLowerInvariant() }
}

if ($ExpectedOS -ne "__MYGARDENWORLD_OS__" -and $ExpectedOS -ne $CurrentOS) {
    throw "this archive is for $ExpectedOS/$ExpectedArch, but this machine is $CurrentOS/$CurrentArch"
}
if ($ExpectedArch -ne "__MYGARDENWORLD_ARCH__" -and $ExpectedArch -ne $CurrentArch) {
    throw "this archive is for $ExpectedOS/$ExpectedArch, but this machine is $CurrentOS/$CurrentArch"
}

$InvocationPath = $MyInvocation.MyCommand.Path
$ScriptDir = if ($InvocationPath) { Split-Path -Parent $InvocationPath } else { (Get-Location).Path }
$Gardend = Join-Path $ScriptDir "gardend.exe"
$Gardenctl = Join-Path $ScriptDir "gardenctl.exe"

if (-not (Test-Path $Gardend) -or -not (Test-Path $Gardenctl)) {
    $DownloadOS = if ($ExpectedOS -ne "__MYGARDENWORLD_OS__") { $ExpectedOS } else { $CurrentOS }
    $DownloadArch = if ($ExpectedArch -ne "__MYGARDENWORLD_ARCH__") { $ExpectedArch } else { $CurrentArch }
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    $Pattern = "${AppName}_*_${DownloadOS}_${DownloadArch}.zip"
    $Asset = $Release.assets | Where-Object { $_.name -like $Pattern } | Select-Object -First 1
    if (-not $Asset) {
        throw "no release asset found for $DownloadOS/$DownloadArch"
    }

    $TempDir = Join-Path ([System.IO.Path]::GetTempPath()) "$AppName-install-$([Guid]::NewGuid().ToString('N'))"
    New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
    try {
        $Archive = Join-Path $TempDir $Asset.name
        Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $Archive
        Expand-Archive -Path $Archive -DestinationPath $TempDir -Force
        $DownloadedGardend = Get-ChildItem -Path $TempDir -Recurse -File -Filter "gardend.exe" | Select-Object -First 1
        $DownloadedGardenctl = Get-ChildItem -Path $TempDir -Recurse -File -Filter "gardenctl.exe" | Select-Object -First 1
        if (-not $DownloadedGardend -or -not $DownloadedGardenctl) {
            throw "gardend.exe or gardenctl.exe not found in downloaded archive"
        }
        $Gardend = $DownloadedGardend.FullName
        $Gardenctl = $DownloadedGardenctl.FullName
    } catch {
        Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
        throw
    }
}

$DefaultInstallDir = Join-Path $env:LOCALAPPDATA "Programs\mygardenworld"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { $DefaultInstallDir }
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Copy-Item -Force -Path $Gardend -Destination (Join-Path $InstallDir "gardend.exe")
Copy-Item -Force -Path $Gardenctl -Destination (Join-Path $InstallDir "gardenctl.exe")
if ($TempDir) {
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
}

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$PathParts = @()
if ($UserPath) {
    $PathParts = $UserPath -split ";"
}
if ($PathParts -notcontains $InstallDir) {
    $NewPath = if ($UserPath) { "$UserPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
    $env:Path = "$env:Path;$InstallDir"
    Write-Host "Added $InstallDir to your user PATH. Restart your terminal if gardend is not found."
}

Write-Host "Installed gardend and gardenctl to $InstallDir"
Write-Host "Run: gardend --help"
