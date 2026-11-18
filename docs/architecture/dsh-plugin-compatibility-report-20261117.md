# DSH 兼容插件状态报告（2026-11-17）

目标版本：`@deepseek-ai/dsh@0.2.2-rc.2`。插件：`plugins-code/dsh-compatible-host`，当前版本 `0.1.305`，测试 276/276。

## 一、总体完成度

| 维度 | 数值 | 完成度 |
| --- | --- | --- |
| UI Slot 映射 | 120 / 120（0 pending） | 100% |
| Client Service | 4 implemented + 4 partial / 8 | 75% |
| Host Service | 15 implemented + 27 delegated + 10 partial / 69（另 4 pending、13 宿主不具备） | 68.1% |
| 加权合计（等权三项） | — | **81.0%** |

## 二、已闭环的能力

1. **原生界面接入**：DSH 客户端模块由宿主 `AppLayout.vue` 常驻装载，shell 渲染 `null` 只做注册；DSH 贡献进入原生槽位（实测 `chat.header.action` 的「归档会话」「置顶会话」生效）。
2. **无第二套页面**：独立 `/dsh-conversation` 页面、路由、侧边栏入口全部删除，DSH 导航指向原生 `/chat?conversationId=`。
3. **数据面**：内核新增 `host.conversation.scope`；`conversation-read` / `conversation-message-send` 走真实宿主能力；消息经 `SessionEventLikeEntry` 投影。
4. **client services**：workspaces 的 create/rename/delete → `project-*`、archive/unarchive → `conversation-archive`；`session.loadOlder` 真实分页（100→200→…→2000）。
5. **host services**：`workspaceFiles`、`fileReferences`、`sessionFileReferences` 接 `host.workspace.read`；`fs` 读写删、`schedule` 全套、`approval.request`、`agents`、`sessions.get/list/flush` 已接缝。

## 三、稳定性证据

- 连续 3 次重载：`editable 1`、消息渲染 1 次、DSH 贡献 6–7、控制台 0 异常。
- 页面回归：`#/chat`、`#/characters`、`#/settings/system`、`#/memory` 均正常、0 异常。
- 截图：`artifacts/dsh-acceptance-20261117/native-chat-with-dsh.png`。

## 四、未闭环项与前置条件

| 项 | 阻塞原因 |
| --- | --- |
| 节点级映射（工具调用/推理卡进原生消息流） | 宿主 `chat.conversation.node` 为 list 槽位，需改为按 `node.kind` 分派 |
| `directoryPicker` / `directoryPickerController` / `workspaceController` | 宿主缺目录选择与工作区控制器语义 |
| `sessionSkillCatalog` / `skills.list` | 宿主缺 `host.skill.list`（后端已有 AgentSkillRuntime，未暴露给扩展 ABI） |
| 13 项 unavailable | 宿主无对应能力 |
| P8 历史版本回滚 / P14 事务化恢复与残留审计 | 独立工程 |
| 跨平台矩阵 | 需 Electron/Core/Android/iOS 实机覆盖 |
| 键盘回车提交 | 受信任按键限制，只能真实窗口人工验证 |

## 五、结论

当前状态为**部分兼容**：UI 面全量映射且可在原生界面共存，服务面约七成落地。达到计划定义的 100% 兼容仍需宿主渲染契约变更、后端通用能力补齐和跨平台验证，不属于"接线"工作量。
