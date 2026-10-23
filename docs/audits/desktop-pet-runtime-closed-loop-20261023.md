# 桌宠运行时闭环修复与生产模拟

日期：2026-10-23。

## 原问题

桌宠 V2 已定义新的权威链路：

`InstallationCoordinator -> DesiredState -> Outbox -> RuntimeCommand -> Runtime ACK -> ActualProjection`

但生产迁移门禁和客户端接入没有完成同一闭环：

- 生产写入 canary 只查询到一条安装和 Runtime 记录后直接返回成功，没有真正执行 Coordinator、RuntimeCommand、ACK 和投影核对。
- 桌面端操作在 `waiting_runtime_ack` 处提前停止跟踪。
- 手机端只有安装和版本操作等待 operation；启用、停用、默认动作、重置位置和卸载直接忽略 operation ID。
- 安装列表只返回期望安装记录，不返回实际应用 revision、实际动作和运行健康状态。
- 手机端仍发送旧 `action_key` 字段，存在新旧契约混用。
- 同一桌宠操作的最终失败可能只写入后端，电脑端和手机端仍显示旧的期望状态。

## 行业标准方案

采用单一操作契约、期望/实际状态分离、terminal ACK 验证和端到端幂等：

1. 所有安装变更返回 `operationId`、`status` 和 `desiredRevision`。
2. 电脑端和手机端统一跟踪 `created/queued/running/waiting_runtime_ack/failed_retryable`，直到完成、失败、取消或明确超时。
3. 安装查询返回独立 `runtime` 视图，包含 desired/applied revision、实际 release、实际动作、健康状态、同步状态和 `pending`。
4. 生产写入门禁执行真实 Coordinator 操作，并验证 RuntimeCommand 完成、实际投影 revision 不落后于 desired revision。
5. 没有任何安装时，canary 只能返回显式 `skipped/not_applicable` 和原因，不能生成虚假 operation。
6. 删除旧手机 `action_key` 契约和页面级原始变更调用。
7. 所有变更使用 `Idempotency-Key`，重复请求返回同一 operation。

## 后端修复

- `CanaryResult` 新增 `skipped` 与 `skipReason`。
- `executeDesktopPetWriteCanary` 现在通过 Coordinator 发起真实待机动作写入。
- canary 等待 operation 进入 `completed`，再核对 desired state、RuntimeCommand 和 installation projection。
- canary 要求 `appliedDesiredRevision >= desiredRevision`，命令必须为 `completed`。
- 无安装时只允许明确跳过；有安装但无 ready Runtime 时直接失败。
- `appWriteLockoutPort.ExecuteCanaryOperation` 删除重复的直接命令实现，统一复用同一 Coordinator canary。
- 安装列表和详情新增嵌套 `runtime`：
  - `desiredRevision`
  - `appliedDesiredRevision`
  - `desiredEnabled`
  - `desiredVisible`
  - `desiredActionKey`
  - `appliedSettingsRevision`
  - `actualReleaseId`
  - `actualVisible`
  - `actualActionKey`
  - `actualHealth`
  - `runtimeSyncState`
  - `pending`
  - `lastHeartbeatAt`
  - `updatedAt`
- 删除 `updateDefaultActionPayload` 的 `action_key` 兼容字段。

## 电脑端修复

- 新增统一 operation 跟踪器。
- `waiting_runtime_ack` 继续轮询，不再提前结束。
- `failed_terminal` 会抛出后端错误码和错误消息。
- 超时后保留真实等待状态，由界面显示“等待运行端确认”。
- 安装卡片显示期望动作、实际动作、applied/desired revision、运行同步和健康状态。
- pending 时显示明确提示，不再把期望动作展示为已生效动作。

## 手机端修复

- `PetReleaseService` 新增统一的启用、停用、默认动作、位置重置和卸载方法。
- 所有方法发送设备身份与 `Idempotency-Key`。
- 页面不再直接调用这些变更 API。
- 所有变更都使用 `waitForOperation` 并处理完成、失败、取消和等待 ACK。
- 安装卡片消费后端 `runtime` 视图，分别展示期望动作和实际动作。
- 默认动作请求改为 `actionKey`，旧 `action_key` 调用已删除。

## 模式覆盖

- 本地 Core：安装路由仍只写设备本地 InstallationCoordinator。
- 云端 Device Agent：保持相同 API、租户、设备和 operation 契约。
- 云端统筹 Core：同一 canary 和投影核对由 Core 权威数据执行。
- 离线或 Runtime 切换：operation 保持等待或失败，实际状态不会由期望状态替代。
- 重复请求：同一 idempotency key 返回同一安装操作。

## 定向验证

- 桌宠 installation、coordinator、projection、recovery 全包测试通过。
- 生产 canary 测试覆盖：
  - 无安装时显式跳过。
  - 有安装但无 ready Runtime 时拒绝成功。
  - Coordinator、RuntimeCommand、desired revision 和 applied projection 全链路通过。
- `go vet` 与 `go build ./cmd/server` 通过。
- 电脑端 operation 测试 3 项通过。
- 电脑端相关语音回归测试共 22 项通过。
- `vue-tsc --noEmit` 和前端生产构建通过。
- 手机端 operation contract 4 项及相关语音回归共 17 项通过。
- 手机端定向分析无 error、warning 或 info。
- 运行源码中不再存在手机 `action_key` 或删除后的页面级桌宠变更直连。
- `git diff --check` 通过。

## 真实生产环境模拟

使用从本次源码重建的完整 Electron、Core、Qdrant、SurrealDB 环境，并通过正式本地认证 HTTP 接口验证：

1. 创建唯一临时安装、desired state、runtime projection 和 actual state。
2. desired revision 为 11，applied revision 为 10。
3. 正式接口返回 `pending=true`。
4. 期望动作返回 `wave`，实际动作仍为 `idle_normal`。
5. 同步状态为 `pending`，未把期望动作冒充为实际动作。
6. 模拟 Runtime ACK 后，projection 和 actual state 更新为 applied revision 11。
7. 重启完整环境后，正式接口返回 `pending=false`。
8. desired/applied 均为 11，期望动作和实际动作均为 `wave`，同步状态为 `applied`。
9. 按精确 Space、device、runtime、installation 和记录 ID 清理全部临时数据。
10. 安装、desired、projection、actual-state 测试残留均为 0，`quick_check` 为 `ok`。

当前源码重建结果：

- Core SHA-256：`644be70b27f5ce3151801d00f97197f715de9f455212a6cecb4551320189171f`
- Source Gate：`02d857ace5165e91b4631692af49b1002acb6a3387f3b368e2c2b12f32d2bc2d`
- 冻结范围：11094 项

## 验证边界

生产模拟验证了正式后端服务、认证、安装查询、期望/实际状态和精确数据清理。Runtime ACK 使用数据库状态推进模拟，没有安装新的真实桌宠资源包，也没有在 Android 真机执行该页面。电脑端和手机端 operation 跟踪使用同一正式 operation contract 的定向测试验证。未修改管理端、版本号或发布配置，未拉取、提交或推送 Git。
