# G41 — 正式冻结 AmitiaX Game Plugin 公共契约

**项目：** Amitia Game Mode

**执行日期：** 2026-08-13

**前置条件：**
- G38 Backend Full Test Gate: PASS
- G39 Flutter Three-Center Full Test Gate: PASS
- G40 Architecture Uniqueness Gate: 12/12 UNIQUE

**冻结协议：** `amitia-game-host/1` (AmitiaX Game Plugin Host Protocol Major Version 1)

---

## 执行摘要

| 维度 | 结果 |
|------|------|
| **Protocol Identifier** | `amitia-game-host/1` — FROZEN |
| **冻结契约类别** | 20 类全部冻结 |
| **Acceptance 标准** | 全部通过 |
| **P0 问题** | 0 |
| **P1 问题** | 0 |
| **P2 观察** | 3 项（非阻塞） |
| **总体状态** | **PASS — CONTRACT FROZEN** |

---

## 一、冻结总表

| # | 契约类别 | 状态 | 核心文件 |
|---|---------|------|---------|
| 1 | Protocol Identifier | FROZEN | `protocol/version.go` |
| 2 | Manifest V2 game_plugin | FROZEN | `manifest_v2/manifest.go` |
| 3 | Manifest V2 desktop_pet_plugin | FROZEN | `manifest_v2/manifest.go` |
| 4 | Go Game Plugin SDK | FROZEN (v1.0.0) | `sdk/go/` |
| 5 | TypeScript Game Plugin SDK | FROZEN (v1.0.0) | `sdk/game-plugin/` |
| 6 | Runtime Identity / Lifecycle | FROZEN | `domain/runtime_instance.go` |
| 7 | Service Contract | FROZEN | `domain/service.go` |
| 8 | RPC Contract | FROZEN | `protocol/envelope.go` |
| 9 | Notification Contract | FROZEN | `notification/notification.go` |
| 10 | Event Contract | FROZEN | `stream/event.go` |
| 11 | Latest State Contract | FROZEN | `state/state_types.go` |
| 12 | Channel Contract | FROZEN | `protocol/channel.go` |
| 13 | Stream / Cursor / Replay | FROZEN | `stream/stream_manager.go` |
| 14 | BinaryRef / FileRef | FROZEN | `stream/binary/reference.go` |
| 15 | Config / SecretRef | FROZEN | `config/value.go` |
| 16 | Permission Contract | FROZEN | `permission/adapter.go` |
| 17 | Host API Invocation | FROZEN | `host_api/gateway.go` |
| 18 | Control Authority / Epoch | FROZEN | `control/manager.go` |
| 19 | Control Effect / Output Gate | FROZEN | `control/gate_service.go` |
| 20 | Error / Capability / Compatibility | FROZEN | `protocol/error.go` |

---

## 二、Protocol Identity 冻结

### 2.1 Protocol Identifier

| 属性 | 值 | 状态 |
|------|-----|------|
| Protocol Name | `amitia-game-host` | FROZEN |
| Protocol Major | `1` | FROZEN |
| Protocol Version | `amitia-game-host/1` | FROZEN |

**定义位置：** `backend/pkg/gameplugin/protocol/version.go:3-7`

**语义：** AmitiaX Game Plugin Host Protocol Major Version 1

**禁止：** 不能再使用 `gamehost-v1`、`game-plugin-v1`、`mock-protocol`、`amitia-plugin-game` 作为同一协议的第二正式名字。

### 2.2 四种版本分离

| 版本维度 | 当前值 | 说明 |
|---------|--------|------|
| Protocol Version | `amitia-game-host/1` | 协议兼容性代 |
| Manifest Schema Version | `v2` | Manifest 结构版本 |
| Go SDK Package Version | `1.0.0` | Go SDK SemVer |
| TS SDK Package Version | `1.0.0` | TypeScript SDK SemVer |

### 2.3 Protocol v1 兼容性规则

**允许的增量变化：**
- 新增 optional field
- 新增 optional capability
- 新增 optional notification
- 新增 optional Host API route
- 新增 optional error metadata
- 新增 optional descriptor metadata

**禁止的变化（Breaking Change 需升级 Major）：**
- 删除 existing required field
- 改变 existing field 类型
- 改变 existing field 语义
- 重命名 required method
- 改变 identity trust 语义
- 改变 correlation 语义
- 改变 cursor 语义
- 改变 BinaryRef 语义
- 改变 Authority Epoch 语义
- 改变 existing ControlMode 语义

---

## 三、Handshake / Negotiation Contract 冻结

### 3.1 状态机

| 状态 | 值 | 终止态 |
|------|-----|--------|
| `HandshakeStateAttached` | `attached` | no |
| `HandshakeStateHandshaking` | `handshaking` | no |
| `HandshakeStateReady` | `ready` | yes |
| `HandshakeStateRejected` | `rejected` | yes |
| `HandshakeStateClosed` | `closed` | yes |

**转换规则：** `attached` → `handshaking` → `ready` | `rejected`。终止态不可再转换。

### 3.2 握手流程

```
connect → identity binding → protocol negotiation → service registration
→ required capability validation → Ready
```

### 3.3 Connected != Ready

