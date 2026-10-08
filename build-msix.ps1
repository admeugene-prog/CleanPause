param(
    [switch] $SkipAppBuild,
    [string] $Version = '1.0.1',
    [string] $IdentityName = 'EvgeniiALEKSEEV.CleanPause',
    [string] $Publisher = 'CN=6E79EBE5-4570-4408-87D0-17F295D9EA33',
    [string] $PublisherDisplayName = 'Evgenii ALEKSEEV'
)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
if ($Version -notmatch '^\d+\.\d+\.\d+(\.0)?$') { throw 'Use a three-component version or a Store version ending in .0.' }
$parts = $Version.Split('.')
foreach ($part in $parts) { if ([int64]$part -gt 65535) { throw 'MSIX version components must be at most 65535.' } }
if ([int]$parts[0] -lt 1) { throw 'MSIX major version must be at least 1.' }
if ($parts.Length -eq 3) { $Version += '.0' }
if ($IdentityName -notmatch '^[A-Za-z0-9][A-Za-z0-9.-]{2,49}$') { throw 'Invalid package identity name.' }
if ([string]::IsNullOrWhiteSpace($Publisher) -or [string]::IsNullOrWhiteSpace($PublisherDisplayName)) { throw 'Publisher values must not be empty.' }
if (-not $SkipAppBuild) { & "$PSScriptRoot\build.ps1" }
$exe = Join-Path $PSScriptRoot 'dist\CleanPause.exe'
if (-not (Test-Path -LiteralPath $exe) -or (Get-Item -LiteralPath $exe).Length -eq 0) { throw 'Build the application first.' }

# Pin the official Microsoft SDK tools; no Visual Studio installation is needed.
$sdkVersion = '10.0.26100.9169'
$sdkDir = Join-Path $env:LOCALAPPDATA "CleanPauseBuildTools\WindowsSDK-$sdkVersion"
$makeAppx = Join-Path $sdkDir 'bin\10.0.26100.0\x64\makeappx.exe'
if (-not (Test-Path -LiteralPath $makeAppx)) {
    New-Item -ItemType Directory -Path $sdkDir -Force | Out-Null
    $archive = Join-Path $sdkDir 'sdk.zip'
    Invoke-WebRequest -Uri "https://api.nuget.org/v3-flatcontainer/microsoft.windows.sdk.buildtools/$sdkVersion/microsoft.windows.sdk.buildtools.$sdkVersion.nupkg" -OutFile $archive
    Expand-Archive -LiteralPath $archive -DestinationPath $sdkDir -Force
    Remove-Item -LiteralPath $archive
}
if (-not (Test-Path -LiteralPath $makeAppx)) { throw 'Microsoft MakeAppx tool missing.' }
$signature = Get-AuthenticodeSignature -LiteralPath $makeAppx
if ($signature.Status -ne 'Valid' -or $signature.SignerCertificate.Subject -notmatch 'O=Microsoft Corporation') { throw 'Microsoft MakeAppx signature verification failed.' }

# Use a new staging directory, so previous packages cannot leave extra payload.
$stage = Join-Path ([IO.Path]::GetTempPath()) ('CleanPause-MSIX-' + [guid]::NewGuid())
try {
    New-Item -ItemType Directory -Path (Join-Path $stage 'Assets') -Force | Out-Null
    Copy-Item -LiteralPath $exe -Destination (Join-Path $stage 'CleanPause.exe')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'LICENSE') -Destination $stage
    foreach ($logo in @('Square44x44Logo', 'Square150x150Logo', 'StoreLogo')) {
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot "assets\icons\$logo.png") -Destination (Join-Path $stage 'Assets')
    }
    [xml]$manifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'msix\AppxManifest.xml') -Raw
    $manifest.Package.Identity.Name = $IdentityName
    $manifest.Package.Identity.Publisher = $Publisher
    $manifest.Package.Identity.Version = $Version
    $manifest.Package.Properties.PublisherDisplayName = $PublisherDisplayName
    $manifest.Save((Join-Path $stage 'AppxManifest.xml'))
    $output = Join-Path $PSScriptRoot 'dist\CleanPause.msix'
    & $makeAppx pack /d $stage /p $output /o
    if ($LASTEXITCODE -ne 0) { throw 'MSIX validation or packaging failed.' }
    Write-Host "Built unsigned MSIX: $output"
    Write-Host "Identity: $IdentityName; publisher: $Publisher; version: $Version"
} finally {
    # The staging directory is uniquely created above, never a caller-supplied path.
    $resolvedStage = [IO.Path]::GetFullPath($stage)
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if (-not $resolvedStage.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or
        (Split-Path $resolvedStage -Leaf) -notmatch '^CleanPause-MSIX-[0-9a-f-]{36}$') {
        throw 'Staging cleanup path is outside the expected temporary directory.'
    }
    if (Test-Path -LiteralPath $resolvedStage) { Remove-Item -LiteralPath $resolvedStage -Recurse -Force }
}
