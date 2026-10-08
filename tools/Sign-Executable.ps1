param([Parameter(Mandatory=$true)][string] $Path)
$ErrorActionPreference = 'Stop'
$target = (Resolve-Path -LiteralPath $Path).Path
if ((Get-Item -LiteralPath $target).Length -eq 0) { throw 'EXE is empty; check antivirus quarantine before signing.' }
$private = Join-Path $env:LOCALAPPDATA 'CleanPause\Signing'
$password = Get-Content -LiteralPath (Join-Path $private 'password.dpapi') | ConvertTo-SecureString
$cert = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new(
    (Join-Path $private 'codesign.pfx'), $password,
    [System.Security.Cryptography.X509Certificates.X509KeyStorageFlags]::DefaultKeySet)
try {
    $result = Set-AuthenticodeSignature -LiteralPath $target -Certificate $cert -HashAlgorithm SHA256 -IncludeChain All
    $check = Get-AuthenticodeSignature -LiteralPath $target
    if (-not $check.SignerCertificate -or $check.SignerCertificate.Thumbprint -ne $cert.Thumbprint) {
        throw "Signing failed: $($result.Status) $($result.StatusMessage)"
    }
    if ($check.Status -notin @('Valid','UnknownError','NotTrusted')) { throw "Signature verification failed: $($check.StatusMessage)" }
    Write-Host "Signed: $target"
    Write-Host "Signer: $($check.SignerCertificate.Subject)"
    Write-Host "Windows verification: $($check.Status) — $($check.StatusMessage)"
} finally { $cert.Dispose() }