**强制保证：** IPC 层 `CanProcess` 门控 — 仅 `ready` 态放行业务方法。

### 3.4 HelloRequest 结构

| 字段 | 类型 | 必填 |
|------|------|------|
| `supportedProtocols` | `[]string` | 是 |
| `capabilities` | `[]string` | 否 |
| `rpcNamespaces` | `[]string` | 否 |
| `channels` | `[]ChannelAdvertisement` | 否 |
| `sdk` | `*SDKInfo` | 否 |
| `metadata` | `map[string]json.RawMessage` | 否 |

### 3.5 HelloResponse 结构

| 字段 | 类型 |
|------|------|
| `protocol` | `string` |
| `capabilities` | `[]string` |
| `rpcNamespaces` | `[]string` |
| `channels` | `[]string` |
| `metadata` | `map[string]json.RawMessage` |

### 3.6 Protocol Negotiation 算法

- Plugin 在 HelloRequest 中声明 `supportedProtocols` 列表
- Host 遍历 Plugin 的列表，逐个比对自身支持的协议
- 返回第一个匹配项（优先 Plugin 声明顺序）
- 无交集 → `HandshakeErrorProtocolMismatch`

### 3.7 Capability Contract

**9 个标准能力：**

| Capability | 值 |
|------------|-----|
| `CapabilityRealtimeControl` | `realtime_control` |
| `CapabilityStateStreaming` | `state_streaming` |
| `CapabilityEventStreaming` | `event_streaming` |
| `CapabilityBinaryStreaming` | `binary_streaming` |
| `CapabilityCustomRPC` | `custom_rpc` |
| `CapabilityHostAPI` | `host_api` |
| `CapabilitySharedControl` | `shared_control` |
| `CapabilityCustomUI` | `custom_ui` |
| `CapabilityMultiService` | `multi_service` |

**自定义能力：** 必须包含 `.`（点分隔命名空间格式），如 `minecraft.pathfinding`

**语义：** Capability 是 Plugin 向 Host 的声明支持，不是 Host 向 Plugin 的授权。

---

## 四、Bootstrap Contract 冻结

### 4.1 传输机制

| 属性 | 值 |
|------|-----|
| 传输类型 | stdio |
| 帧格式 | 4 字节 Big-Endian 长度头 + JSON 载荷 |
| 最大帧 | 16 MB |
| 端点发现 | 隐式 stdin/stdout |

### 4.2 入口

| 侧 | 入口 |
|----|------|
| Host | `controlPlane.Attach(ctx, peer, transport)` |
| Plugin (Go) | `Runner.Run(ctx)` |
| Plugin (TS) | `Runner.run(defaultRegistry)` |

---

## 五、Envelope Contract 冻结

### 5.1 结构

| JSON 字段 | 类型 | 必填 |
|-----------|------|------|
| `protocol` | string | 是 |
| `type` | MessageType | 是 |
| `id` | string | 是 |
| `requestId` | string | response/error |
| `method` | string | request/notification |
| `runtimeId` | string | 可选 |
| `pluginId` | string | 可选 |
| `serviceId` | string | 可选 |
| `payload` | json.RawMessage | 可选 |
| `error` | ProtocolError | error 必填 |
| `metadata` | map | 可选 |

### 5.2 消息类型

| 类型 | 值 |
|------|-----|
| `MessageTypeRequest` | `request` |
| `MessageTypeResponse` | `response` |
| `MessageTypeNotification` | `notification` |
| `MessageTypeError` | `error` |

### 5.3 保留命名空间

| 前缀 | 归属 |
|------|------|
| `host.` | Host 核心 |
| `plugin.` | Plugin 管理 |
| `runtime.` | Runtime 管理 |
| `service.` | 服务管理 |
| `channel.` | 通道管理 |
| `control.` | 控制面 |

---

## 六、Manifest V2 Contract 冻结

### 6.1 game_plugin Contribution Type

| 属性 | 值 | 状态 |
|------|-----|------|
| ContributionKind | `game_plugin` | FROZEN |
| ExtensionDomain | `ExtensionDomainGame` | FROZEN |
| ManagementTarget | `game_center` | FROZEN |
| protocolVersion 必填 | 是 | FROZEN |

### 6.2 desktop_pet_plugin Contribution Type

| 属性 | 值 | 状态 |
|------|-----|------|
| ContributionKind | `desktop_pet_plugin` | FROZEN |
| ExtensionDomain | `ExtensionDomainDesktopPet` | FROZEN |
| ManagementTarget | `desktop_pet_center` | FROZEN |
| protocolVersion 必填 | 否 | FROZEN |

### 6.3 互斥规则

一个 extension 不能同时包含 game_plugin 和 desktop_pet_plugin contributions。

### 6.4 ManagementTarget 映射

| Contribution Kind | ExtensionDomain | ManagementTarget |
|-------------------|-----------------|------------------|
| game_plugin | ExtensionDomainGame | `game_center` |
| desktop_pet_plugin | ExtensionDomainDesktopPet | `desktop_pet_center` |
| 其他所有 kind | ExtensionDomainGeneral | `extension_center` |

### 6.5 Manifest 顶层字段

