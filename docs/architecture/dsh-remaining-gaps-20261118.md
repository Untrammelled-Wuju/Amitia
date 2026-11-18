# DSH 兼容宿主剩余缺口清单（2026-11-18）

口径：以 `plugins-code/dsh-compatible-host/catalog/compatibility.v1.json` 为准，缺口说明逐条取自代码中的错误文案，不写推测。

## 统计

- UI 槽位：120/120 已映射。
- Host Service：69 项 —— implemented 21、partial 7、pending 1、delegated 27、unavailable 13，覆盖率 93.5%（2026-11-18 晚更新）。
- Client Service：8 项 —— implemented 4、partial 4。
- 加权（UI 40% / Host 40% / Client 20%）：约 92%。

## partial 12 项的实际缺口

| 能力 | 缺的宿主接口 | 证据 |
| --- | --- | --- |
| llm | `host.model.defaults`（默认温度/最大 token/推理强度）、附件引用文本 | 998/1004 行 |
| sessions | `host.conversation.prepare`（fork 已打通并有用例） | 1055 行 |
| subagents | `host.session.subagent.names` | 1021 行 |
| agents | `host.agent.factory`、`host.event.emit` 的 session-start 语义 | 1032/1038 行 |
| directoryPicker | `host.directory.pick`（原生对话框在渲染进程 `window.amitiaDesktop.selectDirectory`） | 399 行 |
| directoryPickerController | 同上，控制器复用具同一 pick | 432 行 |
| sessionQuery | `observeSession`/`inspectSession`/`searchEvents`/`filterSessions`/`filterEvents`/`traceSession`/`traceEvent` 共 7 项缺持续事件流、Session 句柄、跨会话检索、目录谓词与 fork 血缘（`readEvent` 已实现） | `src/runtime/host-service-bridges.mjs` |




## pending 1 项

workspaceController：官方包 `static inject = ['remoteAccess', 'typert', 'workspaceRegistry']`，并要求 `@deepseek-ai/dsh-workspace` 的 `WorkspaceId` / `workspaceDomainState` / `workspaceRecord`。本安装树没有该包，扩展运行时也没有 typert remote 与 remoteAccess 服务。

## 数据源缺口（非接缝）

节点 `conversation.chat.node` 的 `steering` 与 `tool-call` 渲染器已注册成功，但 Amitia 消息投影只有 `contentSummary` / `role` / `timestamp`；扫描 8 个会话、单会话最多 200 条消息，角色只有 `assistant` / `user`，没有任何工具调用字段，因此没有可喂给渲染器的事件。

## 已验证的正向能力（同日）

归档/取消归档、置顶/取消置顶（真实点击，双向可逆）、DSH→Amitia 发送（`content` 字段，消息 7→8）、`host.directory.list/create`、`host.skill.list`、`sessionQuery.readEvent`、P8 子包快照与 `npm ci` 确定性还原（101 pins，mismatched=0）、P14 陈旧 `requires_recovery` 终结。
