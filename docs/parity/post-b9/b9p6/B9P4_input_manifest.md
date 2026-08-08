# B9P4 Input Manifest

## 来源
本文档引用 B9P6 正式输出，作为 B9P4 的 Extension Kernel 输入依据。

## 引用文件（B9P6 输出）

| 文件 | 路径 | 用途 |
|------|------|------|
| canonical_system_resolution.json | docs/parity/post-b9/b9p6/canonical_system_resolution.json | 正式Canonical系统解析 |
| duplicate_system_guard.json | docs/parity/post-b9/b9p6/duplicate_system_guard.json | 禁止第二套系统Guard |
| production_entry_guard.json | docs/parity/post-b9/b9p6/production_entry_guard.json | 生产入口Guard |
| production_tool_chain.json | docs/parity/post-b9/b9p6/production_tool_chain.json | 正式Tool生产链 |
| production_permission_chain.json | docs/parity/post-b9/b9p6/production_permission_chain.json | 正式Permission链 |
| production_runtime_chain.json | docs/parity/post-b9/b9p6/production_runtime_chain.json | 正式Runtime链 |
| b9p4_kernel_input.json | docs/parity/post-b9/b9p6/b9p4_kernel_input.json | B9P4 Kernel输入 |

## 明确排除的依赖
- 本文档不依赖 B9P3 输出
- 本文档不依赖 B9P5 输出

## Canonical 系统摘要
- One ToolRegistry: `backend/internal/extension/kernel/capability/registry.go`
- One ExecutionPipeline: `backend/internal/extension/kernel/execution/pipeline.go`
- One PermissionBroker: `backend/internal/extension/kernel/permission/broker.go`
- One Model Tool Facade: `backend/internal/extension/kernel/tool_facade.go`
- One Container Builder: `backend/internal/extension/kernel/container_builder.go`

## 阻断项
services.go 发现哈希漂移（Post-B9P1 commit d42ddf7 桌宠P0修复导致），Extension Kernel 核心文件无漂移。

## 时间戳
2026-08-07T18:30:00+08:00