| 字段 | 类型 | 必填 | 稳定性 |
|------|------|------|--------|
| `manifestVersion` | int | 是 | FROZEN |
| `extension` | ExtensionMeta | 是 | FROZEN |
| `publisher` | PublisherMeta | 是 | FROZEN |
| `compatibility` | Compatibility | 否 | FROZEN |
| `modules` | []ModuleMeta | 是 | FROZEN |
| `dependencies` | []Dependency | 否 | FROZEN |
| `permissions` | []PermissionReq | 否 | FROZEN |
| `resources` | []ResourceMeta | 否 | FROZEN |
| `lifecycle` | LifecycleMeta | 否 | FROZEN |
| `integrity` | IntegrityMeta | 是 | FROZEN |
| `development` | DevelopmentMeta | 否 | FROZEN |

### 6.6 Contribution Kind 完整枚举

`tool`, `agent_skill`, `workflow`, `mcp_server`, `provider`, `hook`, `event_subscription`, `schedule`, `background_task`, `ui_page`, `ui_panel`, `ui_chat`, `ui_context_action`, `ui_desktop`, `resource`, `game_plugin`, `desktop_pet_plugin`

---

## 七、SDK Public API 冻结

### 7.1 Go SDK (v1.0.0)

**Stable API 分类：**

| 分类 | API 数量 | 说明 |
|------|---------|------|
| Bootstrap | 8 | Runner, RunnerConfig, HelloConfiguration 等 |
| Client | 18 | SendRequest, SendResponse, SendNotification 等 |
| Transport | 5 | StdioTransport, StdioTransportConfig 等 |
| Handler | 5 | RequestHandler, NotificationHandler, HandlerRegistry |
| Service | 6 | RegisterService, UnregisterService 等 |
| Event | 3 | PublishEvent |
| State | 5 | PublishState, GetState |
| Channel | 5 | ChannelPublish, ChannelSubscribe 等 |
| Binary | 4 | RegisterBinary, ReleaseBinary |
| Stream | 8 | StreamOpen, StreamWrite, StreamRead 等 |
| Control | 25 | AuthoritySnapshot, ControlOutput, Takeover 等 |
| HostAPI | 10 | InvokeHostAPI, QueryHostAPICapabilities 等 |
| Permission | 10 | CheckPermission, GetPermissionSnapshot 等 |
| Secret | 6 | AcquireSecret, ReleaseSecret 等 |
| Plugin/Descriptor | 6 | Plugin, Descriptor, DescriptorBuilder |
| Payload | 4 | MarshalPayload, DecodePayload |
| ID | 3 | IDGenerator, UUIDGenerator |
| Errors | 8 | SDKError, NewProtocolError 等 |

**Experimental API：**
- `TimestampGenerator` — 消费者未被观测到实际使用

**Internal API：**
- `Runner.findRegistryForRequest` — package-private
- `Runner.performHandshake` — 内部握手实现
- `HelloResponse` — 内部类型

### 7.2 TypeScript SDK (v1.0.0)

**Stable API 分类：**

| 分类 | API 数量 |
|------|---------|
| Bootstrap | 10 |
| Constants | 6 |
| Manifest | 15 |
| Types | 10 |
| Errors | 15 |
| Runtime | 12 |
| Host | 18 |
| Tools | 8 |
| Events | 10 |
| Hooks | 12 |
| Tasks | 15 |
| Storage | 12 |
| Secrets | 8 |
| UI | 10 |

**Experimental API：**
- `createAmitiaUI(bridge)` — API 意图未定
- `ExtensionRuntime` 抽象类 — 消费者未直接使用
- `registerRuntime/getRegistrations` — 全局注册表单例模式
- `EventPublisher` 接口 — 消费者未直接实现

### 7.3 Wire Parity

Go SDK 是传输级 SDK（transport-level），TS SDK 是扩展级 SDK（extension-level）。两者不在同一抽象层，通过共享 `protocol.Envelope` 实现 wire 兼容。

---

## 八、Runtime Identity Contract 冻结

### 8.1 四种身份

| 身份 | 类型 | 语义 | 状态 |
|------|------|------|------|
| ExtensionID | `string` (reverse-DNS) | Extension package identity | FROZEN |
| PluginID | `string` | game_plugin contribution identity | FROZEN |
| RuntimeInstanceID | `string` | Host-created runtime identity | FROZEN |
| ServiceID | `string` | Host-bound service identity | FROZEN |

### 8.2 Identity 不相等语义

| 断言 | 状态 |
|------|------|
| ExtensionID != PluginID | FROZEN |
| PluginID != RuntimeInstanceID | FROZEN |
| RuntimeInstanceID != ServiceID | FROZEN |
| Plugin 不能自选可信 RuntimeID | FROZEN |

---

## 九、Runtime Lifecycle Contract 冻结

### 9.1 RuntimeState 枚举

| 状态 | 值 | 终止态 |
|------|-----|--------|
| `RuntimeStateCreated` | `created` | no |
| `RuntimeStateStarting` | `starting` | no |
| `RuntimeStateRunning` | `running` | no |
| `RuntimeStateDegraded` | `degraded` | no |
| `RuntimeStateSuspended` | `suspended` | no |
| `RuntimeStateStopping` | `stopping` | no |
| `RuntimeStateStopped` | `stopped` | yes |
| `RuntimeStateFailed` | `failed` | yes |

