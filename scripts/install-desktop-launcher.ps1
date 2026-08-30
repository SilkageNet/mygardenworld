#Requires -Version 5.1
<#
.SYNOPSIS
  Build desktop launcher binaries and place shortcuts on the user desktop.

.DESCRIPTION
  Creates:
    - 花园脚本启动.lnk  -> bin/desktop_launcher.exe -instance all
#>
$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$LauncherDir = Join-Path $ScriptDir "desktop_launcher"
$BinDir = Join-Path $RepoRoot "bin"
$LauncherExe = Join-Path $BinDir "desktop_launcher.exe"
$Desktop = [Environment]::GetFolderPath("Desktop")

function New-DesktopShortcut {
    param(
        [string]$ShortcutPath,
        [string]$TargetPath,
        [string]$Arguments,
        [string]$WorkingDirectory,
        [string]$Description
    )

    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($ShortcutPath)
    $shortcut.TargetPath = $TargetPath
    $shortcut.Arguments = $Arguments
    $shortcut.WorkingDirectory = $WorkingDirectory
    $shortcut.WindowStyle = 1
    if ($Description) {
        $shortcut.Description = $Description
    }
    $shortcut.Save()
}

if (-not (Test-Path $LauncherDir)) {
    throw "missing launcher source: $LauncherDir"
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
Push-Location $LauncherDir
try {
    & go build -ldflags "-s -w" -o $LauncherExe .
    if ($LASTEXITCODE -ne 0) {
        throw "go build desktop_launcher failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}

$shortcuts = @(
    @{
        Name = "花园脚本启动.lnk"
        Args = "-instance all"
        Desc = "启动主实例+隔离实例，登录控制台、确认全部游戏账号在线后用 Chrome 打开两个前端"
    }
)

# Remove legacy desktop copies that ignore -instance all or only start one stack.
foreach ($legacyName in @("花园脚本启动.exe", "花园脚本隔离.exe", "花园脚本隔离.lnk")) {
    $legacyPath = Join-Path $Desktop $legacyName
    if (Test-Path -LiteralPath $legacyPath) {
        Remove-Item -LiteralPath $legacyPath -Force
        Write-Host "Removed legacy desktop item: $legacyName"
    }
}

foreach ($item in $shortcuts) {
    $lnkPath = Join-Path $Desktop $item.Name
    New-DesktopShortcut `
        -ShortcutPath $lnkPath `
        -TargetPath $LauncherExe `
        -Arguments $item.Args `
        -WorkingDirectory $RepoRoot `
        -Description $item.Desc
    Write-Host "Installed $($item.Name)"
    Write-Host "  target: $LauncherExe $($item.Args)"
}

Write-Host ""
Write-Host "Launcher binary: $LauncherExe"
Write-Host "Configs:"
Write-Host "  scripts/autostart.env"
Write-Host "  scripts/autostart-b.env"
