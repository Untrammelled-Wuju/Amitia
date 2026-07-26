# Amitia 扩展系统重构第 35 步实施文档

## 第 35 步：确定桌面插件 Runtime 技术方案

---

## 一、步骤目标

在 Extension Kernel、`.amitiax` v2、包事务和 Runtime Supervisor 已定义的基础上，正式确定 Amitia 桌面第三方插件的主 Runtime 技术路线。

本步骤直接作出架构决定：

> Amitia 第三方主插件 Runtime 采用“独立 Node.js 子进程 + TypeScript/JavaScript 插件 SDK + 自定义受控模块加载器 + 内部 JSON-RPC + Host API Gateway”的方案。

不采用以下方案作为主 Runtime：

- Electron Renderer 直接执行插件；
-Electron Main Process 直接 `require` 插件；
-Go 进程动态加载第三方 Go Plugin；
-仅使用 iframe 作为插件执行环境；
-仅使用 WASM 作为全部插件能力；
-在宿主进程内使用弱隔离 JavaScript VM 作为主要安全边界。

---

## 二、为什么选择独立 Node.js Runtime

核心原因：

1. 与 Electron/Vue/TypeScript 技术栈一致。
2. 插件开发者生态成熟。
3. 支持 TypeScript SDK。
4. 支持异步任务、流式调用和模块化。
5. 支持 Windows、macOS、Linux。
6. 可以独立进程崩溃隔离。
7. 可以通过 IPC 完全阻断插件直接访问 Go 内部服务。
8. 可以独立设置工作目录、环境变量、资源限制和进程组。
9. 可以逐步扩展调试、热重载和测试工具。
10. 可将主 Runtime、Task Runtime 和 UI Runtime 分离。

---

## 三、为什么不直接复用 Electron Renderer

禁止把插件主逻辑直接放在 Electron Renderer 中，原因：

-Renderer 与 UI 生命周期耦合；
-窗口刷新会中断 Runtime；
-容易错误开放 Node Integration；
-容易产生 DOM、文件系统和 Electron API 越权；
-难以稳定管理后台任务；
-难以独立限制内存和 CPU；
-崩溃影响界面；
-多个 UI Contribution 容易互相污染。

UI Runtime 与主插件 Runtime 必须分离。

---

## 四、为什么不在 Electron Main 执行

Electron Main 是桌面宿主关键进程。

第三方代码进入 Main 会产生：

-宿主完全控制；
-进程崩溃；
-文件和系统无限访问；
-无法可靠撤销权限；
-无法进行资源限制；
-无法卸载清理；
-无法阻止插件调用 Electron 内部对象。

因此：

```text
Electron Main
≠
Plugin Runtime
```

---

## 五、为什么不使用动态 Go Plugin

Go `plugin` 机制不适合作为跨平台第三方插件基础：

-平台支持不一致；
-ABI 和 Go 版本耦合；
-无法安全卸载；
-第三方代码进入宿主地址空间；
-崩溃与内存破坏影响核心；
-权限隔离困难；
-Windows 支持问题；
-发布和构建复杂。

现有 Go Plugin 只能作为 `legacy_go` 过渡 Runtime。

---

## 六、为什么不只用 WASM

WASM 适合：

-纯计算；
-确定性转换；
-资源受限算法；
-跨平台模块。

但不适合作为全部插件主 Runtime：

-异步宿主 API 复杂；
-生态与开发体验弱于 TypeScript；
-桌面、网络、流式、长期任务需要大量 Host Binding；
-复杂 UI/Provider/MCP 编排不自然。

因此 WASM 是补充 Runtime，第 40 步单独实现。

---

## 七、总体架构

