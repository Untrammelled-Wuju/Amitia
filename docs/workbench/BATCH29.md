# Workbench 第 29 轮：Multi-Agent 任务状态可靠性与查询性能

## 开发范围

本轮沿用已有任务历史查询 API，没有新建第二套任务调度器或未授权写接口。目标是让 Web、Electron 和 Flutter 展示同一份经过验证的状态，同时缩小数据库热路径成本。

### 数据投影

- `workbenchtasks/validation.go`：校验协调 ID、父任务 Space/对话归属、子任务 ID 唯一性、状态枚举及输入大小。非法持久化记录不进入工作区结果。
- 严格匹配 Runtime 观察快照的父交互与子任务 ID/角色。历史检查点已终结时，不接受运行态的降级；需要人工协调的历史记录不得被实时快照掩盖。
- `workbenchtasks/summary.go`：从**当前实际返回的任务**计算总数、进行中、成功、失败、暂停、仅检查点、运行时已观察以及待协调数量。截断时 UI 明确说明汇总不覆盖未加载任务。
- Task Status V1 JSON Schema 增加 `summary`，同步更新桌面和 Flutter 展示。

### 查询性能

- SQLite 添加 `(space_id,conversation_id,updated_at DESC,id DESC)` 组合索引，原有任务查询上限保持 121 条候选记录。
- 相同更新时间时使用 `id DESC` 排序，避免跨刷新排序抖动。
- `sqlite3` 独立内存测试使用 `EXPLAIN QUERY PLAN` 验证范围查询命中该索引，并验证返回数据只属于被请求的 Space。

### 客户端生命周期

- Web 任务视图切换对话时立即作废旧请求；组件停用时取消请求和刷新定时器，防止旧响应覆盖当前标签。
- Flutter 任务视图只在工作区活动标签、应用前台时轮询；与系统生命周期和对话 ID 同步，失活后作废历史响应。
- 摘要状态只读。用户看到的 `succeeded` 仅表示 Agent 工作阶段结束，不表示 Worktree 测试、审批或合并通过。

## 已执行验证

- `go test -race -count=1`：Go 任务投影隔离测试通过（使用原始任务投影源码以及最小 `interaction` 依赖桩）。
- TypeScript 5 份源码转译检查通过，包括任务摘要 DTO 和 Vue 组件 `<script setup>`。
- Task Status V1 的 Draft 2020-12 JSON Schema 正反例通过。
- SQLite 实际 `EXPLAIN QUERY PLAN` 验证索引命中与 Space 隔离通过。

## 未完成验收

- 完整 Go 1.26.1 工程构建、真实 Gin 鉴权 HTTP 链路、真实 SQLite 数据迁移、大规模数据查询基准、Flutter Release、Electron 和真机联调未执行。
- 当前 Runtime 快照只是状态观察，不等于子任务的执行日志或测试证明。
- 数据库索引在首次运行时的创建时间需根据生产数据库容量核验。
