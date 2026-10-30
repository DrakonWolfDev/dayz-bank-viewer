$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
trap { [Console]::Error.WriteLine(($_ | Format-List * -Force | Out-String)); exit 1 }
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()
$hc = [System.Windows.Forms.SystemInformation]::HighContrast
$bg = [System.Drawing.ColorTranslator]::FromHtml('#202226')
$surface = [System.Drawing.ColorTranslator]::FromHtml('#2A2D31')
$fg = [System.Drawing.ColorTranslator]::FromHtml('#F1F0EC')
$muted = [System.Drawing.ColorTranslator]::FromHtml('#B8BDC4')
$border = [System.Drawing.ColorTranslator]::FromHtml('#565C64')
$gold = [System.Drawing.ColorTranslator]::FromHtml('#D8AE62')
if ($hc) {
 $bg = [System.Drawing.SystemColors]::Window
 $surface = $bg
 $fg = [System.Drawing.SystemColors]::WindowText
 $muted = $fg
 $border = $fg
 $gold = [System.Drawing.SystemColors]::Highlight
}
$form = New-Object System.Windows.Forms.Form
$form.Text = 'DayZ Bank Viewer'
$form.Font = New-Object System.Drawing.Font('Segoe UI',10)
$form.AutoScaleDimensions = New-Object System.Drawing.SizeF(96,96)
$form.AutoScaleMode = 'Dpi'
$form.ClientSize = New-Object System.Drawing.Size(620,740)
$form.MinimumSize = New-Object System.Drawing.Size(620,540)
$form.StartPosition = 'CenterScreen'
$form.BackColor = $bg
$form.ForeColor = $fg
$iconPath = Join-Path (Split-Path $env:VIEWER_EXE) 'app.ico'
if (!(Test-Path $iconPath)) { $iconPath = Join-Path (Split-Path $env:VIEWER_EXE) 'web/app.ico' }
$logoImage = $null
if (Test-Path $iconPath) {
 $form.Icon = New-Object System.Drawing.Icon($iconPath)
}
$logoPath = Join-Path (Split-Path $env:VIEWER_EXE) 'app.png'
if (!(Test-Path $logoPath)) { $logoPath = Join-Path (Split-Path $env:VIEWER_EXE) 'web/app.png' }
if (Test-Path $logoPath) {
 try {
  $source = [System.Drawing.Image]::FromFile($logoPath)
  try { $logoImage = New-Object System.Drawing.Bitmap($source) } finally { $source.Dispose() }
 } catch { $logoImage = $null }
}
function New-Stack {
 $stack = New-Object System.Windows.Forms.TableLayoutPanel
 $stack.AutoSize = $true
 $stack.AutoSizeMode = 'GrowAndShrink'
 $stack.Dock = 'Top'
 $stack.ColumnCount = 1
 [void]$stack.ColumnStyles.Add((New-Object System.Windows.Forms.ColumnStyle('Percent',100)))
 return $stack
}
function Add-Row($parent, $control) {
 $row = $parent.RowCount
 $parent.RowCount += 1
 [void]$parent.RowStyles.Add((New-Object System.Windows.Forms.RowStyle('AutoSize')))
 $control.Margin = New-Object System.Windows.Forms.Padding(0,0,0,10)
 $parent.Controls.Add($control,0,$row)
}
function New-Text($text, $size = 10, $bold = $false) {
 $item = New-Object System.Windows.Forms.Label
 $item.Text = $text
 $item.AutoSize = $true
 $item.MaximumSize = New-Object System.Drawing.Size(520,0)
 $style = if ($bold) { [System.Drawing.FontStyle]::Bold } else { [System.Drawing.FontStyle]::Regular }
 $item.Font = New-Object System.Drawing.Font('Segoe UI',$size,$style)
 $item.ForeColor = $fg
 return $item
}
function New-Card {
 $card = New-Stack
 $card.BackColor = $surface
 $card.Padding = New-Object System.Windows.Forms.Padding(18,16,18,8)
 return $card
}
function New-ActionRow {
 $row = New-Object System.Windows.Forms.FlowLayoutPanel
 $row.AutoSize = $true
 $row.Dock = 'Top'
 $row.WrapContents = $true
 return $row
}
$scroll = New-Object System.Windows.Forms.Panel
$scroll.Dock = 'Fill'
$scroll.AutoScroll = $true
$form.Controls.Add($scroll)
$layout = New-Stack
$layout.Padding = New-Object System.Windows.Forms.Padding(22)
$scroll.Controls.Add($layout)

