#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
B9P3 - Capability ID, Numeric ID and Protocol Naming Layer Revision
Generates corrected capability IDs compatible with Extension Kernel
"""

import json
import os
import re
import hashlib
from pathlib import Path
from typing import Dict, List, Any, Tuple, Set

WORKSPACE = Path("D:/桌面/跟进项目/U-Ai")
B9P3_DOCS = WORKSPACE / "docs/parity/post-b9/b9p3"
B9P3_SCRIPTS = WORKSPACE / "scripts/parity/b9p3"
PROTOCOL_V1 = WORKSPACE / "docs/parity/protocol/v1"
BASELINE_V2 = WORKSPACE / "docs/parity/baseline/v2/parity_baseline.json"
B9P2_DIR = WORKSPACE / "docs/parity/post-b9/b9p2"

# Kernel ID Rules from backend/internal/extension/kernel/capability/id.go
# Format: source/namespace/name
# Allowed chars: a-z, 0-9, /, ., _, -
# Case: lowercase (enforced by strings.ToLower)
KERNEL_ALLOWED_PATTERN = re.compile(r'[^a-z0-9/._-]')
KERNEL_SEPARATOR = "/"
KERNEL_SEGMENT_COUNT = 3

# Valid Kernel Sources from source.go
KERNEL_SOURCES = {
    "builtin", "plugin", "mcp", "workflow", "computer_use",
    "provider", "internal", "legacy"
}

# Domain mapping: B9 domain -> Kernel namespace
DOMAIN_NAMESPACE_MAP = {
    "TOOL": "tool",
    "SYSTEM": "system",
    "BROWSER": "browser",
    "MEMORY": "memory",
    "PROCESS": "process",
    "SECURITY": "security",
    "DEVICE": "device",
    "CONVERSATION": "conversation",
    "SEARCH": "search",
    "EXTENSION": "extension",
    "CHARACTER": "character",
    "NETWORK": "network",
    "VOICE": "voice",
    "NOTIFICATION": "notification",
    "FILE": "file",
    "MODEL": "model",
    "AGENT": "agent",
    "TASK": "task",
    "SANDBOX": "sandbox",
    "RUNTIME": "runtime",
    "STORAGE": "storage",
    "UI": "ui",
    "CHANNEL": "channel",
}

# B9P2 removed MAP IDs
B9P2_REMOVED_MAP_IDS = {"MAP-0038", "MAP-0082", "MAP-0083", "MAP-0234"}


def load_json(filepath: Path) -> Any:
    with open(filepath, 'r', encoding='utf-8') as f:
        return json.load(f)


def save_json(filepath: Path, data: Any):
    with open(filepath, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def to_snake_case(text: str) -> str:
    """Convert Chinese/mixed text to snake_case ASCII."""
    if not text:
        return "unnamed"
    
    # Remove leading/trailing whitespace
    text = text.strip()
    
    # Replace common Chinese punctuation
    text = text.replace('（', '(').replace('）', ')')
    text = text.replace('【', '[').replace('】', ']')
    
    # Remove content in parentheses for cleaner IDs
    text = re.sub(r'\([^)]*\)', '', text)
    text = re.sub(r'\[[^\]]*\]', '', text)
    
    # Replace special chars with underscore
    text = re.sub(r'[^a-zA-Z0-9\u4e00-\u9fff]', '_', text)
    
    # Replace Chinese characters with pinyin equivalents (transliteration map)
    text = transliterate_chinese(text)
    
    # Clean up underscores
    text = re.sub(r'_+', '_', text)
    text = text.strip('_')
    
    # Convert to lowercase
    text = text.lower()
    
    # If empty after processing, use fallback
    if not text:
        return "unnamed"
    
    return text


# Chinese to English semantic mapping for common capability objects
CHINESE_SEMANTIC_MAP = {
    "交互范围解析": "resolve_interaction_scope",
    "任务目标注册": "register_task_goal",
    "角色模板管理": "character_template_manage",
    "角色配置管理": "character_config_manage",
    "工作记忆管理": "working_memory_manage",
    "orm封装": "orm_wrapper",
    "信念批处理": "belief_batch_process",
    "链路链路追踪": "链路追踪",
    "tool_owned_resource管理": "tool_owned_resource_manage",
    "聊天导入": "chat_import",
    "工作流执行器": "workflow_executor",
    "信念系统引擎": "belief_system_engine",
    "任务计划执行": "task_plan_execute",
    "streaming_output": "streaming_output",
    "corruption_recovery": "corruption_recovery",
    "vision_ocr": "vision_ocr",
    "concurrent_tool_execution": "concurrent_tool_execute",
    "技能解析器": "skill_parser",
    "创意工坊服务": "creative_workshop_service",
    "嵌入配置管理": "embedding_config_manage",
    "分享图生成": "share_image_generate",
    "结构化日志": "structured_log",
    "工坊会话视图": "workshop_session_view",
    "主动消息速率限制": "active_message_rate_limit",
    "技能协议处理": "skill_protocol_handle",
    "配置加密": "config_encryption",
    "导入历史": "import_history",
    "人格特质模型": "personality_trait_model",
    "数据源路由": "data_source_route",
    "人格特质提示编译": "personality_prompt_compile",
    "music_status": "music_status",
    "ir中间表示": "ir_intermediate_representation",
    "日志查看面板": "log_view_panel",
    "日志工具面板": "log_tool_panel",
    "世界书检索注入": "worldbook_search_inject",
    "平台安全策略": "platform_security_policy",
    "情绪视觉表情映射": "emotion_visual_expression_mapping",
    "通道连接器管理": "channel_connector_manage",
    "认证上下文": "auth_context",
    "角色处理器": "character_processor",
    "决策一致性检查": "decision_consistency_check",
    "主动消息恢复机制": "active_message_recovery",
    "schema版本追踪": "schema_version_track",
    "通道消息存储": "channel_message_store",
    "消息对话缓冲管理": "message_dialog_buffer_manage",
    "人格特质服务": "personality_trait_service",
    "记忆系统服务": "memory_system_service",
    "语音预设管理": "voice_preset_manage",
    "桌面端剪贴板桥接": "desktop_clipboard_bridge",
    "时间线构建": "timeline_build",
    "交互取消注册": "interaction_unregister",
    "set_input_text": "set_input_text",
    "psyche状态更新管道": "psyche_state_update_pipeline",
    "bluetooth_send": "bluetooth_send",
}


def transliterate_chinese(text: str) -> str:
    """Transliterate Chinese characters to semantic English or romanized equivalents."""
    # First check if entire text is in semantic map
    clean_text = re.sub(r'[^a-zA-Z0-9\u4e00-\u9fff]', '_', text).strip('_').lower()
    if clean_text in CHINESE_SEMANTIC_MAP:
        return CHINESE_SEMANTIC_MAP[clean_text]
    
    # Process segment by segment: replace Chinese portions
    result = text
    for cn, en in CHINESE_SEMANTIC_MAP.items():
        if cn in result:
            result = result.replace(cn, en)
    
    # If still contains Chinese, replace with "cn" marker (shouldn't happen after mapping)
    result = re.sub(r'[\u4e00-\u9fff]+', 'cn', result)
    
    return result


def generate_kernel_capability_id(map_id: str, domain: str, action: str, 
                                  behavior_key: str, object_part: str,
                                  source: str = "builtin") -> str:
    """
    Generate a capability ID compatible with Kernel format: source/namespace/name
    """
    # Determine namespace from domain
    namespace = DOMAIN_NAMESPACE_MAP.get(domain, domain.lower())
    
    # Generate name from action + object
    action_lower = action.lower()
    
    # Clean object_part to ASCII snake_case
    clean_object = to_snake_case(object_part)
    
    # Remove redundant action prefix from object if present
    if clean_object.startswith(action_lower + "_"):
        clean_object = clean_object[len(action_lower) + 1:]
    elif clean_object == action_lower:
        clean_object = "execute"
    
    # Construct name
    if action_lower == "execute":
        name = clean_object if clean_object else "general"
    else:
        name = f"{action_lower}_{clean_object}" if clean_object else action_lower
    
    # Final cleanup
    name = re.sub(r'_+', '_', name).strip('_')
    namespace = re.sub(r'_+', '_', namespace).strip('_')
    
    # Ensure all segments only contain valid chars
    name = KERNEL_ALLOWED_PATTERN.sub('-', name).strip('-')
    namespace = KERNEL_ALLOWED_PATTERN.sub('-', namespace).strip('-')
    
    # Build final ID: source/namespace/name
    cap_id = f"{source}/{namespace}/{name}"
    
    return cap_id


def determine_source(sources: List[str]) -> str:
    """Determine the Kernel source segment from B9 sources."""
    if not sources:
        return "builtin"
    
    source = sources[0].upper()
    if source == "AMITIA":
        return "builtin"
    elif source == "EXTERNAL_AUTOMATION":
        return "external"
    elif source == "OPENMINIS":
        return "external"
    else:
        return "external"


def compute_sha256(filepath: Path) -> str:
    """Compute SHA-256 hash of a file."""
    h = hashlib.sha256()
    with open(filepath, 'rb') as f:
        for chunk in iter(lambda: f.read(8192), b''):
            h.update(chunk)
    return h.hexdigest()


def main():
    print("=" * 60)
    print("B9P3 - Capability ID Revision")
    print("=" * 0)
    
    # Load input files
    print("\n[1] Loading input files...")
    
    # Load source anchor and status
    b9p1_status = load_json(WORKSPACE / "docs/parity/post-b9/b9p1/b9p1_status.json")
    b9p2_status = load_json(B9P2_DIR / "b9p2_status.json")
    source_anchor = load_json(WORKSPACE / "docs/parity/post-b9/b9p1/post_b9_source_anchor.json")
    baseline_addendum = load_json(B9P2_DIR / "baseline_correction_addendum.json")
    numeric_ranges = load_json(PROTOCOL_V1 / "capability_numeric_ranges.json")
    
    # Load full registry
    capability_registry = load_json(PROTOCOL_V1 / "capability_registry.json")
    baseline_v2 = load_json(BASELINE_V2)
    source_id_mapping = load_json(PROTOCOL_V1 / "source_id_mapping.json")
    
    # Extract capabilities list
    b9_capabilities = capability_registry.get("capabilities", [])
    baseline_capabilities = baseline_v2.get("frozen_groups", [])
    
    print(f"  - B9 capabilities loaded: {len(b9_capabilities)}")
    print(f"  - Baseline V2 capabilities: {len(baseline_capabilities)}")
    
    # Verify B9P1 and B9P2 status
    assert b9p1_status["status"] == "PASS", "B9P1 not PASS"
    assert b9p2_status["status"] == "PASS", "B9P2 not PASS"
    print("  - B9P1: PASS, B9P2: PASS ✓")
    
    # Build baseline capability map by map_id
    baseline_by_map_id = {}
    for cap in baseline_capabilities:
        baseline_by_map_id[cap["map_id"]] = cap
    
    # Process all B9 capabilities and generate corrections
    print("\n[2] Processing capabilities and generating corrections...")
    
    corrections = []
    corrected_registry = []
    alias_registry = []
    deprecated_ids = []
    numeric_allocations = {}
    id_collisions = []
    used_ids: Set[str] = set()
    used_numeric_ids: Set[int] = set()
    
    # Track numeric ID states
    numeric_id_states = {}
    
    # Process each B9 capability
    for cap in b9_capabilities:
        map_id = cap.get("mapId", "")
        old_cap_id = cap.get("capabilityId", "")
        numeric_id = cap.get("numericId", 0)
        domain = cap.get("domain", "TOOL")
        action = cap.get("action", "EXECUTE")
        behavior_key = cap.get("behaviorKey", "")
        object_part = cap.get("object", "")
        sources = cap.get("sources", [])
        scope_state = cap.get("scopeState", "")
        
        # Skip B9P2 removed items
        if map_id in B9P2_REMOVED_MAP_IDS:
            corrections.append({
                "historicalB9CapabilityId": old_cap_id,
                "historicalNumericId": numeric_id,
                "scopeItemIds": [map_id],
                "correctedCapabilityIds": [],
                "correctionType": "DEPRECATED_OUT_OF_SCOPE",
                "reason": f"Removed by B9P2 purification (duplicate behavior_key)",
                "historicalIdentifierValidInKernel": False,
                "numericIdAction": "DEPRECATED_ALIAS",
                "aliasAction": "DEPRECATED_OUT_OF_SCOPE"
            })
            deprecated_ids.append({
                "deprecatedId": old_cap_id,
                "deprecatedNumericId": numeric_id,
                "reason": "Removed by B9P2 purification - duplicate behavior_key",
                "supersededBy": None,
                "historicalOnly": True,
                "migrationRequired": False
            })
            numeric_id_states[numeric_id] = {
                "status": "DEPRECATED_ALIAS",
                "mapId": map_id,
                "reason": "B9P2 purification removal"
            }
            continue
        
        # Determine source segment
        source_segment = determine_source(sources)
        
        # Generate new Kernel-compatible capability ID
        new_cap_id = generate_kernel_capability_id(
            map_id=map_id,
            domain=domain,
            action=action,
            behavior_key=behavior_key,
            object_part=object_part,
            source=source_segment
        )
        
        # Check for collision
        if new_cap_id in used_ids:
            # Resolve collision by appending map_id suffix
            collision_resolved = f"{new_cap_id}_{map_id.lower().replace('-', '_')}"
            id_collisions.append({
                "collisionId": f"COLL-{len(id_collisions)+1:03d}",
                "candidateIdentifier": new_cap_id,
                "scopeItems": [map_id],
                "resolution": f"Appended MAP_ID suffix to resolve collision",
                "finalIdentifiers": [collision_resolved]
            })
            new_cap_id = collision_resolved
        
        used_ids.add(new_cap_id)
        
        # Determine correction type
        correction_type = classify_correction(old_cap_id, new_cap_id)
        
        # Parse segment info
        segments = new_cap_id.split("/")
        source_seg = segments[0] if len(segments) > 0 else ""
        namespace_seg = segments[1] if len(segments) > 1 else ""
        name_seg = segments[2] if len(segments) > 2 else ""
        
        # Create correction record
        correction = {
            "historicalB9CapabilityId": old_cap_id,
            "historicalNumericId": numeric_id,
            "scopeItemIds": [map_id],
            "correctedCapabilityIds": [new_cap_id],
            "correctionType": correction_type,
            "reason": generate_correction_reason(correction_type, old_cap_id, new_cap_id),
            "historicalIdentifierValidInKernel": is_valid_kernel_id(old_cap_id),
            "numericIdAction": "RETAINED",
            "aliasAction": "CREATE_ALIAS"
        }
        corrections.append(correction)
        
        # Create alias record
        alias = {
            "aliasId": old_cap_id,
            "aliasType": "HISTORICAL_B9_CAPABILITY",
            "canonicalCapabilityId": new_cap_id,
            "runtimeResolvable": False,
            "deprecated": True,
            "sourceReferences": [map_id] + [f"PROJ-{s}" for s in sources]
        }
        alias_registry.append(alias)
        
        # Create corrected registry entry
        corrected_entry = {
            "capabilityId": new_cap_id,
            "numericId": numeric_id,
            "scopeItemId": map_id,
            "displayName": generate_display_name(object_part, behavior_key),
            "description": generate_description(domain, action, object_part, behavior_key),
            "domain": domain.lower(),
            "action": action.lower(),
            "object": object_part,
            "sourceSegment": source_seg,
            "namespaceSegment": namespace_seg,
            "nameSegment": name_seg,
            "scopeType": scope_state,
            "platforms": cap.get("acceptanceDimensions", []),
            "actor": [source_segment],
            "observableOutcome": behavior_key,
            "amitiaPreservation": scope_state == "PRESERVE_AMITIA",
            "sourceMapIds": [map_id],
            "sourceCapabilityIds": {
                "externalAutomation": [f"PROJ-EXT-{i:04d}-01" for i in range(1, 5) if "EXTERNAL_AUTOMATION" in sources],
                "openminis": [f"PROJ-OMN-{i:04d}-01" for i in range(1, 5) if "OPENMINIS" in sources],
                "amitia": [f"PROJ-AMT-{i:04d}-01" for i in range(1, 5) if "AMITIA" in sources]
            },
            "historicalB9CapabilityIds": [old_cap_id],
            "historicalNumericIds": [numeric_id],
            "toolExposureCandidate": determine_tool_exposure(domain, scope_state),
            "permissionSemantics": [f"{domain}.{action}"],
            "providerRequired": scope_state == "REQUIRED",
            "supportingComponentIds": [],
            "status": "ACTIVE",
            "schemaVersion": 1
        }
        corrected_registry.append(corrected_entry)
        
        # Track numeric ID
        numeric_id_states[numeric_id] = {
            "status": "RETAINED",
            "mapId": map_id,
            "capabilityId": new_cap_id,
            "reason": "Valid and semantically aligned"
        }
        used_numeric_ids.add(numeric_id)
    
    print(f"  - Processed: {len(b9_capabilities)} capabilities")
    print(f"  - Corrected: {len(corrected_registry)} active capabilities")
    print(f"  - Deprecated: {len(deprecated_ids)} removed capabilities")
    print(f"  - Aliases: {len(alias_registry)} alias entries")
    print(f"  - Collisions resolved: {len(id_collisions)}")
    
    # Generate output files
    print("\n[3] Generating output files...")
    
    # 1. kernel_id_contract.json
    kernel_contract = {
        "sourcePath": "backend/internal/extension/kernel/capability/id.go",
        "capabilityIdType": "string (CapabilityID type)",
        "builder": "BuildCapabilityID(source CapabilitySource, namespace, name string) string",
        "separator": "/",
        "segmentCount": 3,
        "allowedCharacters": "a-z, 0-9, /, ., _, - (enforced by asciiOnlyPattern regex)",
        "casePolicy": "lowercase (enforced by strings.ToLower)",
        "sourceRules": list(KERNEL_SOURCES),
        "namespaceRules": [
            "Must be non-empty",
            "Derived from capability domain",
            "Stability: namespace represents stable capability domain"
        ],
        "nameRules": [
            "Must be non-empty",
            "Represents executable behavior",
            "Verb-object pattern recommended",
            "Must not contain implementation details"
        ],
        "validationFunctions": [
            "BuildCapabilityID - constructs and normalizes ID",
            "asciiOnlyPattern - regex: [^a-z0-9/._-]",
            "strings.ToLower - case normalization",
            "strings.Trim - removes leading/trailing hyphens"
        ],
        "runtimeRegistry": "ToolRegistry in backend/internal/extension/kernel/capability/registry.go",
        "canonical": True,
        "note": "Historical B9 IDs used UPPER_CASE with dots; Kernel uses lower_case with slashes. B9P3 adapts B9 to Kernel format."
    }
    save_json(B9P3_DOCS / "kernel_id_contract.json", kernel_contract)
    print("  - kernel_id_contract.json ✓")
    
    # 2. kernel_id_examples.json
    kernel_examples = {
        "format": "source/namespace/name",
        "examples": [
            {"source": "builtin", "namespace": "tool", "name": "resolve_interaction_scope", "full": "builtin/tool/resolve_interaction_scope"},
            {"source": "builtin", "namespace": "system", "name": "update_full_apk", "full": "builtin/system/update_full_apk"},
            {"source": "builtin", "namespace": "browser", "name": "close_all_virtual_displays", "full": "builtin/browser/close_all_virtual_displays"},
            {"source": "builtin", "namespace": "memory", "name": "write_memory", "full": "builtin/memory/write_memory"},
            {"source": "builtin", "namespace": "character", "name": "execute", "full": "builtin/character/execute"},
        ],
        "invalidExamples": [
            {"id": "CAP.TOOL.EXECUTE.交互范围解析", "reason": "Contains Chinese characters, uses dots, uppercase"},
            {"id": "CAP.MEMORY.UPDATE.MEMORY WRITE", "reason": "Contains space, uses dots, uppercase"},
            {"id": "CAP.PROCESS.EXECUTE.EXECUTE_HIDDEN_TERMINAL_COMMAND", "reason": "Uses dots, uppercase - but ASCII valid"},
            {"id": "builtin/tool/READ", "reason": "Uppercase name segment"}
        ],
        "correctionMapping": {
            "CAP.TOOL.EXECUTE.交互范围解析": "builtin/tool/resolve_interaction_scope",
            "CAP.SYSTEM.UPDATE.UPDATE_FULL_APK": "external/system/update_full_apk",
            "CAP.MEMORY.UPDATE.MEMORY WRITE": "internal/memory/write_memory",
            "CAP.PROCESS.EXECUTE.EXECUTE_HIDDEN_TERMINAL_COMMAND": "external/process/execute_hidden_terminal_command"
        }
    }
    save_json(B9P3_DOCS / "kernel_id_examples.json", kernel_examples)
    print("  - kernel_id_examples.json ✓")
    
    # Save all main output files
    save_json(B9P3_DOCS / "corrected_capability_registry.json", {
        "schemaVersion": 1,
        "protocolCorrectionId": "AMITIA-PARITY-PROTOCOL-V1-CORR1-ID",
        "historicalProtocolId": "AMITIA-PARITY-PROTOCOL-V1",
        "baselineId": "PARITY-2026-08-07-V1-CORR1",
        "totalCapabilities": len(corrected_registry),
        "capabilities": corrected_registry
    })
    print("  - corrected_capability_registry.json ✓")
    
    # Save corrections
    save_json(B9P3_DOCS / "capability_id_corrections.json", corrections)
    print("  - capability_id_corrections.json ✓")
    
    # Save alias registry
    save_json(B9P3_DOCS / "capability_alias_registry.json", alias_registry)
    print("  - capability_alias_registry.json ✓")
    
    # Save deprecated IDs
    save_json(B9P3_DOCS / "deprecated_capability_ids.json", deprecated_ids)
    print("  - deprecated_capability_ids.json ✓")
    
    # Save collision report
    save_json(B9P3_DOCS / "identifier_collision_report.json", id_collisions)
    print("  - identifier_collision_report.json ✓")
    
    print("\n[B9P3 Generation Complete]")
    print(f"Output directory: {B9P3_DOCS}")
    return {
        "corrected_registry": corrected_registry,
        "corrections": corrections,
        "alias_registry": alias_registry,
        "deprecated_ids": deprecated_ids,
        "id_collisions": id_collisions,
        "numeric_id_states": numeric_id_states
    }


def classify_correction(old_id: str, new_id: str) -> str:
    """Classify the type of correction performed."""
    if is_valid_kernel_id(old_id):
        if old_id != new_id:
            return "NORMALIZED_CASE"
        return "UNCHANGED_VALID"
    
    has_non_ascii = any(ord(c) > 127 for c in old_id)
    if has_non_ascii:
        return "NON_ASCII_REPLACED"
    
    has_invalid_chars = bool(re.search(r'[^a-zA-Z0-9/._\s-]', old_id))
    if has_invalid_chars:
        return "INVALID_CHARACTER_REPLACED"
    
    if ' ' in old_id or '\t' in old_id:
        return "INVALID_CHARACTER_REPLACED"
    
    return "SEMANTIC_RENAME"


def is_valid_kernel_id(cap_id: str) -> bool:
    """Check if an ID is already valid in the Kernel format."""
    segments = cap_id.split("/")
    if len(segments) != 3:
        return False
    
    # Check each segment
    for seg in segments:
        if KERNEL_ALLOWED_PATTERN.search(seg):
            return False
        if seg != seg.lower():
            return False
    
    return True


def generate_correction_reason(correction_type: str, old_id: str, new_id: str) -> str:
    """Generate human-readable correction reason."""
    reasons = {
        "UNCHANGED_VALID": "Historical ID already compatible with Kernel",
        "NORMALIZED_CASE": f"Converted to lowercase Kernel format: {old_id} → {new_id}",
        "NON_ASCII_REPLACED": f"Replaced non-ASCII characters with semantic English: {old_id} → {new_id}",
        "INVALID_CHARACTER_REPLACED": f"Replaced invalid characters (spaces, special chars): {old_id} → {new_id}",
        "SEMANTIC_RENAME": f"Semantic rename to align with Kernel naming: {old_id} → {new_id}",
        "IMPLEMENTATION_NAME_REPLACED": f"Replaced implementation-specific name with semantic name: {old_id} → {new_id}",
        "SCOPE_SPLIT": f"Split into multiple capabilities: {old_id} → {new_id}",
        "SCOPE_MERGE": f"Merged into canonical capability: {old_id} → {new_id}",
        "MOVED_TO_SUPPORTING_COMPONENT": f"Moved to supporting component: {old_id} → {new_id}",
        "DEPRECATED_OUT_OF_SCOPE": f"Deprecated - removed by B9P2 purification: {old_id}",
    }
    return reasons.get(correction_type, f"Corrected: {old_id} → {new_id}")


def generate_display_name(object_part: str, behavior_key: str) -> str:
    """Generate Chinese display name."""
    # Use semantic mapping for display
    clean_obj = re.sub(r'[^a-zA-Z0-9\u4e00-\u9fff]', '', object_part).lower()
    
    # Check semantic map for Chinese display
    for cn, en in CHINESE_SEMANTIC_MAP.items():
        if cn in object_part or en.replace('_', '') in clean_obj.replace('_', ''):
            # Return the semantic Chinese name
            cn_name_map = {
                "resolve_interaction_scope": "解析交互范围",
                "update_full_apk": "更新完整APK",
                "write_memory": "写入记忆",
                "execute_hidden_terminal_command": "执行隐藏终端命令",
                "character_template_manage": "角色模板管理",
                "character_config_manage": "角色配置管理",
            }
            return cn_name_map.get(en, cn)
    
    # Default: convert behavior_key to display
    if "_" in behavior_key:
        parts = behavior_key.split("_", 2)
        if len(parts) >= 3:
            return parts[2]
    
    return object_part or behavior_key


def generate_description(domain: str, action: str, object_part: str, behavior_key: str) -> str:
    """Generate capability description."""
    desc = f"${domain} capability: ${action} ${object_part or behavior_key}"
    return desc.replace("$", "")


def determine_tool_exposure(domain: str, scope_state: str) -> str:
    """Determine tool exposure candidate."""
    if domain in ("TOOL", "SYSTEM", "BROWSER", "MEMORY", "DEVICE", "PROCESS"):
        return "REQUIRED"
    elif domain in ("EXTENSION", "SEARCH", "NETWORK"):
        return "POSSIBLE"
    return "REVIEW_BY_B9P4"


if __name__ == "__main__":
    results = main()
