# Workbench 第 31 轮：真实构建前置门禁与文件预览协议闭环

## 开发范围

本轮以 Batch30 累计替换包覆盖原始 `U-Ai-source-20261014-1052` 工程继续。优先修复真实 API、Web 和 Flutter 工作台间的文件读取不一致，并为 80%→85% 阶段引入可以运行、按状态分类的构建与验收门禁。没有重新开发已有 Browser、Worktree 或 Terminal 执行器。

## 已修改并保存到工程的代码

- `backend/internal/workspace/read_chunk_response.go` 新增可选 `rawBytes=1` 文件读取响应，返回 Base64 原始字节、实际读取长度、下一字节偏移、续读提示，避免 UTF-8 跨片导致错误偏移；原来不传参数的客户端仍返回旧文本结构。`handler.go` 对负偏移、非法大小和偏移溢出拒绝，对过大请求做兼容性截断，文件读取响应禁止缓存。
- `front/src/components/workbench/workbench-document-chunks.ts` 按原始字节有界累积并对结果校验；`WorkbenchResourceView.vue` 使用新协议，同时对标签资源变化清除历史字节和游标，阻止旧请求结果覆盖新资源。
- `mobile_app/lib/features/workbench/runtime/workbench_document_chunks.dart` 采用同样的字节与偏移边界，恢复被分片拆开的 UTF-8 多字节字符；`workbench_document_view.dart` 在资源切换、销毁时撤销旧回调，并限制累计预览为 8 MiB。
- `mobile_app/lib/features/workbench/presentation/pages/workbench_task_checkpoints.dart` 对可空 HTTP 响应先判空，避免空安全编译问题。
- `backend/internal/workbenchtasks/projector.go` 和 `checkpoint_history.go` 在数据库访问前检查取消信号；任务投影循环遇到取消则终止，不发回失效请求的历史数据。
- `scripts/workbench/integration-gate.mjs` 提供无下载、无 Git 修改、按状态记账的构建前置检查和定向验证入口。前置用法：`node scripts/workbench/integration-gate.mjs --preflight-only`；有完整环境时去掉参数可执行目标 Go 包测试、后端编译、前端及 Electron 类型检查、Flutter 定向检查。真实认证、Coding Worker 合并和设备端到端必须另外做实测。

## 已真实执行

- 上传的原始源码 ZIP 与 Batch30 累计替换 ZIP 均通过 CRC 完整性检查；本轮基于两者合成的累计工作目录开发。
- Go 1.23.2 的标准库隔离测试：`GO111MODULE=off go test -race models.go read_chunk_response.go read_chunk_response_test.go` 通过。仅证明字节包装逻辑，不代表完整 Workspace 包编译通过。
- 终端缓冲区独立单测：`GO111MODULE=off go test -race buffer.go buffer_test.go` 通过。
- Linux Node 22 + TypeScript 全局二进制：新分片工具的 `tsc --strict --noEmit` 通过；实际编译后运行断言通过；Web Vue `script setup` 语法解析无错误。未执行完整 Vue 应用类型检查。
- 构建前置门禁实际执行：本环境 Go 1.23.2，而主工程声明 Go 1.26.1；缺少 pnpm 工程依赖和 Flutter SDK。Go 全工程编译、Vite/Vue-TSC、Electron/Flutter 构建因此未通过验收。

## 尚未通过的真实验收

服务真实启动和迁移、认证 HTTP、Coding Agent 从自然语言任务到 Worktree/工具/测试/审批/安全合并的完整链路、Windows ConPTY、Chromium 同页控制、Device Mesh 多设备隔离、Flutter Android Release 与 iOS 构建、并发及故障恢复。绝对不能把本轮的标准库或静态检查称作以上验收通过。

## 阶段位置

Batch30 的研发范围估算为 80%；Batch31 新增必要的接口修复、跨端文件预览一致性以及可复现门禁，研发范围暂估 **81%**。80%→85% 阶段尚未完成，进入下一轮时首要目标仍是拿到 Go 1.26.1、真实前后端环境并修复首次完整编译错误；不得以新增未验收功能替代全工程测试。
