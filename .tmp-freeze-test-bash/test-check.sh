#!/usr/bin/env bash
#
# Amitia 扩展系统冻结检查脚本（Bash 版本，供跨平台 / CI 使用）。
#
# 用法:
#   bash scripts/check-extension-freeze.sh
#
# 本脚本在扩展系统重构冻结期间，自动检测旧架构是否被违规扩展。
# 以 2026-07-25 冻结第一天的代码基线为白名单基线，检测"新增"的违规。
#
# 检查项:
#   1. Manifest v1 Schema 变更（新增 Entry 类型 / Runtime / UI / Hook / Provider / 顶层字段）
#   2. 扩展相关新增数据库表（CREATE TABLE）
#   3. Registry 增量（NewRegistry / RegisterSkill / RegisterPlugin / RegisterMCP /
#      RegisterWorkflow / RegisterProvider）
#   4. 前端扩展中心 / 创意工坊新增静态路由
#
# 退出码:
#   0 = PASS，无违规
#   1 = FAIL，存在违规
#
# 白名单基线来源:
#   - backend/internal/extension/schema/manifest.schema.json
#   - backend/internal/migration/*.go
#   - backend/internal/extension/**/*.go, backend/internal/mcp/**/*.go
#   - front/src/router/index.ts
#
# 详见 docs/extension-kernel/01-system-freeze.md。

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="/d/桌面/跟进项目/U-Ai/.tmp-freeze-test-bash"

VIOLATIONS=()

# ============================================================================
# 基线白名单（基于 2026-07-25 冻结第一天的代码实际内容硬编码）
# ============================================================================

# 检查 1：Manifest v1 Schema 基线
BASELINE_MANIFEST_TOPLEVEL='$schema $id $defs oneOf'
BASELINE_MANIFEST_DEFS='semver metadata compatibility entry skill plugin'
BASELINE_MANIFEST_KINDS='Skill Plugin'
BASELINE_ENTRY_KINDS='builtin legacy_tool workflow instructions'
BASELINE_SKILL_PROPERTIES='$schema apiVersion kind metadata compatibility entry capabilities triggers execution inputSchema outputSchema configSchema defaultConfig enabled allowLLM allowManual'
BASELINE_PLUGIN_PROPERTIES='$schema apiVersion kind metadata compatibility entry capabilities hooks subscriptions registeredSkills execution configSchema defaultConfig state surface enabled'
BASELINE_PLUGIN_HOOKS='on_load on_enable before_prompt after_reply on_event on_schedule on_disable on_unload'
BASELINE_SKILL_TRIGGERS='llm manual schedule system_event'

# 检查 2：扩展相关数据表白名单（前缀 extension_ / mcp_ / 以及扩展迁移文件中的表）
BASELINE_EXTENSION_TABLES='
extension_agent_skill_activations
extension_agent_skill_metadata
extension_artifacts
extension_audits
extension_capability_grants
extension_configs
extension_event_deliveries
extension_events
extension_owned_resources
extension_package_exports
extension_package_import_sessions
extension_package_installations
extension_package_signers
extension_plugin_runs
extension_runs
extension_schedules
extension_scope_bindings
extension_state_revisions
extension_states
extension_version_dependencies
extension_versions
extension_workshop_revisions
extension_workshop_sessions
extension_workshop_test_runs
extensions
mcp_audit_logs
mcp_dependency_links
mcp_oauth_sessions
mcp_operations
mcp_prompts
mcp_resource_templates
mcp_resources
mcp_server_capabilities
mcp_server_credentials
mcp_server_scope_bindings
mcp_servers
mcp_tasks
mcp_tools
schedules
'

# 检查 3：Registry 关键词注册点白名单（格式：相对路径:关键词）
# 注：白名单键使用"相对路径:关键词"格式，不依赖行号，避免因行号漂移导致误报。
# 报警时仍会输出实际行号作为定位参考。
BASELINE_REGISTRY_POINTS='
backend/internal/extension/plugin_builtin_diagnostic.go:RegisterSkill
backend/internal/extension/plugin_host.go:RegisterSkill
backend/internal/extension/plugin_protocol.go:RegisterSkill
backend/internal/extension/registry.go:NewRegistry
backend/internal/extension/runtime.go:NewRegistry
'

