[CmdletBinding()]
param(
    [string[]]$Paths = @("backend/internal/desktoppet", "backend/cmd/server"),
    [switch]$Json
)

$ErrorActionPreference = "Stop"

$forbiddenPatterns = @(
    @{ Pattern = 'ioutil\.ReadFile|ioutil\.WriteFile'; Reason = 'use os.ReadFile/os.ReadFile instead of deprecated ioutil (post go1.16)' },
    @{ Pattern = '(?:db|tx|sql|DB|Tx)\.(?:Exec|Query|QueryRow|Prepare|Select|Get|First|Find|Update|Delete|Create|Scan|Transact)\s*\(\s*fmt\.Sprintf'; Reason = 'SQL must not be built via fmt.Sprintf (SQL injection risk)' },
    @{ Pattern = 'fmt\.Sprintf\s*\(\s*["`''](?:\s*(?:SELECT|INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|EXEC(?:UTE)?)\s)'; Reason = 'SQL must not be built via fmt.Sprintf (SQL injection risk)' },
    @{ Pattern = '\b(MigrationRepo|JournalRepo|Repository|Repo)\.\w+\([^,)]*,\s*fmt\.Sprintf'; Reason = 'Repository method called with fmt.Sprintf-built argument (possible SQL injection)' },
    @{ Pattern = '\bsha1\.New\b|\bmd5\.New\b|\bmd5\.Sum\b'; Reason = 'weak hash: use crypto/sha256 or stronger (md5/sha1 are insecure)' },
    @{ Pattern = '\bDES\.|\bRC4\.'; Reason = 'weak cipher: use AES-GCM / SHA-256 or stronger' }
)

$findings = [System.Collections.ArrayList]::new()
$scanned = 0

foreach ($root in $Paths) {
    if (-not (Test-Path $root)) { continue }
    $files = Get-ChildItem -Recurse -Filter "*.go" -Path $root -File |
             Where-Object { $_.FullName -notmatch '\\_test\.go$' -and $_.FullName -notmatch '\\vendor\\' }
    foreach ($f in $files) {
        $scanned++
        $lineNum = 0
        foreach ($line in Get-Content -Path $f.FullName -Encoding UTF8) {
            $lineNum++
            foreach ($fp in $forbiddenPatterns) {
                if ($line -match $fp.Pattern) {
                    $findings.Add([PSCustomObject]@{
                        File   = $f.FullName
                        Line   = $lineNum
                        Reason = $fp.Reason
                        Match  = $line.Trim()
                    })
                }
            }
        }
    }
}

if ($Json) {
    [PSCustomObject]@{
        Scanned   = $scanned
        Findings  = $findings.Count
        Details   = $findings
    } | ConvertTo-Json -Depth 6
    return
}

Write-Output "desktop_pet_forbidden_patterns: scanned=$scanned findings=$($findings.Count)"
foreach ($d in $findings) {
    Write-Output "$($d.File):$($d.Line): $($d.Reason)"
    Write-Output "  $($d.Match)"
}
exit $findings.Count
