# Amitia 扩展系统重构第 38 步实施文档

## 第 38 步：实现内部 JSON-RPC 协议

---

## 一、步骤目标

建立 Go Core 与 Plugin Host、Task Runtime、Service Runtime、WASM Host 之间统一的内部 RPC 协议。

目标：

```text
Framed Transport
→ Session Handshake
→ Authentication
→ Version Negotiation
→ Request/Response/Notification
→ Cancellation
→ Streaming
→ Backpressure
→ Diagnostics
```

本协议仅供 Amitia 内部 Runtime 使用，不是公共远程 API。

---

## 二、为什么采用 JSON-RPC 风格

优势：

-结构明确；
-跨语言；
-可调试；
-支持 Request/Response/Notification；
-容易生成 TypeScript/Go 类型；
-可在后续替换编码而保持语义。

但必须增加：

-帧；
-认证；
-版本；
-大小限制；
-取消；
-流；
-背压；
-错误分类。

---

## 三、Transport

优先：

```text
stdio framed transport
```

可选：

-本地命名管道；
-Unix Domain Socket；
-Windows Named Pipe。

第一版统一从 stdio 开始。

---

## 四、帧格式

禁止依赖“每行一个 JSON”处理任意大消息。

建议长度前缀：

```text
4/8-byte length prefix
+ UTF-8 JSON payload
```

或明确 Header Frame。

必须：

-最大帧；
-超长拒绝；
-部分读取；
-多帧；
-关闭检测；
-协议污染处理。

---

## 五、消息模型

```json
{
  "jsonrpc": "2.0",
  "id": "request-id",
  "method": "runtime.invoke",
  "params": {}
}
```

Notification 无 ID。

---

## 六、Handshake

```text
runtime.hello
host.welcome
runtime.ready
```

Hello 包含：

-协议版本；
-Runtime 类型；
-Instance ID；
-Generation；
-Definition Hash；
-Nonce；
-支持 Feature。

Host Welcome：

-选中版本；
-Session ID；
-短期认证；
-Host API 版本；
-限制；
-时钟信息。

---

## 七、认证

Runtime 启动时 Supervisor 通过安全继承通道提供一次性 Nonce。

Handshake 验证：

-Instance；
-PID/Process Handle 绑定预留；
-Generation；
-Nonce；
-Definition Hash；
-启动时限。

Session 建立后每个消息绑定 Session。

---

## 八、方法命名

```text
runtime.*
host.*
stream.*
task.*
diagnostic.*
lifecycle.*
```

自定义 Extension 不得注册协议级 Method。

---

## 九、核心 Runtime Method

```text
runtime.initialize
runtime.invoke
runtime.cancel
runtime.health
runtime.shutdown
runtime.reload
```

---

## 十、Host Method

Host API Gateway 提供：

```text
host.call
```

或按路由映射。

建议对传输层保持少量稳定 Method，业务 Method 放 params 内。

---

## 十一、错误模型

```json
{
  "code": "permission_denied",
  "message": "Request denied",
  "data": {
    "retryable": false,
    "category": "permission"
  }
}
```

禁止直接传递内部堆栈。

---

## 十二、Request Tracker

双方都需：

-唯一 ID；
-Pending Map；
-Deadline；
-取消；
-未知响应；
-重复响应；
-连接关闭 FailAll。

---

## 十三、取消

```text
runtime.cancel
```

包含目标 Request ID 和原因。

取消是尽力而为，但宿主超时后忽略延迟结果。

---

## 十四、Streaming

定义：

```text
stream.open
stream.chunk
stream.close
stream.error
stream.cancel
stream.credit
```

使用 Credit/Window 实现背压。

---

## 十五、背压

禁止无界发送。

每个方向：

-最大在途字节；
-最大 Stream；
-Chunk 上限；
-消费者 Credit；
-超时；
-取消。

---

## 十六、通知

用于：

-日志；
-Progress；
-Health；
-Event Loop Lag；
-Resource Usage；
-Task Checkpoint Ready。

通知需要限流。

---

## 十七、消息大小

分类限制：

-控制消息；
-Tool Input；
-Tool Output；
-日志；
-Stream Chunk；
-Checkpoint。

大文件使用 Artifact/Resource Handle。

---

## 十八、Schema

所有内部 RPC Method 生成 JSON Schema 或类型定义。

Go 和 TypeScript 从同一协议定义生成或进行兼容测试。

---

## 十九、版本

协议版本：

```text
amitia-runtime-rpc/1
```

支持：

-主版本不兼容；
-Feature Negotiation；
-弃用；
-最小兼容版本；
-测试矩阵。

---

## 二十、心跳

Runtime 定期发送 Health 或由 Host Ping。

心跳不应过频。

卡死识别结合：

-无响应；
-事件循环延迟；
-进程状态；
-活跃调用；
-资源使用。

---

## 二十一、日志隔离

stdout 只用于协议。

stderr 用于受控诊断，但仍需 Runtime Host 捕获和限流。

插件普通 `console` 应重定向为协议日志，不能污染 stdout。

---

## 二十二、协议关闭

Graceful：

```text
runtime.shutdown request
→ runtime.shutdown_ack
→ transport close
```

异常关闭：

-Pending fail；
-Runtime Crash；
-资源清理；
-审计。

---

## 二十三、安全

必须防止：

-帧注入；
-JSON Bomb；
-Method spoofing；
-Session 重放；
-旧 Generation；
-超大消息；
-日志污染；
-Stream 资源耗尽；
-未认证 Host Call；
-插件直接获取底层 Channel。

---

## 二十四、调试抓包

开发模式可记录脱敏协议摘要。

生产默认不记录完整 Payload。

---

## 二十五、测试要求

覆盖：

-Handshake；
-错误 Nonce；
-版本不兼容；
-分帧；
-部分读取；
-并发请求；
-双向请求；
-取消；
-超时；
-未知 ID；
-重复响应；
-Stream；
-背压；
-日志污染；
-大消息；
-连接断开；
-进程崩溃；
-协议 Fuzz；
-Go/TS 兼容。

---

## 二十六、实施任务

1. 定义协议规范。
2. 实现 Framing。
3. 实现 Handshake/Auth。
4. 实现 Version Negotiation。
5. 实现双向 Request Tracker。
6. 实现 Cancel。
7. 实现 Streaming/Backpressure。
8. 实现 Notification 限流。
9. 实现 Go Client/Server。
10. 实现 TypeScript Client/Server。
11. 接入 Main Runtime。
12. 接入 Task Runtime。
13. 建立协议生成和兼容测试。
14. 建立 Fuzz。
15. 实现诊断。

---

## 二十七、验收标准

1. 内部 Runtime 使用统一 RPC。
2. stdout 协议不被日志污染。
3. Session 与 Generation 验证。
4. 双向调用可用。
5. 取消和超时可用。
6. Stream 有背压。
7. 大文件不走普通消息。
8. Go/TS 类型兼容。
9. 协议 Fuzz 不崩溃。
10. 可进入第 39 步 Trusted Service Runtime。

---

## 二十八、执行约束

> 内部 RPC 是受认证的本地 Runtime 通道，不是插件可任意扩展的方法空间，也不是远程网络 API。

禁止：

-无帧行协议作为最终方案；
-stdout 日志；
-无认证；
-无消息上限；
-无背压流；
-完整 Payload 默认日志；
-插件自定义 host.* Method；
-远程暴露。