# 检查 4：前端扩展中心 / 创意工坊静态路由白名单（格式：path|name）
BASELINE_EXTENSION_ROUTES='
/extensions|extensionCenter
/extensions/mcp|extensionMCP
/extensions/packages|extensionPackages
/extensions/skills|extensionSkills
/extensions/skills/:id|extensionSkillDetail
/extensions/agent-skills|extensionAgentSkills
/extensions/plugins|extensionPlugins
/extensions/plugins/:id|extensionPluginDetail
/extensions/workshop|extensionWorkshop
/extensions/workshop/:id|extensionWorkshopSession
/extensions/runs|extensionRuns
/creative-workshop|creativeWorkshop
/creative-workshop/pet|creativeWorkshopPet
/creative-workshop/pet/create|creativeWorkshopPetCreate
/creative-workshop/pet/tasks|creativeWorkshopPetTasks
/creative-workshop/pet/processing/:processingTaskId|creativeWorkshopPetProcessing
/creative-workshop/pet/installations|pet-installations
'

REGISTRY_KEYWORDS='NewRegistry RegisterSkill RegisterPlugin RegisterMCP RegisterWorkflow RegisterProvider'

# ============================================================================
# 工具函数
# ============================================================================

to_forward_slash() {
    printf '%s' "$1" | sed 's|\\|/|g'
}

project_relative_path() {
    local abs="$1"
    local root="$2"
    local norm_root norm_path
    norm_root="$(to_forward_slash "$root")"
    norm_root="${norm_root%/}"
    norm_path="$(to_forward_slash "$abs")"
    # 不区分大小写比较前缀（bash 内置 ${var#prefix} 区分大小写，故用 awk）
    local prefix_stripped
    prefix_stripped="$(printf '%s\n' "$norm_path" | awk -v r="$norm_root" '
        BEGIN { lower_r = tolower(r); }
        {
            lower_p = tolower($0);
            if (substr(lower_p, 1, length(lower_r)+1) == tolower(r) "/") {
                print substr($0, length(r)+2);
            } else {
                print $0;
            }
        }
    ')"
    printf '%s' "$prefix_stripped"
}

contains_item() {
    local needle="$1"
    local haystack="$2"
    local item
    for item in $haystack; do
        if [ "$item" = "$needle" ]; then
            return 0
        fi
    done
    return 1
}

contains_multiline_item() {
    local needle="$1"
    local haystack="$2"
    local item
    while IFS= read -r item; do
        item="${item# }"
        item="${item% }"
        [ -z "$item" ] && continue
        if [ "$item" = "$needle" ]; then
            return 0
        fi
    done <<EOF
$haystack
EOF
    return 1
}

is_extension_related_table() {
    local table_name="$1"
    local source_file="$2"
    case "$table_name" in
        extension_*|mcp_*) return 0 ;;
    esac
    case "$source_file" in
        extension_*|*mcp_*) return 0 ;;
    esac
    if [ "$table_name" = "schedules" ]; then
        return 0
    fi
    return 1
}

is_migration_auxiliary_table() {
    local table_name="$1"
    local lower
    lower="$(printf '%s' "$table_name" | tr '[:upper:]' '[:lower:]')"
    case "$lower" in
        *migration*|*snapshot*|*temporary*) return 0 ;;
        *) return 1 ;;
    esac
}

# ============================================================================
# 检查 1：Manifest v1 Schema 变更检查
# ============================================================================

