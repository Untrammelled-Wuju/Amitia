# B9P4: Capability → Tool → Permission → Provider/Runtime 正式映射收口

## 任务定位

B9P4是B9P3、B9P5、B9P6三条并行补丁轨道的汇合步骤，负责正式确定：

- Corrected Capability 是否暴露为 Agent Tool
- 现有 ToolRegistry 中的 Tool 映射
- 现有 PermissionDefinition / PermissionBroker 的权限需求
- RuntimeBinding / RuntimeAdapter 的运行时需求
- Provider 的实现需求
- 状态/错误 Projection 的协议合并

## 关键设计原则

1. **Capability ≠ Tool**：1 Capability → 0..N Tool，不是所有能力都需要成为 Agent Tool
2. **现有 Kernel 唯一**：resolved_tool_registry.json 只是 Protocol/Mapping Manifest，运行时事实源仍是 `extension/kernel/capability.ToolRegistry`
3. **Permission 归一**：所有 Permission 映射到现有 PermissionDefinitionRegistry + PermissionBroker
4. **Provider 不直接对 Agent**：Agent → Tool → Kernel → Provider/Runtime

## 重要声明

⚠️ **本目录包含的 JSON 文件均标记 `runtimeAuthority: false`。**

任何 `resolved_*.json` 文件只能用于：
- 生成/验证协议合同
- 迁移规划
- 审计追溯

**禁止将这些 JSON 当作运行时注册表使用。** 它们不得：
- 在应用启动时加载为独立 Tool Registry
- 与 Kernel Registry 双写
- 作为运行时权限决策源

## 输入补丁

- B9P3: PASS (Corrected Capability Registry, ID Correction)
- B9P5: PASS (State/Error Protocol Projection)
- B9P6: PASS (Canonical System Resolution)

## 输出文件

### 核心注册表
- `resolved_capability_registry.json` - 502个Corrected Capability完整注册
- `capability_exposure_policy.json` - 502项Capability暴露策略
- `capability_tool_mapping.json` - Capability → Tool映射
- `resolved_tool_registry.json` - 253个Agent Tool合同（非运行时注册表）
- `agent_callable_capabilities.json` - 253个Agent Callable能力
- `non_tool_capabilities.json` - 249个Non-Agent能力

### 权限/Provider
- `permission_semantic_registry.json` - 53个Permission语义
- `capability_permission_mapping.json` - 502项Capability权限映射
- `resolved_permission_mapping.json` - 语义→Kernel Permission解析
- `permission_gap_inventory.json` - 14个Permission GAP
- `platform_permission_requirements.json` - 按平台分组的权限需求
- `provider_semantic_registry.json` - 32个Provider语义
- `capability_provider_mapping.json` - Capability → Provider映射
- `provider_gap_inventory.json` - 24个Provider GAP

### 运行时
- `runtime_binding_mapping.json` - Tool → RuntimeBinding映射
- `runtime_adapter_mapping.json` - 13个Adapter状态

### 协议合同
- `capability_execution_contracts.json` - Capability执行合同
- `capability_state_projection.json` - 状态Projection映射
- `capability_error_projection.json` - 错误Projection映射
- `resolved_protocol_contract.json` - 统一协议合同

### 历史/Migration
- `historical_tool_resolution.json` - 506个V1历史Tool决议
- `legacy_tool_alias_mapping.json` - 旧Tool Alias映射
- `legacy_permission_mapping.json` - 旧Permission映射
- `composite_tool_mapping.json` - 复合Tool映射
- `migration_requirements.json` - 迁移需求

### Guard
- `duplicate_registry_guard.json` - 禁止第二Registry
- `execution_bypass_guard.json` - 禁止执行旁路
- `unresolved_mapping_items.json` - 未解决项（应为0）

### 状态/验证
- `b9p4_status.json` - 任务状态
- `input_manifest.json` - 输入文件清单
- `mapping_validation.json` - 映射验证指标
- `verification.log` - 验证日志
- `B9P7_input_manifest.json` - B9P7输入

## 分类统计

### Capability Exposure
| 类型 | 数量 |
|------|------|
| AGENT_TOOL_REQUIRED | 147 |
| AGENT_TOOL_OPTIONAL | 106 |
| INTERNAL_ONLY_SUPPORT | 225 |
| EXTENSION_API_ONLY | 7 |
| SYSTEM_EVENT_ONLY | 3 |
| UI_ONLY | 8 |
| PLATFORM_ONLY | 4 |
| NOT_AGENT_CALLABLE | 2 |
| **总计** | **502** |

### 历史Tool决议
| 分类 | 数量 |
|------|------|
| MIGRATE_TO_KERNEL_TOOL_ID | 253 |
| NOT_ACTUALLY_A_TOOL | 253 |
| 其他 | 0 |

### Permission语义
| 分类 | 数量 |
|------|------|
| REUSE_EXISTING | 39 |
| MISSING_PERMISSION_DEFINITION | 14 |

### Runtime Adapter
| 状态 | 数量 |
|------|------|
| EXISTING | 10 |
| NEW_PLATFORM_ADAPTER_REQUIRED | 3 |

## 后续步骤

B9P4完成后，B9P7负责将后续B10-B154每一步映射到具体实现决策。

## 状态

任务状态: PASS
创建时间: 2026-08-07
