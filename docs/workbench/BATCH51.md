# Workbench Batch 51 — Web 与 Flutter 多 Agent 任务响应一致性

## 研发范围

- Web 任务面板新增有界、严格的 `task-status-v1` 响应解析器。读取任务列表时核对协议版本、来源、当前 Conversation ID、Task/Assignment 身份、计数、状态、时间戳和字段长度；异常数据不进入工作台展示。
- Flutter 任务列表增加相同的对话身份与协议校验。错误响应以读取失败显示，不再把跨对话结果或者未知协议版本当作空任务列表。
- Web 任务面板从后台返回或网络重新连接时主动刷新；组件退出时撤销相应监听器，保留已有的 AbortController 和请求代次校验。
- Web 新增原始 TypeScript 解析器运行测试，覆盖匹配/错配、重复任务、无效子任务、超限输入、伪造统计、异常时间戳等 19 项断言；Flutter 新增相应的 Dart 测试源码，待 SDK 可用时执行。
- 研发门禁接入原始 TS 协议测试与 Flutter 任务测试。各模块责任独立，不新增任务执行引擎或数据库结构。

## 本轮确实运行的检查

1. `node front/scripts/workbench-task-protocol-smoke.cjs`：19 项通过，真实 TypeScript 解析器转译运行，并做 Web/Flutter 消费端源码连线检查。
2. `node front/scripts/workbench-lifecycle-smoke.cjs`：既有 Store 与视图生命周期 9 项通过。
3. TypeScript 编译器 `--noEmit`：任务解析器及摘要模块静态类型检查通过。
4. `node --check scripts/workbench/integration-gate.mjs`：通过。

## 尚待验收

- Flutter SDK `flutter analyze` 与任务列表 Dart 测试、Android Release 及 iOS 真机。Dart 新测试目前仅完成源码写入，未宣称通过。
- 前端完整 Vue 类型检查与 Electron 启动、Go 1.26.1 后端启动、真实认证 HTTP 与多端后台恢复。
- Device Mesh 真机断线与重新配对、浏览器同页控制、长任务性能和发布验收。

本轮研发范围为估算，不等于生产级验收通过。没有更新数据库或远程 Git，也没有修改编译产物。
