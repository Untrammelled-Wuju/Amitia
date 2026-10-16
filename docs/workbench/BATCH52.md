# U-Ai Workbench 第 52 轮：跨端任务状态对账与断线恢复

本轮基于第 51 轮完整累计源码继续实施。研发范围估算 96.0%，不是生产环境验收比例。

## 已实施

1. Web 和 Flutter 任务列表除身份、协议、数量范围外，重新根据每个任务及子任务计算 active、paused、succeeded、failed、runtimeObserved、checkpointOnly、needsReconciliation、codingWorkersRequiringReview，并核对服务端 summary；完成、失败的子任务统计与实际子任务终态必须一致。拒绝与真实任务明细矛盾的成功或待审核摘要。
2. Web 任务列表、检查点历史统一复用有截止时间的异步请求执行器。应用回到前台及网络重连时，强制撤销旧请求并触发最新查询；轮询保持请求不重叠。即使传输层不响应中止信号，请求等待也会按约定截止。
3. Flutter 任务列表、检查点历史查询设定 25 秒超时；用户手动刷新可以在旧请求仍未完成时重新尝试，旧响应由 generation 检查拒绝覆盖新对话或新结果。
4. Web 与 Flutter 检查点记录校验时间、协调状态、完成数量和失败数量之间的关系，不能展示超过子任务总数的完成/失败证据。
5. 增加任务协议、检查点协议与请求取消/超时的定向测试，并接入既有研发门禁。

## 本轮实测证据

- `node front/scripts/workbench-task-protocol-smoke.cjs`：25 项通过（真实 TypeScript 解析器转译后运行）。
- `node front/scripts/workbench-task-checkpoint-smoke.cjs`：12 项通过（真实 TypeScript 解析器转译后运行）。
- `node front/scripts/workbench-task-request-smoke.cjs`：8 项通过（真实 TypeScript 请求模块运行，覆盖旧请求取消、挂起传输、超时与迟到结果）。
- `node front/scripts/workbench-lifecycle-smoke.cjs`：9 项通过。
- 受影响 TypeScript 模块 `tsc --strict --noEmit`、研发门禁及新测试脚本 `node --check` 通过。

## 未完成的生产验收

- 尚未在安装完整前端依赖的环境中执行完整 Vue/Electron 类型检查、构建和启动。
- Flutter 代码与测试文件已更新，但本环境无 Flutter SDK，未执行 Dart analyze/test、Android Release/iOS 构建或真机验证。
- 完整 Go 1.26.1 服务、真实身份 HTTP、Coding Agent 整体业务、Chromium/CDP 与跨设备 Device Mesh 仍缺端到端验收；本轮没有对这些模块宣称通过。
- 研发进度仅是模块实施范围估计，不等同于稳定性、发布质量或正式上线通过率。
