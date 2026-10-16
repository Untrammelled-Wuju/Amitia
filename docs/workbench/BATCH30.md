# Workbench 第 30 轮：任务检查点取证与恢复冲突识别

本轮建立可查询的任务检查点取证入口，而非完整事件回放系统。

## 核心设计

- `workbenchtasks/checkpoint_history.go`：只读取当前 Space 和 Conversation 中与指定 CoordinationID 匹配的持久化记录，核验恢复指纹、父任务作用域和子任务结构；最多保留 20 条样本，查询最多取最近 121 个候选记录。
- `checkpoint_history_handler.go`：已认证 Local UI 只读 HTTP 接口；非用户设备身份拒绝；不能通过不存在的任务 ID 判断其他对话中任务是否存在。
- `reconciliation.go`：明确区分父交互终态冲突和 Runtime 检查点冲突。此状态只表明需要人工协调，并不触发自动恢复、自动提交或自动合并。
- `projector.go`：一次查询最多访问 8 个 Runtime 快照，超过上限向客户端返回限流提示。持久化检查点和观察到的 Runtime 快照不被当成构建、测试或 Git 审批证明。
- `front/.../workbench-task-checkpoints.ts` 与 Flutter `workbench_task_checkpoints.dart`：分别实现有界响应解析与按需加载。Web 使用 AbortController 防止旧对话结果覆盖当前标签；Flutter 仅在具体任务展开时加载。

## 真实验证

- 使用已安装的 Go 1.23.2，以独立依赖桩运行实际 `workbenchtasks` 投影代码、历史查询代码与 HTTP 处理函数，启用 `go test -race`。验证登录身份、跨任务、跨 Space/对话隔离、历史截断、数据篡改拒绝、并发和 Runtime 冲突。
- 独立 TypeScript 转译与可执行协议断言验证组件语法、身份一致性、字段约束。
- Draft 2020-12 JSON Schema 正反例测试。
- ZIP 累计替换包以最初源码为基线；全部文件覆盖重建与本轮累计源码一致。

## 未经验证的部分

由于本机 Go 1.23.2 低于 `go.mod` 指定的 Go 1.26.1，当前无法完成完整 Go 构建或启动全服务；缺少 Flutter SDK 和 Web 前端全部依赖，不能认定已通过 Flutter Release、Electron 或移动端真实设备验收。独立依赖桩只用于验证业务逻辑与处理函数，不能替代真实认证中间件及生产数据库集成。

任务历史只包含当前可查询的**最近持久化检查点样本**，并非逐条执行事件，也不能证明所有历史状态都被保存。需要协调的任务不会被自动重跑或自动合并。
