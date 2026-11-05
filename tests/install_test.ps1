$ErrorActionPreference = 'Stop'
$repoDir = Split-Path $PSScriptRoot -Parent
$fixture = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path (Join-Path $fixture 'source') | Out-Null
try {
    $binary = Join-Path $fixture 'source/redmine.exe'
    & go build -ldflags '-X redmine-cli/internal/cli.Version=0.1.0' -o $binary ./cmd/redmine
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed.' }
    $cpu = $env:PROCESSOR_ARCHITEW6432
    if (-not $cpu) { $cpu = $env:PROCESSOR_ARCHITECTURE }
    $arch = if ($cpu -eq 'ARM64') { 'arm64' } else { 'amd64' }
    $archive = "redmine-cli_0.1.0_windows_${arch}.zip"
    $archivePath = Join-Path $fixture $archive
    Compress-Archive -LiteralPath $binary -DestinationPath $archivePath
    $hash = (Get-FileHash $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    $checksums = Join-Path $fixture 'checksums.txt'
    Set-Content $checksums "$hash  $archive"
    function Invoke-RestMethod { return @{ tag_name = 'v0.1.0' } }
    function Invoke-WebRequest {
        param($Uri, $OutFile, [switch]$UseBasicParsing)
        if ($failDownload) { throw 'Simulated download failure.' }
        if ($Uri -notlike 'https://github.com/OliverChoi-iOS/redmine-cli/releases/download/v0.1.0/*') {
            throw "Unexpected URL: $Uri"
        }
        Copy-Item -LiteralPath (Join-Path $fixture ($Uri.Split('/')[-1])) -Destination $OutFile
    }
    $installDir = Join-Path $fixture 'bin with spaces'
    & "$repoDir/install.ps1" -Version 0.1.0 -InstallDir $installDir
    & "$repoDir/install.ps1" -Version '' -InstallDir $installDir
    $target = Join-Path $installDir 'redmine.exe'
    if ((& $target --version) -ne '0.1.0') { throw 'Installed version mismatch.' }
    $original = (Get-FileHash $target).Hash
    foreach ($failure in @('version', 'download', 'checksum', 'missing-checksum')) {
        $failDownload = $failure -eq 'download'
        $version = if ($failure -eq 'version') { 'bad' } else { 'v0.1.0' }
        if ($failure -eq 'checksum') { Add-Content $archivePath 'corrupt' }
        if ($failure -eq 'missing-checksum') { Set-Content $checksums '' }
        $failed = $false
        $expectedError = switch ($failure) {
            'version' { 'Expected a stable version' }
            'download' { 'Simulated download failure' }
            'checksum' { 'Checksum verification failed' }
            'missing-checksum' { 'Missing or invalid checksum' }
        }
        try { & "$repoDir/install.ps1" -Version $version -InstallDir $installDir } catch {
            $failed = $true
            if (-not $_.Exception.Message.Contains($expectedError)) { throw }
        }
        if (-not $failed) { throw "Expected failure: $failure" }
        if ((Get-FileHash $target).Hash -ne $original) { throw 'Failed install changed the existing binary.' }
    }
    Write-Host 'Windows installer checks passed.'
} finally {
    Remove-Item -LiteralPath $fixture -Recurse -Force
}
