# Amitia 扩展系统重构第 36 步实施文档

## 第 36 步：实现 JavaScript Main Runtime

---

## 一、步骤目标

实现 `amitia-plugin-host` 主 Runtime，使 JavaScript/TypeScript Extension Module 可以长期运行并安全处理 Tool、Hook、Event、Provider、Lifecycle 和轻量后台入口。

---

## 二、组件

```text
Plugin Host Process
├── Bootstrap Reader
├── RPC Client
├── Runtime Auth
├── Module Loader
├── SDK Context
├── Handler Registry
├── Invocation Dispatcher
├── Cancellation Registry
├── Log Bridge
├── Health Reporter
└── Shutdown Coordinator
```

---

## 三、入口协议

插件默认导出：

```ts
interface AmitiaExtensionModule {
  activate(context: ExtensionContext): Promise<void> | void;
  deactivate?(reason: DeactivateReason): Promise<void> | void;
}
```

未导出 `activate` 则启动失败。

---

## 四、Handler Registry

插件只绑定 Manifest 已声明的 Entry。

SDK：

```ts
context.handlers.bind("tool", "get_forecast", handler);
context.handlers.bind("event", "on_message", handler);
context.handlers.bind("hook", "filter_message", handler);
```

绑定未声明 Entry：

```text
entry_not_declared
```

---

## 五、激活流程

1. 验证 Bootstrap。
2. 建立 RPC。
3.认证 Session。
4.获取允许的 Entry 列表。
5.加载主模块。
6.创建冻结 Context。
7.调用 `activate`。
8.校验 Handler 绑定完整性。
9.报告 Ready。

---

## 六、SDK Context

包含受控客户端：

```ts
interface ExtensionContext {
  readonly extension: ExtensionInfo;
  readonly module: ModuleInfo;
  readonly handlers: HandlerBinder;
  readonly host: HostAPIClient;
  readonly storage: StorageClient;
  readonly secrets: SecretReferenceClient;
  readonly logger: Logger;
  readonly lifecycle: RuntimeLifecycle;
}
```

不包含：

-文件路径；
-Node Process；
-数据库；
-Electron；
-RPC Socket；
-Session Token 明文访问；
-其他 Extension 对象。

---

## 七、调用模型

```ts
type InvocationHandler<I, O> =
  (input: I, context: InvocationContext) => Promise<O> | O;
```

InvocationContext：

-Invocation ID；
-Trace；
-Deadline；
-Cancellation Signal；
-Scope 摘要；
-Host API Client；
-Logger；
-幂等键。

---

## 八、Cancellation

每个调用创建 `AbortSignal`。

宿主取消：

```text
RPC cancel
→ AbortController.abort()
```

插件应合作取消。

超时后 Runtime Host 丢弃延迟结果并记录。

---

## 九、并发

Runtime 内不自行无限并发。

Dispatcher 使用 Supervisor 下发的配额：

-全局并发；
-Entry 并发；
-队列；
-优先级；
-取消。

---

## 十、结果序列化

结果必须是 JSON 可序列化数据或 SDK Resource Reference。

禁止：

-函数；
-Class Instance；
-Circular Object；
-Stream 原对象；
-Buffer 无限制；
-Error 原对象；
-句柄对象。

---

## 十一、流式结果

预留：

```text
stream.open
stream.chunk
stream.close
stream.cancel
```

第一版可仅对明确 Tool/Provider Entry 启用。

流有：

-大小；
-速率；
-背压；
-超时；
-取消。

---

## 十二、日志

SDK Logger：

```ts
logger.debug()
logger.info()
logger.warn()
logger.error()
```

自动附加：

-Extension；
-Module；
-Runtime；
-Invocation；
-Trace。

参数脱敏和大小限制。

---

## 十三、模块加载器

只允许：

-主 Entry；
-包内相对模块；
-构建 Bundle；
-`@amitia/sdk` 虚拟模块；
-允许的纯标准库。

禁止：

