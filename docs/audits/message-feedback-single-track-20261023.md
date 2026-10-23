# 消息反馈单轨修复与生产验收

记录时间：2026-10-23。范围：本轮确认的一项 P1 问题——消息反馈后端 Owner 链路已存在，但双端没有真实提交入口，同时旧消息反馈 HTTP 和全表统计仍并存。

## 原问题

- 新 `chat-management/feedback/create|list|stats|delete` 已实现 Owner 和角色隔离。
- 手机端 `FeedbackService` 只有统计被 Dashboard 调用，提交、查询和删除是未接入业务页面的死方法。
- 电脑端只读取反馈统计，没有消息反馈操作。
- 旧 `/api/messages/:id/feedback`、统计、最近列表和删除路由仍在线。
- 旧统计直接扫描整张 `message_feedback`，新统计只扫描当前角色消息映射键；两端共享表但事实范围不同。
- `unsafe` 反馈的安全事件只能由后端产生，正式客户端无法触发。

## 单轨方案

1. `chat-management/feedback/*` 成为唯一正式消息反馈契约，复用 Owner、角色、Core、ExecutionScope 和撤销授权。
2. `message_feedback` 扩展为显式 Scoped 实体，新增 Space、Owner、Core、角色、会话、requestId、revision、更新时间和软删除字段。
3. 新反馈按显式归属查询；不再使用 Owner、角色、消息拼接哈希作为正式键。
4. requestId 幂等，并增加 Scoped 唯一索引。同一 requestId 参数不同返回冲突。
5. 删除使用 revision 和 tombstone；聊天清理只删除对应 Owner、角色、消息范围的反馈。
6. `unsafe` 反馈与 `user_reported_unsafe` 安全事件在同一数据库事务提交。
7. 启动时幂等迁移旧记录：
   - 可确认会话和角色的旧反馈补齐归属。
   - 无法确认归属的记录标记为 `legacy-unscoped`，只读保留且不进入正式统计。

## 功能迁移

| 旧功能 | 新正式入口 |
| --- | --- |
| 单条消息提交反馈 | `chat-management/feedback/create` |
| 查询消息反馈 | `chat-management/feedback/list` |
| 全量/最近统计 | `chat-management/feedback/stats` |
| 删除反馈 | `chat-management/feedback/delete` |
| 不安全内容报告 | `unsafe` 反馈与安全事件事务 |

电脑端和手机端均新增：

- AI 回复“有帮助”按钮。
- “需要改进”原因输入。
- “报告不安全内容”原因输入。
- 已提交状态显示和撤销。
- Dashboard 最近反馈、类型和删除操作。

## 旧代码清理

已删除旧的 `feedback` Handler、Router、Service、Repository、所有权判断和旧权限测试。旧路由实测全部返回 HTTP 404。

保留 `message_feedback` 表作为正式存储和旧数据迁移载体，没有不可逆删表。

## 定向验证

- Scoped 反馈创建、requestId 幂等、冲突、列表、统计、软删除通过。
- Owner 和角色分区测试通过。
- 旧记录归属迁移、无法归属隔离和重复执行通过。
- 旧表存在时新基线迁移不再提前创建依赖新列的索引。
- `unsafe` 与安全事件原子事务通过。
- 本地 Owner、设备 Owner 和统筹 Core 的聊天管理测试通过。
- 后端 `go vet`、服务器定向测试和编译通过。
- 电脑端 typecheck、生产构建和 3 项反馈接入测试通过。
- 手机端静态分析无问题，反馈角色/类型/原因传递测试通过。
- 云设备执行门禁通过。
- 冻结清单 11082 项通过。

## 完整生产模拟

使用正式 Electron、Core、Qdrant、SurrealDB 和实际数据库：

1. 启动前插入一条旧格式 `message_feedback` 和对应旧会话、AI 消息。
2. 新 Core 启动后，旧反馈自动补齐 Space、Core Owner、角色、会话和 requestId，正式统计返回 `good=1`。
3. 通过正式 Owner 接口提交 `other` 和 `unsafe` 反馈，统计变为 total 3、good/other/unsafe 各 1。
4. 相同 unsafe requestId 重放仍返回原 ID 3，没有重复行。
5. 安全事件接口实际返回 `user_reported_unsafe`、会话 ID、原因和未处理状态。
6. 删除 other 反馈后 revision 从 1 更新为 2，统计变为 total 2。
7. 完整重启后 tombstone 保持，统计仍为 total 2，deleted 记录未复活。
8. 旧 `/api/messages/feedback/recent` 和旧单消息反馈路由均返回 404。
9. 使用正式聊天契约发送真实模型消息，实际回复成功保存，回复消息 ID 为 `feedback-model-e2e-20261023-1642/assistant`。
10. 对该真实回复提交 `good` 反馈，正式统计实际返回 total 1、byType good 1，列表包含该 Owner 消息。
11. 按精确消息、会话、反馈 ID、安全事件、模型用量、交互记录和旧夹具清理；同时移除心理、关系、需求状态中的测试 request/conversation/turn 来源标记。
12. 最终统计 total 0、recent 空，测试会话与反馈资源均不再存在。

最终运行状态：

- 前端：`http://127.0.0.1:60286`
- Core：`http://127.0.0.1:60293`
- Qdrant：`60290/60291`
- SurrealDB：`60292`
- `/api/health`：HTTP 200，ready=true，模型已配置
- `/readyz`：ready，6 ready、0 degraded、0 blocking、0 failed
- 旧消息反馈路由：HTTP 404
- Core SHA-256：`205b1d071832c74d540284ffdad3f24284f7f53ddd146fdb492aa4906f21947f`
- Source Gate：`9e479390bde0d6856b9cf36d5569725e57acc7c13981bac330eee725c85513f4`

## 验证边界

本轮未构建 Android APK 或桌面安装包，未手工点击桌面/手机安装包 UI；双端通过源码、生产构建、契约测试和正式 HTTP Owner 接口验证。真实模型验证在本地完整生产配置运行，未在远程云服务器部署。

未修改管理端、版本号或发布配置，未拉取、提交或推送 Git。
