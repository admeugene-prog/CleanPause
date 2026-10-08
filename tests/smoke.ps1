$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$exePath = Join-Path $projectRoot 'dist/CleanPause.exe'
if (Get-Process CleanPause -ErrorAction SilentlyContinue) { throw 'Close the running CleanPause before this isolated smoke test' }
$configPath = Join-Path $PSScriptRoot 'ui-config.json'
$fixture = '{"version":1,"autostart":false,"language":"ru","theme":"light","notify_completion":true,"cleaning":{"duration_seconds":60,"preparation_seconds":3},"reminders":{"enabled":false,"schedule":{"type":"workdays","days":[1,2,3,4,5],"time":"10:00"},"snooze_minutes":15}}'
Set-Content -LiteralPath $configPath -Value $fixture -Encoding utf8
$arguments = @('--simulate-input', '--show-settings', '--config', ('"' + $configPath + '"'))
$first = Start-Process -FilePath $exePath -ArgumentList $arguments -WindowStyle Hidden -PassThru
try {
    $limit = [DateTime]::UtcNow.AddSeconds(8)
    do {
        Start-Sleep -Milliseconds 250
        $first.Refresh()
        if ($first.HasExited) { throw "GUI exited: $($first.ExitCode)" }
    } while ($first.MainWindowHandle -eq 0 -and [DateTime]::UtcNow -lt $limit)
    if ($first.MainWindowHandle -eq 0 -or -not $first.Responding) { throw 'GUI did not open a responding window' }
    $second = Start-Process -FilePath $exePath -ArgumentList $arguments -WindowStyle Hidden -PassThru
    if (-not $second.WaitForExit(5000)) { Stop-Process -Id $second.Id; throw 'Second instance did not exit' }
    if ($second.ExitCode -ne 0) { throw "Second instance failed: $($second.ExitCode)" }
    $first.Refresh()
    Write-Host "PASS: GUI is responsive, second instance exited; working set $([Math]::Round($first.WorkingSet64 / 1MB, 2)) MiB"
} finally {
    if (-not $first.HasExited) { Stop-Process -Id $first.Id }
}
