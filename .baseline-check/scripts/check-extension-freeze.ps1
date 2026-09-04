<#
.SYNOPSIS
    Amitia 扩展系统冻结检查脚本（PowerShell 7 版本）。

.DESCRIPTION
    用法:
        pwsh scripts/check-extension-freeze.ps1

    本脚本在扩展系统重构冻结期间，自动检测旧架构是否被违规扩展。
    以 2026-07-25 冻结第一天的代码基线为白名单基线，检测"新增"的违规。

    检查项:
      1. Manifest v1 Schema 变更（新增 Entry 类型 / Runtime / UI / Hook / Provider / 顶层字段）
      2. 扩展相关新增数据库表（CREATE TABLE）
      3. Registry 增量（NewRegistry / RegisterSkill / RegisterPlugin / RegisterMCP /
         RegisterWorkflow / RegisterProvider）
      4. 前端扩展中心 / 创意工坊新增静态路由

    退出码:
      0 = PASS，无违规
      1 = FAIL，存在违规

    白名单基线来源:
      - backend/internal/extension/schema/manifest.schema.json
      - backend/internal/migration/*.go
      - backend/internal/extension/**/*.go, backend/internal/mcp/**/*.go
      - front/src/router/index.ts

    详见 docs/extension-kernel/01-system-freeze.md。
#>

$ErrorActionPreference = "Stop"
$OutputEncoding = [System.Text.Encoding]::UTF8
try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
} catch {}

if ($PSScriptRoot) {
    $ScriptRoot = $PSScriptRoot
} else {
    $ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
}
$ProjectRoot = (Resolve-Path (Join-Path $ScriptRoot '..')).Path

$script:violations = New-Object System.Collections.Generic.List[string]

# ============================================================================
# 基线白名单（基于 2026-07-25 冻结第一天的代码实际内容硬编码）
# ============================================================================

# 检查 1：Manifest v1 Schema 基线
$BaselineManifestTopLevel = @('$schema', '$id', '$defs', 'oneOf')
$BaselineManifestDefs = @('semver', 'metadata', 'compatibility', 'entry', 'skill', 'plugin')
$BaselineManifestKinds = @('Skill', 'Plugin')
$BaselineEntryKinds = @('builtin', 'legacy_tool', 'workflow', 'instructions')
$BaselineSkillProperties = @(
    '$schema', 'apiVersion', 'kind', 'metadata', 'compatibility', 'entry',
    'capabilities', 'triggers', 'execution', 'inputSchema', 'outputSchema',
    'configSchema', 'defaultConfig', 'enabled', 'allowLLM', 'allowManual'
)
$BaselinePluginProperties = @(
    '$schema', 'apiVersion', 'kind', 'metadata', 'compatibility', 'entry',
    'capabilities', 'hooks', 'subscriptions', 'registeredSkills', 'execution',
    'configSchema', 'defaultConfig', 'state', 'surface', 'enabled'
)
$BaselinePluginHooks = @(
    'on_load', 'on_enable', 'before_prompt', 'after_reply',
    'on_event', 'on_schedule', 'on_disable', 'on_unload'
)
$BaselineSkillTriggers = @('llm', 'manual', 'schedule', 'system_event')

# 检查 2：扩展相关数据表白名单（前缀 extension_ / mcp_ / 以及扩展迁移文件中的表）
$BaselineExtensionTables = @(
    'extension_agent_skill_activations',
    'extension_agent_skill_metadata',
    'extension_artifacts',
    'extension_audits',
    'extension_capability_grants',
    'extension_configs',
    'extension_event_deliveries',
    'extension_events',
    'extension_owned_resources',
    'extension_package_exports',
    'extension_package_import_sessions',
    'extension_package_installations',
    'extension_package_signers',
    'extension_plugin_runs',
    'extension_runs',
    'extension_schedules',
    'extension_scope_bindings',
    'extension_state_revisions',
    'extension_states',
    'extension_version_dependencies',
    'extension_versions',
    'extension_workshop_revisions',
    'extension_workshop_sessions',
    'extension_workshop_test_runs',
    'extensions',
    'mcp_audit_logs',
    'mcp_dependency_links',
    'mcp_oauth_sessions',
    'mcp_operations',
    'mcp_prompts',
    'mcp_resource_templates',
    'mcp_resources',
    'mcp_server_capabilities',
    'mcp_server_credentials',
    'mcp_server_scope_bindings',
    'mcp_servers',
    'mcp_tasks',
    'mcp_tools',
    'schedules'
)

