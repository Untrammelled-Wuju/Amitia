# Amitia 扩展系统重构第 27 步实施文档

## 第 27 步：实现插件存储与 Secret Broker

---

## 一、步骤目标

建立 Extension Kernel 统一 Storage Broker 与 Secret Broker，使所有 Runtime、Module 和 Contribution 使用隔离命名空间、版本化状态、配额、CAS、备份、迁移和受控 Secret 引用。

目标：

```text
Runtime
→ Host API Gateway
→ Storage/Secret Broker
→ Namespace/Owner/Scope/Permission
→ Encrypted or Structured Storage
→ Audit
```

---

## 二、职责分离

### Storage Broker

负责：

-结构化 KV；
-文档状态；
-Module Namespace；
-角色/会话 Scope；
-CAS；
-事务；
-配额；
-备份；
-迁移；
-导出；
-删除与保留；
-审计摘要。

### Secret Broker

负责：

-Secret 创建；
-引用；
-加密；
-读取；
-轮换；
-撤销；
-共享；
-导出策略；
-最小暴露；
-审计。

不得混用普通 Storage 保存 Secret。

---

## 三、Storage 接口

```go
type ExtensionStorageBroker interface {
    Get(ctx context.Context, request StorageGetRequest) (StorageValue, error)
    CompareAndSwap(ctx context.Context, request StorageCASRequest) (StorageValue, error)
    Delete(ctx context.Context, request StorageDeleteRequest) error
    List(ctx context.Context, request StorageListRequest) (StoragePage, error)
    Transaction(ctx context.Context, request StorageTransactionRequest) StorageTransactionResult
    Usage(ctx context.Context, owner ResourceOwner) StorageUsage
}
```

---

## 四、命名空间

规范：

```text
extensions/<extension-id>/modules/<module-id>/<scope-type>/<scope-id>/<namespace>
```

Runtime 只提交逻辑 Namespace，不提交物理路径。

---

## 五、Scope

支持：

```text
global
character
conversation
extension
module
runtime
```

Conversation Namespace 必须验证 Character 归属。

---

## 六、状态值

```go
type StorageValue struct {
    Key       string
    Version   int64
    Value     json.RawMessage
    Hash      string
    SizeBytes int64
    UpdatedAt time.Time
}
```

---

## 七、CAS

所有并发写建议使用 ExpectedVersion。

冲突：

```text
storage_version_conflict
```

不得自动无限重试非幂等业务。

---

## 八、事务

支持同 Namespace 内小型原子事务。

禁止跨：

-多个 Extension；
-Secret Store；
-外部系统；
-文件系统大型资源；

伪装成单数据库事务。

---

## 九、配额

按：

- Extension；
-Module；
-Namespace；
-Scope；
-单值；
-总量；
-写入频率。

超限返回明确错误，不自动删除用户数据。

---

## 十、数据分类

```text
configuration
state
cache
user_data
derived
temporary
```

删除策略不同：

- cache/derived 可重建；
-configuration 可保留；
-user_data 默认保护；
-temporary 自动清理。

---

## 十一、Schema

Module 可声明 Storage Schema：

-键模式；
-值 Schema；
-版本；
-迁移；
-敏感等级；
-保留策略。

未知键是否允许必须明确。

---

## 十二、数据迁移

更新 Extension 时：

```text
old schema
→ migration plan
→ backup
→ idempotent migration
→ validation
→ commit
```

不可逆迁移必须确认。

---

## 十三、备份与回滚

Snapshot 包含：

- Namespace；
-Version；
-Hash；
-Schema Version；
-Owner；
-引用。

不包含 Secret 明文。

---

## 十四、Secret 模型

```go
type SecretRecord struct {
    SecretID       string
    Owner          ResourceOwner
    Type           string
    Purpose        string
    Scope          ScopeRef
    ExportPolicy   string
    RotationPolicy string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    RevokedAt      *time.Time
}
```

Secret 内容单独加密存储。

---

## 十五、Secret 接口

```go
type SecretBroker interface {
    Create(ctx context.Context, request SecretCreateRequest) (SecretReference, error)
    Resolve(ctx context.Context, request SecretResolveRequest) (SecretLease, error)
    Rotate(ctx context.Context, request SecretRotateRequest) error
    Revoke(ctx context.Context, secretID string) error
    Transfer(ctx context.Context, request SecretTransferRequest) error
    ListMetadata(ctx context.Context, query SecretQuery) ([]SecretRecord, error)
}
```

---

## 十六、Secret Reference

Definition、MCP、Runtime 只保存：

```text
secret_id
purpose
```

不得保存明文。

---

## 十七、Secret Lease

