# DSH 兼容宿主交付清单（2026-11-18）

交付物版本：插件 `com.amitia/dsh-compatible-host` 0.2.80；Core 由同源码重建，Core SHA-256 `41b75e3b878b5c7ba85ac14829fca30aa9d21e1e5a497ed57df8327dce048fa7`（与 `desktop/resources/core/macos-arm64/AmitiaCore` 实测一致）。

## 1. 验收项清单（19 项，全部运行时实测）

| # | 项 | 证据要点 |
| --- | --- | --- |
| 1 | `core.health.ready` | HTTP 200，`ready=true`，runtime ready |
| 2 | `core.readyz` | 7 ready / 0 degraded / 0 blocking / 0 failed |
| 3 | `extension.installed` | 0.2.80，installed，enabled，failureCount=0 |
| 4 | `ui.bridge` | `__dshHostCommand` 为 function，无错误边界，composer 存在 |
| 5 | `ui.slotHost` | `<html>` 带 `data-dsh-theme=dark`、`data-dsh-color-scheme=dark` |
| 6 | `conversation.list` | 分页可用 |
| 7 | `directory.list` | 家目录返回条目 |
| 8-12 | `sessions.prepare.*` | 草稿创建 → 发布前不可见 → 发布 → 可见 → 清理 |
| 13-16 | `conversation.archive/unarchive/pin/unpin` | 布尔与时间戳同时落库，列表状态同步 |
| 17 | `conversation.fork` | 派生会话且目录项 `parentSession` 指回源会话 |
| 18 | `project.reorder` | 顺序值前移 + 目录顺序同步 + 删除后不可见 |
| 19 | `conversation.reorder` | 返回顺序与落库 `order` 前移 |

## 2. 能力交付（host 服务面 69 项：implemented 25 / partial 4 / delegated 27 / unavailable 13 / pending 0）

新增/补齐并带测试的能力：

- 会话：`create`（含 draft 草稿语义）、`list`、`read`、`message.append`、`archive`、`pin`、`fork`、`reorder`
- 项目：`list`（按 `order` 排序、排除隐藏/删除）、`create`（自动分配稀疏 `order`）、`update`（含 `order`）、`reorder`、`delete`
- 会话检索：`searchSessions`、`searchEvents`（分词命中、片段、游标分页）、`traceSession`（血缘）、`traceEvent`（替换链/来源/派生）
- 工作区：`workspaceController` 全量方法（含 `insertBefore`/`insertSessionBefore`）、`workspaceFiles`（含 `changes` 轮询帧）、`directoryPicker` 与 `directoryPickerController`（native 能力）
- 子代理：`prompt`、`sendMessage`（树根授权）、`rootOf`、`namesOf`（名字路径）、`listDescendants`、`interrupt`、`interruptByUser`、`waitForTreeWork`、`drainDescendants`
- 其他：`llm.fileRequestText`、`agents.setFactory/announce`、`approval`、`userQuestions`、`schedule`

## 2.1 客户端服务面（8 项：implemented 7 / partial 1）

已实现 6 项：`locale`、`sessions`、`slots`、`theme`、`timer`、`workspaces`（其中 `workspaces` 的 `WorkspaceId` 经 project 映射，fork 走 `conversation-fork` 并回调 childId）。

仍 partial 的 1 项（`layout` 已转 implemented，见下表）：

| 能力 | 已闭环 | 仍缺 |
| --- | --- | --- |
| `uiWorkspace` | 7 个方法 + `prompt` + `agentPreset`（会话级/工作区级）+ 草稿模式 `whenDraftExists=replace/append/keep`（`clearPreviousDraft` 等同 replace，直接读写 `webchat_draft:new:*`） | `openWorkspace` 的 `beforeOpen`（工作区路由不携带 SessionId） |
| `layout` | 官方 Conversation / Plugins / Schedules 面板与侧栏、右栏开关、导航会话、自定义主面板（`registerMainPanel` + `/main-panel/:panelId` 容器路由） | 无 |

## 3. 剩余 4 项 partial（均为宿主侧前置缺失）

`sessions.enter/announce/registerMessageProjection`、`sessionQuery.inspectSession`、`llm` 附件可读路径、`directoryPicker` 桌面点选/移动端端点。

原因与证据见 `dsh-compatibility-audit-20261118.md` §2 与 §2.1：容器内未装载 DSH agent loop/tool 运行时，这些 API 没有消费者；硬做等于另建会话系统，与项目规则冲突。

## 4. 复现命令

```bash
# 插件单测（含契约守卫）
AmitiaData/node/bin/node --test plugins-code/dsh-compatible-host/tests/

# 服务面审计 + 一致性
AmitiaData/node/bin/node artifacts/dsh-acceptance-20261118/audit-services.mjs
AmitiaData/node/bin/node artifacts/dsh-acceptance-20261118/verify-consistency.mjs

# 运行时验收（端口取 AmitiaData/run/endpoint.json）
AMITIA_CORE_BASE=http://127.0.0.1:<port> \
AMITIA_LOCAL_TOKEN="$(cat AmitiaData/security/local-token)" \
AmitiaData/node/bin/node artifacts/dsh-acceptance-20261118/acceptance.mjs

# 后端
cd backend && go build ./... && go vet ./cmd/server/ ./internal/extension/kernel/... ./internal/devicemesh/...
```

## 5. 已知既存问题（与本次交付无关）

- `cmd/server` 3 条 `TestOwnedChatCleanup*` 断言「请至少选择一个清理条件」，既有失效断言。
- `internal/extension/kernel/trusted_service` 生命周期用例在整树并行跑偶发超时，单跑通过。
