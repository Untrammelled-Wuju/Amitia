# Amitia 扩展系统重构第 26 步实施文档

## 第 26 步：实现 Host API Gateway

---

## 一、步骤目标

建立 Extension Runtime 访问 Amitia 宿主能力的唯一 Host API Gateway。

目标：

```text
Runtime Call
→ Runtime Identity
→ Host API Route
→ Schema Validation
→ Permission Broker
→ Scope Manager
→ Rate/Concurrency/Depth
→ Host Service Adapter
→ Result Sanitization
→ Audit
```

禁止 Runtime 直接持有 Repository、Service、数据库、文件路径、Secret 或内部对象。

---

## 二、职责

Gateway 负责：

- Runtime 身份认证；
-API 路由；
-版本；
-Schema；
-Permission；
-Scope；
-Deadline；
-Cancel；
-Trace；
-Depth；
-Rate；
-Concurrency；
-输入限制；
-输出限制；
-脱敏；
-资源句柄；
-错误映射；
-审计；
-兼容版本；
-Capability 查询。

不负责：

- Extension 安装；
-业务 Enabled；
-Runtime 启停；
-Tool 主执行；
-前端页面；
-插件状态持久化细节；
-Secret 内容管理。

---

## 三、API 命名

建议：

```text
host.tool.execute
host.state.get
host.state.cas
host.secret.get
host.resource.open
host.resource.read
host.resource.write
host.event.emit
host.event.subscribe
host.schedule.create
host.schedule.cancel
host.ui.notify
host.character.read
host.conversation.read
host.memory.query
host.provider.invoke
host.desktop.*
```

每个 API 必须版本化。

---

## 四、请求模型

```go
type HostAPICallRequest struct {
    CallID          string
    RuntimeIdentity RuntimeIdentity
    Method          string
    Version         int
    Input           json.RawMessage
    ScopeSnapshotID string
    TraceID         string
    InvocationID    string
    ParentID        string
    Deadline        time.Time
}
```

RuntimeIdentity 只能由 Supervisor 注入。

---

## 五、响应模型

```go
type HostAPICallResult struct {
    Status       string
    Output       json.RawMessage
    Error        *HostAPIError
    ResourceRefs []ResourceReference
    SideEffects  []RecordedSideEffect
    Metadata     map[string]any
}
```

不得返回：

-内部 Service；
-数据库连接；
-文件描述符原始对象；
-Secret 明文日志；
-函数；
-Go 指针；
-未受控路径。

---

## 六、Route Definition

```go
type HostAPIRoute struct {
    Method          string
    Version         int
    InputSchema     json.RawMessage
    OutputSchema    json.RawMessage
    Permission      []PermissionRequirement
    ScopePolicy     HostAPIScopePolicy
    RiskLevel       RiskLevel
    SideEffectLevel SideEffectLevel
    RatePolicy      RatePolicy
    Timeout         time.Duration
    Handler         HostAPIHandler
}
```

Handler 只由宿主注册。

---

## 七、权限

每个 API 路由映射明确 Permission。

例如：

```text
host.state.get → extension.storage.read
host.state.cas → extension.storage.write
host.secret.get → extension.secret.read:<secret-ref>
host.tool.execute → tool.invoke:<tool-id>
host.desktop.input → desktop.input.control
```

不得使用笼统：

```text
host_api_access
```

授予全部能力。

---

## 八、Scope

Gateway 必须校验：

- Runtime 所属 Extension/Module；
-父 Invocation Scope；
-请求 Scope；
-目标资源 Scope；
-角色；
-会话；
-用户；
-Extension Storage Namespace。

子调用 Scope 只能收窄。

---

## 九、Host API Session

Runtime 启动时创建 Session：

```text
session_id
runtime_identity
generation
allowed_api_versions
created_at
expires_at
```

Runtime Stop 后 Session 立即失效。

旧 Generation Session 不可继续调用。

---

## 十、API 版本

支持：

```text
method + version
```

升级策略：

-兼容新增字段；
-破坏性变更新版本；
-旧版本有弃用期；
-Extension Manifest 声明需要版本；
-Host 启动前检查兼容。

---

## 十一、错误模型

统一：

```text
method_not_found
version_unsupported
identity_invalid
generation_stale
permission_denied
scope_denied
approval_required
input_invalid
output_invalid
rate_limited
timeout
cancelled
resource_not_found
state_conflict
host_unavailable
internal_error
```

---

## 十二、Tool 调用

Plugin 内调用 Tool：

```text
host.tool.execute
→ ExecutionSecurityKernel
```

必须创建子 Invocation。

不得直接调用 Runtime Adapter。

---

## 十三、State API

只通过 PluginStateBroker：

```text
host.state.get
host.state.cas
host.state.delete
host.state.list
```

必须绑定 Namespace、Scope 和 Owner。

---

## 十四、Secret API

只返回最小生命周期 Secret Handle 或受控值。

要求：

-按 Secret ID；
-用途绑定；
-不允许列出全部 Secret；
-不允许导出不可导出 Secret；
-内存生命周期短；
-日志脱敏；
-调用审计；
-权限撤销即时生效。

