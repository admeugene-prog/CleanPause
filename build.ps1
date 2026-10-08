param([switch] $Sign)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot
go run ./tools/resources
if ($LASTEXITCODE -ne 0) { throw 'Resource generation failed' }
go test -short ./...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Vet failed' }
New-Item -ItemType Directory -Path dist -Force | Out-Null
go build -trimpath -ldflags '-H=windowsgui -s -w' -o dist/CleanPause.exe ./cmd/cleanpause
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
if ($Sign) { & "$PSScriptRoot\tools\Sign-Executable.ps1" -Path "$PSScriptRoot\dist\CleanPause.exe" }
Write-Host 'Built dist/CleanPause.exe'
