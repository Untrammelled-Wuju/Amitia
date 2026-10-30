# 手机端 SnowLuma 接入失败路径记录

记录时间：2026-10-30

适用范围：Amitia Android 手机端 QQ 个人号插件，Android ARM64，PRoot，Linux QQ，SnowLuma 2.2.0。

## 目标与约束

目标是在手机端继续使用 SnowLuma 原版协议链路，让登录账号能够接收其他 QQ 用户的真实私聊消息，并触发 Amitia 的 AI 回复。

排查期间遵守以下约束：

- 不使用 NapCat。
- 不切换到其他 QQ 协议实现。
- 保持 SnowLuma 发布文件原样，不修改其 `.node`、`.so` 和前端资源。
- 不使用 Android 真机运行层替换现有 PRoot。
- 不在 3000 端口启动服务。

## 验证环境

| 项目 | 值 |
| --- | --- |
| 模拟器 | `emulator-5554` |
| Android 应用 | `com.amitia.amitia_app` |
| QQ | Linux QQ `3.2.32-52194` |
| SnowLuma | `2.2.0` |
| Node.js | `26.12.1` |
| 登录账号 | `3366726108` |
| 插件验证范围 | `0.2.129` 至 `0.2.141` |

## 共同故障表现

QQ 可以正常启动、登录并在 noVNC 界面收到其他用户消息，但 SnowLuma 与 Amitia 收不到真实入站消息。

典型状态：

```text
hookReady=true
qqOnline=false
onebotConnected=false
protocolReady=false
lastMessageAt=""
```

SnowLuma 日志稳定出现：

```text
[HookFrame] op=22 status=-1 flags=0 detail=-
[QQHook.Runtime] Client private-message reporting is unavailable: PID=<pid> status=-1
[Hook] login identity discovered: PID=<pid> UIN=3366726108; awaiting readiness
```

QQ 界面实际收到了测试消息，但以下计数没有变化：

- SnowLuma Hook 的入站 `op=6` 帧数量保持 0。
- Amitia 插件接收数保持 0。
- Amitia 回复数保持 0。

因此不是 QQ 没有收到消息，也不是 Amitia 的 AI 路由没有处理消息，而是消息没有从原生 Hook 进入 SnowLuma。

## 已尝试且失败的方法

### 1. 直接使用官方 `loadModuleManualAsync` 注入

做法：

```text
在项目 Node 中加载 snowluma-linux-arm64.node，
调用 loadModuleManualAsync(QQ_PID, snowluma-linux-arm64.so)。
```

结果：失败。

关键证据：

```text
component loading failed [COMPONENT_LOAD_FAILED]
```

官方注入器会执行 `PTRACE_SEIZE`、`PTRACE_INTERRUPT`、寄存器读写和远程函数调用。当前 PRoot 环境中远程代码执行后目标线程收到：

```text
status=0xb7f
```

该状态对应 `SIGSEGV`，随后组件加载失败。官方注入器在其他 ARM64 Android 环境也有相同问题，上游 issue #590、#595、#596 记录了 `COMPONENT_LOAD_FAILED` 和 `SIGSEGV(pc=0)`。

结论：当前 PRoot 下不能继续依赖官方 ptrace 手动映射注入。

### 2. 将 `PTRACE_SEIZE` 替换为 `PTRACE_ATTACH`

做法：

```text
拦截官方 .node 对 ptrace 的调用；
将不带 address/data 的 PTRACE_SEIZE 转为 PTRACE_ATTACH；
虚拟 PTRACE_INTERRUPT 返回成功。
```

结果：失败。

日志证明适配已经命中：

```text
process_vm relocations: ... read=1 write=1 ptrace=1
ptrace seize compatibility: attached through virtual PTRACE_ATTACH
```

但官方注入仍在远程代码执行阶段崩溃：

```text
waitpid: ... status=0xb7f
component loading failed [COMPONENT_LOAD_FAILED]
```

结论：`PTRACE_SEIZE` 和 `PTRACE_ATTACH` 的同步停获差异不是唯一原因。替换调用名义上成功，不代表 PRoot 下的远程调用语义完整。

### 3. 通过 GOT 重定位替换 `process_vm_readv/process_vm_writev`

做法：

