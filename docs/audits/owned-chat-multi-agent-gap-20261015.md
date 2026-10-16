# Owned 聊天与原生多 Agent 能力接入审计

日期：2026-10-15。

结论：确认一项 P1 重大接入缺陷后停止扩展审计。本轮只审计，没有修改业务源码、调用外部模型、写入实际业务库、构建 APK 或操作 Git 提交与推送。

## P1：新 Owned 聊天未接入旧聊天已有的原生多 Agent 委派与执行生命周期

后端已有多 Agent 协调器、Coding Worker、委派/查询/等待工具，以及任务检查点。旧聊天将这些工具追加到模型可用工具目录，并通过统一 Interaction 入口登记父任务。

当前手机端、电脑端在 Owned 服务可用时使用新 Owned 聊天入口。它采用另一套工具运行时，仅从 Kernel ToolFacade 获取目录，没有接入旧适配器追加的三个原生工具，也没有接入这些工具依赖的父 Interaction 登记流程。

因此，在当前 Owned 聊天路径中，即使用户绑定了工作区，模型仍无法通过现有原生入口委派多 Agent/Coding Worker 任务。读取任务列表正常不能证明聊天能实际创建这些任务。此项与上一轮任务列表、检查点的会话身份和云端读授权修复属于不同环节。

## 源码证据

### 旧能力已经存在并仍被装配

- `backend/cmd/server/services.go:604` 创建 `chatToolRuntimeAdapter`，`:605` 将其设为旧工具运行时，`:758` 注入实际多 Agent 协调器、InteractionTracker、GoalRegistry。
- `backend/cmd/server/tool_facade_wiring.go:121` 的 `ModelTools` 先读取 Kernel 目录，`:126` 再调用 `appendMultiAgentTools`；执行入口 `:130`、`:154` 对原生多 Agent 工具进行专门分发。
- `backend/cmd/server/tool_multi_agent.go` 提供 `harness_multi_agent_delegate`、`harness_multi_agent_status`、`harness_multi_agent_wait`。`:58` 的授权检查要求当前请求已存在有效、未结束且会话/角色匹配的父 Interaction。
- `backend/internal/chat/handler.go:261` 的旧聊天路径调用 `UnifiedEntry.Handle`；`backend/internal/interaction/unified_entry.go:349` 进入 Orchestrator；`backend/internal/interaction/orchestrator_process.go:56`、`:57` 创建并保存 InteractionRecord。

### 当前两端使用另一条路径

- `front/src/composables/useWebChatSend.ts:212` 在 Owned 服务可用时进入 Owned 分支，`:240` 调用 `owned.send`。
- `mobile_app/lib/features/chat/runtime/conversation_runtime_controller.dart:337`、`:338` 进入 `_sendOwned`，`:596` 调用 Owned 发送；`mobile_app/lib/core/services/device_owned_chat_service.dart:1277` 请求 `/api/device-mesh/v1/business/messages`。
- `backend/cmd/server/mesh_business_router.go:462` 直接调用 `services.OwnedBusiness.RunEvents`。
- `backend/cmd/server/services.go:1343` 建立 Owned Engine，`:1354` 为其模型装配独立的 `meshOwnedToolRuntime`。

### 新链路遗漏原生能力及其前置生命周期

- `backend/cmd/server/mesh_owned_tools.go:96` 直接读取 `ToolFacade.ModelTools`，没有调用旧适配器的 `appendMultiAgentTools`。
- `backend/internal/extension/kernel/tool_facade.go:814` 从注册目录构建工具；原生 Harness 的三个工具由旧适配器追加，并未在本项目 Kernel 注册代码中注册为同等能力。
- `backend/cmd/server/mesh_owned_tools.go:72` 直接按模型工具名查询注册目录；`:74` 会拒绝未注册的原生委派工具，没有对应的原生协调器分发分支。
- `backend/internal/chat/owned_tools.go:26` 使用独立 Owned 工具目录，`:82` 拒绝目录外模型调用，`:86` 只调用 Owned 执行入口。
- Owned Engine 与上述 Owned 工具循环没有调用旧统一入口的父 Interaction 登记。即使仅追加工具名称，旧工具的 `multiAgentAvailable` 检查仍无法获得所需父任务。

## 隔离复现

诊断使用项目已有测试夹具和真实函数，没有修改函数返回值。临时诊断测试文件在执行后已删除，源码存档位于 `artifacts/audits/owned-multi-agent-gap-20261015/repro-test.go.txt`。

### 工具目录与执行入口对比

两条路径使用同一份受控 Kernel ToolRegistry/ToolFacade。受控注册目录为空，是为了单独观察旧适配器额外提供的原生能力；这不代表运行中项目没有其他插件工具。

| 检查 | 结果 |
| --- | --- |
| 已配置协调器且有合法父任务、绑定工作区的旧 `ModelTools` | 返回委派、状态、等待三个原生工具 |
| 已绑定工作区的 Owned `Tools`，使用相同 Kernel 目录 | 未返回这三个工具 |
| Owned `Execute` 直接调用 `harness_multi_agent_delegate` | 返回“能力不存在、未启用或不允许模型调用” |

### 现有聊天 HTTP 入口复现

使用真实 `registerMeshBusinessRouter` 注册 `/api/device-mesh/v1/business/messages`，真实 Owned Engine、协调服务及隔离 SQLite 数据端口。注入已认证设备主体上下文，使用受控模型观察进入模型时的父 Interaction 状态；没有实际调用外部模型。

| 检查 | 结果 |
| --- | --- |
| Owned 聊天请求 | HTTP 200 |
| 请求是否真正进入模型端口 | 是，观察到请求 `audit-owned-http` |
| 当前请求是否登记到已配置的 InteractionTracker | 否 |

此 HTTP 测试验证现有业务路由和生命周期，不冒称验证了真实设备签名、生产云服务或真实 Coding Worker 执行。另对运行中服务调用了只读 `/api/public/runtime/capabilities`，返回 200；该健康与能力声明响应不作为多 Agent 已接通的证据。

完整诊断结果：`artifacts/audits/owned-multi-agent-gap-20261015/repro-result.json`。

测试日志：`artifacts/audits/owned-multi-agent-gap-20261015/repro.log`，测试通过。

## 影响与修复边界

影响当前使用 Owned 聊天的电脑端、手机端，包括云端场景。用户看得到工作台任务能力，后端协调器也已实现，但现用聊天不能通过原生 Harness 入口创建这些任务。

不能据此声称所有插件工具、独立插件任务或旧聊天入口的多 Agent 能力都失效；已确认的是上述原生多 Agent 链路在 Owned 聊天中的缺失。

后续修复应统一父任务生命周期，并在 Owned 授权上下文中复用现有原生委派、状态、等待执行能力，同时保留 Core、owner、角色、工作区及权限隔离。只给模型追加三个工具名或只修页面均不足以完成接入。验收应由真实聊天请求触发协调任务、产生 Worker 和检查点，再由两端读取与核验，而不是只预置任务记录测试列表。

已按要求确认一项重大问题后停止，其余模块未继续穷举，无法据此统计项目剩余问题总数。
