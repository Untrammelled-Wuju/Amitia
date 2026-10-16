# Workbench Batch 50 — Web / Flutter 会话生命周期一致性

## 修改范围

- Flutter 工作台在 `conversationId` 变化时立即失效旧读取代次，重置 WebView、地址栏、已访问标签和活动标签，并重新读取目标对话的持久化标签状态。
- 本地 WebView 页面回调、远程 Chromium URL 事件携带原对话和资源代次；对话切换、标签移除后迟到的回调不会修改新会话。
- Flutter 关闭 Browser 标签只解除 UI 观察关系，不对后端发出会话删除请求；真正结束 Agent 浏览器仍需独立、明确授权的资源终止操作。
- Web Dock 按 `(conversationId, tabId)` 标识缓存视图，避免不同对话中的相同 Tab ID 复用旧视图。
- Web 文件工作区异步挂载发现和后续初始化增加同一序列号检查，阻止旧请求写入新视图。
- Web 工作台持久化解码使用无原型容器，拒绝构造器相关危险键，避免持久化对象劫持会话容器。
- 研发门禁新增 Web Store 真实 TS 转译运行测试与 Flutter 页面生命周期源码断言。

## 实际执行的定向验证

- `node front/scripts/workbench-lifecycle-smoke.cjs`：9 项通过。测试覆盖了实际 TS Store 的对话隔离、后台 Agent 标签事件、持久化幂等、危险键拒绝；Flutter 和 Vue 部分为源码断言。
- `python3 scripts/workbench/flutter-browser-lifecycle-source-gate.py`：5 项通过；这是 Flutter Browser 组件源码级检查，而非 Dart 分析和真机运行。
- `node --check scripts/workbench/integration-gate.mjs`：通过。

## 仍需验收

- Flutter SDK 的 `flutter analyze`、签名 Android Release 和 iOS 平台运行。
- Web/Electron 全量依赖下的 `vue-tsc` 和 Electron 启动。
- 真实 Browser/CDP、Device Mesh 配对切换、多设备状态交接与权限复核。
- Go 1.26.1 完整工程构建、服务启动及生产故障恢复。

本批次研发范围进度是估算，不等同于上述生产验收通过。没有修改编译产物、数据库迁移或 Git 远端。
