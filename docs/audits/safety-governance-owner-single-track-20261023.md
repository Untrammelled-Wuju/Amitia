# 安全治理 Owner 单轨化审计报告

审计日期：2026-10-23

审计范围：安全配置、聊天运行时安全决策、消息安全反馈、安全事件查询与处理、旧数据迁移、数据可移植性、桌面端与手机端接入。

## 审计结论

本轮确认并修复一个 P1 级单轨化缺陷：项目同时存在旧的安全配置与事件表，以及新的 Owner 范围安全策略与事件账本。旧配置没有稳定进入聊天运行时，旧安全事件没有完整的 Owner、Core、角色、消息和反馈关联，三端读取与处理语义不一致。

同时发现并修复一个链接路关联缺陷：真实聊天请求进入运行时后，安全事件使用内部父任务幂等键 `owned-parent/...`，没有使用调用方真实 `requestId`。修复后统一从可信执行上下文读取请求号，并回退到请求对象和交互记录。

还发现安全阻断后的收尾会因不存在活动推理快照而返回额外授权错误，并遗留 `running` 检查点。修复后失败收尾提交明确为 `failed` 检查点，安全阻断只对外返回安全原因。

## 行业标准修复方案

1. 以 Owner 范围策略作为唯一运行时权威。

   `safety_policies` 以 `space_id + owner_id` 为主键，保存 Core、revision、配置和 BDI。每次聊天动态读取当前 Owner 策略，不使用进程启动时的全局默认值。

2. 以统一安全事件账本作为唯一审计权威。

   `safety_incidents` 保存 Space、Owner、Core、角色、会话、消息、反馈、请求、策略 revision、处理状态、处理人和 revision CAS。安全反馈与事件在同一数据库事务中提交，通过反馈 ID 保证幂等。

3. 旧表只作为一次性迁移源。

   新库基线不再创建 `safety_events`。现有数据库启动时先确认旧表存在，迁移到新账本后立即删除旧表；没有旧表时迁移保持幂等。

4. 使用可信执行上下文关联真实请求。

   安全事件按照可信执行上下文请求号、请求对象请求号、交互记录的优先级关联请求。禁止使用内部父任务键冒充调用方请求号。

5. 三端使用同一契约。

   电脑端和手机端均通过 Owner 范围 API 读取配置、事件和处理状态，并发送 `expectedRevision` 执行单条 CAS 处理。删除旧的安全检查路由和手机旧调用，避免双轨。

6. 安全治理纳入数据可移植性。

   本地、Cloud Core、Device Agent 当前 Owner 范围均可导出和恢复安全策略与事件；支持 skip、replace、duplicate 冲突策略。

7. 失败必须可解释且可恢复。

   策略读取失败、事件写入失败均 fail-closed。安全阻断后的运行检查点必须落为失败状态；修复动作不得覆盖原始安全错误。

## 实施结果

- 安全运行时改用 Owner 策略和统一 Governor。
- 输入在模型调用前检查，输出在持久化前检查。
- `block`、`rewrite`、`audit_only` 语义分离。
- 事件写入失败时阻断请求，不静默降级。
- `unsafe` 消息反馈与安全事件同事务提交并保持幂等。
- 安全事件查询、清空和单条处理均按当前 Space、Owner、Core 隔离。
- 旧 `/api/safety/check-input`、`/api/safety/check-output`、`/api/safety/check-import` 路由已删除。
- 旧 `app_settings.safety_config` 和 `app_settings.safety_bdi_config` 仅保留一次性迁移读取。
- 旧 `safety_events` 生产代码不再写入，迁移后直接删除。
- 删除无生产调用方的旧 `decision.SafetyGovernor`，保留表达式计划实际使用的 `SafetyCheckResult` 契约。
- 安全阻断的聊天检查点正确落为 `failed`，不再拼接无关授权错误。

## 生产环境模拟

模拟通过完整项目真实启动的 Owner 聊天入口执行：

1. 发送受限输入后，SSE 只返回 `orchestrator: safety blocked: policy:adult_content`。
2. 未调用模型生成，未保存 assistant 消息。
3. 安全事件 Owner 和 Core 正确，`requestId` 与调用方真实请求号一致。
4. 对应聊天检查点状态为 `failed`。
5. 单条处理将事件 revision 从 1 更新到 2，状态为 `resolved`，处理人为真实设备 ID。
6. 使用旧 revision 重复处理返回 HTTP 409，未发生重复覆盖。
7. 新数据库不存在 `safety_events` 表。
8. 三个旧安全检查路由均返回 HTTP 404。
9. 完整项目 `/api/health` 返回 ready，桌面端、前端、Core、Qdrant、SurrealDB、插件宿主均保持运行。

模拟结束后按唯一测试前缀清除安全事件、交互记录、会话、消息、检查点、inbox、outbox 和业务请求。此前真实聊天测试造成的心理、关系和需求状态变更及记忆强化已按历史 revision 和服务端公式回滚。清理后测试标识记录均为 0，SQLite integrity check 为 `ok`。

## 定向验证

- `go test ./internal/safety ./internal/interaction ./internal/decision ./internal/configwrite ./internal/migration ./internal/devicemesh/business`
- `go test ./cmd/server -run 'TestUnsafeOwnedFeedback|TestOwnedChatFeedback|TestLegacyMessageFeedbackScopeMigration' -count=1`
- 相关包 `go vet`
- 前端 `typecheck` 与生产构建
- 手机端安全服务、反馈服务和传输 Provider 定向测试
- 手机端 `flutter analyze` 无 error

## 边界与迁移说明

旧 `safety_events` 不包含 Owner、Core、消息、反馈和 requestId 元数据。迁移时只能将这些历史事件归属到启动时确认的当前活动 Owner 和 Core，原平台、设备来源等无法证明的信息不伪造。新事件从本次单轨化开始均具备完整归属和关联。
