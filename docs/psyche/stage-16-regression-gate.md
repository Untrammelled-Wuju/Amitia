# 第16步主链路修复回归门

对应实施计划：V3.1 第16步。

## 目标

确认第1至第15步没有破坏现有聊天、工具、语音和消息存储主链路；严重回归未关闭前，不进入后续作用域迁移。

## 验收标准

| 门禁项 | 验收方式 | 通过条件 |
|---|---|---|
| 基线样本 | `docs/psyche/baseline/scenario-fixtures.json`、`docs/psyche/regression-cases.md` | 正常文本、工具、记忆检索、语音意图输出形态保持兼容；差异必须记录原因 |
| 竞态测试 | `go test ./internal/agent/tool -run "Concurrent|Cancelled|Audit"` | 工具上下文不串角色、强制语音不串请求、取消不写副作用 |
| 迁移测试 | `go test ./cmd/server ./internal/migration` | 旧库可升级，失败迁移回滚，检索日志旧数据可读并正确标记 legacy |
| 消息事务故障测试 | `go test ./internal/chat -run "FindRequestMessages|LoadHistory|MessageCommit"` | 同一 request_id 的用户消息、助手消息、会话计数更新保持事务一致 |
| 请求追踪准备 | 第15步追踪测试完成后补跑 | 失败请求可通过 request_id 定位输入、模型调用、数据库提交阶段 |

## 输出形态对比

| 链路 | 改造前形态 | 第1至第15步后允许形态 | 差异原因 |
|---|---|---|---|
| 普通聊天 | 返回单个或多段 assistant 文本 | 仍返回文本，额外携带服务端 userMessageId/requestId | 支持失败重试、幂等写入和端到端追踪 |
| 工具调用 | 工具结果只影响当前回复 | 工具意图和结果写入审计表，返回文本兼容 | 支持取消、幂等和副作用追踪 |
| 强制语音 | 全局语音标记可能串请求 | forceVoice 只在当前工具结果内生效 | 消除并发串扰 |
| 记忆检索 | 检索日志可能把 character_id 写成 conversation_id | 新日志写真实 conversation_id、character_id、request_id、channel | 支持追踪真实聊天链路，旧数据以 legacy 标记 |
| 消息存储 | 失败时可能删除用户消息或产生重复消息 | 用户消息保留状态，重试按 request_id 幂等查找 | 支持前端重试和失败定位 |

## 当前关闭证据

| 用例 | 证据文件 | 状态 |
|---|---|---|
| REG-B-010 | `backend/internal/agent/tool/context_test.go` | 已有并发工具上下文测试 |
| REG-B-011 | `backend/internal/agent/tool/context_test.go` | 已有并发 forceVoice 隔离测试 |
| REG-B-012 | `backend/cmd/server/services_test.go` | 已有服务容器单例构建测试 |
| REG-B-014 | `backend/internal/agent/tool/context_test.go`、`backend/internal/memory/repository_test.go` | 已有 expiresAt/entityId 持久化测试 |
| REG-B-015 | `backend/internal/memory/service_retrieval_log_test.go`、`backend/cmd/server/main_test.go` | 已有真实会话检索日志与 legacy 迁移测试 |
| REG-B-021 至 REG-B-023 | `backend/internal/chat/service_history_test.go`、前端发送链路改动 | 已有 request_id 幂等查询与历史去重基础测试 |
| 第16步事务门 | `backend/internal/chat/message_transaction_test.go` | 新增消息提交回滚测试 |

## 阻断条件

以下任一条件出现时，不允许进入后续作用域迁移：

| 条件 | 处理 |
|---|---|
| `go test ./internal/chat ./internal/agent/tool ./internal/memory ./cmd/server ./internal/migration` 失败 | 修复直接回归；无法修复则写入根目录 `待修复.md` |
| 工具并发测试发现角色、会话、request_id 串扰 | 停止迁移，先修复上下文隔离 |
| 消息事务测试发现半提交 | 停止迁移，先修复事务边界 |
| 第15步追踪测试缺失或失败 | 只允许继续追踪补测，不允许进入作用域迁移 |

## 本阶段建议执行命令

```bash
go test ./internal/chat ./internal/agent/tool ./internal/memory ./cmd/server ./internal/migration
```

执行目录：`backend`。
