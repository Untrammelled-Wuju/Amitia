# 聊天新旧双轨单轨修复与生产模拟

日期：2026-10-23。范围：本轮确认的一项 P1 问题——聊天主数据已经迁入 `kernel_device_owned_resources`，但旧 `conversations/messages/conversation_summaries` 仍通过独立 DTO 参与当前和历史查询，电脑端与手机端又自行合并两套结果，导致重复、优先级相反和旧记录被误认为可编辑。

## 原问题

- 设备数据端口同时返回新 Owner 资源与 `legacyMessages`、`legacyConversations`、`legacySummary`。
- 电脑端和手机端分别实现会话、消息、摘要合并，且两端的会话与消息优先级不一致。
- 旧记录缺少统一 `origin`、`readOnly` 和 revision 语义，客户端可能对旧记录发起原地编辑。
- 业务摘要、导出、搜索、清理、引用和工作台等链路直接读取旧 DTO，绕过了统一权限边界。
- 云端统筹 Core 继续旧设备会话时，当前 Owner 的续接记录与历史设备旧镜像可能同时出现在查询和 AI 上下文中。

## 行业标准方案

1. `kernel_device_owned_resources` 成为聊天运行时的唯一资源事实源。
2. 所有消费者只接收统一 `Resources`，不得在协议中并排返回旧聊天数组。
3. 旧表只作为一次性迁移和缺失记录兜底，经 Owner 适配器转换为 `origin=legacy`、`readOnly=true`、`revision=0` 的只读资源。
4. 当前 Owner 已存在同 `kind/id` 时，旧镜像不参与查询；当前 Owner 墓碑存在时，旧记录不得复活。
5. 客户端不自行猜测 Owner 或合并第二套数组，只消费统一资源并校验 ExecutionScope、Owner、revision 和只读状态。
6. 续接历史必须携带显式 `conversationOrigin`。没有来源的同 ID 冲突返回 409，禁止按名称或 ID 静默合并。
7. 历史镜像过滤只依据显式谱系：只移除当前续接已经承接的旧会话、同 ID 消息、工作记忆和摘要，保留旧设备独有记录。

## 功能迁移

| 旧功能 | 新位置 |
| --- | --- |
| 新 Owner 会话查询 | `kernel_device_owned_resources` 的 `conversation` |
| 旧会话只读展示 | 统一 `Resource`，标记 `origin=legacy` |
| 新 Owner 消息 | `kernel_device_owned_resources` 的 `message` |
| 旧消息兜底 | 统一只读 `message`，不进入独立 DTO |
| 新 Owner 摘要 | `kernel_device_owned_resources` 的 `summary` |
| 旧摘要兜底 | 统一只读 `summary` |
| 会话搜索、导出、清理统计 | 按 Owner 权威去重后的统一资源 |
| AI 历史和上下文 | 统一资源及显式续接谱系 |
| 电脑端和手机端读取 | 只读 `snapshot.resources` 和 `historicalSnapshot.resources` |
| 旧记录编辑 | 禁止原地编辑，保留原始只读能力 |

旧 `legacyMessages`、`legacyConversations`、`legacySummary` 协议字段及其客户端合并分支已删除。旧数据库表没有在本次改动中直接删除，因为它们是现存用户的迁移数据源；运行时不再把它们视为第二套权威系统。

## 跨模式语义

- 本地 Core：设备就是当前资源 Owner，同一 Owner 只返回一套资源；旧记录仅在缺失时以只读资源兜底。
- Device Agent：代理目标设备按同一 Owner 规则读取，不访问其他设备的同名记录。
- 云端统筹 Core：当前 Owner 为 Core 时，显式续接旧设备会生成稳定 continuation ID；历史查询继续以原设备为 Owner。
- 真正跨设备同 ID：没有 `conversationOrigin` 时保留为独立记录并返回冲突，不进行全局同 ID 去重。
- 显式续接：仅过滤当前续接已经承接的旧镜像，旧设备独有的消息和摘要继续可见。

## 本轮补充修复

- 新增业务层 `filterHistoricalContinuationMirrors`，同时接入查询和推理路径。
- 过滤条件要求显式来源、历史 Owner、历史会话 ID 和当前 continuation 同时匹配。
- 只过滤已有当前承接的旧会话、同 ID 消息/工作记忆和旧摘要。
- 当前 continuation 尚未建立时，不隐藏来源历史。
- 清理手机端已删除“回复通知”系统的遗留测试文件。
- 修复手机端旧 Transport Provider 测试缺少 Riverpod 依赖覆盖和异步等待的问题，使其验证当前 Provider 结构。

## 定向验证

- `go test ./internal/devicemesh/business` 通过。
- `go test ./cmd/server -run 'Test(OwnedChat|Mesh|Historical|Conversation)'` 通过。
- `go vet ./internal/devicemesh/business ./internal/devicemesh/coordination ./internal/chat ./internal/interaction` 通过。
- 电脑端 3 个相关测试文件共 33 项通过。
- 电脑端 `vue-tsc` 与 Vite 生产构建通过。
- 手机端聊天单轨测试 37 项通过。
- 手机端 Transport Provider 5 项测试通过。
- 手机端 `flutter analyze --no-fatal-infos --no-fatal-warnings` 无 error；仓库仍有既有 info 和 warning。

## 真实生产模拟

使用正式 Electron、Core、Qdrant、SurrealDB、项目内 Node 和实际数据库，在统筹模式中执行：

1. 写入唯一会话来源、当前 continuation、同 ID 当前消息、同 ID 历史镜像和仅历史存在消息。
2. 通过正式本地会话凭据调用业务聊天查询接口。
3. 返回当前 continuation，当前 Owner 的同 ID 消息内容胜出且只出现一次。
4. `historicalSnapshot` 不再返回已承接的旧会话与同 ID 消息。
5. 历史设备独有的消息仍然返回，Owner 和目标设备正确。
6. 不带 `conversationOwnerId` 的同 ID 查询返回 HTTP 409，没有静默合并。
7. 测试数据按精确 Owner、kind 和 resource_id 删除，设备新库与旧库计数均为 0。

最终完整项目由本次源码重建并重启：

- 前端：`http://127.0.0.1:55527/`
- Core：`http://127.0.0.1:55532/`
- 兼容端口：`http://127.0.0.1:18899/`
- `/api/health`：HTTP 200，ready=true，模型已配置
- `/readyz`：ready，6 ready、0 degraded、0 blocking、0 failed
- Core SHA-256：`423bd0ef320941fcb5c110143ec82f6a7b637662cbb305062821c07b3c5637f6`
- Source Gate：`324f54a0a67f937873deb8fd5040373eb8aa5052a6bcb360f9d785a3d6127c3a`

## 验证边界

本轮生产模拟使用本地真实 Core、正式数据库和正式 HTTP 契约，并用单元测试覆盖云端 Core、Device Agent、本地 Owner 和真正跨设备同 ID 语义。未在当前环境启动独立远程 Cloud Core 群集，未构建 Android APK 或桌面安装包，未执行 Windows/Linux 真机 UI 验收。未修改管理端、版本号或发布配置，未拉取、提交或推送 Git。
