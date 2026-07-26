# Amitia 扩展系统重构第 56 步实施文档

## 第 56 步：实现 TypeScript Plugin SDK

---

## 一、步骤目标

在第 35—40 步完成 JavaScript Main Runtime、Task Runtime、内部 JSON-RPC、Trusted Service Runtime 和 WASM Runtime，第 41—48 步完成 UI Contribution 体系，第 49—55 步完成正式迁移准备后，实现面向扩展开发者的 TypeScript Plugin SDK。

本步骤目标是：

> 为 `.amitiax` 扩展开发提供唯一、稳定、类型安全、版本化、可测试、可生成文档的 TypeScript SDK，使开发者只能通过 SDK 绑定 Manifest 已声明的 Entry、调用 Host API、访问 Storage/Secret、处理 Tool/Event/Hook/Task/UI Action，而不能依赖内部 RPC、Electron、Go Service、数据库或宿主实现细节。

SDK 的核心定位：

```text
Manifest v2
→ 生成类型
→ @amitia/sdk
→ Extension Runtime
→ Internal RPC
→ Host API Gateway
```

---

## 二、SDK 包结构

建议拆分：

```text
@amitia/sdk
@amitia/sdk/runtime
@amitia/sdk/tasks
@amitia/sdk/ui
@amitia/sdk/testing
@amitia/sdk/schema
@amitia/sdk/manifest
@amitia/sdk/devtools
```

### `@amitia/sdk`

开发者默认入口。

### `@amitia/sdk/runtime`

Main Runtime 类型与 Handler。

### `@amitia/sdk/tasks`

Task Runtime。

### `@amitia/sdk/ui`

UI Bridge 和 UI Action 类型。

### `@amitia/sdk/testing`

Mock Host、测试工具。

### `@amitia/sdk/schema`

JSON Schema、类型生成辅助。

### `@amitia/sdk/manifest`

Manifest Builder 和校验类型。

### `@amitia/sdk/devtools`

开发模式诊断接口。

---

## 三、SDK 版本策略

SDK 版本与以下版本分离：

- Amitia 应用版本；
-Manifest Version；
-Host API Version；
-Runtime RPC Version；
-Domain Schema Version。

SDK 发布使用 SemVer。

SDK 必须声明兼容矩阵：

```text
SDK Version
Supported Manifest Version
Supported Host API Range
Supported Runtime RPC Range
Minimum Amitia Version
```

---

## 四、扩展主入口

建议：

```ts
import { defineExtension } from "@amitia/sdk";

export default defineExtension({
  async activate(context) {},
  async deactivate(reason) {},
});
```

`defineExtension` 负责：

-类型约束；
-入口规范；
-生命周期检查；
-开发模式诊断；
-阻止非法导出；
-生成元数据辅助。

不负责：

-安装；
-权限授予；
-动态声明 Contribution；
-启动子进程。

---

## 五、Extension Context

建议：

```ts
export interface ExtensionContext {
  readonly extension: ExtensionIdentity;
  readonly module: ModuleIdentity;
  readonly runtime: RuntimeContext;
  readonly handlers: HandlerBinder;
  readonly host: HostClient;
  readonly storage: StorageClient;
  readonly secrets: SecretReferenceClient;
  readonly resources: ResourceClient;
  readonly events: EventClient;
  readonly logger: Logger;
  readonly lifecycle: RuntimeLifecycleClient;
}
```

Context 必须冻结或提供只读类型。

---

## 六、Handler Binder

开发者只能绑定 Manifest 已声明 Entry：

```ts
context.handlers.bindTool("get_forecast", handler);
context.handlers.bindEvent("on_message", handler);
context.handlers.bindHook("filter_message", handler);
context.handlers.bindProvider("image_provider", handler);
context.handlers.bindUIAction("save_settings", handler);
```

运行时根据 Bootstrap 传入的 Entry Allowlist 校验。

绑定未声明 Entry 必须失败：

```text
entry_not_declared
```

---

## 七、Tool Handler 类型

建议：

```ts
export type ToolHandler<I, O> = (
  input: I,
  context: ToolInvocationContext,
) => Promise<O> | O;
```

`ToolInvocationContext` 包含：

-Invocation ID；
-Trace；
-Deadline；
-AbortSignal；
-Scope Summary；
-Idempotency Key；
-Logger；
-受控 Host Client。

不包含：

-真实用户对象；
-数据库连接；
-宿主 Token；
-内部 RPC；
-文件路径。

---

## 八、Event Handler 类型

```ts
export type EventHandler<T> = (
  event: EventEnvelope<T>,
  context: EventDeliveryContext,
) => Promise<void>;
```

必须支持：

-Delivery ID；
-Idempotency Key；
-AbortSignal；
-重试语义；
-事件版本；
-Scope；
-Trace。

---

## 九、Hook Handler 类型