manifest_check() {
    local manifest_path="$PROJECT_ROOT/backend/internal/extension/schema/manifest.schema.json"
    if [ ! -f "$manifest_path" ]; then
        VIOLATIONS+=("VIOLATION: $manifest_path - manifest schema file missing")
        return
    fi

    local has_jq=0
    if command -v jq >/dev/null 2>&1; then
        has_jq=1
    fi

    local top_level defs kinds entry_kinds skill_props plugin_props plugin_hooks skill_triggers
    if [ "$has_jq" -eq 1 ]; then
        top_level="$(jq -r 'keys[]' "$manifest_path" 2>/dev/null)"
        defs="$(jq -r '."$defs" | keys[]' "$manifest_path" 2>/dev/null)"
        kinds="$(jq -r '[."$defs".skill.properties.kind.const, ."\$defs".plugin.properties.kind.const] | .[]' "$manifest_path" 2>/dev/null)"
        entry_kinds="$(jq -r '.."\$defs".skill.properties.entry.allOf[]? | .properties.kind.enum[]? ' "$manifest_path" 2>/dev/null)"
        skill_props="$(jq -r '.."\$defs".skill.properties | keys[]' "$manifest_path" 2>/dev/null)"
        plugin_props="$(jq -r '.."\$defs".plugin.properties | keys[]' "$manifest_path" 2>/dev/null)"
        plugin_hooks="$(jq -r '.\"\$defs\".plugin.properties.hooks.items.enum[]?' "$manifest_path" 2>/dev/null)"
        skill_triggers="$(jq -r '.\"\$defs\".skill.properties.triggers.items.enum[]?' "$manifest_path" 2>/dev/null)"
    else
        # 无 jq 时降级：用 sed/awk 提取 enum 数组与 const 字段的字符串字面量
        # 这种降级能检测新增的 enum/const 值，但不检测顶层字段与 properties（保守跳过）
        local enum_values const_values all_values e
        # 提取 "enum": [ ... ] 数组中的标识符字符串字面量（先剥离 "enum": [ 前缀，避免把 enum 关键字误当作值）
        enum_values="$(grep -oE '"enum":[[:space:]]*\[[^]]*\]' "$manifest_path" 2>/dev/null \
                        | sed -E 's/"enum":[[:space:]]*\[//' \
                        | grep -oE '"[a-zA-Z_][a-zA-Z0-9_]*"' | sed 's/"//g')"
        # 提取 "const": "..." 字段的字符串值，仅保留标识符格式（过滤 URL / 版本号等）
        const_values="$(grep -oE '"const":[[:space:]]*"[^"]+"' "$manifest_path" 2>/dev/null \
                        | sed -E 's/.*"const":[[:space:]]*"([^"]+)".*/\1/' \
                        | grep -E '^[a-zA-Z_][a-zA-Z0-9_]*$')"
        all_values="$(printf '%s\n%s\n' "$enum_values" "$const_values" | sort -u)"
        for e in $all_values; do
            [ -z "$e" ] && continue
            if ! contains_item "$e" "$BASELINE_ENTRY_KINDS" \
               && ! contains_item "$e" "$BASELINE_MANIFEST_KINDS" \
               && ! contains_item "$e" "$BASELINE_PLUGIN_HOOKS" \
               && ! contains_item "$e" "$BASELINE_SKILL_TRIGGERS"; then
                VIOLATIONS+=("VIOLATION: $manifest_path - new manifest value '$e' detected (not in baseline)")
            fi
        done
        return
    fi

    local k
    for k in $top_level; do
        if ! contains_item "$k" "$BASELINE_MANIFEST_TOPLEVEL"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new top-level field '$k' added to Manifest v1 schema")
        fi
    done

    for k in $defs; do
        if ! contains_item "$k" "$BASELINE_MANIFEST_DEFS"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new \$defs entry '$k' added to Manifest v1 schema")
        fi
    done

    for k in $kinds; do
        if ! contains_item "$k" "$BASELINE_MANIFEST_KINDS"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new manifest kind '$k' added")
        fi
    done

    for k in $entry_kinds; do
        if ! contains_item "$k" "$BASELINE_ENTRY_KINDS"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new entry kind '$k' added to Manifest v1 schema")
        fi
    done

    for k in $skill_props; do
        if ! contains_item "$k" "$BASELINE_SKILL_PROPERTIES"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new field '$k' added to Skill manifest properties")
        fi
    done

    for k in $plugin_props; do
        if ! contains_item "$k" "$BASELINE_PLUGIN_PROPERTIES"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new field '$k' added to Plugin manifest properties")
        fi
    done

    for k in $plugin_hooks; do
        if ! contains_item "$k" "$BASELINE_PLUGIN_HOOKS"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new plugin hook '$k' added to Manifest v1 schema")
        fi
    done

    for k in $skill_triggers; do
        if ! contains_item "$k" "$BASELINE_SKILL_TRIGGERS"; then
            VIOLATIONS+=("VIOLATION: $manifest_path - new skill trigger '$k' added to Manifest v1 schema")
        fi
    done
}

# ============================================================================
# 检查 2：扩展相关新增数据库表检查
# ============================================================================

