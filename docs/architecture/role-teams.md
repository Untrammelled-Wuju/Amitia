# 角色与团队

## 范围

角色团队是当前角色协作的唯一业务入口。每个角色使用同一份角色档案参与普通对话、群聊和团队协作；团队负责负责人、成员、共享上下文、共享工作区和任务交接。

子智能体运行由主对话创建，通过工作台查看运行状态、消息和持久化记录。它属于团队协作的执行视图，不提供独立的 Agent OS 管理面。

## 后端模块

| 模块 | 责任 |
| --- | --- |
| `backend/internal/character` | 角色配置、角色生命周期和团队成员来源 |
| `backend/internal/agentplatform` | 角色 Agent 档案同步、团队、成员、共享上下文、任务路由、审批持久化和资源租约 |
| `backend/internal/agentrun` | 主对话与子智能体运行、消息和状态同步 |
| `backend/internal/extension/kernel` | 工具、技能、审批和插件运行边界 |

## 业务 API

- `GET /api/roles`：读取可加入团队的角色。
- `GET /api/roles/teams/templates`：读取团队模板。
- `POST /api/roles/teams/from-template`：从模板创建团队。
- `GET /api/roles/teams`：读取团队列表。
- `POST /api/roles/teams`：创建团队。
- `GET /api/roles/teams/:id`：读取团队、成员、共享上下文和任务交接。
- `PUT /api/roles/teams/:id`：更新团队配置。
- `POST /api/roles/teams/:id/members`：添加团队成员。
- `POST /api/roles/teams/:id/contexts`：写入共享上下文。
- `GET /api/roles/teams/:id/context`：读取共享上下文。
- `POST /api/roles/teams/:id/route`：计算团队消息路由。
- `POST /api/roles/teams/:id/handoffs`：创建任务交接。
- `POST /api/roles/teams/:id/handoffs/:handoffId/accept`：接收任务交接。
- `POST /api/roles/teams/:id/handoffs/:handoffId/complete`：完成交接。
- `GET /api/roles/runs?conversationId=:id`：读取当前对话的智能体运行。
- `GET /api/roles/runs/:id/messages`：读取运行消息。
- `POST /api/roles/runs/:id/messages`：向运行发送消息。

宿主不提供 `/api/agent-platform` 业务入口，也不提供独立的 Agent 生命周期、角色电脑、评估或 A2A 管理 API。

## 团队生命周期

1. 角色创建或更新时同步唯一 Agent 档案和当前版本。
2. 用户从普通角色中选择负责人和成员创建团队。
3. 团队路由按显式提及、能力或负责人选择目标。
4. 团队上下文和工作区按团队配置共享。
5. 主对话创建子智能体运行时，工作台展示运行和消息。
6. 所有写入返回原始错误，不通过固定文案覆盖。

## 前端边界

- Web 与 Electron 使用 `front/src/api/role-runtime.ts`。
- Flutter 使用 `mobile_app/lib/features/characters/presentation/pages/character_teams_page.dart`。
- 子智能体运行分别由 Web 工作台和 Flutter 工作台接入。
- 角色与团队共用 `front/src/api/role-types.ts`，不再保留独立 Agent OS 客户端。

## 信任边界

- 团队只引用当前空间内存在的角色 Agent 档案。
- 团队写入使用 revision 校验。
- 工具执行继续受权限、审批和资源租约约束。
- 子智能体消息按空间和运行标识隔离。
- 插件、市场、模型、记忆和主动任务继续使用各自现有模块，不经过 Agent OS 管理面。