```text
在加载 SnowLuma 原生模块时修改其 GOT；
将进程内存读写切换到 /proc/<pid>/mem。
```

结果：部分生效，但不足以恢复消息。

日志确认 GOT 已被替换：

```text
process_vm relocations: ... read=1 write=1
process_vm transfer: ... bytes=1048576 error=0
```

官方目标解析器随后能够看到：

```text
sendSsoInternal
msfDidRecvResponseData
msfGetStatus
selfInfoOnLoginSuccess
MSFCoreService
```

后期也能观察到：

```text
msfServiceCandidates=2..7
msfCoreCandidates=2..5
```

但 SnowLuma 仍持续输出：

```text
op=22 status=-1
awaiting readiness
```

结论：缺失 `process_vm_readv` 确实是一个兼容问题，已解决内存读取失败，但它不是导致真实入站消息为 0 的唯一原因。

### 4. LD_PRELOAD 预加载 SnowLuma 原版 Hook

做法：

```text
通过项目兼容加载器预加载 snowluma-linux-arm64.so；
由 QQ 包装脚本在启动时调用 snowluma_linux_hook_start_dynamic()。
```

结果：Hook 能加载，但登录确认失败。

成功部分：

```text
hook started
Hook pipe connected
login identity discovered
```

失败部分：

```text
op=22 status=-1
awaiting readiness
```

该结果说明：

- 控制 socket 和接收 socket 可以创建。
- QQ 账号身份可以被本地文件探测。
- 原版 Hook 仍没有完成 SnowLuma 所需的登录就绪确认。
- 其他用户发来的真实消息没有产生 `op=6`。

### 5. 延迟启动 Hook，等待登录和 MSF 服务就绪

分别测试了 4 秒、20 秒和 40 秒启动延迟。

结果：失败。

20 秒延迟：

```text
Hook 正常建立控制管道
login identity discovered
op=22 status=-1
```

40 秒延迟且解析器已经看到：

```text
msfServiceCandidates=6
msfCoreCandidates=3
```

Hook 仍然返回：

```text
op=22 status=-1
awaiting readiness
```

结论：单纯把 Hook 加载得更晚不能修复。即使 `MSFCoreService` 候选已经存在，原版 ARM64 Hook 仍没有完成账号就绪确认。

### 6. 登录后在 QQ 进程内停止并重新启动 Hook

做法：

```text
首次预加载 Hook；
登录后通过 SIGUSR1 触发 stop_dynamic()；
随后再次调用 start_dynamic()。
```

结果：失败，并导致 QQ 主进程退出。

关键日志：

```text
hook stopped for restart
hook thread started
hook started
qq 已退出（code 1）
```

结论：在已经运行的 QQ 进程里二次停止和启动原版 ARM64 Hook 不稳定，不能作为修复路径。

### 7. 登录后首次延迟启动 Hook

做法：

```text
QQ 启动时不加载 Hook；
登录完成并发现 MSFCoreService 后，通过 SIGUSR1 首次启动 Hook。
```

结果：失败。

Hook 日志显示启动调用发生，随后 QQ 以 code 1 退出。

结论：Android/PRoot 环境中，晚于 QQ 主流程启动原版 Hook 存在进程稳定性问题。

### 8. 升级 SnowLuma 2.2.0 到 2.2.1

做法：

```text
下载官方 2.2.1 linux-arm64-lite；
比较 ARM64 原生产物。
```

结果：没有可用差异。

文件哈希完全相同：

```text
snowluma-linux-arm64.node
b1473272d00fc96aab106e8f170022b3f88c1d7cd0a4e7307c0932ef23f6f746

snowluma-linux-arm64.so
8f8838b909bb55f9bd146e6ecd8114e4b6e94284627e5295f2003651fc320a51
```

结论：升级到 2.2.1 不会改变 ARM64 原生 Hook 行为。

### 9. 将 `status=-1` 视为纯日志警告

上游 issue #603 中，Windows 环境虽然报告 `status=-1`，其他功能仍可工作。

本项目的实际测试结果不同：

- QQ 客户端能收到真实消息。
- SnowLuma 没有出现 `op=6` 入站帧。
- OneBot 没有连接。
- Amitia 接收数和回复数均为 0。

