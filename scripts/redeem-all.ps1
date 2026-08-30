#Requires -Version 5.1
<#
.SYNOPSIS
  Redeem a gift code on all local gardend stacks (all accounts).

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\scripts\redeem-all.ps1 -Code SUMMER2026
#>
param(
    [Parameter(Mandatory = $true)]
    [string]$Code
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$Launcher = Join-Path $RepoRoot "bin\desktop_launcher.exe"

if (-not (Test-Path $Launcher)) {
    & (Join-Path $ScriptDir "install-desktop-launcher.ps1")
}

& $Launcher -redeem $Code
