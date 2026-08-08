# B9P6 Extension Kernel唯一生产入口收口审计

## 概述

本目录包含 B9P6 的完整审计输出，确认 Extension Kernel 作为 Amitia 唯一正式扩展底座的地位，
防止 B10~B154 期间创建第二套 Tool/Permission/Execution/MCP/Skill/Workflow/Runtime/Registry 系统。

## 核心结论

- Extension Kernel 已确认是唯一正式扩展底座
- ToolFacade 已确认是正式模型 Tool 入口
- ToolRegistry 是唯一 Global Agent Tool Registry
- ExecutionPipeline 是唯一通用 Tool 执行链
- PermissionBroker 是唯一 Kernel 权限决策中心
- Runtime Adapter 体系是后续 Provider 唯一接入合同
- 旧 agent/tool 只剩类型、迁移或测试职责
- 旧 extension Runtime 不再承担生产 Tool 执行（仅作为 Kernel bridge）
- 无生产 Legacy fallback
- 无生产 Fake Provider
- 无第二套 Registry/Executor/Permission/Runtime
- 已生成禁止第二套系统的正式 Guard

## 关键架构说明

### ToolFacade 不是第二套 Execution Kernel

ToolFacade 是模型 Tool 统一入口（Facade 模式），不判断权限、不选择 Provider、不独立执行 Tool。
所有安全判断和实际执行仍在 ExecutionPipeline 中完成。

### chatToolRuntimeAdapter 不是第二套 Tool Runtime

chatToolRuntimeAdapter 是 BOUNDARY_ADAPTER，它不保存 Registry、不独立执行 Tool、不判权限、
不选择 Provider。只把 Chat 接口转给 ToolFacade。

### MCP Registry 不是第二套 Agent Tool Registry

MCP Domain 管理器负责 Connection/Transport/OAuth/Health/Server，但最终 Agent Tool 曝光
必须由 ToolRegistry 统一。mcpToolFacadeSyncerAdapter 确保路径正确。

### Workflow Executor 不是第二套通用 Tool Executor

Workflow Executor 是 domain 能力（处理 workflow-step、compensation、checkpoint），
当 Workflow 作为 Agent Tool 暴露时，通过 WorkflowRuntimeAdapter 进入 Kernel 路径。

### OS Permission 不是第二套 Permission Broker

Android CAMERA、iOS HealthKit 等 PLATFORM_PERMISSION_PROVIDER 是 OS 级权限，
不算 Extension Kernel 重复。后续映射进统一 Capability Permission 合同。

### Runtime Adapter 不是第二套 Runtime Manager

所有 Runtime 通过统一 RuntimeAdapter / RuntimeBinding 合同接入，
每个 Runtime 各自发明 Tool 调用协议是禁止的。

## 目录结构

| 文件 | 说明 |
|------|------|
| B9P6_ExtensionKernel唯一生产入口收口报告.md | 主报告 |
| b9p6_status.json | 执行状态 |
| input_manifest.json | 输入清单 |
| extension_kernel_inventory.json | Extension Kernel 组件清单 |
| production_entrypoints.json | 生产入口分类 |
| production_tool_chain.json | 正式 Tool 生产链 |
| production_registration_chain.json | 正式注册链 |
| production_permission_chain.json | 正式 Permission 链 |
| production_runtime_chain.json | 正式 Runtime 链 |
| chat_tool_wiring.json | Chat 接线分析 |
| mcp_tool_wiring.json | MCP 接线分析 |
| skill_wiring.json | Skill 接线分析 |
| workflow_wiring.json | Workflow 接线分析 |
| hook_event_schedule_wiring.json | Hook/Event/Schedule 接线分析 |
| runtime_adapter_inventory.json | Runtime Adapter 清单 |
| canonical_registry_inventory.json | Canonical Registry 清单 |
| canonical_executor_inventory.json | Canonical Executor 清单 |
| canonical_permission_inventory.json | Canonical Permission 清单 |
| canonical_runtime_inventory.json | Canonical Runtime 清单 |
| legacy_registry_inventory.json | Legacy Registry 清单 |
| legacy_executor_inventory.json | Legacy Executor 清单 |
| legacy_runtime_inventory.json | Legacy Runtime 清单 |
| legacy_usage_classification.json | Legacy 分类 |
| legacy_reference_inventory.json | Legacy 引用清单 |
| production_bypass_inventory.json | 生产旁路清单 |
| direct_execution_inventory.json | 直接执行清单 |
| duplicate_registry_inventory.json | 重复 Registry 分析 |
| duplicate_permission_inventory.json | 重复 Permission 分析 |
| duplicate_executor_inventory.json | 重复 Executor 分析 |
| duplicate_runtime_inventory.json | 重复 Runtime 分析 |
| fake_mock_provider_inventory.json | Fake/Mock Provider 分析 |
| migration_adapter_inventory.json | Migration Adapter 清单 |
| compatibility_layer_inventory.json | 兼容性层清单 |
| legacy_counter_inventory.json | Legacy Counter 清单 |
| final_gate_inventory.json | Final Gate 清单 |
| canonical_system_resolution.json | Canonical System 决议 |
| duplicate_system_guard.json | 禁止第二套系统 Guard |
| production_entry_guard.json | 生产入口 Guard |
| migration_exit_conditions.json | Migration 退出条件 |
| b140_preconditions.json | B140/B141 预条件 |
| b9p4_kernel_input.json | B9P4 输入 |
| B9P4_input_manifest.md | B9P4 输入清单 |
| verification.log | 验证日志 |

## 限制

B9P6 不修改业务代码、不删除 Legacy 代码、不修复生产旁路。
物理删除留给后续明确迁移步骤（B140+）。

## 阻断项

services.go 存在哈希漂移（Post-B9P1 commit d42ddf7 导致），
Extension Kernel 核心文件无漂移。漂移文件不影响 Kernel 架构分析。
