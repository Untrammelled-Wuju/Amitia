# Workbench Batch 28：受认证的 Multi-Agent 工作区任务状态

本轮在前 27 轮源码上累计修改，目标是把已持久化的多 Agent 协调状态开放为只读、可恢复的工作区视图，并避免在读取状态时触发 Worker 重新派发。任务状态、测试证据、审批和合并审计仍是不同概念，UI 不自动推断生产验收成功。

## 设计与模块

- `backend/internal/workbenchtasks/types.go`：工作区只读状态协议及字段限制。
- `backend/internal/workbenchtasks/projector.go`：Space/对话过滤、Fingerprint 检查、父任务状态投影、实时快照与持久记录一致性验证。
- `backend/internal/workbenchtasks/handler.go`：受认证的 `GET /api/workbench/tasks?conversationId=...`，限定 LocalUI 主体及所请求 Space。无修改操作接口。
- `backend/internal/interaction/tracker_sqlite_workbench_tasks.go`：定向读取至多 121 条候选记录，避免前端周期刷新导致全对话无界扫描。
- `front/src/components/workbench/WorkbenchTasksView.vue`：按对话展示任务状态、子任务和审查提醒，禁用 Agent 执行及后台恢复。
- `mobile_app/lib/features/workbench/presentation/pages/workbench_tasks_view.dart`：Flutter 等价只读视图；其请求保持业务 Runtime 路由，不因面板而开启设备执行权限。
- `contracts/workbench/task-status-v1.schema.json`：固定版本、输出数量及状态语义。

## 状态语义

`checkpoint-only` 表示来自最后一次持久化的状态；`runtime-observed` 表示当前进程已有的协调器快照，且父交互与全部 Assignment ID 与持久记录严格匹配；`needs-reconciliation` 表示持久记录中存在不一致信息。无任何一种状态可用作“测试通过”或“人工已批准”的证明。

最多返回 30 个任务和每任务 8 个 Assignment，目标说明截断到 220 个 Unicode code point，避免用户界面展示过长的 Agent 提示内容。

## 已运行的验证

- `go test -race -count=1 ./...`：在独立 Go 1.23 标准库适配环境中编译实际 Projector 和测试文件，通过任务过滤、篡改、重复、数量上限、实时快照溯源和有界读取用例。
- 7 个 Go 文件的 parser 语法检查通过；新增 TypeScript/Vue 脚本通过 TypeScript 转译解析。
- Schema 通过 Draft 2020-12 校验和正反例测试；路由、服务、桌面与 Flutter 接线完成静态检查。
- 累计替换包已基于最初原始源码重建核验，详见累计替换校验记录。

## 尚未完成

- 实际 Go 1.26.1 全工程编译与受认证 HTTP 请求测试；当前环境没有工具链，尝试下载时 DNS 被拒绝。
- `flutter analyze`、Android Release、Electron/Vue 完整依赖构建与真实设备测试。
- 任务面板现可展示任务协调生命周期，但尚未把 Git Worktree 的完整测试、审批与合并证据直接整合到用户端卡片；父 Agent 的证据门禁仍独立存在。
- 读取身份依赖 Auth Middleware 生成的 LocalUI Principal 和 Space 作用域，尚未针对多租户 Cloud Core 的每个对话成员 ACL 做独立生产环境验收。

源码新增功能继续按各层模块拆分，未重置前 27 轮修改。
