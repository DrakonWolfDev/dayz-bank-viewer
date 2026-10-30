param([string]$InstallDir, [switch]$Uninstall)
$ErrorActionPreference = 'Stop'
try {
 $service = Get-Service DayZBankViewer -ErrorAction SilentlyContinue
 $wasRunning = $service -and $service.Status -eq 'Running'
 if ($service) {
  if ($Uninstall) { Set-Service DayZBankViewer -StartupType Disabled }
  Stop-Service DayZBankViewer -ErrorAction Stop
  $service.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(20))
 }
 # Only processes loaded from this installation are closed. No name-only process kill.
 $binary = Join-Path $InstallDir 'dayz-bank-viewer.exe'
 Get-Process -Name dayz-bank-viewer -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $binary } | ForEach-Object { $_.Kill(); $_.WaitForExit(10000) | Out-Null }
 if ($Uninstall -and $service) {
  & sc.exe delete DayZBankViewer | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Could not remove the service registration' }
 }
 if ($wasRunning) { exit 10 }
 exit 0
} catch { Write-Output $_.Exception.Message; exit 1 }