---

## 十五、Resource Handle

文件、连接、窗口等使用 Host Handle：

```text
handle_id
owner_runtime
resource_type
scope
expires_at
capabilities
```

Runtime 不接触宿主真实路径或对象。

---

## 十六、文件 API

必须基于授权 Root：

```text
host.resource.open
host.resource.read
host.resource.write
host.resource.close
```

防止：

-路径穿越；
-符号链接逃逸；
-跨 Extension；
-访问 Secret；
-超量读写；
-句柄泄漏。

---

## 十七、Event API

Runtime 发事件：

```text
host.event.emit
```

必须：

-Schema；
-权限；
-Scope；
-Depth；
-Rate；
-审计。

订阅通过 Manifest Contribution，不允许 Runtime 任意订阅所有事件。

---

## 十八、Schedule API

动态创建 Schedule 仅允许 Manifest 声明的范围。

必须：

-Owner；
-Entry；
-Scope；
-Permission；
-最大数量；
-Recurrence 限制；
-卸载清理。

---

## 十九、UI API

第一阶段仅支持受控通知、状态和 Contribution 通信。

禁止 Runtime 任意 DOM 注入。

---

## 二十、Character/Conversation API

返回最小必要 DTO。

禁止默认返回：

-完整记忆；
-全部聊天历史；
-其他角色；
-系统 Prompt；
-Secret；
-内部数据库字段。

---

## 二十一、Memory API

如开放：

-明确 Permission；
-明确角色 Scope；
-查询大小；
-结果数量；
-敏感字段；
-写入副作用；
-审计。

---

## 二十二、Desktop API

属于高风险分类：

```text
desktop.window.*
desktop.clipboard.*
desktop.notification.*
desktop.input.*
desktop.screen.*
desktop.process.*
```

每项单独 Permission，不提供万能 Desktop 权限。

---

## 二十三、调用深度

Host API 子 Tool、Event、Workflow 调用必须增加 Depth。

防止 Runtime 循环调用宿主后回到自身。

---

## 二十四、限流

按：

- Runtime；
-Method；
-Extension；
-角色；
-会话；
-资源；
-风险。

高频日志、状态读取与高风险写操作使用不同策略。

---

## 二十五、缓存

只缓存：

- Route；
-Schema；
-Permission Definition；
-静态 Capability。

Permission Decision、Scope、Secret 不长期缓存，或使用 Generation。

---

## 二十六、审计

记录：

- Runtime；
-Method；
-目标；
-Permission；
-Scope；
-结果；
-耗时；
-副作用；
-拒绝；
-错误；
-资源 Handle。

不记录 Secret 内容。

---

## 二十七、SDK Contract

Host API 定义应可生成：

- TypeScript 类型；
-JSON Schema；
-SDK Client；
-文档；
-Mock；
-版本兼容测试。

---

## 二十八、测试要求

覆盖：

-身份伪造；
-旧 Generation；
-权限；
-Scope；
-输入输出 Schema；
-Tool 子调用；
-State CAS；
-Secret；
-文件路径；
-Handle；
-Event Depth；
-Schedule 限制；
-Desktop 高风险；
-取消；
-超时；
-限流；
-审计；
-API 版本；
-高并发性能。

---

## 二十九、实施任务

1. 定义 Host API Route。
2. 建立 Route Registry。
3. 实现 Runtime Session。
4. 实现身份校验。
5. 接入 Permission Broker。
6. 接入 Scope Manager。
7. 接入执行安全组件。
8. 实现 Tool API。
9. 实现 State API。
10. 实现 Secret API 边界。
11. 实现 Resource Handle。
12. 实现文件 API。
13. 实现 Event/Schedule 边界。
14. 实现 Character/Conversation 最小 API。
15. 建立 Desktop 高风险 API 分类。
16. 实现版本和弃用。
17. 生成 SDK Schema。
18. 迁移 Legacy Go Plugin 直接 Service 调用。
19. 完成安全测试。

---

## 三十、验收标准

1. Runtime 访问宿主只有 Gateway。
2. 身份不可自报。
3. 每个 Route 有 Schema、Permission、Scope。
4. 子调用不扩大 Scope。
5. Tool 调用经过 ExecutionSecurityKernel。
6. State 经过 Broker。
7. Secret 不经普通状态或日志。
8. 文件使用授权 Root 和 Handle。
9. Event/Schedule 受声明限制。
10. Desktop API 按能力拆分。
11. API 可版本化和生成 SDK。
12. 旧 Plugin 直接 Service 调用已有迁移报告。
13. 关键安全测试通过。
14. 可进入第 27 步 Storage/Secret Broker。

---

## 三十一、执行约束

> Host API Gateway 是 Runtime 与宿主之间唯一受控边界。

禁止：

- Runtime 持有 Repository；
-传递 Go Service；
-万能 Host 权限；
-插件自报 Extension ID；
-返回真实 Secret 到日志；
-文件 API 接受任意绝对路径；
-动态订阅全部事件；
-绕过 Gateway 的兼容后门。