### 9.2 有效状态转换

```
created → starting
starting → running | degraded | stopping | failed
running → degraded | suspended | stopping | failed
degraded → running | suspended | stopping | failed
suspended → running | degraded | stopping | failed
stopping → stopped | failed
```

### 9.3 Lifecycle 操作区分

| 操作 | 语义 | 独立性 |
|------|------|--------|
| Start | 正常启动 | 独立 |
| Stop | 正常停止 | 独立 |
| Restart | 产生 fresh generation/process/connection/handshake | 独立 |
| Emergency Stop | 安全链：关闭 gate → 暂停 authority → 取消工作 → 停止 → 关闭连接 → 撤销 lease | 独立 |

**关键区分：**
- Restart != UI/Client Stop + Start
- Stop != Emergency Stop
- Takeover != Stop Runtime

### 9.4 Generation Contract

| 属性 | 值 |
|------|-----|
| 类型 | `int64` |
| 语义 | 每次 Restart 递增 |
| 用途 | stale-detection, ownership proof, connection routing |

---

## 十、Service Contract 冻结

### 9.5 ServiceState 枚举

| 状态 | 值 | 终止态 |
|------|-----|--------|
| `ServiceStateCreated` | `created` | no |
| `ServiceStateStarting` | `starting` | no |
| `ServiceStateRunning` | `running` | no |
| `ServiceStateStopping` | `stopping` | no |
| `ServiceStateStopped` | `stopped` | yes |
| `ServiceStateFailed` | `failed` | yes |

### 9.6 Required Service 语义

| 条件 | 结果 |
|------|------|
| Required Service 未启动 | Runtime → `RuntimeStateFailed` |
| Optional Service 失败 | Runtime → `RuntimeStateDegraded` |

### 9.7 RPC Method Namespace

| 命名空间 | 保留 |
|----------|------|
| `host` | 是 |
| `plugin` | 是 |
| `runtime` | 是 |
| `service` | 是 |
| `channel` | 是 |
| `control` | 是 |
| 自定义 (需 `custom_rpc` capability) | 否 |

---

## 十一、RPC Contract 冻结

### 11.1 RPC 保证

| 保证 | 语义 |
|------|------|
| request | 请求消息 |
| response | 响应消息 |
| correlation | 由协议层负责 |
| timeout | 默认 30s, 最大 5min, 最小 100ms |
| cancel | `control.request.cancel` 方法 |
| business error | 通过 ProtocolError 返回 |
| generation isolation | Pending 不跨 generation 复活 |

### 11.2 Timeout 语义

| 参数 | 值 |
|------|-----|
| Default | 30 秒 |
| Maximum | 5 分钟 |
| Minimum | 100 毫秒 |
| Metadata Key | `rpc.timeout_ms` |

### 11.3 Unknown Method

| 场景 | 错误码 |
|------|--------|
| 方法格式无效 | `invalid_argument` |
| 保留命名空间无处理器 | `unsupported` |
| 自定义命名空间未注册 | `not_found` |

### 11.4 Malformed Payload

| 场景 | 错误码 |
|------|--------|
| 方法名格式错误 | `invalid_argument` |
| 消息 ID 为空/超长 | `invalid_request` |
| Envelope 字段冲突 | `invalid_request` |

### 11.5 Pending 不跨 Reconnect/Generation 自动复活

| 规则 | 说明 |
|------|------|
| Pending 上限 | 每 peer 256，全局 4096 |
| 终端状态 | completed / failed / timed_out / cancelled |
| 连接断开 | 仅列出，不自动复活 |
| Generation 隔离 | generation 变化触发 `ErrGenerationMismatch` |

---

## 十二、Notification Contract 冻结

| 属性 | 值 |
|------|-----|
| 语义 | one-way, no response |
| 验证 | Route/Method/Metadata 三重验证 |
| 方法长度 | 最大 1024 字符 |
| 元数据键长 | 最大 256 字符 |
| 禁止控制字符 | 0x00-0x1F |

---

## 十三、Event / Latest State Contract 冻结

### 13.1 Event Contract

| 属性 | 值 |
|------|-----|
| 语义 | something happened |
| 不可变 | 是 |
| 有序 | Sequence 严格递增 |
| 历史 | 有界回放，不保证无限历史 |

### 13.2 Latest State Contract

| 属性 | 值 |
|------|-----|
| 语义 | current value |
| 历史 | 仅最新 |
| 版本 | 每 key 单调递增 |
| 容量 | 每 runtime 最大 4096 个 key |
| 载荷 | 单条最大 1MB |

### 13.3 Event != State

| 维度 | Event | State |
|------|-------|-------|
| 语义 | 发生过的事 | 当前值 |
| 历史 | 有界回放 | 仅最新 |
| 订阅 | 增量推送 | 全量获取 |
| 重连 | bounded_replay / latest | 通过 GetLatestState 重新获取 |

---

## 十四、Channel Contract 冻结

### 14.1 Channel 定义

| 属性 | 值 |
|------|-----|
| 语义 | opaque plugin-defined communication lane |
| 不包含游戏语义 | Host 不定义 movement/inventory/world channel |
| Handle | opaque string |