按 Phase 提供不同类型：

```ts
type BeforeHook<I> = (input: I, ctx: HookContext) => Promise<BeforeHookResult<I>>;
type FilterHook<I> = (input: I, ctx: HookContext) => Promise<FilterHookResult>;
type TransformHook<I, O> = (input: I, ctx: HookContext) => Promise<O>;
type ObserveHook<I> = (input: I, ctx: HookContext) => Promise<void>;
```

SDK 不允许 Transform Hook 返回未声明字段。

---

## 十、Task SDK

示例：

```ts
import { defineTask } from "@amitia/sdk/tasks";

export default defineTask(async (input, context) => {
  context.progress.report({ current: 1, total: 100 });
  await context.checkpoint.save({ cursor: 1 });
  return { success: true };
});
```

Task Context：

-Progress；
-Checkpoint；
-Artifact；
-Storage；
-AbortSignal；
-Logger；
-最小 Host API。

---

## 十一、Host Client

Host Client 只能暴露经过版本化的 Host API。

示例：

```ts
await context.host.tools.execute(toolId, input);
await context.host.network.request(request);
await context.host.messages.get(messageId);
await context.host.desktop.notifications.show(payload);
```

SDK 必须：

-生成输入输出类型；
-自动携带 Session；
-自动传 Trace；
-支持取消；
-映射错误；
-不暴露底层 `host.call` 给普通开发者。

---

## 十二、Storage Client

建议：

```ts
interface StorageClient {
  get<T>(key: string, options?: StorageReadOptions): Promise<StorageValue<T> | null>;
  compareAndSwap<T>(request: StorageCASRequest<T>): Promise<StorageValue<T>>;
  delete(key: string, options?: StorageDeleteOptions): Promise<void>;
  list<T>(query?: StorageListQuery): Promise<StoragePage<T>>;
  transaction<T>(callback: StorageTransactionCallback<T>): Promise<T>;
}
```

要求：

-自动绑定 Extension/Module Namespace；
-开发者不能指定其他 Extension；
-支持 Scope；
-支持版本冲突；
-支持配额错误；
-Secret 值禁止写入普通 Storage。

---

## 十三、Secret Client

SDK 默认只处理 Reference。

```ts
interface SecretReferenceClient {
  create(request: SecretCreateRequest): Promise<SecretReference>;
  use<T>(
    reference: SecretReference,
    purpose: string,
    callback: (lease: SecretLeaseHandle) => Promise<T>,
  ): Promise<T>;
  revoke(reference: SecretReference): Promise<void>;
}
```

普通扩展代码不应长期持有 Secret 明文。

---

## 十四、Resource Client

提供：

-读取包内资源；
-读取 Host Resource Handle；
-写 Result Artifact；
-选择用户文件；
-保存导出；
-关闭 Handle。

不提供真实绝对路径。

---

## 十五、Logger

建议：

```ts
logger.debug(message, fields?)
logger.info(message, fields?)
logger.warn(message, fields?)
logger.error(message, fields?)
```

SDK 应：

-自动附加 Trace；
-字段大小限制；
-检测常见 Secret Key；
-禁止循环对象；
-映射 Error；
-支持开发/生产级别。

---

## 十六、错误类型

统一 SDK Error：

```ts
class AmitiaError extends Error {
  code: string;
  category: string;
  retryable: boolean;
  details?: Record<string, unknown>;
}
```

具体：

```text
PermissionDeniedError
ScopeDeniedError
ValidationError
ConflictError
TimeoutError
CancelledError
DependencyUnavailableError
RuntimeUnavailableError
RateLimitError
StorageConflictError
```

---

## 十七、取消与 Deadline

所有长操作接受：

```ts
AbortSignal
```

SDK 不鼓励自行创建无限 Promise。

Host Client 自动使用 Invocation Deadline。

---

## 十八、流式 API

定义统一：

```ts
AsyncIterable<Chunk>
```

SDK 将内部 RPC Stream 映射为：

```ts
for await (const chunk of stream) {}
```

必须：

-支持取消；
-背压；
-错误；
-最大 Chunk；
-自动关闭；
-退出循环时取消。

---

## 十九、Manifest 类型生成

SDK 提供：

```ts
defineManifest(...)
defineModule(...)
defineTool(...)
defineWorkflow(...)
defineUIContribution(...)
```

但最终 `manifest.json` 仍需要静态生成和 Schema 校验。

不能运行时动态 Manifest。

---

## 二十、Schema 到 TypeScript

CLI 根据 Manifest 中的 Tool/Input/Output Schema 生成：

```text
generated/entries.ts
generated/host-api.ts
generated/events.ts
generated/ui-actions.ts
```

开发者绑定 Handler 时使用生成类型。

---

## 二十一、UI SDK

Restricted Web UI 使用独立、权限更小的 SDK：

