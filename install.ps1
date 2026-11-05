param(
    [string]$Version = $env:VERSION,
    [string]$InstallDir = $env:INSTALL_DIR
)

$ErrorActionPreference = 'Stop'
$repo = 'https://github.com/OliverChoi-iOS/redmine-cli'
$tempDir = $null
$stagedBinary = $null
try {
    if ($env:OS -ne 'Windows_NT') { throw 'Use install.sh on macOS or Linux.' }
    if (-not $InstallDir) { $InstallDir = Join-Path $env:LOCALAPPDATA 'redmine-cli\bin' }
    $cpu = $env:PROCESSOR_ARCHITEW6432
    if (-not $cpu) { $cpu = $env:PROCESSOR_ARCHITECTURE }
    switch ($cpu) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'arm64' }
        default { throw "Unsupported CPU: $cpu. Supported: AMD64, ARM64." }
    }
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    if (-not $Version) {
        $release = Invoke-RestMethod 'https://api.github.com/repos/OliverChoi-iOS/redmine-cli/releases/latest'
        $Version = $release.tag_name
        if (-not $Version) { throw 'No stable release found.' }
    }
    if (-not $Version.StartsWith('v')) { $Version = "v$Version" }
    if ($Version -cnotmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw 'Expected a stable version such as v0.1.0.' }
    $number = $Version.Substring(1)
    $archive = "redmine-cli_${number}_windows_${arch}.zip"
    $download = "$repo/releases/download/$Version"
    $tempDir = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    $archivePath = Join-Path $tempDir $archive
    $checksumsPath = Join-Path $tempDir 'checksums.txt'
    Invoke-WebRequest "$download/$archive" -OutFile $archivePath -UseBasicParsing
    Invoke-WebRequest "$download/checksums.txt" -OutFile $checksumsPath -UseBasicParsing
    $pattern = '^([a-f0-9]{64})  ' + [regex]::Escape($archive) + '$'
    $entries = @(Get-Content $checksumsPath | Select-String -Pattern $pattern)
    if ($entries.Count -ne 1) { throw 'Missing or invalid checksum.' }
    $expected = $entries[0].Matches[0].Groups[1].Value
    if ((Get-FileHash $archivePath -Algorithm SHA256).Hash -ine $expected) { throw 'Checksum verification failed.' }
    Expand-Archive -LiteralPath $archivePath -DestinationPath $tempDir
    $binary = Join-Path $tempDir 'redmine.exe'
    $reportedVersion = & $binary --version
    if ($LASTEXITCODE -ne 0 -or $reportedVersion -cne $number) { throw 'Binary version check failed.' }
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $target = Join-Path $InstallDir 'redmine.exe'
    if (Test-Path $target -PathType Container) { throw 'Install target is a directory.' }
    $stagedBinary = Join-Path $InstallDir ([Guid]::NewGuid().ToString() + '.exe')
    Copy-Item -LiteralPath $binary -Destination $stagedBinary
    Move-Item -LiteralPath $stagedBinary -Destination $target -Force
    $stagedBinary = $null
    Write-Host "Installed redmine $number to $target"
    Write-Host "Add $InstallDir to your user PATH to run redmine from any terminal."
} finally {
    if ($stagedBinary -and (Test-Path $stagedBinary)) { Remove-Item -LiteralPath $stagedBinary -Force }
    if ($tempDir -and (Test-Path $tempDir)) { Remove-Item -LiteralPath $tempDir -Recurse -Force }
}