# 检查 3：Registry 关键词注册点白名单（格式：相对路径:关键词）
# 注：白名单键使用"相对路径:关键词"格式，不依赖行号，避免因行号漂移导致误报。
# 报警时仍会输出实际行号作为定位参考。
$BaselineRegistryPoints = @(
    'backend/internal/extension/plugin_builtin_diagnostic.go:RegisterSkill',
    'backend/internal/extension/plugin_host.go:RegisterSkill',
    'backend/internal/extension/plugin_protocol.go:RegisterSkill',
    'backend/internal/extension/registry.go:NewRegistry',
    'backend/internal/extension/runtime.go:NewRegistry'
)

# 检查 4：前端扩展中心 / 创意工坊静态路由白名单（格式：path|name）
$BaselineExtensionRoutes = @(
    '/extensions|extensionCenter',
    '/extensions/mcp|extensionMCP',
    '/extensions/packages|extensionPackages',
    '/extensions/skills|extensionSkills',
    '/extensions/skills/:id|extensionSkillDetail',
    '/extensions/agent-skills|extensionAgentSkills',
    '/extensions/plugins|extensionPlugins',
    '/extensions/plugins/:id|extensionPluginDetail',
    '/extensions/workshop|extensionWorkshop',
    '/extensions/workshop/:id|extensionWorkshopSession',
    '/extensions/runs|extensionRuns',
    '/creative-workshop|creativeWorkshop',
    '/creative-workshop/pet|creativeWorkshopPet',
    '/creative-workshop/pet/create|creativeWorkshopPetCreate',
    '/creative-workshop/pet/tasks|creativeWorkshopPetTasks',
    '/creative-workshop/pet/processing/:processingTaskId|creativeWorkshopPetProcessing',
    '/creative-workshop/pet/installations|pet-installations'
)

$RegistryKeywords = @(
    'NewRegistry', 'RegisterSkill', 'RegisterPlugin',
    'RegisterMCP', 'RegisterWorkflow', 'RegisterProvider'
)

# ============================================================================
# 工具函数
# ============================================================================

function Convert-ToForwardSlash {
    param([string]$P)
    return ($P -replace '\\', '/')
}