migration_table_check() {
    local migration_dir="$PROJECT_ROOT/backend/internal/migration"
    if [ ! -d "$migration_dir" ]; then
        return
    fi
    local file
    while IFS= read -r file; do
        [ -z "$file" ] && continue
        local base
        base="$(basename "$file")"
        case "$base" in
            *_test.go) continue ;;
        esac
        local table
        while IFS= read -r table; do
            [ -z "$table" ] && continue
            if ! is_extension_related_table "$table" "$base"; then
                continue
            fi
            if ! contains_multiline_item "$table" "$BASELINE_EXTENSION_TABLES"; then
                if is_migration_auxiliary_table "$table"; then
                    VIOLATIONS+=("VIOLATION: $base - new migration auxiliary table '$table' added; ensure it has explicit deletion plan and is cleaned in later migration step")
                else
                    VIOLATIONS+=("VIOLATION: $base - new extension-related table '$table' added; permanent extension tables are forbidden during freeze")
                fi
            fi
        done < <(grep -oiE 'CREATE[[:space:]]+TABLE[[:space:]]+(IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?[`"\[]?[a-zA-Z0-9_]+' "$file" 2>/dev/null \
                    | sed -E 's/.*[[:space:]](IF[[:space:]]+NOT[[:space:]]+EXISTS[[:space:]]+)?[`"\[]?([a-zA-Z0-9_]+)$/\2/i')
    done < <(find "$migration_dir" -maxdepth 1 -type f -name '*.go' 2>/dev/null)
}

# ============================================================================
# 检查 3：Registry 增量检查
# ============================================================================

registry_check() {
    local dirs=( "$PROJECT_ROOT/backend/internal/extension" "$PROJECT_ROOT/backend/internal/mcp" )
    local keyword_pattern
    keyword_pattern="$(printf '%s|' $REGISTRY_KEYWORDS)"
    keyword_pattern="${keyword_pattern%|}"
    local dir file
    for dir in "${dirs[@]}"; do
        [ -d "$dir" ] || continue
        while IFS= read -r file; do
            [ -z "$file" ] && continue
            case "$file" in
                *_test.go) continue ;;
                *node_modules*) continue ;;
            esac
            local rel_path
            rel_path="$(project_relative_path "$file" "$PROJECT_ROOT")"
            local line_no kw line
            local grep_output
            grep_output="$(grep -nE "$keyword_pattern" "$file" 2>/dev/null)"
            while IFS= read -r line; do
                [ -z "$line" ] && continue
                line_no="${line%%:*}"
                local rest="${line#*:}"
                # 提取所有匹配的关键词（可能一行多个）
                for kw in $REGISTRY_KEYWORDS; do
                    if printf '%s' "$rest" | grep -qE "\\b${kw}\\b"; then
                        local key="${rel_path}:${kw}"
                        if ! contains_multiline_item "$key" "$BASELINE_REGISTRY_POINTS"; then
                            VIOLATIONS+=("VIOLATION: ${rel_path}:${line_no} - new Registry keyword '$kw' detected (not in baseline)")
                        fi
                    fi
                done
            done <<EOF
$grep_output
EOF
        done < <(find "$dir" -type f -name '*.go' 2>/dev/null)
    done
}

# ============================================================================
# 检查 4：前端路由检查
# ============================================================================

frontend_route_check() {
    local router_path="$PROJECT_ROOT/front/src/router/index.ts"
    if [ ! -f "$router_path" ]; then
        VIOLATIONS+=("VIOLATION: $router_path - frontend router file missing")
        return
    fi
    # 提取 path 和 name 组合，仅匹配 /extensions 或 /creative-workshop 开头的路径
    local path name line
    while IFS= read -r line; do
        # 形如: path: "/extensions", name: "extensionCenter"
        path="$(printf '%s' "$line" | sed -nE 's/.*path:[[:space:]]*"([^"]+)".*/\1/p')"
        name="$(printf '%s' "$line" | sed -nE 's/.*name:[[:space:]]*"([^"]+)".*/\1/p')"
        [ -z "$path" ] && continue
        [ -z "$name" ] && continue
        case "$path" in
            /extensions/*|/extensions|/creative-workshop/*|/creative-workshop)
                local key="${path}|${name}"
                if ! contains_multiline_item "$key" "$BASELINE_EXTENSION_ROUTES"; then
                    VIOLATIONS+=("VIOLATION: $router_path - new extension/creative-workshop route added: path='$path' name='$name'")
                fi
                ;;
        esac
    done < <(grep -nE 'path:[[:space:]]*"/(extensions|creative-workshop)' "$router_path" 2>/dev/null)
}

# ============================================================================
# 主流程
# ============================================================================

main() {
    manifest_check
    migration_table_check
    registry_check
    frontend_route_check

    if [ "${#VIOLATIONS[@]}" -gt 0 ]; then
        local v
        for v in "${VIOLATIONS[@]}"; do
            printf '%s\n' "$v"
        done
        printf 'Extension freeze check: FAIL (%d violations)\n' "${#VIOLATIONS[@]}"
        exit 1
    fi
    printf 'Extension freeze check: PASS\n'
    exit 0
}

main "$@"