### 14.2 ChannelKind 枚举

`event`, `state`, `log`, `metric`, `binary`, `custom`

### 14.3 ChannelDirection 枚举

`plugin_to_host`, `host_to_plugin`, `bidirectional`

### 14.4 FrequencyHint 枚举

`low`, `normal`, `high`, `realtime`

### 14.5 Channel Lifecycle

| 阶段 | 操作 |
|------|------|
| Open | `Registry.Register()` |
| Use | `Registry.Get()`, `Registry.Resolve()` |
| Close | `Registry.Unregister()` |
| Invalidate | `Registry.RemoveByRuntime()`, `Registry.RemoveByService()` |

---

## 十五、Stream / Cursor / Replay Contract 冻结

### 15.1 Stream 定义

| 属性 | 值 |
|------|-----|
| 语义 | ordered, bounded, backpressure-aware, cursor-capable |
| 不等于 Event | 是 |
| 不等于 Game Tick | 是 |
| 不承诺无限持久化 | 是 |

### 15.2 Replay Contract

| 属性 | 值 |
|------|-----|
| 类型 | bounded replay |
| 默认容量 | event=256, state=1, log=128, metric=64, binary=64, custom=0 |
| 字节上限 | 64MB |

### 15.3 Cursor Contract

| 属性 | 值 |
|------|-----|
| 语义 | opaque resume token |
| 结构 | RuntimeID + ServiceID + ChannelID + Generation + Sequence |
| 不可解析 | Do not parse Cursor |
| 过期错误 | `ErrCursorStale` |

### 15.4 StreamPolicy 默认值

| ChannelKind | QueueCapacity | ReplayCapacity | Overflow | Resume |
|-------------|--------------|----------------|----------|--------|
| event | 1024 | 256 | reject | bounded_replay |
| state | 64 | 1 | coalesce | latest |
| log | 512 | 128 | drop_oldest | bounded_replay |
| metric | 256 | 64 | drop_oldest | latest |
| binary | 128 | 64 | drop_oldest | bounded_replay |
| custom | 256 | 0 | reject | none |

---

## 十六、BinaryRef Contract 冻结

### 16.1 BinaryRef 定义

| 属性 | 值 |
|------|-----|
| 语义 | opaque managed binary reference |
| 不是 | absolute path / raw pointer / file descriptor |
| ID 格式 | `bin_{uuid}` |

### 16.2 BinaryStorageKind 枚举

`file`, `shared_memory`

### 16.3 BinaryLifetime 枚举

`message` (TTL 5min), `runtime` (随 runtime 生命周期)

### 16.4 ObjectState 枚举

`writing`, `ready`, `releasing`, `released`

### 16.5 BinaryRef Lifecycle

| 阶段 | 操作 | 状态 |
|------|------|------|
| Create | `InsertWriting()` | `writing` |
| Publish | `SealObject()` | `ready` |
| Resolve | `Get()` | — |
| Release | `Release()` | `released` |
| Invalidate | `RemoveByRuntime()` | `released` |

---

## 十七、Config / SecretRef / SecretLease Contract 冻结

### 17.1 SecretRef 定义

| 属性 | 值 |
|------|-----|
| 格式 | `secret://{provider}/{key}` |
| Legacy | `mcp-secret://{provider}/{key}` (自动规范化) |
| Config 保存 | SecretRef 而非 SecretValue |

### 17.2 SecretLease 语义

| 属性 | 值 |
|------|-----|
| Temporary | `ExpiresAt` + `MaxUses` |
| Scoped | RuntimeInstanceID + ExtensionID + ModuleID + Generation + InvocationID |
| Revocable | `Revoked` 标志 + `MarkRevoked()` |
| Runtime-bound | 绑定到 RuntimeInstanceID |
| Not checkpoint-restorable | 是 |

---

## 十八、Permission Contract 冻结

### 18.1 三层权限模型

| 层 | 名称 | 说明 |
|----|------|------|
| L1 | Declared Permission | Manifest 声明 |
| L2 | Granted Permission | Grant 系统 |
| L3 | Effective Permission | G4 计算结果 |

### 18.2 G4 有效权限计算

```
Effective = Declaration ∩ Grant ∩ Scope ∩ Host Policy
```

### 18.3 Default Deny

`DecisionDenied` 是零值 — 默认拒绝。

### 18.4 Permission Identifier 列表

**GameHost 级：**
- `gamehost.control.request`
- `gamehost.control.output`
- `gamehost.channel.use`
- `gamehost.channel.register`
- `gamehost.host_api.invoke`

**Kernel 级：**
- `storage.state.read` / `storage.state.write`
- `secret.read`
- `resource.read` / `resource.write`
- `event.emit` / `event.subscribe`
- `schedule.create` / `schedule.manage`
- `tool.invoke`
- `character.read`
- `conversation.read`
- `memory.read`
- `provider.invoke`
- `ui.notify` / `ui.dialog` / `ui.navigate`
- `clipboard.write` / `clipboard.read`
- `runtime.health.read`

### 18.5 DenyReason 枚举

`not_declared`, `not_granted`, `scope_denied`, `host_policy_denied`, `unknown_permission`, `invalid_subject`, `runtime_inactive`, `snapshot_unavailable`

