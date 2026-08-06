$ErrorActionPreference = 'Stop'

$identity =
    [Security.Principal.WindowsIdentity]::GetCurrent()

$principal =
    [Security.Principal.WindowsPrincipal]::new(
        $identity
    )

if (-not $principal.IsInRole(
    [Security.Principal.WindowsBuiltInRole]::Administrator
)) {
    throw 'R6-5 verification requires Administrator PowerShell.'
}

$backendRoot =
    Split-Path -Parent $PSScriptRoot

$artifactDirectory =
    Join-Path $backendRoot 'artifacts\r6-5'

New-Item `
    -ItemType Directory `
    -Path $artifactDirectory `
    -Force |
    Out-Null

Push-Location $backendRoot

try {
    $goVersion =
        (& go env GOVERSION).Trim()

    if ($goVersion -ne 'go1.26.1') {
        throw "Expected Go go1.26.1, actual $goVersion"
    }

    @(
        "Timestamp=$([DateTime]::UtcNow.ToString('O'))"
        "OS=$([Environment]::OSVersion.VersionString)"
        "GoVersion=$goVersion"
        "GOARCH=$(& go env GOARCH)"
        "CGO_ENABLED=$(& go env CGO_ENABLED)"
    ) |
        Set-Content `
            -Path (
                Join-Path `
                    $artifactDirectory `
                    'environment.txt'
            )

    go test `
        ./internal/extension/kernel/... `
        -run '^$' `
        -count=1 `
        2>&1 |
        Tee-Object `
            -FilePath (
                Join-Path `
                    $artifactDirectory `
                    'windows-compile.log'
            )

    if ($LASTEXITCODE -ne 0) {
        throw 'Windows compilation failed.'
    }

    go test `
        ./internal/extension/kernel/... `
        -run '^TestR64|^TestR65|^TestR6_4|^TestR6_5' `
        -count=1 `
        -v `
        2>&1 |
        Tee-Object `
            -FilePath (
                Join-Path `
                    $artifactDirectory `
                    'windows-tests.log'
            )

    if ($LASTEXITCODE -ne 0) {
        throw 'Windows R6-4/R6-5 tests failed.'
    }

    go test `
        ./internal/extension/kernel/... `
        -run 'R6_5.*Junction|R65.*Junction' `
        -count=20 `
        -v `
        2>&1 |
        Tee-Object `
            -FilePath (
                Join-Path `
                    $artifactDirectory `
                    'windows-junction-20.log'
            )

    if ($LASTEXITCODE -ne 0) {
        throw 'Windows Junction repetition tests failed.'
    }

    & (
        Join-Path `
            $PSScriptRoot `
            'run-r65-windows-mount-tests.ps1'
    )

    go test -race `
        ./internal/extension/kernel/... `
        -run '^TestR64|^TestR65|^TestR6_4|^TestR6_5' `
        -count=1 `
        -v `
        2>&1 |
        Tee-Object `
            -FilePath (
                Join-Path `
                    $artifactDirectory `
                    'windows-race.log'
            )

    if ($LASTEXITCODE -ne 0) {
        throw 'Windows Race Test failed.'
    }

    'PASS' |
        Set-Content `
            -Path (
                Join-Path `
                    $artifactDirectory `
                    'result.txt'
            )
}
finally {
    Pop-Location
}