-绝对路径；
-其他 Extension；
-网络 import；
-原生模块；
-任意动态 require；
-Electron；
-内部 RPC 包。

---

## 十四、代码完整性

启动前 Supervisor 验证：

-Definition Hash；
-Entry Hash；
-Artifact；
-Generation。

Runtime Loader 可再次校验 Entry。

---

## 十五、生命周期入口

支持：

```text
activate
deactivate
onHostSuspend
onHostResume
```

安装、更新、卸载迁移不在 Main Runtime 随意执行。

---

## 十六、Provider

Provider Handler 需支持：

-能力声明；
-请求 Schema；
-流式；
-取消；
-配额；
-错误；
-指标。

不能直接替换 Amitia 内部 Provider Registry。

---

## 十七、Hook

Hook 调用使用短 Deadline。

Runtime 不得在 Hook 内阻塞等待长任务。

---

## 十八、Event

Event Handler 必须使用 Delivery ID 和幂等键。

SDK 可提供 Inbox helper，但最终幂等由宿主记录。

---

## 十九、状态

Runtime 无内存持久性保证。

需要持久状态必须使用 Storage Client。

进程重启后内存全部丢失是正常行为。

---

## 二十、崩溃

-未捕获 Promise；
-进程退出；
-内存错误；
-协议断开；
-死锁 Watchdog；

映射为 Runtime Crash。

Supervisor 决定重启。

---

## 二十一、Watchdog

Plugin Host 定期报告：

-事件循环延迟；
-内存；
-活跃调用；
-队列；
-最后响应；
-日志丢弃。

持续无响应可终止进程。

---

## 二十二、Graceful Shutdown

1.拒绝新调用。
2.取消队列。
3.等待运行中调用。
4.调用 `deactivate`。
5.关闭 Host API Session。
6.发送 stopped。
7.退出。

超时由 Supervisor 强制终止。

---

## 二十三、开发模式

支持：

-Source Map；
-文件监听由宿主负责；
-代际热重载；
-结构化 Console；
-调用 Mock；
-开发错误全栈。

不允许直接修改生产安装目录。

---

## 二十四、测试 Host

提供独立测试运行器：

-加载 Extension；
-Mock Host API；
-模拟 Permission；
-模拟 Scope；
-调用 Entry；
-超时；
-取消；
-崩溃；
-资源限制。

---

## 二十五、协议兼容

Runtime Host 和 Go Core 通过内部 RPC Version 协商。

不兼容时不得启动。

---

## 二十六、实施任务

1. 建立 plugin-host 工程。
2. 实现 Bootstrap。
3. 实现 RPC Client。
4. 实现 Session Auth。
5. 实现 Module Loader。
6. 实现 SDK Context。
7. 实现 Handler Registry。
8. 实现 Invocation Dispatcher。
9. 实现 Cancel/Timeout。
10. 实现结果序列化。
11. 实现日志。
12. 实现 Health/Watchdog。
13. 实现 Shutdown。
14. 实现 Runtime Factory。
15. 接入 Supervisor。
16. 建立测试 Host。
17. 完成跨平台打包。
18. 完成安全和性能测试。

---

## 二十七、验收标准

1. 主 Runtime 可独立启动。
2. 插件只绑定已声明 Entry。
3. Host API 使用受控 Client。
4. 模块加载受限。
5. 调用支持取消和超时。
6. 结果严格序列化。
7. 日志结构化脱敏。
8. 内存状态不作为持久真值。
9. Crash 不影响宿主。
10. Watchdog 可识别卡死。
11. Graceful Stop 可用。
12. Windows/macOS/Linux 验证通过。
13. 可进入第 37 步 Task Runtime。

---

## 二十八、执行约束

> Main Runtime 用于长期、轻量、可取消的扩展入口，不用于任意原生执行和不可控长任务。

禁止：

-直接 fs/net/process；
-未声明 Handler；
-全局无限定时器；
-持久状态只放内存；
-绕过 SDK；
-直接 Console 输出 Secret；
-运行时在线依赖安装。
