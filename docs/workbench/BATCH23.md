# Batch 23：终端会话生命周期事件、双端观察与有界导出

## 范围

在既有 `workbenchterminal` 模块增加独立生命周期事件层：`lifecycle.go` 定义序列与环形缓冲，`lifecycle_handler.go` 仅提供认证后的只读查询，`lifecycle_test.go` 覆盖并发、游标、越权、退出、截断与导出。服务端启动、窗口变更与实际进程退出会生成结构化记录；用户输入及终端原始内容不会写入事件数组。

浏览器/Web/Electron 通过 `terminalLifecycle.ts` 解码、验证、去重，生命周期面板与已有终端输出分开。Flutter 提供只读的当前会话事件查看入口。已有有限缓冲区导出增加 `lifecycle`、`lifecycleLatestSequence` 和 `lifecycleTruncated`，以便人工排查会话状态。协议分别放在 `terminal-lifecycle-v1.schema.json` 与 `terminal-transcript-v1.schema.json` 中。

## 语义边界

事件序列属于**正在保存于当前进程的终端会话**。会话从 Manager 移除、Runtime 重启或超过环形容量后，不承诺仍可读取。最多保留最近 64 条事件。`truncated` 标识请求游标之前的事件可能丢失；`lifecycleTruncated` 指导出时更早事件已淘汰。事件不是安全审计日志、不是持久工作流检查点，也不是独立测试执行证明。

终端退出码为零仅说明终端进程正常结束，不等价于 Coding Agent 任务、构建或测试通过。任务绑定仍由现有 `TaskVerifier` 校验；跨主体、跨对话访问继续被拒绝，不改变任何执行权限。

## 本轮可执行验证

- Go 终端核心 isolated `go test -race -count=1`：通过，覆盖生命周期定向测试。
- TypeScript/Vue 转译检查、终端协议解析器行为检查：通过。
- 两份 JSON Schema 正反例：通过。
- Go 解析和 `gofmt`：通过。
- 累计覆盖包：以最初原始工程为基线逐文件重建，与累计完整源码比对。

## 尚未完成的生产验收

项目指定 Go 1.26.1，而容器只有 Go 1.23.2；完整 Go 工程编译与服务重启尚不能验证。Flutter SDK 与前端依赖不存在，Flutter 分析、Android 安装、Electron 真机运行和正式 Vitest 均未执行。不能把本轮静态检查当作生产验收。Agent Task Runtime 真正的任务状态事件与持久化运行证据归档仍需后续接入。