读取返回短生命周期 Lease：

```text
lease_id
secret_id
runtime_id
purpose
expires_at
```

Lease 到期失效。

---

## 十八、用途绑定

Secret 创建时声明用途：

```text
mcp_oauth
api_key
http_header
environment
provider_token
certificate
private_key
```

Runtime 不能把某用途 Secret 用于任意其他 API。

---

## 十九、共享 Secret

共享必须显式：

-Owner；
-引用者；
-用途；
-Scope；
-撤销影响；
-删除阻塞。

单个 Extension 卸载只解除引用。

---

## 二十、加密

要求：

-使用平台安全存储或成熟加密库；
-主密钥不存普通数据库；
-密钥轮换；
-完整性校验；
-随机 Nonce；
-避免自行设计密码学协议。

---

## 二十一、日志与错误

Secret 内容永不进入：

-日志；
-审计；
-错误；
-前端；
-Metrics；
-运行记录。

只记录 Secret ID 摘要与动作。

---

## 二十二、环境变量 Secret

启动 Runtime 时按需注入。

要求：

-最小集合；
-不打印；
-停止后释放；
-子进程继承范围受控；
-不写入 Command Preview；
-不返回前端。

---

## 二十三、OAuth

OAuth Token 由 Secret Broker 保存。

MCP OAuth Provider 只使用 Reference 和 Lease。

---

## 二十四、卸载

Extension 卸载：

-私有 Secret 删除或按策略保留；
-共享 Secret 解除引用；
-用户 Secret 保留；
-Storage 按数据分类处理；
-用户资产需确认；
-Cache 删除；
-审计保留。

---

## 二十五、导入导出

Storage 导出：

-结构化；
-版本；
-Schema；
-Hash；
-可选择用户数据。

Secret 默认不导出。

允许导出时必须二次确认和加密。

---

## 二十六、前端

Storage 页面显示：

-使用量；
-Namespace；
-数据分类；
-保留策略；
-迁移；
-清理。

Secret 页面只显示：

-名称；
-用途；
-Owner；
-引用；
-状态；
-轮换；
-撤销。

不显示明文。

---

## 二十七、持久化

建议：

```text
extension_storage_namespaces
extension_storage_entries
extension_storage_versions
extension_storage_quotas
extension_storage_snapshots
extension_storage_migrations
extension_secret_metadata
extension_secret_references
extension_secret_leases
extension_secret_rotation_records
```

Secret 密文使用专用安全后端。

---

## 二十八、测试要求

覆盖：

- Namespace 隔离；
-跨 Extension 拒绝；
-角色/会话；
-CAS；
-事务；
-配额；
-Schema；
-迁移；
-备份；
-回滚；
-用户数据保护；
-Secret 创建；
-Lease；
-用途；
-轮换；
-撤销；
-共享；
-卸载；
-日志脱敏；
-并发；
-损坏密文；
-平台安全存储。

---

## 二十九、实施任务

1. 定义 Storage Broker。
2. 实现 Namespace Resolver。
3. 实现 Scope 校验。
4. 实现 KV/CAS。
5. 实现事务与配额。
6. 实现数据分类。
7. 实现 Schema 与 Migration。
8. 实现 Snapshot/Restore。
9. 定义 Secret Broker。
10. 接入平台安全存储。
11. 实现 Secret Reference/Lease。
12. 实现用途绑定。
13. 实现共享与引用图。
14. 实现轮换与撤销。
15. 接入 Host API Gateway。
16. 接入 MCP OAuth/Headers/Env。
17. 接入 Lifecycle Update/Uninstall。
18. 迁移旧 Plugin State 和 Secret。
19. 改造前端。
20. 完成安全测试。

---

## 三十、验收标准

1. Runtime 不直接访问数据库。
2. 所有状态使用隔离 Namespace。
3. 支持 CAS 和配额。
4. 用户数据与 Cache 分类。
5. 更新数据迁移可回滚或明确不可逆。
6. Secret 不存普通 Storage。
7. Secret 只通过 Reference/Lease。
8. Secret 用途绑定。
9. 共享 Secret 有引用图。
10. 卸载不误删用户 Secret。
11. 日志无 Secret。
12. MCP 与 Runtime 已接入。
13. 关键安全测试通过。
14. 可进入第 28 步 Event Bus/Hook Pipeline。

---

## 三十一、执行约束

> Storage Broker 管理扩展数据，Secret Broker 管理凭据；两者必须分离。

禁止：

-插件建表；
-插件拼物理路径；
-Secret 写 KV；
-列出全部 Secret 内容；
-长期 Lease；
-卸载直接删共享 Secret；
-明文导出；
-新旧 State 双写。
