param([switch] $SkipAppBuild)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
if (-not $SkipAppBuild) { & "$PSScriptRoot\build.ps1" -Sign }
$exe = Join-Path $PSScriptRoot 'dist\CleanPause.exe'
if (-not (Test-Path -LiteralPath $exe) -or (Get-Item -LiteralPath $exe).Length -eq 0) { throw 'Build the application first.' }
if (-not (Get-AuthenticodeSignature -LiteralPath $exe).SignerCertificate) { throw 'Application must be signed before packaging.' }
$compilerDir = Join-Path $env:LOCALAPPDATA 'CleanPauseBuildTools\InnoSetup-6.7.3'
$compiler = Join-Path $compilerDir 'ISCC.exe'
if (-not (Test-Path -LiteralPath $compiler)) {
    New-Item -ItemType Directory -Path $compilerDir -Force | Out-Null
    $download = Join-Path $compilerDir 'innosetup-6.7.3.exe'
    Invoke-WebRequest -Uri 'https://github.com/jrsoftware/issrc/releases/download/is-6_7_3/innosetup-6.7.3.exe' -OutFile $download
    $signature = Get-AuthenticodeSignature -LiteralPath $download
    if ($signature.Status -ne 'Valid' -or $signature.SignerCertificate.Subject -notmatch 'Pyrsys B.V.') { throw 'Official compiler signature verification failed.' }
    $arguments = @('/PORTABLE=1', '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/CURRENTUSER', '/NOICONS', ('/DIR="' + $compilerDir + '"'))
    $process = Start-Process -FilePath $download -ArgumentList $arguments -WindowStyle Hidden -Wait -PassThru
    if ($process.ExitCode -ne 0) { throw "Compiler extraction failed: $($process.ExitCode)" }
}
if (-not (Test-Path -LiteralPath $compiler)) { throw 'Inno Setup compiler missing.' }
$shellExe = (Get-Process -Id $PID).Path
$signScript = Join-Path $PSScriptRoot 'tools\Sign-Executable.ps1'
$signCommand = '/Scleanpause=$q' + $shellExe + '$q -NoProfile -NonInteractive -File $q' + $signScript + '$q -Path $f'
& $compiler $signCommand (Join-Path $PSScriptRoot 'installer\CleanPause.iss')
if ($LASTEXITCODE -ne 0) { throw 'Installer compilation failed.' }
$setup = Join-Path $PSScriptRoot 'dist\CleanPause-Setup.exe'
if (-not (Get-AuthenticodeSignature -LiteralPath $setup).SignerCertificate) { throw 'Installer signature missing.' }
Write-Host "Built and signed: $setup"
