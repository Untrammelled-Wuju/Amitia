# runtime.command.sync_desired_state

Durable 命令，同步 Runtime 期望状态。

## Payload

```json
{
  "desiredRevision": 45,
  "desiredHash": "sha256...",
  "ensureAbsent": false,
  "installationId": "install_xxx",
  "petId": "pet_xxx",
  "characterId": "char_xxx",
  "releaseId": "release_xxx",
  "releaseVersion": "1.2.3",
  "contentRootHash": "sha256...",
  "manifestHash": "sha256...",
  "runtimeContractVersion": "2.0.0",
  "defaultActionKey": "idle",
  "settingsRevision": 12,
  "settingsSnapshot": { ... },
  "resourceSnapshot": { ... }
}
```

## 状态流

```
created → queued → transport_dispatched → runtime_received
→ runtime_accepted → completed
```

## Coalesce

- CoalesceKey: `desired:{deviceID}`
- 只保留最新 DesiredRevision
- 旧命令标 `superseded`

## ACK 规则

- `runtime_received`: Runtime 解包并验证身份/Envelop/Sequence/Dedup 后返回
- `runtime_accepted`: Runtime 校验 Capability/Release/Installation/PayloadHash 后返回
- `completed`: Runtime 应用完成后返回 `runtime.state.desired_applied` 事件

## 错误处理

| 错误 | 处理 |
|------|------|
| Release 不存在 | desired_rejected(event), 等待安装 |
| Hash 不匹配 | desired_rejected(event), 重新获取 |
| Runtime 不兼容 | desired_rejected(event), 拒绝并日志 |
| 应用超时 | Command 失败，等待下次 Reconcile |

## 注意

- 命令发送前必须校验 Runtime 支持 `runtime.sync_desired_v2` 能力
- DesiredRevision 必须来自 DB，禁止随机生成
- Restricted fields: 禁止携带绝对路径