结论：不能把当前手机端的 `status=-1` 当作仅影响诊断输出。它与 SnowLuma 没有进入可收发状态同时发生。

### 10. 在 QQ 界面退出账号后重新登录

做法：

```text
保持 Hook 和 SnowLuma 运行；
从 QQ 左下角菜单退出账号，再点击登录。
```

结果：失败。

QQ 退出账号后会启动新的主进程，新进程没有经过项目包装脚本，因此不会加载 SnowLuma Hook。SnowLuma 随后检测到原 PID 消失，Hook、OneBot 连接和账号状态全部断开。

结论：该操作不能刷新原生登录状态，只会绕过包装启动链路。

### 11. 等真实消息创建 `MSFCoreService` 后再首次加载 Hook

做法：

```text
先不加载 SnowLuma Hook；
持续用官方解析器等待 msfCoreCandidates > 0；
用户发送真实私聊后再启动 Hook。
```

结果：失败。

QQ 窗口已经收到 `手机收消息测试新的3`，但解析器仍持续返回：

```text
msfServiceCandidates=1
msfCoreCandidates=0
```

结论：真实入站消息本身不会创建可供 Hook 使用的 `MSFCoreService` 实例。必须先加载 Hook，之后候选对象才会出现；因此不能用“候选先出现”作为首次加载条件。

### 12. 在已加载 Hook 上再次调用 `start_dynamic()`

做法：

```text
保留已启动的 Hook；
候选对象出现后再次调用原版 start_dynamic()；
不调用 stop_dynamic()。
```

结果：失败。

即使解析器已经返回：

```text
msfServiceCandidates=5
msfCoreCandidates=2
```

再次调用后 QQ 主进程仍以 `SIGTRAP` 退出。

结论：原版 ARM64 Hook 不是可重复初始化的实现，不能在运行中刷新。

### 13. 直接在 `msfDidRecvResponseData` 入口安装 ARM64 trampoline

做法：

```text
按固定 QQ 3.2.32-52194 偏移定位 wrapper.node；
在 msfDidRecvResponseData 入口写入绝对跳转；
在跳板执行原函数前两条指令后返回原函数。
```

结果：只读阶段有效，和 SnowLuma Hook 共存时失败。

探针确实捕获了 QQ 收包命令：

```text
Heartbeat.Alive
Client.CorrectTime
wtlogin.trans_emp
OidbSvcTrpcTcp.0xcde_2
ConfigPushSvc.PushReq
```

说明入口地址和目标函数判断正确。但 SnowLuma Hook 加载后，QQ 进程触发 `SIGTRAP` 并退出。

结论：当前函数签名、指令边界与 SnowLuma 自身 Hook 共存关系尚未完整还原，不能把该 trampoline 作为生产方案。

### 14. 将 `msfDidRecvResponseData` 的 vtable 指针替换为兼容函数

做法：

```text
保留原函数不改；
扫描 wrapper.node 和匿名可写内存中的函数指针；
把等于 msfDidRecvResponseData 的指针替换为带 BTI 入口的兼容函数。
```

结果：失败。

兼容库已使用 `-mbranch-protection=bti` 编译，`receive_vtable_probe` 入口已确认包含 `bti c`。但替换后加载 SnowLuma Hook，QQ 主进程触发 `SIGSEGV` 并退出。

结论：该节点还被其他对象或 SnowLuma 自身 Hook 同时引用，不能只替换单一函数指针。

## 上游已知情况

- issue #590：Android 16 / ARM64 下官方注入出现 `SIGSEGV(pc=0)` 和 `COMPONENT_LOAD_FAILED`。
- issue #591：PRoot 下预加载 Hook 可能没有控制管道；维护者明确不对 Android/手机平台作可用性保证。
- issue #595、#596：QQ `3.2.32` 在 Android/ARM64 上重复出现 `COMPONENT_LOAD_FAILED`。
- issue #603：Windows QQ `9.9.36` 上 `status=-1`，但基础消息仍可能工作。
- issue #604：内核缺少跨进程内存读取时，SnowLuma 会静默进入等待登录。项目已通过 `/proc/<pid>/mem` 部分规避该问题。
- issue #518、#395：Linux QQ `3.2.32-52194` 上可复现“`loginIdentityHint` 已 ACK，但 `loggedIn=false`、`uin=0`、无数据包”，与当前原生 Hook 行为一致。

