$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()
$form = New-Object System.Windows.Forms.Form
$form.Text = 'DayZ Bank Viewer'
$form.Size = New-Object System.Drawing.Size(540,650)
$form.StartPosition = 'CenterScreen'
$form.AutoScaleMode = 'Dpi'
$iconPath = Join-Path (Split-Path $env:VIEWER_EXE) 'app.ico'
if (!(Test-Path $iconPath)) { $iconPath = Join-Path (Split-Path $env:VIEWER_EXE) 'web/app.ico' }
if (Test-Path $iconPath) { $form.Icon = New-Object System.Drawing.Icon($iconPath) }
$label = New-Object System.Windows.Forms.Label
$label.Location = New-Object System.Drawing.Point(24,20)
$label.Size = New-Object System.Drawing.Size(475,135)
$form.Controls.Add($label)
function Refresh-Status {
 $state = & $env:VIEWER_EXE -config $env:VIEWER_CONFIG -control status 2>&1 | Out-String
 $boot = Get-Service DayZBankViewer -ErrorAction SilentlyContinue
 $startup = if ($boot) { $boot.StartType } else { 'Service not installed' }
 $label.Text = "DayZ Bank Viewer $env:VIEWER_VERSION`n$state`nStartup: $startup"
}
function Show-About {
 $about = New-Object System.Windows.Forms.Form
 $about.Text = 'About DayZ Bank Viewer'
 $about.Size = New-Object System.Drawing.Size(480,260)
 $about.AutoScaleMode = 'Dpi'
 $about.StartPosition = 'CenterParent'
 $text = New-Object System.Windows.Forms.Label
 $text.Location = New-Object System.Drawing.Point(20,20)
 $text.Size = New-Object System.Drawing.Size(430,100)
 $text.Text = "DayZ Bank Viewer $env:VIEWER_VERSION`nA Wolf Forge Project 🐾`nRead-only local banking dashboard.`nGNU Affero General Public License v3 only."
 $about.Controls.Add($text)
 $forge = New-Object System.Windows.Forms.LinkLabel
 $forge.Text = 'Wolf Forge'
 $forge.Location = New-Object System.Drawing.Point(20,130)
 $forge.AutoSize = $true
 $forge.Add_LinkClicked({ Start-Process 'https://wolf-forge.co.uk/' })
 $about.Controls.Add($forge)
 $licence = New-Object System.Windows.Forms.LinkLabel
 $licence.Text = 'Licence (AGPL-3.0-only) and corresponding source'
 $licence.Location = New-Object System.Drawing.Point(20,170)
 $licence.AutoSize = $true
 $licence.Add_LinkClicked({ Start-Process 'https://github.com/DrakonWolfDev/dayz-bank-viewer/blob/main/LICENSE' })
 $about.Controls.Add($licence)
 [void]$about.ShowDialog($form)
}
function Invoke-Control($action) {
 $output = & $env:VIEWER_EXE -config $env:VIEWER_CONFIG -control $action 2>&1 | Out-String
 if ($LASTEXITCODE -ne 0) { [System.Windows.Forms.MessageBox]::Show($output,'DayZ Bank Viewer') | Out-Null }
 Refresh-Status
}
$buttons = @('Open dashboard','Start viewer','Stop viewer','Restart viewer','Check for updates','Start automatically when the computer starts','Disable automatic startup','About and licence')
$y = 165
foreach ($caption in $buttons) {
 $button = New-Object System.Windows.Forms.Button
 $button.Text = $caption
 $button.AccessibleName = $caption
 $button.Location = New-Object System.Drawing.Point(24,$y)
 $button.Size = New-Object System.Drawing.Size(475,43)
 $button.Add_Click({
  switch ($this.Text) {
   'Open dashboard' {
    Invoke-Control 'start'
    try { $s = Get-Content -Raw ($env:VIEWER_CONFIG + '.status') | ConvertFrom-Json; Start-Process $s.url } catch { [System.Windows.Forms.MessageBox]::Show($_.Exception.Message) }
   }
   'Start viewer' { Invoke-Control 'start' }
   'Stop viewer' { Invoke-Control 'stop' }
   'Restart viewer' { Invoke-Control 'restart' }
   'Check for updates' {
    $result = & $env:VIEWER_EXE -config $env:VIEWER_CONFIG -control updates 2>&1 | Out-String
    if ($result -match '(https://github.com/DrakonWolfDev/dayz-bank-viewer/releases/tag/[a-zA-Z0-9.]+)') {
     $url = $Matches[1]
     if ([System.Windows.Forms.MessageBox]::Show($result + "`nOpen the release page?",'Updates','YesNo') -eq 'Yes') { Start-Process $url }
    } else { [System.Windows.Forms.MessageBox]::Show($result,'Updates') | Out-Null }
   }
   'Start automatically when the computer starts' {
    if ([System.Windows.Forms.MessageBox]::Show('Enable automatic startup? This changes a system setting and requires administrator permission.','Automatic startup','YesNo') -eq 'Yes') { Invoke-Control 'enable' }
   }
   'Disable automatic startup' { Invoke-Control 'disable' }
   'About and licence' { Show-About }
  }
 }.GetNewClosure())
 $form.Controls.Add($button)
 $y += 49
}
$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 3000
$timer.Add_Tick({ if (!(Get-Process -Id ([int]$env:VIEWER_PARENT_PID) -ErrorAction SilentlyContinue)) { $form.Close() } else { Refresh-Status } })
$timer.Start()
Refresh-Status
[void]$form.ShowDialog()
$timer.Dispose()