```text
Amitia Go Core
├── Extension Kernel
├── Runtime Supervisor
├── Host API Gateway
├── Permission / Scope
├── Storage / Secret
└── Internal RPC Server
        │
        │ framed JSON-RPC
        ▼
Amitia Plugin Host
├── Runtime Bootstrap
├── Module Loader
├── SDK Bridge
├── Invocation Dispatcher
├── Cancellation
├── Structured Logs
└── Extension Main Module
```

---

## 八、运行进程

建议发布独立运行器：

```text
amitia-plugin-host
```

每个 Extension Runtime Module 默认一个独立子进程。

不直接复用系统 Node。

不依赖用户安装 Node。

Amitia 安装包内附带受控 Runtime。

---

## 九、进程粒度

默认：

```text
singleton_per_module
```

即一个需要 JavaScript Runtime 的 Module 对应一个插件宿主子进程。

可配置：

```text
singleton_per_extension
pool
per_invocation
```

但第一版只正式支持 `singleton_per_module`。

---

## 十、主 Runtime 与 Task Runtime 分离

### Main Runtime

长期运行，处理：

-Tool；
-Hook；
-Event；
-Provider；
-状态；
-轻量后台逻辑。

### Task Runtime

短期或隔离运行，处理：

-数据迁移；
-长计算；
-批量导入；
-可取消后台任务；
-一次性构建。

禁止在 Main Runtime 执行不可控长任务。

---

## 十一、模块加载

使用 Amitia 自定义模块加载器。

允许：

-包内相对模块；
-Manifest 声明的入口；
-构建产物；
-SDK 虚拟模块；
-受控标准库子集。

默认禁止：

-任意绝对路径；
-任意 `require`；
-动态下载模块；
-访问其他 Extension；
-直接加载原生 `.node` 模块；
-直接访问 Electron；
-直接访问 Go IPC；
-运行 Shell。

---

## 十二、Node 标准库策略

不能依赖“禁止所有标准库”作为唯一安全策略，也不能默认开放全部。

建议分类：

### 可直接使用的纯计算模块

按白名单逐步开放。

### 通过 Host API 替代

-文件；
-网络；
-进程；
-系统；
-Secret；
-数据库；
-桌面；
-计时 Schedule。

### 默认拒绝

-`child_process`；
-任意 `fs`；
-任意 `net`；
-`dgram`；
-`worker_threads` 未受控创建；
-`vm`；
-原生模块加载；
-调试端口自行开启。

即使 Runtime 本身可以绕过加载器，最终安全边界仍依赖独立进程、最小权限 OS 账户策略预留、Host API 和不提供 Secret/路径。

---

## 十三、网络访问

插件不直接使用 Node 网络 API。

统一使用：

```text
host.network.request
```

由 Host API Gateway 执行：

-域名约束；
-方法；
-Header；
-Secret；
-代理；
-TLS；
-响应大小；
-审计；
-取消。

---

## 十四、文件访问

插件不直接使用任意 `fs`。

统一使用：

```text
host.resource.*
host.storage.*
```

包内只读资源可由 Runtime Loader 提供受控读取。

---

## 十五、依赖打包

生产 `.amitiax` 不允许运行时 `npm install`。

推荐构建方式：

-将依赖 Bundle 到 `dist`；
-保留 License；
-生成 Dependency Snapshot；
-禁止动态原生依赖；
-禁止远程解析。

---

## 十六、SDK

插件入口示例：

```ts
import { defineExtension } from "@amitia/sdk";

export default defineExtension({
  async activate(context) {
    context.tools.registerHandlers({
      get_forecast: async (input, call) => {
        return { city: input.city };
      },
    });
  },

  async deactivate() {}
});
```

注意：

-`registerHandlers` 只绑定 Manifest 已声明 Entry；
-不能动态创建未声明 Tool；
-Context 是受控 SDK 客户端；
-不能访问内部 IPC。

---

## 十七、Runtime Bootstrap

启动流程：

```text
Process Start
→ Read Runtime Bootstrap Spec
→ Open RPC Channel
→ Authenticate Runtime Session
→ Verify Definition Hash/Generation
→ Initialize SDK
→ Load Entry Module
→ Call activate
→ Report Ready
```