## 当前不能重复使用的方法

以下方法已经证明不能解决当前手机端问题：

1. 继续调用官方 `loadModuleManualAsync`。
2. 仅把 `PTRACE_SEIZE` 改成 `PTRACE_ATTACH`。
3. 只修复 `process_vm_readv` 后等待消息自动恢复。
4. 只延迟 Hook 的加载时间。
5. 登录后在 QQ 进程内停止并重启 Hook。
6. 通过外部信号在登录后首次启动 Hook。
7. 仅升级到 SnowLuma 2.2.1。
8. 以 `status=-1` 只是日志为由跳过真实入站消息验证。
9. 等待真实消息来创建 `MSFCoreService` 后再加载 Hook。
10. 对已经运行的 ARM64 Hook 再次调用 `start_dynamic()`。
11. 在 SnowLuma Hook 已加载时复用通用 ARM64 inline trampoline 探针。

## 管道协议兼容方案的当前结果

当前源码中已经实现一条不修改 SnowLuma 文件的管道协议兼容方案：

- 拦截原版 Hook 收到的 `loginIdentityHint` 帧。
- 在不修改 SnowLuma 文件的前提下，补发官方登录状态帧。

插件版本 `0.2.142` 至 `0.2.177` 的验证结果：

```text
op=22 status=-1
login identity hint received
login state bridge: uin=3366726108 result=40
[HookFrame] op=7 status=1 flags=4
[Hook] login detected: PID=4749 UIN=3366726108
qqOnline=true
onebotConnected=true
protocolReady=true
```

这证明：

- 原版 Hook 的 `loginIdentityHint` 已进入兼容层。
- 补发的官方登录状态帧被 SnowLuma 接受。
- SnowLuma 已从 `awaiting readiness` 进入在线状态。
- OneBot 反向 WebSocket 已建立。
- 原生 Hook 随后发送的 `op=7 status=0` 会在兼容层中被改写为当前 UIN 的在线状态，避免再次回落。
- 当前状态曾达到 `qqOnline=true`、`onebotConnected=true`、`protocolReady=true`。
- `0.2.154` 起增加登录态保护，能够把原生连续发出的 `op=7 status=0` 改写为当前 UIN 的登录态。
- `0.2.156` 已移除会触发 `SIGTRAP/SIGSEGV` 的收包入口 trampoline 和 vtable 探针，保留稳定单次 Hook 启动路径。

仍需完成：

- 用另一账号发送全新私聊，确认原生 Hook 产生 `op=6`。
- 确认 Amitia 接收数、AI 处理和 `send_private_msg` 回复均成功。
- 测试表情包插件在同一真实私聊中的发送。

### 15. 只修 PRoot ptrace 后加载官方 ARM64 Hook

做法：

```text
从固定 PRoot 源码构建实验版本；
补齐 PTRACE_SEIZE、PTRACE_INTERRUPT、PTRACE_GETREGSET、PTRACE_SETREGSET；
为 process_vm_readv 增加 /proc/<pid>/mem 后备；
保持 SnowLuma 原版 native 文件不变。
```

结果：原生 Hook 可以建立控制管道和接收管道，但账号仍停在等待登录。

进一步确认：

- 当前 PRoot 下高地址 `mmap(MAP_FIXED_NOREPLACE)` 返回 `ENOMEM`，即使 ptrace 调用全部成功，官方注入 stub 仍无法稳定执行。
- 官方 issue #590、#591、#595、#596 已记录 Android/ARM64 注入问题，维护者明确不对 Android/手机平台作可用性保证。
- 实验 PRoot 改动没有写入正式 `libamitia_proot.so`。

### 16. 按加载时机调整 Hook 启动

分别测试：

```text
wrapper.node 加载后延迟 40 秒启动；
wrapper.node 加载后延迟 4 秒启动；
wrapper.node dlopen 完成后立即启动；
QQ 进程构造期立即启动并等待目标就绪。
```

结果：四种方式都能让 Hook 建立管道，但都没有得到可用的原生登录状态或 `op=6` 入站帧。

结论：启动时机不是根因。

### 17. 关闭登录状态伪桥接，等待原版 Hook 自行确认