### 18.6 Permission Revoke 立即生效

通过 Host Policy 路径实时阻断。

---

## 十九、Host API Contract 冻结

### 19.1 Gateway 接口

| 方法 | 说明 |
|------|------|
| `RegisterRoute` | 注册路由 |
| `OpenSession` | 打开会话 |
| `CloseSession` | 关闭会话 |
| `Call` | 调用方法 |
| `QueryCapability` | 查询能力 |
| `ListMethods` | 列出方法 |

### 19.2 已注册 27 个 Method

全部复用现有 Gateway Route，无 GameHost 平行 API。

### 19.3 Host API 经过治理

| 治理层 | 说明 |
|--------|------|
| Effective Permission | 权限检查 |
| Scope | 作用域检查 |
| Rate Limit | 限流 |
| Timeout | 超时 |
| Cancel | 取消 |

### 19.4 Provider Invoke

Plugin 不得到 Provider Credential — Route Handler 托管 Plugin Credential。

---

## 二十、Control Authority / Epoch / Output Gate Contract 冻结

### 20.1 ControlMode 枚举 (G7)

| 枚举 | 值 | 允许 Output |
|------|-----|-------------|
| `ControlModeObserveOnly` | `observe_only` | NO |
| `ControlModeAssist` | `assist` | YES |
| `ControlModeSharedControl` | `shared_control` | YES |
| `ControlModePluginControl` | `plugin_control` | YES |
| `ControlModeUserControl` | `user_control` | NO |
| `ControlModeSuspended` | `suspended` | NO |

### 20.2 Authority Epoch Contract

| 属性 | 值 |
|------|-----|
| 初始值 | `1` |
| 递增 | 每次 Transition 严格 +1 |
| Stale Epoch | DENY |
| Future Epoch | DENY |
| Intent Epoch == Current | 要求 |

### 20.3 Takeover Contract

| 属性 | 值 |
|------|-----|
| User Takeover 不要求 Plugin ACK | 是 |
| 改变 Host authority truth | 是 |
| Invalidate stale control intents | 是 |
| Takeover != Stop Runtime | 是 |

### 20.4 Release Contract

| 属性 | 值 |
|------|-----|
| 重新验证 Permission | 是 |
| 重新验证 Policy | 是 |
| 重新验证 Eligibility | 是 |
| Release 不等于恢复 previousMode | 是 |

### 20.5 Emergency Stop Contract

| 步骤 | 动作 |
|------|------|
| 1 | Close Plugin Output Gate |
| 2 | Suppress Restart |
| 3 | Authority → suspended + Epoch++ |
| 4 | Cancel pending RPC + HostAPI work |
| 5 | Stop Runtime + Cleanup Process Tree |
| 6 | Close Connections + Invalidate Ready |
| 7 | Revoke SecretLease |
| 8 | Stop Streams + Cleanup Channels + Release Binary |
| 9 | Final Verification |

**关键语义：**
- Emergency 不依赖 Plugin ACK
- Emergency 不自动 Recovery
- Restart Suppress 30s

### 20.6 Output Gate (G9) 判定因素

| 因素 | 拒绝原因 |
|------|----------|
| Trusted Identity | `invalid_peer` |
| Control Authority Mode | `authority_mode_denied` |
| Exact Authority Epoch | `stale_epoch` |
| Runtime Eligibility | `runtime_not_eligible` |
| Effective Permission | `permission_denied` |
| Host Policy | `host_policy_denied` |
| Gate Closed | `gate_closed` |

### 20.7 Final Effect Commit 前重新验证

在 Permit 有效且 Epoch 匹配时才调用 `sink.ExecuteAuthorized()`。

### 20.8 Control Effect 由 formal sink/descriptor 识别

不由 Host 解析 payload 关键词。

---

## 二十一、Error Contract 冻结

### 21.1 Protocol 层错误码 (15 个)

| Code | 值 | Retryable |
|------|----|-----------|
| `ErrorInvalidRequest` | `invalid_request` | NO |
| `ErrorInvalidArgument` | `invalid_argument` | NO |
| `ErrorNotFound` | `not_found` | NO |
| `ErrorAlreadyExists` | `already_exists` | NO |
| `ErrorUnsupported` | `unsupported` | NO |
| `ErrorProtocolMismatch` | `protocol_mismatch` | NO |
| `ErrorCapabilityUnsupported` | `capability_unsupported` | NO |
| `ErrorRuntimeUnavailable` | `runtime_unavailable` | YES |
| `ErrorServiceUnavailable` | `service_unavailable` | YES |
| `ErrorInvalidRuntimeState` | `invalid_runtime_state` | NO |
| `ErrorPermissionDenied` | `permission_denied` | NO |
| `ErrorResourceExhausted` | `resource_exhausted` | NO |
| `ErrorTimeout` | `timeout` | NO |
| `ErrorCancelled` | `cancelled` | NO |
| `ErrorInternal` | `internal` | NO |

### 21.2 Host API Gateway 错误码 (19 个)

`method_not_found`, `version_unsupported`, `identity_invalid`, `generation_stale`, `permission_denied`, `scope_denied`, `approval_required`, `input_invalid`, `output_invalid`, `rate_limited`, `timeout`, `cancelled`, `resource_not_found`, `state_conflict`, `host_unavailable`, `ui_host_unavailable`, `dialog_host_unavailable`, `navigation_host_unavailable`, `internal_error`

