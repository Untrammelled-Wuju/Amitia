# U-Ai Workbench Batch 40：Coding Worker 隔离边界与研发验证口径

## 研发进度与验收状态

本轮将研发范围估算推进至 **85.5%**。该百分比反映研发功能覆盖，不代表正式服务已构建或通过生产级验收。是否能获取 Go 1.26.1、Flutter SDK 和 Windows 真机不再成为研发范围进度的硬性上限；这些条件仍作为正式构建和发布验收中未验证的项目如实记录。

## 本轮功能闭环修复

- Coding Worker 在申请 Worktree 前检查父工作区 URI 和挂载 ID 的一致性，拒绝嵌入路径、查询参数及其他非标准挂载引用。
- Worktree 分配完成后检查返回的本机挂载 ID、URI、WorkspaceKind，以及新挂载与父挂载必须不同。结果不可信时不启动子 Agent，也不退回父工作区。
- 新增针对共享目录、跨挂载伪造、路径穿越、非法挂载 ID、错误类型和合法斜杠形式的测试。
- 检查点测试启动工具不可用时不再只留下未验证的占位记录，而是写入实际失败证据；进程重启后仍无法以该记录审批或合并。
- 原有真实 Git Worktree 生命周期、审批与合并回归继续执行，不替换原有 Git 引擎。

## 独立研发验证与生产验收分离

`node scripts/workbench/integration-gate.mjs --development-only` 是研发验证模式，允许正式环境缺失时仍执行 Go race 与 Node 定向测试，同时保留 blocked 状态。测试失败依然导致非零退出码。

生产级验收须单独具备完整 Go 服务构建、真实身份认证 HTTP、Coding Agent 调度调用、Web/Electron/Flutter 以及设备安全联调的证据。研发验证模式不会将这些项目变成 passed。

## 定向验证范围

1. `go test -race -count=1` 运行完整 `gitworktree` 模块，覆盖真实临时 Git 仓库和持久化检查点。
2. 独立 Go race 测试直接编译 `multi_agent_coding_worktree.go` 与其测试，使用最小接口桩，只证明该包装器的安全拒绝行为，并不等价于完整 Interaction 模块。
3. 前端 WebSocket、终端缓冲区、会话 Manager、已认证只读 HTTP 模拟测试继续作为回归门禁。
4. 压缩包必须通过原始源码 + 累计替换包 = 累计完整源码的逐文件字节一致性检查。

## 剩余真实验收项

完整 Go 1.26.1 后端构建与真实 API 认证、Coding Agent 真实消息调度到测试审批合并、Windows ConPTY 与 Electron、Flutter Android/iOS Release、Device Mesh 多设备联调、生产级安全及性能压测尚未获得通过证据。不得依据本轮的研发范围百分比推断这些能力已经通过验收。