function Get-ProjectRelativePath {
    param([string]$AbsPath, [string]$Root)
    $normRoot = (Convert-ToForwardSlash $Root).TrimEnd('/')
    $normPath = Convert-ToForwardSlash $AbsPath
    if ($normPath.StartsWith($normRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $normPath.Substring($normRoot.Length).TrimStart('/')
    }
    return $normPath
}

function Get-ObjectProperties {
    param($obj)
    if ($null -eq $obj) { return @() }
    $list = New-Object System.Collections.Generic.List[string]
    foreach ($p in $obj.PSObject.Properties) {
        [void]$list.Add($p.Name)
    }
    return $list
}

function Test-ExtensionRelatedTable {
    param([string]$TableName, [string]$SourceFile)
    if ($TableName -match '^(extension_|mcp_)') { return $true }
    if ($SourceFile -match 'extension_' -or $SourceFile -match '\bmcp_') { return $true }
    if ($TableName -eq 'schedules') { return $true }
    return $false
}

function Test-MigrationAuxiliaryTable {
    param([string]$TableName)
    $lower = $TableName.ToLower()
    if ($lower -match 'migration') { return $true }
    if ($lower -match 'snapshot') { return $true }
    if ($lower -match 'temporary') { return $true }
    return $false
}

# ============================================================================
# 检查 1：Manifest v1 Schema 变更检查
# ============================================================================

function Invoke-ManifestSchemaCheck {
    $manifestPath = Join-Path $ProjectRoot 'backend/internal/extension/schema/manifest.schema.json'
    if (-not (Test-Path $manifestPath)) {
        $script:violations.Add("VIOLATION: $manifestPath - manifest schema file missing")
        return
    }
    $json = Get-Content $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json

    # 顶层字段
    $topLevel = Get-ObjectProperties $json
    foreach ($k in $topLevel) {
        if ($BaselineManifestTopLevel -notcontains $k) {
            $script:violations.Add("VIOLATION: $manifestPath - new top-level field '$k' added to Manifest v1 schema")
        }
    }

    # $defs 子定义
    $defsObj = $json.'$defs'
    $defs = Get-ObjectProperties $defsObj
    foreach ($d in $defs) {
        if ($BaselineManifestDefs -notcontains $d) {
            $script:violations.Add("VIOLATION: $manifestPath - new `$defs entry '$d' added to Manifest v1 schema")
        }
    }

    # kind 常量（Skill/Plugin）
    $kinds = New-Object System.Collections.Generic.List[string]
    if ($defsObj.skill.properties.kind.const) {
        [void]$kinds.Add($defsObj.skill.properties.kind.const)
    }
    if ($defsObj.plugin.properties.kind.const) {
        [void]$kinds.Add($defsObj.plugin.properties.kind.const)
    }
    foreach ($k in $kinds) {
        if ($BaselineManifestKinds -notcontains $k) {
            $script:violations.Add("VIOLATION: $manifestPath - new manifest kind '$k' added")
        }
    }

    # entry kind enum（在 skill.entry.allOf 中查找 properties.kind.enum）
    $entryKindEnums = New-Object System.Collections.Generic.List[string]
    $skillEntryAllOf = $defsObj.skill.properties.entry.allOf
    if ($skillEntryAllOf) {
        foreach ($cond in $skillEntryAllOf) {
            $condProps = Get-ObjectProperties $cond
            if ($condProps -contains 'properties' -and $cond.properties.kind) {
                if ($cond.properties.kind.enum) {
                    foreach ($e in $cond.properties.kind.enum) {
                        [void]$entryKindEnums.Add($e)
                    }
                }
            }
        }
    }
    $entryKindSet = $entryKindEnums | Sort-Object -Unique
    foreach ($e in $entryKindSet) {
        if ($BaselineEntryKinds -notcontains $e) {
            $script:violations.Add("VIOLATION: $manifestPath - new entry kind '$e' added to Manifest v1 schema")
        }
    }

    # skill properties
    $skillProps = Get-ObjectProperties $defsObj.skill.properties
    foreach ($p in $skillProps) {
        if ($BaselineSkillProperties -notcontains $p) {
            $script:violations.Add("VIOLATION: $manifestPath - new field '$p' added to Skill manifest properties")
        }
    }

    # plugin properties
    $pluginProps = Get-ObjectProperties $defsObj.plugin.properties
    foreach ($p in $pluginProps) {
        if ($BaselinePluginProperties -notcontains $p) {
            $script:violations.Add("VIOLATION: $manifestPath - new field '$p' added to Plugin manifest properties")
        }
    }

    # plugin hooks enum
    if ($defsObj.plugin.properties.hooks -and $defsObj.plugin.properties.hooks.items -and $defsObj.plugin.properties.hooks.items.enum) {
        foreach ($h in $defsObj.plugin.properties.hooks.items.enum) {
            if ($BaselinePluginHooks -notcontains $h) {
                $script:violations.Add("VIOLATION: $manifestPath - new plugin hook '$h' added to Manifest v1 schema")
            }
        }
    }

    # skill triggers enum
    if ($defsObj.skill.properties.triggers -and $defsObj.skill.properties.triggers.items -and $defsObj.skill.properties.triggers.items.enum) {
        foreach ($t in $defsObj.skill.properties.triggers.items.enum) {
            if ($BaselineSkillTriggers -notcontains $t) {
                $script:violations.Add("VIOLATION: $manifestPath - new skill trigger '$t' added to Manifest v1 schema")
            }
        }
    }
}

# ============================================================================
# 检查 2：扩展相关新增数据库表检查
# ============================================================================

function Invoke-MigrationTableCheck {
    $migrationDir = Join-Path $ProjectRoot 'backend/internal/migration'
    if (-not (Test-Path $migrationDir)) {
        return
    }
    $files = Get-ChildItem -Path $migrationDir -Filter '*.go' | Where-Object { $_.Name -notmatch '_test\.go$' }
    foreach ($f in $files) {
        $content = Get-Content $f.FullName -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
        if (-not $content) { continue }
        $matches = [regex]::Matches($content, '(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[`"\[]?([a-zA-Z0-9_]+)[`"\]]?')
        foreach ($m in $matches) {
            $tableName = $m.Groups[1].Value
            if (-not (Test-ExtensionRelatedTable -TableName $tableName -SourceFile $f.Name)) {
                continue
            }
            if ($BaselineExtensionTables -notcontains $tableName) {
                if (Test-MigrationAuxiliaryTable -TableName $tableName) {
                    $script:violations.Add("VIOLATION: $($f.Name) - new migration auxiliary table '$tableName' added; ensure it has explicit deletion plan and is cleaned in later migration step")
                } else {
                    $script:violations.Add("VIOLATION: $($f.Name) - new extension-related table '$tableName' added; permanent extension tables are forbidden during freeze")
                }
            }
        }
    }
}

# ============================================================================
# 检查 3：Registry 增量检查
# ============================================================================

function Invoke-RegistryCheck {
    $dirs = @(
        (Join-Path $ProjectRoot 'backend/internal/extension'),
        (Join-Path $ProjectRoot 'backend/internal/mcp')
    )
    $keywordPattern = ($RegistryKeywords | ForEach-Object { [regex]::Escape($_) }) -join '|'
    foreach ($dir in $dirs) {
        if (-not (Test-Path $dir)) { continue }
        $files = Get-ChildItem -Path $dir -Filter '*.go' -Recurse -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -notmatch '_test\.go$' -and $_.FullName -notmatch '\\node_modules\\' }
        foreach ($f in $files) {
            $lines = Get-Content $f.FullName -Encoding UTF8 -ErrorAction SilentlyContinue
            if (-not $lines) { continue }
            for ($i = 0; $i -lt $lines.Count; $i++) {
                $line = $lines[$i]
                $matched = [regex]::Matches($line, $keywordPattern)
                foreach ($m in $matched) {
                    $kw = $m.Value
                    $lineNo = $i + 1
                    $relPath = Get-ProjectRelativePath -AbsPath $f.FullName -Root $ProjectRoot
                    $key = "${relPath}:${kw}"
                    if ($BaselineRegistryPoints -notcontains $key) {
                        $script:violations.Add("VIOLATION: ${relPath}:$lineNo - new Registry keyword '$kw' detected (not in baseline)")
                    }
                }
            }
        }
    }
}

