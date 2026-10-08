$ErrorActionPreference = 'Stop'
$openssl = 'C:\Program Files\OpenSSL-Win64\bin\openssl.exe'
if (-not (Test-Path -LiteralPath $openssl)) { $openssl = (Get-Command openssl -ErrorAction Stop).Source }
$private = Join-Path $env:LOCALAPPDATA 'CleanPause\Signing'
$public = Join-Path (Split-Path $PSScriptRoot -Parent) 'signing'
if (Test-Path -LiteralPath (Join-Path $private 'codesign.pfx')) { throw 'Certificate already exists; refusing to replace keys.' }
New-Item -ItemType Directory -Path $private,$public -Force | Out-Null
$sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
& icacls.exe $private /grant:r "*$($sid.Value):(OI)(CI)F" '*S-1-5-18:(OI)(CI)F' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot restrict private directory permissions.' }
& icacls.exe $private /inheritance:r | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot disable inherited permissions.' }
$bytes = New-Object byte[] 48
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
$rng.Dispose()
$password = [Convert]::ToBase64String($bytes)
$passfile = Join-Path $private 'password.tmp'
$exportPassfile = Join-Path $private 'export-password.tmp'
[IO.File]::WriteAllText($passfile, $password, [Text.Encoding]::ASCII)
[IO.File]::WriteAllText($exportPassfile, $password, [Text.Encoding]::ASCII)
ConvertTo-SecureString $password -AsPlainText -Force | ConvertFrom-SecureString | Set-Content -LiteralPath (Join-Path $private 'password.dpapi')
$p = $private.Replace('\','/')
$config = @"
[ ca ]
default_ca = local_ca
[ local_ca ]
database = $p/index.txt
new_certs_dir = $p/issued
certificate = $p/root-ca.crt
private_key = $p/root-ca.key
serial = $p/serial
default_md = sha256
default_days = 365
policy = local_policy
x509_extensions = codesign
unique_subject = no
copy_extensions = none
[ local_policy ]
commonName = supplied
organizationName = optional
[ req ]
distinguished_name = dn
[ dn ]
[ root_ca ]
basicConstraints = critical,CA:true,pathlen:0
keyUsage = critical,keyCertSign,cRLSign
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always
[ codesign ]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = codeSigning
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
"@
$configPath = Join-Path $private 'openssl.cnf'
[IO.File]::WriteAllText($configPath, $config, [Text.Encoding]::ASCII)
New-Item -ItemType Directory -Path (Join-Path $private 'issued') -Force | Out-Null
[IO.File]::WriteAllText((Join-Path $private 'index.txt'), '')
[IO.File]::WriteAllText((Join-Path $private 'serial'), '1000')
function Invoke-OpenSSL([string[]] $Arguments) {
    & $openssl @Arguments
    if ($LASTEXITCODE -ne 0) { throw 'OpenSSL operation failed.' }
}
Push-Location $private
try {
    Invoke-OpenSSL @('genpkey','-algorithm','RSA','-pkeyopt','rsa_keygen_bits:4096','-aes-256-cbc','-pass',"file:$passfile",'-out','root-ca.key')
    Invoke-OpenSSL @('req','-new','-x509','-sha256','-days','3650','-config',$configPath,'-extensions','root_ca','-key','root-ca.key','-passin',"file:$passfile",'-subj','/CN=CleanPause Local Root CA/O=CleanPause','-out','root-ca.crt')
    Invoke-OpenSSL @('genpkey','-algorithm','RSA','-pkeyopt','rsa_keygen_bits:3072','-aes-256-cbc','-pass',"file:$passfile",'-out','codesign.key')
    Invoke-OpenSSL @('req','-new','-sha256','-config',$configPath,'-key','codesign.key','-passin',"file:$passfile",'-subj','/CN=CleanPause Local Code Signing/O=CleanPause','-out','codesign.csr')
    Invoke-OpenSSL @('ca','-batch','-config',$configPath,'-passin',"file:$passfile",'-in','codesign.csr','-out','codesign.crt','-notext')
    Invoke-OpenSSL @('verify','-CAfile','root-ca.crt','codesign.crt')
    Invoke-OpenSSL @('pkcs12','-export','-inkey','codesign.key','-passin',"file:$passfile",'-in','codesign.crt','-certfile','root-ca.crt','-name','CleanPause Code Signing','-passout',"file:$exportPassfile",'-out','codesign.pfx')
    Copy-Item -LiteralPath 'root-ca.crt','codesign.crt' -Destination $public
} finally {
    Pop-Location
    Remove-Item -LiteralPath $passfile -Force
    Remove-Item -LiteralPath $exportPassfile -Force
    $password = $null
}
Write-Host "Public certificates: $public"
Write-Host "Private keys and PFX: $private (password protected with Windows DPAPI for this user)"