```ts
import { createAmitiaUI } from "@amitia/sdk/ui";

const ui = createAmitiaUI();
await ui.ready();
await ui.actions.invoke("save_settings", payload);
```

UI SDK 不包含：

-Storage 直接访问；
-Secret；
-任意 Tool；
-Electron；
-Node；
-内部 RPC。

---

## 二十二、测试 SDK

`@amitia/sdk/testing` 提供：

```ts
createMockHost()
createExtensionTestRuntime()
invokeTool()
emitEvent()
executeHook()
mockPermission()
mockScope()
mockStorage()
mockSecretReference()
advanceTime()
simulateCancellation()
simulateRuntimeCrash()
```

---

## 二十三、Contract Test

扩展包发布前必须通过：

-Manifest Schema；
-Entry 绑定；
-输入输出；
-Permission；
-Scope；
-取消；
-超时；
-Storage；
-Event；
-Hook；
-UI Bridge；
-资源；
-生命周期。

---

## 二十四、兼容检查

SDK 在构建阶段输出：

```text
Required Host API
Required Runtime RPC
Required Manifest Version
Unsupported APIs
Deprecated APIs
```

运行时仍需协商。

---

## 二十五、弃用

SDK API 使用：

-`@deprecated`；
-替代方案；
-移除版本；
-迁移文档；
-CLI Lint；
-运行时 Warning。

不得无期限保留所有旧 API。

---

## 二十六、Tree-shaking 与 Bundle

SDK 应分包，避免将测试和 UI 代码打进 Main Runtime。

构建工具检查：

-Node Builtin；
-未允许依赖；
-动态 import；
-原生模块；
-包大小；
-License。

---

## 二十七、安全设计

SDK 不是安全边界，但应减少误用：

-不导出底层 RPC；
-不导出 Session Token；
-不提供任意 Host Call；
-不提供真实路径；
-不提供 `process`；
-不提供 Electron；
-不提供直接数据库；
-类型层区分 Main/Task/UI。

---

## 二十八、文档生成

从 TypeScript 类型和 Host API 定义生成：

-API 文档；
-权限说明；
-示例；
-错误；
-版本；
-平台；
-风险；
-最佳实践。

---

## 二十九、示例扩展

至少提供：

1. 纯 Tool 扩展；
2. Agent Skill + Tool；
3. Workflow；
4. MCP 集成；
5. Schema UI；
6. Restricted Web UI；
7. Event/Hook；
8. Task；
9. Provider；
10. Desktop Command。

---

## 三十、包发布

建议：

```text
packages/sdk
packages/sdk-ui
packages/sdk-testing
packages/sdk-manifest
```

可使用 monorepo 管理。

---

## 三十一、测试要求

覆盖：

-Context；
-Handler Binding；
-未声明 Entry；
-Tool/Event/Hook；
-Task；
-Host API；
-Storage；
-Secret；
-Resource；
-错误；
-取消；
-Stream；
-UI；
-Mock；
-类型生成；
-版本；
-Deprecated；
-Bundle；
-恶意插件误用；
-Windows/macOS/Linux。

---

## 三十二、实施任务

1. 建立 SDK monorepo。
2. 定义 Extension/Task/UI Context。
3. 实现 Handler Binder。
4. 生成 Host API Client。
5. 实现 Storage/Secret/Resource Client。
6. 实现 Error Mapping。
7. 实现 Cancellation/Streaming。
8. 实现 Manifest Builder 类型。
9. 实现 Schema 类型生成。
10. 实现 UI SDK。
11. 实现 Testing SDK。
12. 建立 Contract Test。
13. 建立文档生成。
14. 建立示例扩展。
15. 接入 Plugin Host。
16. 建立版本兼容矩阵。
17. 发布首个 SDK 版本。
18. 完成安全和开发体验测试。

---

## 三十三、验收标准

1. 开发者只需使用公开 SDK。
2. SDK 不暴露底层 RPC。
3. Handler 只能绑定声明 Entry。
4. Main/Task/UI 类型隔离。
5. Host API 类型化。
6. Storage/Secret/Resource 使用统一 Broker。
7.取消和流式可用。
8.测试 SDK 可离线测试。
9. Manifest/Schema 可生成类型。
10.版本兼容可检查。
11.示例覆盖主要能力。
12.可进入第 57 步 Plugin CLI。

---

## 三十四、执行约束

> SDK 只能包装和约束 Extension Kernel 已提供的稳定能力，不能为了开发便利重新开放内部 Service、任意 RPC、Node/Electron 或未声明的动态 Contribution。

禁止：

-导出 Session Token；
-导出 `host.call(any)`；
-暴露 Repository；
-UI SDK 访问 Main Runtime底层；
-运行时生成 Manifest；
-自动申请权限；
-隐藏 Deprecated；
-测试 Mock 行为与真实协议严重偏离。
