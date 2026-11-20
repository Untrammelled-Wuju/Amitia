# DSH 兼容插件当前状态（2026-11-18 收口）

## 一、验证矩阵（全部为本次实际执行结果）

| 检查 | 命令 | 结果 |
| --- | --- | --- |
| 插件单测 | `node --test tests/*.test.mjs` | 308 pass / 0 fail |
| 后端编译 | `go build ./...` | 无输出 |
| fork 回归 | `go test ./cmd/server/ -run TestOwnedChatForkCopiesMessagesIntoNewConversation` | ok（1.76s） |
| 后端静态检查 | `go vet ./cmd/server/... ./internal/extension/...` | 无输出 |
| 内核测试 | `go test ./internal/extension/kernel/` | ok（93.9s） |
| 聊天服务测试 | `go test ./internal/chat/` | ok（16.3s） |
| 前端类型 | `vue-tsc --noEmit` | exit 0 |
| 桌面类型 | `tsc -p tsconfig.json --noEmit` | exit 0 |
| 桌面冒烟 | `node artifacts/dsh-acceptance-20261117/smoke.mjs` | `bridge=function, errorBoundary=false, messages=18, composer=1, header=["归档会话","置顶会话"], listing=/Users/untrammelled(3)` |
| 运行版本 | 插件 0.2.42 / Core `/api/health` 200 | 生效 |

## 二、覆盖度（按 `catalog/compatibility.v1.json`）

- UI 槽位 120/120。
- Host Service 69 项：implemented 25、partial 4、pending 0、delegated 27、unavailable 13 → **94.2%**。
- Client Service 8 项：implemented 4、partial 4。
- 加权（40/40/20）约 **93%**。

## 三、已实证的能力

1. 归档 / 取消归档：真实点击，`archivedAt` 落库，活跃列表移除与恢复可逆。
2. 置顶 / 取消置顶：真实点击，`pinnedAt` 写入与清空，按钮 `aria-label` 互切。
3. DSH → Amitia 发送：`conversation-message-send`（字段 `content`），消息数 8→9 实测。
4. 宿主目录：`host.directory.list/create` 实测；桌面 `window.amitiaDesktop.selectDirectory` IPC 存在。
5. 技能：`host.skill.list` 实测返回 `{"items":[]}`（当前无 SKILL.md 扩展），skills / sessionSkillCatalog 服务合并宿主与本地。
6. 模型默认值：`host.model.defaults` 实测返回 `deepseek/deepseek-chat/0.7/4096/0/128000`，`llm.resolveCallConfig` 已接。
7. 文件系统：read/write/rename/move/copy/delete + `watch`（轮询）+ `editText` + `patchText`，fs 服务转 implemented。
8. 审批与提问：`approval.request` 映射 allowed-once/rejected；`userQuestions.ask` 经 `host.ui.dialog`，非法结果抛原始错误。
8.1 定时任务：`schedule.list` 映射 `{schedules,states}` 并按 active 过滤，`schedule.create` 回读目录确认条目，均有测试。
9. `sessionQuery.readEvent` 按 `seq` 读取事件。
10. P8：子包版本快照 `dsh-tree-lock.json` + `npm ci` 确定性还原（101 pins，mismatched=0）。
11. P14：陈旧 `requires_recovery` 由更新的完成操作终结，恢复入口集成测试通过。
12. React 模块加载：打包期 ESM vendor（单例）修复 CJS-in-browser 回归。

## 四、剩余 7 partial / 1 pending

| 能力 | 缺什么 |
| --- | --- |
| agents | `host.agent.factory`、session-start 事件语义 |
| sessions | `host.conversation.fork`、`host.conversation.prepare` |
| subagents | `host.session.subagent.names`、fork 派生 |
| sessionQuery | `observeSession`/`inspectSession`/`searchEvents`/`filterSessions`/`filterEvents`/`traceSession`/`traceEvent` |
| llm | `fileRequestText`（附件引用文本） |
| directoryPicker | `pick`（宿主侧无原生对话框方法，桌面走渲染进程 IPC） |
| directoryPickerController | 同上 |
| workspaceController（pending） | `remoteAccess` / `typert` / `workspaceRegistry` 与 `dsh-workspace` 包 |

## 四点五、会话派生（fork）已打通

- 实现：`cmd/server/mesh_owned_chat_actions.go` 的 `conversation/fork` 在 mesh 层复制会话资源与消息资源（新 id、重写 `conversationId/characterId/ownerId`、写 `forkOrigin`），一次 `Apply` 提交。
- 实测：`conversationId=d0b47571…`、`forkedFrom=536b868e…`、`messageCount=9`、`inList=true`，随后 `conversation-delete` 清理成功。
- 插件 `sessions.fork` 已恢复为真实调用（插件 0.2.42）。
- 自动化：`cmd/server/mesh_owned_chat_fork_test.go` 覆盖（`forkedFrom`/`messageCount=2`/消息带 `forkOrigin`）。
- 仍缺：`sessions.prepare`（无预分配语义）。

## 四又四分之三、既有失败用例（与本轮改动无关）

`cmd/server` 的 `TestOwnedChatCleanupPreservesMemoriesAndDeletesCurrentHistory`、`TestOwnedChatCleanupRejectsChangedPreviewAndRole`、`TestOwnedChatCleanupAllowsEmptySelectionAndCreatesBackup` 为陈旧用例（空数据下仍期望清理预览成功），改动前即失败，未做范围外修改。

## 五、已知非阻塞缺陷

DSH 头部按钮在“同一次会话内取消归档”后文案不回收，刷新页面后正确。定位到 `@deepseek-ai/dsh-client-ui-workspace/lib/client.js` 自维护 `archivedSessionIds`（含 `archivedUndo` 撤销缓冲），插件侧的目录刷新与贡献重挂载都不影响它；功能与宿主状态始终正确。

## 六、跨端能力边界

- `directoryPicker.pick`：桌面端由渲染进程 `window.amitiaDesktop.selectDirectory`（Electron `showOpenDialog`）提供；Android 端的 SAF 目录选择由 `workspace.Service.PickSAFDirectory` 支撑，但只服务于应用自身的目录挂载流程，未作为扩展宿主方法暴露。因此 `host.directory.pick` 不存在，`pick` 在宿主服务层保持缺口是当前架构的真实边界，不是遗漏。