做法：

```text
不再把 op=17 身份提示直接改写为 op=7 登录成功；
让原版 Hook 自己完成 MSF 会话探测。
```

结果：

```text
awaiting login: PID=...
login identity discovered: PID=... UIN=3366726108; awaiting readiness
```

号可以从本地账号数据库识别，但原生 Hook 始终不发送有效的
`op=7 status=1 flags=4` 登录帧。

### 18. 延迟补发登录态并验证真实入站

做法：

```text
身份提示 ACK 后等待 5 秒；
只补发一次真实 UIN 的登录态；
不再在首次连接时立即绑定。
```

结果：

```text
login detected: UIN=3366726108
OneBot connected
发送 OIDB 请求返回 -39: The QQ connection changed
```

真实测试私聊 `最新测试 171`、`最新测试 172`、`最新测试 173`
均在 QQ 客户端界面正常出现，但 SnowLuma 没有产生 `op=6`，Amitia 接收数与回复数保持 0。

结论：延迟后可以避免过早登录帧，但没有修复原版 ARM64 Hook 的发送会话和入站接收链路。

### 19. 只读核对 `MSFService`、`MSFCoreService` 和函数补丁

使用官方 `.node` 的解析接口读取运行状态：

```text
msfServiceVtables=5
msfServiceCandidates=5
msfCoreCallbackVtables=1
msfCoreCandidates=2
msfCoreCallbackOffset=0x40
```

同时读取 `sendSsoInternal`、`msfDidRecvResponseData`、`msfGetStatus`
和 `selfInfoOnLoginSuccess` 的入口机器码。Hook 加载前它们与官方
`wrapper.node` 原文件一致；加载后部分代码入口被改写，说明补丁确实安装，
问题不是“找不到目标函数”。

对象内存满足 `MSFService` 内包含 `MSFCoreService` 的结构特征，说明解析器也找到了真实对象。

### 20. 在 `TaskManager::DidRecvResponseData` 外围增加只读探针

做法：

```text
按 QQ 3.2.32 wrapper.node 偏移定位 TaskManager 接收入口；
保存原始指令后安装 ARM64 trampoline；
记录参数并调用原函数，不改变返回值。
```

结果：探针可以捕获 QQ 的网络收包，包括：

```text
Heartbeat.Alive
Client.CorrectTime
wtlogin.trans_emp
ConfigPushSvc.PushReq
OidbSvcTrpcTcp.0x...
```

但外部私聊在 QQ 界面出现后，探针仍未看到对应的私聊上报数据，SnowLuma 也没有产生 `op=6`。

对照 `onMSFPacketState` 增加深层探针时，QQ 进程会触发 `SIGTRAP/SIGSEGV`
退出；已确认该虚调用不能继续使用通用 inline 探针。

### 21. 上游版本核对

官方当前 Linux ARM64 发行包 `v2.2.1` 与 `v2.2.0` 的
`snowluma-linux-arm64.so` 哈希一致：

```text
8f8838b909bb55f9bd146e6ecd8114e4b6e94284627e5295f2003651fc320a51
```

因此升级到 v2.2.1 不能解决该问题。

相关上游记录：

- #395：Linux QQ 3.2.32 已登录，双管道连接成功，但持续等待登录；维护者关闭后仍有 v2.2.0 复现记录。
- #518：`loginIdentityHint` 已 ACK，但 `loggedIn=false`、`uin=0`、无数据包；以 `not_planned` 关闭。
- #537：Docker/QQ 3.2.32 同样停在等待登录。
- #590、#591、#595、#596：Android/ARM64 注入和管道问题。
- #603：`status=-1` 时 Windows 仍可通过 `TaskManager` 收包，说明不能只修 `status` 字段。
- #604：缺少跨进程内存读取会使原生解析静默失败；本项目已用 `/proc/<pid>/mem` 规避，但后续仍卡在原生就绪和接收链路。

当前可稳定工作的部分：

- Android ARM64 上的 Linux QQ、Xvfb、fluxbox、x11vnc、noVNC 和 SnowLuma 服务可以启动。
- PRoot ptrace 与 `process_vm_readv` 兼容层可以让官方 native 解析器返回对象候选。
- SnowLuma 原版控制管道和接收管道可以建立。
- 通过官方 `loginIdentityHint` 后，兼容层可以补发一次有效登录态，OneBot WebSocket 可以连接。