# ============================================================================
# 检查 4：前端路由检查
# ============================================================================

function Invoke-FrontendRouteCheck {
    $routerPath = Join-Path $ProjectRoot 'front/src/router/index.ts'
    if (-not (Test-Path $routerPath)) {
        $script:violations.Add("VIOLATION: $routerPath - frontend router file missing")
        return
    }
    $lines = Get-Content $routerPath -Encoding UTF8 -ErrorAction SilentlyContinue
    if (-not $lines) { return }

    # 提取扩展中心/创意工坊相关路由：path/name 组合
    # 匹配形如: { path: "/extensions", name: "extensionCenter", ... }
    $extensionPathPattern = '/(extensions|creative-workshop)(/[^"]*)?'
    $routeRegex = "path:\s*`"($extensionPathPattern)`"\s*,\s*name:\s*`"([^`"]+)`""

    $currentRoutes = New-Object System.Collections.Generic.List[string]
    foreach ($line in $lines) {
        $m = [regex]::Match($line, $routeRegex)
        if ($m.Success) {
            $path = $m.Groups[1].Value
            $name = $m.Groups[4].Value
            [void]$currentRoutes.Add("${path}|${name}")
        }
    }

    foreach ($r in $currentRoutes) {
        if ($BaselineExtensionRoutes -notcontains $r) {
            $parts = $r -split '\|', 2
            $script:violations.Add("VIOLATION: $routerPath - new extension/creative-workshop route added: path='$($parts[0])' name='$($parts[1])'")
        }
    }
}

# ============================================================================
# 主流程
# ============================================================================

function Main {
    Invoke-ManifestSchemaCheck
    Invoke-MigrationTableCheck
    Invoke-RegistryCheck
    Invoke-FrontendRouteCheck

    if ($script:violations.Count -gt 0) {
        foreach ($v in $script:violations) {
            Write-Host $v
        }
        $count = $script:violations.Count
        Write-Host "Extension freeze check: FAIL ($count violations)"
        exit 1
    }
    Write-Host 'Extension freeze check: PASS'
    exit 0
}

Main
