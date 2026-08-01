# Desktop Pet Forbidden Patterns Audit Script
# Scans for security anti-patterns in Go source code

param(
    [string[]]$Paths = @("backend/internal/desktoppet", "backend/cmd/server"),
    [string]$OutputFile = ""
)

$ErrorActionPreference = "Stop"

$patterns = @(
    @{ Pattern = 'return\s+"default"'; Category = "auth_fail_open"; Severity = "critical"; Description = "Auth fail-open returning default" },
    @{ Pattern = '(?m)(?<!func\s)(?<!\.)\bResolveUserID\s*\((?!ctx)'; Category = "deprecated_auth"; Severity = "critical"; Description = "Deprecated ResolveUserID in handler (gin Context signature)" },
    @{ Pattern = 'c\.Param\("userId"\)|c\.Query\("userId"\)'; Category = "insecure_userid"; Severity = "critical"; Description = "Trusting userID from URL query" },
    @{ Pattern = 'c\.File\(fullPath\)|c\.File\(path\)|c\.File\(srcPath\)'; Category = "unsafe_file_serve"; Severity = "critical"; Description = "Serving raw file path" },
    @{ Pattern = 'os\.RemoveAll\('; Category = "unsafe_delete"; Severity = "critical"; Description = "Unprotected os.RemoveAll" },
    @{ Pattern = 'copyDirContents\('; Category = "unsafe_copy"; Severity = "critical"; Description = "Unprotected directory copy" },
    @{ Pattern = 'make\(\[\]byte,\s*file\.Size\)'; Category = "unsafe_memory"; Severity = "high"; Description = "Memory allocation based on file.Size header" },
    @{ Pattern = 'time\.Now\(\)\.UnixNano\(\s*\)'; Category = "unsafe_revision_id"; Severity = "medium"; Description = "Clock-based revision ID" },
    @{ Pattern = 'TaskEventsStream|ProcessingEventsStream'; Category = "sse_check"; Severity = "info"; Description = "Verify SSE handler ownership" },
    @{ Pattern = 'SetRevisionPromoter|ImportLegacyRevision|NewReleaseBuilder|NewInstaller'; Category = "legacy_chain"; Severity = "high"; Description = "Legacy production writer invocation" }
)

$findings = @()
$totalViolations = 0

function Test-IsInDeprecatedFunction {
    param([string[]]$Lines, [int]$LineIndex)
    $start = [Math]::Max(0, $lineIndex - 5)
    for ($i = $start; $i -le $lineIndex; $i++) {
        if ($Lines[$i] -match '//\s*DEPRECATED') { return $true }
    }
    return $false
}

foreach ($p in $Paths) {
    if (-not (Test-Path $p)) {
        Write-Warning "Path not found: $p"
        continue
    }

    $files = Get-ChildItem -Path $p -Recurse -Filter "*.go"
    foreach ($f in $files) {
        $content = Get-Content -Path $f.FullName -Raw -ErrorAction SilentlyContinue
        if (-not $content) { continue }

        $lines = $content -split "`n"
        foreach ($pat in $patterns) {
            $regex = [regex]::new($pat.Pattern, [System.Text.RegularExpressions.RegexOptions]::Singleline)
            $matches = $regex.Matches($content)
            foreach ($m in $matches) {
                $lineNum = $content.Substring(0, $m.Index).Split("`n").Length
                $lineText = $lines[$lineNum - 1]
                if ($lineText -match '// audit:ok:') { continue }
                if (Test-IsInDeprecatedFunction -Lines $lines -LineIndex ($lineNum - 1)) { continue }
                $totalViolations++
                $finding = [PSCustomObject]@{
                    File     = $f.FullName
                    Line     = $lineNum
                    Pattern  = $pat.Pattern
                    Category = $pat.Category
                    Severity = $pat.Severity
                    Source   = $lineText.Trim()
                    Message  = $pat.Description
                }
                $findings += $finding
            }
        }
    }
}

$result = [PSCustomObject]@{
    TotalViolations = $totalViolations
    Findings = $findings
    Categories = $findings | Group-Object -Property Category | ForEach-Object {
        [PSCustomObject]@{ Category = $_.Name; Count = $_.Count }
    }
}

if ($OutputFile) {
    $result | ConvertTo-Json -Depth 5 | Set-Content -Path $OutputFile
} else {
    Write-Output "=== Desktop Pet Forbidden Patterns Audit ==="
    Write-Output "Total violations: $totalViolations"
    foreach ($cat in $result.Categories) {
        Write-Output "  $($cat.Category): $($cat.Count)"
    }
    foreach ($f in $findings) {
        Write-Output "$($f.Severity.ToUpper()) $($f.File):$($f.Line) - $($f.Message)"
    }
}

exit $totalViolations