当前不可工作的核心部分：

- 官方 Linux ARM64 Hook 没有建立可用的 QQ 私聊接收和发送会话。
- 真实私聊能在 QQ 客户端显示，但不会产生 SnowLuma `op=6`。
- 在现有闭源 native 代码上继续替换 `MSFService`/`MSFCoreService` 虚表会崩溃，不能作为生产修复。

结论：问题已经不是 Android UI、PRoot 启动、二维码、OneBot 路由或 Amitia 回复逻辑，而是 SnowLuma 当前闭源 Linux ARM64 原生层不支持该 Linux QQ 版本的实际会话布局。项目侧能安全完成的兼容工作已经做到边界；继续打通需要 SnowLuma 上游修复 native，或提供可验证的官方 hook 扩展接口。

### 最终真实入站复测

已从本机“无拘.”账号向 `3366726108` 真实发送测试私聊，QQ 客户端界面确认收到消息。

同时，插件状态仍为：

```text
hookReady=true
onebotConnected=true
qqOnline=false
protocolReady=false
lastMessageAt=""
op=6 数量=0
Amitia 接收数=0
Amitia 回复数=0
```

结论：登录态兼容桥和 OneBot 连接已经生效，但原生 Hook 仍未把 QQ 私聊上抛为 `op=6`。该轮不能判定为通过。

在该方案完成真实消息收发验证前，不得把插件标记为手机端 QQ 收发可用。

### 15. 补齐 PRoot 的 ptrace 与 process_vm 用户态模拟

做法：

```text
在固定版本 Termux PRoot 源码中补充 PTRACE_SEIZE；
补充 PTRACE_INTERRUPT 和 PTRACE_EVENT_STOP；
修正 PTRACE_GETREGSET/SETREGSET 的错误返回；
在 ptrace 关系内模拟 process_vm_readv/process_vm_writev；
从源码构建隔离 ARM64 PRoot，仅用于 emulator-5554 验证。
```

结果：运行层测试通过，官方注入仍失败。

通过项：

- 父子进程和兄弟进程的 `PTRACE_SEIZE` 均成功。
- `PTRACE_INTERRUPT` 返回真实的 `SIGTRAP | PTRACE_EVENT_STOP`。
- ARM64 寄存器读取、写入、`PTRACE_CONT`、二次中断和 `PTRACE_DETACH` 通过。
- PRoot 与原生 Linux 的官方注入调用序列一致，包括 1 次 `SEIZE`、1 次 `INTERRUPT`、8 次 `GETREGSET`、15 次 `SETREGSET`、7 次 `CONT` 和 1 次 `DETACH`。
- 原生 Linux ARM64 使用相同 QQ、Node 和 SnowLuma 文件时，官方 `loadModuleManualAsync` 返回有效 handle。
- PRoot 下新增的 `process_vm_writev` 模拟能够返回实际传输字节数。

失败项：

- Android 模拟器中的 QQ 仍返回 `component loading failed [COMPONENT_LOAD_FAILED]`。
- 官方注入器在 Android 中执行远程 `mmap` 后，得到的目标地址与原生 Linux 路径不同，随后进入错误分支。
- 独立测试向 `0xffff00000000` 执行 `mmap(MAP_FIXED_NOREPLACE)`，原生 ARM64 Linux 成功，当前 Android 模拟器返回 `ENOMEM(12)`。
- 当前 Android 用户态虚拟地址布局不能提供官方注入器要求的固定高地址映射；这不是 ptrace 请求、wait status、寄存器读写或 `process_vm` 传输本身的错误。

源码实验补丁状态：

```text
已删除
```

该补丁未接入正式 Runtime Package 构建，也没有替换项目现有 `libamitia_proot.so`。当前 ARM64 PRoot 已恢复为仅应用项目既有 Android 基线补丁的 `v5.1.107.78` 构建。继续在该方向绕过地址限制，需要为 QQ 目标进程增加高地址影子映射和全局地址翻译，复杂度和稳定性风险都高于 SnowLuma 回调兼容层，因此不作为当前推荐路径。
