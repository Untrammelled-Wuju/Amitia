# Amitia 扩展系统重构第 39 步实施文档

## 第 39 步：实现 Trusted Service Runtime

---

## 一、步骤目标

为确实需要原生二进制、长期本地服务、特殊硬件或高性能能力的扩展实现受信任 Service Runtime。

该 Runtime 属于高风险能力，不是普通插件默认选项。

目标：

```text
Trusted Publisher
→ Explicit Service Contribution
→ High-risk Confirmation
→ Platform Binary Verification
→ Process Supervisor
→ Internal RPC
→ Host API Gateway
→ Resource Limits
```

---

## 二、适用范围

仅适用于：

-本地模型侧车；
-音视频编解码服务；
-硬件驱动桥；
-高性能索引；
-特殊协议服务；
-无法由 JavaScript/WASM/MCP 合理实现的能力。

不适用于普通 Tool 或简单网络请求。

---

## 三、信任要求

默认要求：

-Official 或 Trusted Publisher；
-有效签名；
-平台二进制完整性；
-明确许可证；
-高风险权限确认；
-用户可见；
-不可静默自动安装。

Unknown Publisher 默认拒绝 Service Runtime。

---

## 四、Service Definition

```go
type ServiceRuntimeDefinition struct {
    RuntimeID       string
    Executables     []PlatformExecutable
    Protocol        string
    InstancePolicy  string
    Limits          RuntimeResourceLimits
    HealthCheck     ServiceHealthCheck
    Recovery        RuntimeRecoveryPolicy
    Shutdown        ServiceShutdownPolicy
}
```

---

## 五、平台二进制

每个平台/架构独立声明：

-路径；
-Hash；
-入口；
-参数模板；
-最低系统版本；
-签名；
-依赖库；
-许可证。

---

## 六、启动命令

禁止 Shell 字符串。

使用：

```text
executable + args array
```

参数由宿主模板化，禁止任意用户输入拼接。

---

## 七、环境变量

只注入：

-Session；
-受控配置；
-Secret Lease；
-临时目录；
-日志级别。

不继承宿主完整环境。

---

## 八、工作目录

使用 Extension 私有运行目录或临时目录。

不得使用用户主目录作为默认工作目录。

---

## 九、通信

优先使用内部 JSON-RPC：

-stdio；
-本地 Pipe；
-Unix Socket。

禁止默认监听公网端口。

如必须本地端口：

-随机端口；
-loopback；
-Session Token；
-防其他本地进程调用；
-生命周期绑定。

---

## 十、进程监督

Runtime Supervisor 管理：

-启动；
-PID；
-进程组；
-子进程；
-Health；
-资源；
-停止；
-重启；
-崩溃；
-Quarantine。

---

## 十一、子进程

Service 默认不允许再任意创建子进程。

如确需：

-Manifest 声明；
-进程数量限制；
-进程树归属；
-停止时完整清理。

---

## 十二、网络

Service 网络权限仍需 Permission。

理想方案使用 OS Sandbox；第一阶段至少：

-声明域名/端口；
-审计；
-代理入口；
-防公网监听；
-用户确认。

不能把原生进程视为可完全技术隔离，必须在产品上标记风险。

---

## 十三、文件

只允许：

-包内只读；
-Extension Storage；
-授权 Root；
-临时目录。

原生进程技术上可能具备更大能力，因此仅对 Trusted Publisher 开放，并尽可能使用平台 Sandbox。

---

## 十四、平台 Sandbox

规划：

### Windows

-Job Object；
-受限 Token/AppContainer 可行性评估；
-防子进程逃逸；
-防控制台窗口。

### macOS

-Sandbox Profile；
-Hardened Runtime；
-Code Signing；
-进程组。

### Linux

-Namespaces；
-seccomp；
-cgroup；
-bubblewrap 等可行性评估。

第一版必须明确“已实现的隔离”和“未实现的隔离”，不得虚假宣传完全沙箱。

---

## 十五、Health Check

支持：

-RPC Ping；
-进程状态；
-自检；
-端口；
-事件循环；
-资源。

Health 失败影响 Circuit 和重启。

---

## 十六、更新

Service 更新必须：

-停止旧进程；
-验证无残留；
-安装新二进制；
-启动新 Generation；
-Health 验证；
-切换 Contribution；
-失败回滚。

---

## 十七、停止

顺序：

1.拒绝新调用。
2.发送 Graceful Shutdown。
3.等待。
4.发送终止信号。
5.终止进程树。
6.关闭 Pipe/Socket。
7.清理临时目录。
8.验证无残留。
9.记录。

---

## 十八、Quarantine

触发：

-签名失效；
-二进制变化；
-无法停止；
-异常子进程；
-高频 Crash；
-监听未声明端口；
-Host API 协议违规；
-资源超限。

---

## 十九、Service Host API

Service 不能获得比 JS Runtime 更直接的 Go Service 访问。

仍通过内部 RPC 和 Host API Gateway。

---

## 二十、日志

捕获 stdout/stderr。

协议与日志通道分离。

限制大小和速率，脱敏。

---

## 二十一、用户界面

安装或启用时明确显示：

-该扩展包含原生服务；
-发布者；
-平台二进制；
-权限；
-网络；
-文件；
-后台运行；
-自启动；
-资源上限；
-卸载清理。

---

## 二十二、测试要求

覆盖：

-各平台启动；
-错误二进制；
-Hash；
-权限；
-工作目录；
-环境；
-Pipe；
-本地端口；
-Health；
-Crash；
-重启；
-进程树；
-无法停止；
-Quarantine；
-更新；
-回滚；
-卸载；
-系统关机；
-资源限制；
-沙箱能力报告。

---

## 二十三、实施任务

1. 定义 Service Runtime。
2. 实现平台 Executable Selector。
3. 实现 Process Supervisor。
4. 实现进程组/树管理。
5. 接入内部 RPC。
6. 实现 Health。
7. 实现 Stop Escalation。
8. 实现网络/文件策略。
9. 评估并接入平台 Sandbox。
10. 接入 Trust Policy。
11. 接入 Lifecycle Update/Rollback。
12. 实现 Quarantine。
13. 改造前端风险提示。
14. 完成跨平台测试。

---

## 二十四、验收标准

1. Service Runtime 不是普通插件默认能力。
2. 只允许可信发布者。
3. 二进制完整性受签名绑定。
4. 禁止 Shell 命令。
5. 环境最小化。
6. 通信本地受认证。
7. 进程树可清理。
8. Health/Restart/Quarantine 可用。
9. 更新回滚可用。
10. 隔离能力真实可解释。
11. 可进入第 40 步 WASM Runtime。

---

## 二十五、执行约束

> Trusted Service Runtime 是高风险原生扩展通道，信任和用户确认是必要条件，不能宣称其与 WASM 或独立 JS Runtime 具有同等强隔离。

禁止：

-Unknown Publisher；
-公网监听默认开启；
-Shell；
-继承全部环境；
-直接 Go Service；
-无法追踪子进程；
-静默后台自启动；
-签名失效仍运行。