---

## 十八、Runtime Bootstrap Spec

只通过安全 IPC 传入：

-Instance ID；
-Extension ID；
-Module ID；
-Definition Hash；
-Generation；
-Entry；
-Host API Version；
-资源限制；
-日志策略；
-开发模式；
-短期 Session Token。

不通过环境变量传递大段敏感配置。

---

## 十九、激活与停用

插件可实现：

```ts
activate(context)
deactivate(reason)
```

`activate` 不得注册 Manifest 未声明 Contribution。

`deactivate` 失败不能阻止宿主强制清理资源。

---

## 二十、错误隔离

未捕获异常：

-捕获并结构化；
-当前 Invocation 失败；
-高频异常影响 Health/Circuit；
-进程未必立即退出；
-进程级错误触发 Runtime Crash。

---

## 二十一、资源限制

建议初始默认：

-内存上限；
-最大并发；
-队列深度；
-单次调用超时；
-日志速率；
-Host API 速率；
-打开句柄数；
-消息大小。

具体数值由产品测试确定，不写死在 Manifest 标准中。

---

## 二十二、调试与生产隔离

开发模式支持：

-Source Map；
-热重载；
-调试日志；
-测试 Host；
-受控 Debug Port。

生产模式：

-不开调试端口；
-不读取源码目录；
-不允许未签名热替换；
-不显示 Secret；
-错误摘要化。

---

## 二十三、Runtime 升级策略

Amitia 可升级内置 Node Runtime，但必须：

-固定 Runtime API；
-提供兼容矩阵；
-插件声明 Host API 版本；
-建立 SDK Compatibility Test；
-不能让系统 Node 版本成为隐式契约。

---

## 二十四、分发

`amitia-plugin-host` 随 Amitia 桌面安装：

```text
runtime/node/<platform-arch>/
```

由 Runtime Supervisor 启动，不暴露给插件选择任意可执行文件。

---

## 二十五、最终决定

主路线：

```text
独立 Node.js 子进程
+ TypeScript SDK
+ 自定义模块加载器
+ 内部 JSON-RPC
+ Host API Gateway
+ 每 Module 独立实例
```

补充路线：

```text
Task Runtime
Trusted Service Runtime
WASM Runtime
Restricted UI Runtime
```

---

## 二十六、实施任务

1. 锁定 Runtime ADR。
2. 定义插件 Host 进程边界。
3. 定义模块加载策略。
4. 定义 SDK 入口。
5. 定义 Bootstrap Spec。
6. 定义 Main/Task 分离。
7. 定义进程粒度。
8. 定义标准库策略。
9. 定义网络/文件替代 API。
10. 定义打包规则。
11. 定义开发/生产差异。
12. 建立威胁模型。
13. 建立跨平台原型。
14. 输出性能基线。

---

## 二十七、验收标准

1. 主 Runtime 技术路线已锁定。
2. 第三方代码不进入 Go/Electron Main。
3. 不依赖用户 Node。
4. Main 与 Task 分离。
5. 文件、网络、进程通过 Host API。
6. Manifest 声明与 Handler 绑定分离。
7. 进程级崩溃隔离成立。
8. SDK 和 RPC 边界明确。
9. 跨平台原型可启动和停止。
10. 可进入第 36 步 Main Runtime。

---

## 二十八、执行约束

> Node.js 只是插件语言和执行引擎，不是权限系统；真正安全边界由独立进程、Runtime Supervisor、Host API Gateway、Permission、Scope 和 Resource Ownership 共同组成。

禁止：

-直接复用 Electron Main；
-直接开放 Node Integration；
-系统 Node 依赖；
-在线 npm install；
-任意 child_process；
-任意 fs/net；
-第三方 `.node`；
-插件自建 IPC 后门。
