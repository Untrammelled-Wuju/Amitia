# Workbench 第 54 轮：AI 任务标签协议与移动端资源实例隔离

## 目标及实现

本轮以前 53 轮累计完整源码为基线，对 Web/Electron/Flutter 的任务标签事件、跨设备资源恢复和异步 UI 状态进行收口，不新增执行引擎。

1. 扩展 `workbench.open` v1 的合法视图类型，允许 `tasks` 作为仅依赖会话 ID 的任务状态标签。同步 Web 事件解析、Workbench Store、Flutter 事件落盘和两份共享 JSON Schema。
2. `tasks` 标签不能附带其他工作目录、文件 URI、Browser Ref、Task ID 或远端设备目标；Web 解析与存储入口、Flutter 入站事件、协议 Schema 均明确拒绝这些不符合任务视图语义的字段。
3. Flutter 恢复同一 Tab ID 的新资源时比较 View Type、URI、Workspace URI、Target Device、Browser Ref 和 Task ID。身份变更后销毁旧地址输入缓存、浏览器控制器映射和旧视图实例，重新创建资源观察者。
4. Flutter 资源代次采用递增序号，同一 ID 被移除后重新创建也不会复用旧代次；迟到的本地 WebView 页面完成回调与远端 Chromium 页面回调只有在原对话及资源代次均未变化时才允许更新标签状态。
5. 将事件协议测试接入已有开发门禁，并更新 Web Store 与 Flutter 生命周期的相关测试断言。

## 当前环境可重复执行的针对性验证

- `node front/scripts/workbench-open-event-smoke.cjs`：9 项，通过。实际执行事件解析 TypeScript 转译后逻辑，额外核查 Flutter 源码接线。
- `node front/scripts/workbench-lifecycle-smoke.cjs`：10 项，通过。实际执行 Pinia Store 逻辑的隔离测试，包含任务标签后台打开、拒绝异常资源绑定。
- `node front/scripts/workbench-resource-lifecycle-smoke.cjs`：10 项，通过。
- `node front/scripts/workbench-history-lifecycle-smoke.cjs`：6 项，通过。
- `node front/scripts/workbench-task-protocol-smoke.cjs`：25 项，通过。
- `node front/scripts/workbench-task-checkpoint-smoke.cjs`：12 项，通过。
- `node front/scripts/workbench-task-request-smoke.cjs`：8 项，通过。
- Draft 2020-12 JSON Schema 正反例：两份 Schema 各 1 个合法任务视图和 5 个非法资源绑定用例，全部通过。
- `node --check scripts/workbench/integration-gate.mjs`：通过。

以上运行检查共 80 项；Flutter 侧是源代码接线与生命周期断言，并未运行 Dart/Flutter 编译。完整 Go 1.26.1 服务启动、真实授权 HTTP、Browser CDP、Windows ConPTY、Device Mesh 配对重连、Electron 启动、Android/iOS Release 构建和生产级压力/恢复/权限验收均未执行；不可据此声称生产验收通过。

## 进度口径

研发范围估算由 96.7% 提升至 97.0%。这是多端代码实现范围的估算，不代表 97% 的生产验收已经完成。后续应优先执行真实构建、安全、稳定性、恢复、发布门禁，未验证项目在生产验收中继续保持未完成状态。