### 21.3 Error Code 稳定性

| 属性 | 状态 |
|------|------|
| Error Code | 稳定，v1 不可无版本重命名 |
| Error Message | 不稳定，自由文本 |
| 自定义错误码格式 | 必须包含 `.` 分隔符 |

---

## 二十二、Game-agnostic 语义边界冻结

### 22.1 GameHost 不定义通用游戏业务语义

Host 不包含 `game.move`, `game.attack`, `game.inventory`, `game.world`, `game.player` 等 Universal API。

### 22.2 游戏行为通过抽象通道实现

| 通道 | 说明 |
|------|------|
| `custom_rpc` | 游戏自定义 RPC |
| `channel` | 双向消息通道 |
| `host_action` | Host 注册的 sink |
| `binary` | 二进制数据流 |

### 22.3 Host 不解析 Payload 内容

G9 仅依据 Intent 声明字段和 Sink 注册信息判定，不解析 payload 关键词。

---

## 二十三、Compatibility / Versioning Policy

### 22.1 兼容规则 — Additive

以下通常属于兼容：
- new optional field
- new optional capability
- new Host API route
- new event type
- new permission identifier
- new SDK helper
- new optional manifest metadata

### 22.2 不兼容规则 — Breaking

以下属于 breaking：
- remove field
- rename field
- change field type
- change requiredness from optional to required
- change identity semantics
- change permission semantics
- change authority mode semantics
- change cursor opacity
- change BinaryRef ownership
- change Restart meaning
- change Ready meaning
- change Event/State meaning

### 22.3 Unknown Enum 规则

| 类型 | 规则 |
|------|------|
| 安全关键 Enum (ControlMode, Permission) | 未知值 fail closed |
| 普通 presentation enum | fallback Unknown |

### 22.4 Deprecation Policy

| 阶段 | 要求 |
|------|------|
| Deprecate | 标记 deprecated_since |
| Compatibility Window | 保持服务 |
| Migration Path | 提供替代 |
| Removal | 仅通过 Major 版本移除 |

### 22.5 Support Window

Host 必须支持 `amitia-game-host/1`。未来 Protocol v2 出现后不能自动删除 v1。

---

## 二十四、Golden Fixtures 和 Baseline Tests

### 24.1 现有 Baseline Fixtures

**Valid Fixtures (可直接作为 baseline):**
- `valid/request.json` — Protocol v1 请求信封
- `valid/response.json` — Protocol v1 响应信封
- `valid/notification.json` — Protocol v1 通知信封
- `valid/error.json` — Protocol v1 错误信封
- `valid/service.json` — 服务描述符
- `valid/channel.json` — 通道描述符
- `valid/descriptor.json` — 插件描述符
- `valid/opaque_payload.json` — 不透明负载测试

**Invalid Fixtures (可作为 negative baseline):**
- `invalid/request_without_id.json`
- `invalid/request_without_method.json`
- `invalid/wrong_protocol.json`
- `invalid/duplicate_service.json`
- `invalid/invalid_capability.json`
- `invalid/invalid_channel.json`
- `invalid/invalid_error.json`
- `invalid/response_without_request_id.json`

**Cross-Language Fixtures:**
- `cross-language/go-generated-request.json`
- `cross-language/go-generated-response.json`
- `cross-language/ts-generated-request.json`
- `cross-language/ts-generated-response.json`
- `cross-language/ts-generated-notification.json`
- `cross-language/ts-generated-error.json`
- `cross-language/roundtrip-complex-payload.json`

### 24.2 现有 Baseline 测试

| 测试文件 | 用途 |
|---------|------|
| `protocol_test.go` | Protocol v1 编码/解码/验证 |
| `conformance/harness_test.go` | 一致性套件运行器 |
| `conformance/suites.go` | StandardSuite 定义 |
| `sdk/game-plugin/test/conformance.test.ts` | TS SDK 一致性 |
| `manifest_v2/game_plugin_test.go` | Manifest game_plugin 验证 |

### 24.3 需要补充的 Baseline Fixtures

| Fixture | 用途 | 优先级 |
|---------|------|--------|
| `valid/descriptor-full.json` | 完整 descriptor | 高 |
| `valid/plugin-schema.json` | 完整 PluginSchema | 高 |
| `cross-language/go-generated-notification.json` | Go SDK 通知 fixture | 中 |
| `invalid/missing-protocol.json` | 缺少 protocol | 中 |
| `invalid/invalid-namespace.json` | 保留命名空间误用 | 中 |

---

## 二十五、Contract Test 编号

### 25.1 Protocol Contract Tests

| 编号 | 说明 |
|------|------|
| PROTO-001 | protocol identifier |
| PROTO-002 | negotiation |
| PROTO-003 | handshake |
| PROTO-004 | ready |
| PROTO-005 | identity |
| PROTO-006 | rpc |
| PROTO-007 | event/state |
| PROTO-008 | channel |
| PROTO-009 | stream/cursor |
| PROTO-010 | binary |
| PROTO-011 | errors |

