# DSH 兼容宿主交付摘要（2026-11-18）

插件：`com.amitia/dsh-compatible-host` 0.2.80 · 状态 `installed` · `failureCount=0`

## 现在能用的

| 场景 | 行为 |
| --- | --- |
| DSH 界面 | 120 个 UI 槽位全部落在 Amitia 原生页面与组件，没有独立的 DSH 页面 |
| DSH 发消息 | 宿主 `host.conversation.message.send`，消息落 Amitia 会话 |
| 会话管理 | 新建、读取、重命名、删除、归档、取消归档、置顶、取消置顶、派生（fork），双向可逆 |
| 草稿会话 | `prepare` 返回 draft 会话，首条消息前不进目录，同 `requestId` 正式创建时升级发布 |
| 工作区 | 列表、创建、重命名、删除、默认工作区初始化、手动排序（`insertBefore`）；归档/置顶按会话作用域快照写 |
| 会话排序 | `insertSessionBefore` 在工作区内按 `order` 稀疏中点插入，间隙耗尽时单事务重排 |
| 工作区变更帧 | `workspaceFiles.changes` 以 `file_list`/`file_stat` 轮询产出 `ready`/`change`/`absent` 帧；文件按 `stat.version` 判变，目录对直接子项逐个 stat（并发 8）后按 version 判变，同尺寸内容改写也能发现 |
| 目录选择 | `directoryPickerController` 报 native 能力，pick/list/createDirectory 委派 `host.directory.*` |
| 子代理投递 | `sendMessage` 按 `forkOrigin` 血缘判定树根，同树才投递，`source=dsh-subagent-steer` |
| 会话检索 | `searchSessions` / `searchEvents` 在会话日志上做分词命中计数、片段与游标分页，返回 DSH `SessionSearchPage` 结构 |
| 目录与文件 | 列目录、建目录、交互式选目录（桌面）、读文件、写文件、编辑、打补丁、移动、删除、监视（轮询） |
| 技能 | `host.skill.list` 目录 + 本地 provider 注册 |
| 模型 | 调用、流式、列模型、prepareCall、默认温度/最大 token/推理强度 |
| 审批与提问 | `approval.request` → `host.approval.request`；`userQuestions` → `host.ui.dialog` |
| 调度 | 目录、创建、列表、查看、编辑、取消、运行、删除失效、历史 |
| 会话查询 | 列表、读取、事件窗口、观察租约、五类会话谓词、事件文档过滤、血缘追踪 |
| 子代理 | 启动、列举、中断、按名解析、提示投递、树工作状态 |
| agent | 工厂替换、注册、`enter`、`announce`（容器 cordis 串行派发 `agent/created`） |
| 附件文本 | `fileRequestText` 生成官方等价句柄文本 |

## 现在不能用的，以及它会怎么报错

| 能力 | 调用结果 |
| --- | --- |
| `sessions.enter` / `announce` / `registerMessageProjection` | `DSH_HOST_OPERATION_UNAVAILABLE`，缺 DSH Session 实例与消息表面 |
| `sessionQuery.inspectSession` / `searchEvents` / `traceEvent` | `DSH_HOST_OPERATION_UNAVAILABLE`，缺 Session 句柄 / 跨会话检索 / 事件来源引用 |
| `subagents.sendMessage` | `DSH_HOST_OPERATION_UNAVAILABLE`，缺子代理树父子关系与发送者授权 |
| `workspaceFiles.changes` | `DSH_HOST_OPERATION_UNAVAILABLE`，缺 `host.workspace.watch` |
| `llm` 附件可读路径 | 无 `ref.uri` 时返回官方「无法访问可读路径」文案，不伪造 |
| `directoryPickerController` | `DSH_HOST_OPERATION_UNAVAILABLE`，缺 `host.workspace.registry` |
| `sessions.prepare` 之外的 draft 初始化（`uiWorkspace.startSession` 的 prompt/clearPreviousDraft） | `unsupported`，缺 Amitia 跨页面草稿桥 |
| tool-call / steering 节点 | 渲染器已注册但无事件来源（消息表没有 `tool_calls`） |

失败一律保留原始错误（含 code、message、cause 与入参），没有固定文案替换、没有吞错、没有假装成功。

## 复核方式

```bash
# 单测（367 项）
node --test tests/*.test.mjs

# 运行时验收（18 项）
AMITIA_LOCAL_TOKEN=$(cat ../../AmitiaData/security/local-token) \
AMITIA_CORE_BASE=http://127.0.0.1:<port> \
node artifacts/dsh-acceptance-20261118/acceptance.mjs

# 一致性（catalog / 文档 / 包 / 运行版本）
node artifacts/dsh-acceptance-20261118/verify-consistency.mjs
```

## 待决策

1. `directoryPicker` 原生对话框点选（需人工在桌面端点一次）
2. 结构性缺口的处置：新增数据源（工具事件、附件落盘、变更帧）或接受当前形态

顺序字段已落地：项目与会话文档写稀疏 `order`（步长 1024，中点插入，间隙耗尽单事务重排），host 侧新增 `host.project.reorder` / `host.conversation.reorder`；DSH 侧 `host.project.list` / `host.conversation.list` 按 `order` 排序，Amitia 原生侧栏仍按置顶与更新时间展示。