$header = New-Object System.Windows.Forms.TableLayoutPanel
$header.AutoSize = $true
$header.Dock = 'Top'
$header.ColumnCount = 2
[void]$header.ColumnStyles.Add((New-Object System.Windows.Forms.ColumnStyle('Absolute',90)))
[void]$header.ColumnStyles.Add((New-Object System.Windows.Forms.ColumnStyle('Percent',100)))
$logo = New-Object System.Windows.Forms.PictureBox
$logo.Size = New-Object System.Drawing.Size(76,76)
$logo.SizeMode = 'Zoom'
$logo.Image = $logoImage
$logo.AccessibleName = 'DayZ Bank Viewer wolf and coin logo'
$header.Controls.Add($logo,0,0)
$heading = New-Stack
Add-Row $heading (New-Text 'DayZ Bank Viewer' 23 $true)
$subtitle = New-Text "Your local banking dashboard  ·  $env:VIEWER_VERSION" 9
$subtitle.ForeColor = $muted
Add-Row $heading $subtitle
$header.Controls.Add($heading,1,0)
Add-Row $layout $header

$statusCard = New-Card
Add-Row $statusCard (New-Text 'VIEWER STATUS' 9 $true)
$statusLabel = New-Text 'Checking viewer…' 15 $true
Add-Row $statusCard $statusLabel
$accountLabel = New-Text '' 9
$accountLabel.ForeColor = $muted
Add-Row $statusCard $accountLabel
Add-Row $statusCard (New-Text 'Dashboard' 9)
$address = New-Object System.Windows.Forms.TextBox
$address.ReadOnly = $true
$address.Dock = 'Top'
$address.BorderStyle = 'FixedSingle'
$address.BackColor = $bg
$address.ForeColor = $fg
$address.AccessibleName = 'Dashboard address (select and copy)'
Add-Row $statusCard $address
$viewerActions = New-ActionRow
$startupCard = New-Card
Add-Row $startupCard (New-Text 'Unattended startup' 13 $true)
$startupLabel = New-Text 'Off' 10 $true
$startupLabel.ForeColor = $gold
Add-Row $startupCard $startupLabel
Add-Row $startupCard (New-Text 'Keep the viewer running after you sign out.' 10)
$startupActions = New-Stack
$otherActions = New-ActionRow
$actionButtons = @{}
$script:pending = $null
$script:probe = $null
$script:state = $null
$script:lastProbe = [DateTime]::MinValue
function New-Request($action, $purpose) {
 $p = New-Object System.Diagnostics.Process
 $p.StartInfo.FileName = $env:VIEWER_EXE
 $p.StartInfo.Arguments = '-config "' + $env:VIEWER_CONFIG + '" -control ' + $action
 $p.StartInfo.UseShellExecute = $false
 $p.StartInfo.CreateNoWindow = $true
 $p.StartInfo.RedirectStandardOutput = $true
 $p.StartInfo.RedirectStandardError = $true
 $p.StartInfo.StandardOutputEncoding = [Text.Encoding]::UTF8
 $p.StartInfo.StandardErrorEncoding = [Text.Encoding]::UTF8
 try {
  [void]$p.Start()
  return [pscustomobject]@{ Process=$p; Output=$p.StandardOutput.ReadToEndAsync(); Error=$p.StandardError.ReadToEndAsync(); Purpose=$purpose }
 } catch { $p.Dispose(); throw }
}
function Request-Done($request) {
 return $request -and $request.Process.HasExited -and $request.Output.IsCompleted -and $request.Error.IsCompleted
}
function Apply-Status($state) {
 $script:state = $state
 $statusLabel.Text = if ($state.running) { if ($state.service) { 'Running as a service' } else { 'Running as you' } } else { 'Stopped or unavailable' }
 $statusLabel.ForeColor = if ($state.running -and !$hc) { [Drawing.ColorTranslator]::FromHtml('#78DA9B') } else { $fg }
 $accountLabel.Text = if ($state.account) { $state.account } else { 'Start the viewer to open your dashboard.' }
 if ($address.Text -ne $state.url) { $address.Text = $state.url }
 $startupLabel.Text = if ($state.service) { 'On — unattended service selected' } else { 'Off — desktop mode' }
 foreach ($button in $actionButtons.Values) { $button.Enabled = !$script:pending }
 $actionButtons['Start viewer'].Enabled = !$script:pending -and !$state.running
 $actionButtons['Stop viewer'].Enabled = !$script:pending -and $state.running
 $actionButtons['Restart viewer'].Enabled = !$script:pending -and $state.running
 $actionButtons['Run without signing in…'].Visible = !$state.service
 $actionButtons['Switch back to desktop mode'].Visible = $state.service
 $actionButtons['About and licence'].Enabled = $true
}
function Request-Status {
 if (!$script:probe) {
  $script:probe = New-Request 'desktop-status' 'status'
  $script:lastProbe = [DateTime]::UtcNow
 }
}
function Begin-Action($action, $purpose = '') {
 if ($script:pending) { return }
 try {
  $script:pending = New-Request $action $purpose
  foreach ($button in $actionButtons.Values) { $button.Enabled = $false }
  $accountLabel.Text = 'Working… You can keep using this window.'
 } catch { [void][Windows.Forms.MessageBox]::Show($_.Exception.Message,'DayZ Bank Viewer') }
}
function Finish-Action($request) {
 $output = $request.Output.Result
 if ($request.Process.ExitCode -ne 0) {
  $detail = ($request.Error.Result + "`r`n" + $output).Trim()
  if (!$detail) { $detail = 'The operation failed. Exit code: ' + $request.Process.ExitCode }
  [void][Windows.Forms.MessageBox]::Show($detail,'DayZ Bank Viewer')
  return
 }
 switch ($request.Purpose) {
  'open-start' { Begin-Action 'dashboard' 'open' }
  'open' { Start-Process $output.Trim() }
  'updates' {
   if ($output -match '(https://github.com/DrakonWolfDev/dayz-bank-viewer/releases/tag/[a-zA-Z0-9.]+)') {
    $url = $Matches[1]
    if ([Windows.Forms.MessageBox]::Show($output + "`nOpen the release page?",'Updates','YesNo') -eq 'Yes') { Start-Process $url }
   } else { [void][Windows.Forms.MessageBox]::Show($output,'Updates') }
  }
 }
}
function Show-About {
 $about = New-Object System.Windows.Forms.Form
 $about.Text = 'About DayZ Bank Viewer'
 $about.ClientSize = New-Object System.Drawing.Size(540,250)
 $about.BackColor = $bg
 $about.ForeColor = $fg
 $about.AutoScaleMode = 'Dpi'
 $about.Font = $form.Font
 $about.StartPosition = 'CenterParent'
 $text = New-Object System.Windows.Forms.Label
 $text.Location = New-Object System.Drawing.Point(20,20)
 $text.Size = New-Object System.Drawing.Size(500,100)
 $text.Text = "DayZ Bank Viewer $env:VIEWER_VERSION`nA Wolf Forge Project 🐾`nRead-only local banking dashboard.`nGNU Affero General Public License v3 only."
 $about.Controls.Add($text)
 $forge = New-Object System.Windows.Forms.LinkLabel
 $forge.Text = 'Wolf Forge'
 $forge.Location = New-Object System.Drawing.Point(20,130)
 $forge.AutoSize = $true
 $forge.Add_LinkClicked({ Start-Process 'https://wolf-forge.co.uk/' })
 $forge.LinkColor = $gold
 $forge.ActiveLinkColor = $fg
 $about.Controls.Add($forge)
 $licence = New-Object System.Windows.Forms.LinkLabel
 $licence.Text = 'Licence (AGPL-3.0-only) and corresponding source'
 $licence.Location = New-Object System.Drawing.Point(20,170)
 $licence.AutoSize = $true
 $licence.Add_LinkClicked({ Start-Process 'https://github.com/DrakonWolfDev/dayz-bank-viewer/blob/main/LICENSE' })
 $licence.LinkColor = $gold
 $licence.ActiveLinkColor = $fg
 $about.Controls.Add($licence)
 [void]$about.ShowDialog($form)
}
$buttons = @('Open dashboard','Start viewer','Stop viewer','Restart viewer','Run without signing in…','Switch back to desktop mode','Check for updates','About and licence')
foreach ($caption in $buttons) {
 $button = New-Object System.Windows.Forms.Button
 $button.Text = $caption
 $button.AccessibleName = $caption
 $button.AutoSize = $true
 $button.MinimumSize = New-Object System.Drawing.Size(166,42)
 $button.Padding = New-Object System.Windows.Forms.Padding(10,6,10,6)
 $button.Margin = New-Object System.Windows.Forms.Padding(0,0,8,0)
 if (!$hc) {
  $button.FlatStyle = 'Flat'
  $button.FlatAppearance.BorderColor = $border
  $button.FlatAppearance.MouseOverBackColor = [System.Drawing.ColorTranslator]::FromHtml('#3B4047')
  $button.FlatAppearance.MouseDownBackColor = [System.Drawing.ColorTranslator]::FromHtml('#454C55')
  $button.BackColor = $surface
  $button.ForeColor = $fg
 }
 $actionButtons[$caption] = $button
 $button.Tag = switch ($caption) {
  'Open dashboard' { 'open' }
  'Start viewer' { 'start' }
  'Stop viewer' { 'stop' }
  'Restart viewer' { 'restart' }
  'Run without signing in…' { 'enable' }
  'Switch back to desktop mode' { 'disable' }
  'Check for updates' { 'updates' }
  default { 'about' }
 }
 $button.Add_Click({
  switch ($this.Tag) {
   'about' { Show-About }
   'open' { Begin-Action 'start' 'open-start' }
   'updates' { Begin-Action 'updates' 'updates' }
   default { Begin-Action $this.Tag }
  }
 })
 switch ($caption) {
  'Open dashboard' {
   $button.Dock = 'Top'
   $button.MinimumSize = New-Object System.Drawing.Size(0,48)
   $button.Font = New-Object System.Drawing.Font('Segoe UI',11,[System.Drawing.FontStyle]::Bold)
   if (!$hc) {
    $button.BackColor = $gold
    $button.ForeColor = $bg
    $button.FlatAppearance.BorderColor = $gold
    $button.FlatAppearance.MouseOverBackColor = [System.Drawing.ColorTranslator]::FromHtml('#E8C487')
    $button.FlatAppearance.MouseDownBackColor = [System.Drawing.ColorTranslator]::FromHtml('#C49C54')
   }
   Add-Row $statusCard $button
   $form.AcceptButton = $button
  }
  { $_ -in 'Start viewer','Stop viewer','Restart viewer' } { $viewerActions.Controls.Add($button) }
  { $_ -in 'Run without signing in…','Switch back to desktop mode' } {
   $button.Dock = 'Top'
   if (!$hc) { $button.FlatAppearance.BorderColor = $gold }
   Add-Row $startupActions $button
  }
  default { $otherActions.Controls.Add($button) }
 }
}
Add-Row $statusCard $viewerActions
Add-Row $layout $statusCard
Add-Row $startupCard $startupActions
$hint = New-Text 'Service setup requires administrator permission.' 9
$hint.ForeColor = $muted
Add-Row $startupCard $hint
Add-Row $layout $startupCard
Add-Row $layout $otherActions
$footer = New-Text 'Closing this window leaves the viewer running.' 9
$footer.ForeColor = $muted
Add-Row $layout $footer
$signature = New-Text 'A Wolf Forge Project' 9
$signature.ForeColor = $muted
Add-Row $layout $signature
$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 150
$timer.Add_Tick({
 try {
  if (!(Get-Process -Id ([int]$env:VIEWER_PARENT_PID) -ErrorAction SilentlyContinue)) { $form.Close(); return }
  if (Request-Done $script:pending) {
   $completed = $script:pending
   $script:pending = $null
   try { Finish-Action $completed } finally { $completed.Process.Dispose() }
   $script:lastProbe = [DateTime]::MinValue
  }
  if (Request-Done $script:probe) {
   $completed = $script:probe
   $script:probe = $null
   try {
    if ($completed.Process.ExitCode -ne 0) { throw $completed.Error.Result }
    Apply-Status ($completed.Output.Result | ConvertFrom-Json)
   } finally { $completed.Process.Dispose() }
  }
  if (!$script:pending -and ([DateTime]::UtcNow - $script:lastProbe).TotalSeconds -ge 3) { Request-Status }
 } catch {
  $statusLabel.Text = 'Status check failed'
  $accountLabel.Text = $_.Exception.Message
  foreach ($button in $actionButtons.Values) { $button.Enabled = !$script:pending }
 }
})
$timer.Start()
try {
 Request-Status
 [void]$form.ShowDialog()
} finally {
 $timer.Dispose()
 foreach ($request in @($script:pending,$script:probe)) { if ($request) { $request.Process.Dispose() } }
 $form.Dispose()
 if ($logoImage) { $logoImage.Dispose() }
}
