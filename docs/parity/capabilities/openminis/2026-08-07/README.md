# OpenMinis 跨平台完整能力审计 (B5, 2026-08-07)

本目录存储 B5 的完整扫描结果。

## 状态

**STATIC_AUDIT_PASS / PRODUCT_RUNTIME_PARTIAL**

静态代码审计完成，产品运行验证未执行（需真机环境）。

## 能力统计

| 指标 | 值 |
|------|------|
| 原子能力总数 | 145 |
| Shared | 89 |
| Shared with Platform Adapter | 2 |
| iOS Only | 31 |
| Android Only | 23 |
| IMPLEMENTED | 133 |
| SOURCE_NOT_VERIFIED_IOS | 12 |

## 文件说明

### 核心目录
- `B5_OpenMinis跨平台完整能力审计报告.md` - 完整审计报告
- `capability_catalog.json` - 机器可读能力目录（145 条）
- `capability_matrix.md` - 可读能力矩阵表
- `B5_summary.json` - JSON 统计汇总
- `verification.log` - 验证日志

### 架构与模块
- `module_architecture.json` - 模块清单（16 个模块）
- `shared_core_inventory.json` - 共享核心组件清单
- `ui_entrypoints.json` - UI 入口点清单
- `navigation_routes.json` - 导航路由清单
- `agent_runtime.json` - Agent Runtime 执行流

### 工具与运行时
- `tool_registry.json` - Tool 注册表（8 个 Tool ID）
- `tool_execution_chains.json` - 工具执行链追踪
- `sandbox_architecture.json` - Linux Sandbox 架构（iOS iSH + Android PRoot）
- `sandbox_tool_inventory.json` - 沙箱能力清单

### Native Offload
- `native_offload_protocol.json` - Offload 通信协议
- `native_offload_inventory.json` - Offload 完整清单（iOS 19+ / Android 20+）

### 专项能力
- `browser_inventory.json` - 浏览器自动化能力
- `workspace_inventory.json` - Workspace 管理能力
- `skill_inventory.json` - Skill 运行时能力
- `skill_format_inventory.json` - Skill 格式规范
- `mcp_inventory.json` - MCP 协议支持
- `memory_inventory.json` - 持久记忆系统
- `model_provider_inventory.json` - 模型 Provider 清单
- `local_model_inventory.json` - 本地模型（无）
- `voice_inventory.json` - 语音系统（STT/TTS）

### 原生平台工具
- `ios_native_tools.json` - iOS Native 工具清单
- `android_native_tools.json` - Android Native 工具清单
- `ios_framework_inventory.json` - iOS Framework 使用清单
- `android_service_inventory.json` - Android Service 清单

### URI、权限、安全
- `resource_uri_inventory.json` - 资源 URI 模型
- `security_privacy_inventory.json` - 安全与隐私机制
- `permission_matrix.json`（通过 ios/android 原生工具体现）

### 后台与通知
- `schedule_background_inventory.json` - 后台任务与定时调度
- `notification_overlay_inventory.json` - 通知与悬浮状态

### 导入导出
- `import_export_backup.json` - 导入/导出/备份/迁移

### 差异矩阵
- `platform_parity_matrix.json` - iOS/Android 平台等价对比
- `source_product_matrix.json` - 源码 vs 产品发布对比
- `release_claims_mapping.json` - Android Release 声明映射
- `appstore_claims_mapping.json` - iOS App Store 声明映射

### 审计与测试
- `product_runtime_verification.json` - 产品运行验证状态
- `test_coverage_mapping.json` - 测试覆盖映射
- `source_scan_coverage.json` - 源码覆盖率（100%）
- `capability_evidence.json` - 能力证据索引
- `capability_relations.json` - 能力依赖关系

### 其他
- `duplicate_and_aliases.json` - 重复项和别名
- `unresolved_items.json` - 未确认项（4 项）
- `excluded_files.json` - 排除文件（0 项）

## 关键发现

1. 代码库：364 Swift + 454 Kotlin + 228 JS + 133 Shell + 37 Python + 19 C/C++
2. Sandbox：iOS iSH 仿真 + Android PRoot 真实 chroot
3. 工具：统一的 8 个核心 Tool ID，iOS/Android 完全对齐
4. Provider：8+ 模型 Provider，支持 36+ 模型
5. 本地 LLM：无设备端推理实现
6. 三层基线重合：源码/Tag/main HEAD 指向同一 commit

## 后续 B6 输入

此目录产物可直接作为 B6 三方能力矩阵整合的 OpenMinis 侧输入。