### 25.2 Manifest Tests

| 编号 | 说明 |
|------|------|
| MAN-001 | v2 game_plugin baseline |
| MAN-002 | v2 desktop_pet_plugin baseline |
| MAN-003 | multi-service |
| MAN-004 | permission |
| MAN-005 | secretref |
| MAN-006 | unknown target fail closed |
| MAN-007 | optional additive field behavior |

### 25.3 SDK Tests

| 编号 | 说明 |
|------|------|
| SDK-GO-001 | baseline compile |
| SDK-GO-002 | connect |
| SDK-GO-003 | current feature conformance |
| SDK-TS-001 | baseline compile |
| SDK-TS-002 | connect |
| SDK-TS-003 | current feature conformance |

### 25.4 Identity Tests

| 编号 | 说明 |
|------|------|
| ID-001 | ExtensionID != PluginID |
| ID-002 | RuntimeID host-bound |
| ID-003 | ServiceID host-bound |
| ID-004 | payload spoof rejected |

### 25.5 Lifecycle Tests

| 编号 | 说明 |
|------|------|
| LIFE-001 | Start |
| LIFE-002 | Stop |
| LIFE-003 | Restart fresh generation |
| LIFE-004 | Stop != EStop |
| LIFE-005 | Recovery fresh execution |

### 25.6 Permission Tests

| 编号 | 说明 |
|------|------|
| PERM-C001 | declaration != grant |
| PERM-C002 | default deny |
| PERM-C003 | scope |
| PERM-C004 | revoke |

### 25.7 Secret Tests

| 编号 | 说明 |
|------|------|
| SEC-C001 | SecretRef |
| SEC-C002 | Lease scoped |
| SEC-C003 | Restart old lease invalid |
| SEC-C004 | no SecretValue contract |

### 25.8 Control Tests

| 编号 | 说明 |
|------|------|
| CTRL-C001 | ControlMode enum |
| CTRL-C002 | Epoch exact match |
| CTRL-C003 | stale epoch deny |
| CTRL-C004 | takeover no ACK |
| CTRL-C005 | release revalidation |
| CTRL-C006 | EStop suspended |
| CTRL-C007 | final OutputGate |

### 25.9 Cursor Tests

| 编号 | 说明 |
|------|------|
| CUR-C001 | opaque |
| CUR-C002 | valid resume |
| CUR-C003 | expired bounded failure |
| CUR-C004 | no infinite durable replay guarantee |

### 25.10 Binary Tests

| 编号 | 说明 |
|------|------|
| BIN-C001 | opaque reference |
| BIN-C002 | no path exposure |
| BIN-C003 | ownership |
| BIN-C004 | generation invalidation |

### 25.11 Error Tests

| 编号 | 说明 |
|------|------|
| ERR-C001 | stable code |
| ERR-C002 | message not contract |
| ERR-C003 | unknown code safe |
| ERR-C004 | no internal stack |

### 25.12 Compatibility Tests

| 编号 | 说明 |
|------|------|
| COMP-001 | baseline v1 plugin → current Host |
| COMP-002 | current SDK → v1 Host fixture |
| COMP-003 | old manifest fixture → current Kernel |
| COMP-004 | old wire fixture → current decoder |
| COMP-005 | additive optional field |
| COMP-006 | unknown optional capability |

---

## 二十六、P2 观察项（非阻塞）

### 观察 1: Go SDK `HelloResponse` 类型可见性

`HelloResponse` 是 exported 类型但仅出现在 `Runner` 内部。建议要么公开文档化，要么标记为 Internal。

### 观察 2: TS SDK `ExtensionRuntime` 抽象类

提供 `OnActivate/OnDeactivate` 抽象方法，但主流消费模式为 `defineExtension({ activate, deactivate })` 函数式 API。建议标记 Experimental 或 Internal。

### 观察 3: TS SDK `bootstrapExtension` 中 event 传输路径

底层调用 `hostBridge.sendMessage("__event__:${type}", ...)`，使用了约定而非类型化的 event 传输路径。建议补充 wire 层 event 传输规范文档。

---

## 二十七、最终结论

### G41 Contract Freeze — PASS

Amitia Game Mode 的 Protocol v1 公共契约已正式冻结。从 G41 完成开始：

1. **第三方 Game Plugin 可以长期依赖被冻结的公开契约**
2. **Amitia Host 内部可以继续重构实现细节**（RuntimeManager、ProcessSupervisor wiring、DI、存储等）
3. **但不能无版本地破坏插件兼容性**

### 冻结边界

```
Internal implementation
        ↓
Can change

Public frozen contract
        ↓
Must remain compatible
        ↓
Third-party Game Plugin
```

### 版本兼容矩阵

| Protocol | Host Support | Go SDK | TS SDK | Status |
|----------|-------------|--------|--------|--------|
| `amitia-game-host/1` | yes | v1.0.0 | v1.0.0 | **stable** |

| Manifest | game_plugin | desktop_pet_plugin | Status |
|----------|-------------|-------------------|--------|
| V2 | yes | yes | **stable** |

---

**Gate 负责人：** CatPaw (AI Agent)

**Gate 通过时间：** 2026-08-13

**签名：** G41-CONTRACT-FROZEN-20260813
