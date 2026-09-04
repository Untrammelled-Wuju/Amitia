$ErrorActionPreference = "Stop"
$root = "D:\桌面\跟进项目\U-Ai\Amitia_提示词系统纯接入Codex任务包_v5\prompt_texts"
$outDir = "D:\桌面\跟进项目\U-Ai\backend\internal\prompt\textlib"

New-Item -ItemType Directory -Path $outDir -Force | Out-Null

function Extract-CodeBlock($filePath) {
    $text = Get-Content $filePath -Raw -Encoding UTF8
    if ($text -match '(?s)`````\w*\r?\n(.*?)`````') {
        return $matches[1].TrimEnd()
    }
    Write-Warning "No code block in $filePath"
    return ""
}

function Go-StringLit($s) {
    if ($s -match '`') {
        $parts = $s -split '`'
        $out = ""
        for ($i = 0; $i -lt $parts.Count; $i++) {
            if ($parts[$i].Length -gt 0) {
                $out += "`" + $parts[$i] + "`"
            }
            if ($i -lt $parts.Count - 1) {
                $out += ' + "`" + '
            }
        }
        return $out
    }
    return "`" + $s + "`"
}

function ConstName($fileName) {
    $base = [System.IO.Path]::GetFileNameWithoutExtension($fileName)
    if ($base -match '^src__main__(.+)$') {
        $base = $matches[1]
    }
    if ($base -match '^core__.*?__com__lianyu__ai__(.+)$') {
        $base = $matches[1]
    }
    if ($base -match '^feature__.*?__com__lianyu__ai__(.+)$') {
        $base = $matches[1]
    }
    if ($base -match '^docs__(.+)$') {
        $base = $matches[1]
    }
    $base = $base -replace '__','_' -replace '\.ts$','' -replace '\.kt$','' -replace '\.md$','' -replace '\.en$','_En'
    $upper = ""
    $next = $true
    foreach ($ch in $base.ToCharArray()) {
        if ($ch -eq '_') { $upper += '_'; $next = $true }
        elseif ($next) { $upper += [char]::ToUpper($ch); $next = $false }
        elseif ($ch -match '[a-zA-Z0-9]') { $upper += $ch }
    }
    $upper = $upper -replace '_+$',''
    if ($upper.Length -gt 0) {
        if ($upper.Substring(0,1) -match '[0-9]') { $upper = 'N' + $upper }
    }
    if ($upper.Length -eq 0) { $upper = "RawUnknown" }
    return "Raw" + $upper
}

function NameToIdent($n) { return ConstName $n }

$ackemDir = Join-Path $root "ackem"
$lianyuDir = Join-Path $root "lianyu"

$disallowed = @('groupchat','group_chat','group-chat','sticker','localmodel','local_model','local-model',
    'desktopagent','desktop_agent','desktop-agent','openforu','open_for_u','plugin','diary','extension')

function IsAllowed($name) {
    $lower = $name.ToLower()
    foreach ($d in $disallowed) {
        if ($lower -match $d) { return $false }
    }
    return $true
}

$ackemFiles = Get-ChildItem $ackemDir -Filter "*.md" | Where-Object { $_.Name -ne "00_本目录原文提示词合集.md" -and (IsAllowed $_.Name) }
$lianyuFiles = Get-ChildItem $lianyuDir -Filter "*.md" | Where-Object { $_.Name -ne "00_本目录原文提示词合集.md" -and (IsAllowed $_.Name) }

Write-Host "Ackem: $($ackemFiles.Count) files, LianYu: $($lianyuFiles.Count) files"

$ackemConsts = @()
foreach ($f in $ackemFiles) {
    $code = Extract-CodeBlock $f.FullName
    if ($code.Length -eq 0) { continue }
    $name = NameToIdent $f.Name
    $srcName = $f.Name -replace '\.md$',''
    $ackemConsts += @{ Name = $name; Code = $code; Src = $srcName; File = $f.Name }
}

$lianyuConsts = @()
foreach ($f in $lianyuFiles) {
    $code = Extract-CodeBlock $f.FullName
    if ($code.Length -eq 0) { continue }
    $name = NameToIdent $f.Name
    $srcName = $f.Name -replace '\.md$',''
    $lianyuConsts += @{ Name = $name; Code = $code; Src = $srcName; File = $f.Name }
}

function Write-GoFile($path, $pkg, $srcSet, $texts) {
    $sb = [System.Text.StringBuilder]::new()
    [void]$sb.AppendLine("package textlib")
    [void]$sb.AppendLine()

    $byName = @{}
    foreach ($t in $texts) {
        $n = $t.Name
        if ($byName.ContainsKey($n)) {
            $suffix = 2
            while ($byName.ContainsKey("$n`_$suffix")) { $suffix++ }
            $n = "$n`_$suffix"
            $t.Name = $n
        }
        $byName[$n] = $t
        [void]$sb.AppendLine("// SourceName: $($t.Src)")
        [void]$sb.AppendLine("// SourceSet: $srcSet")
        $lit = Go-StringLit $t.Code
        [void]$sb.AppendLine("const $n = $lit")
        [void]$sb.AppendLine()
    }
    [System.IO.File]::WriteAllText($path, $sb.ToString(), [System.Text.UTF8Encoding]::new($false))
    Write-Host "  Wrote $path ($($byName.Count) constants, $((Get-Item $path).Length) bytes)"
}

Write-GoFile (Join-Path $outDir "ackem_texts.go") "textlib" "ackem" $ackemConsts
Write-GoFile (Join-Path $outDir "lianyu_texts.go") "textlib" "lianyu" $lianyuConsts

Write-Host "Done."
