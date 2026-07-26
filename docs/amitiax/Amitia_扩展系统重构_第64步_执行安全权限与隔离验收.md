# Amitia 扩展系统重构第 64 步实施文档

## 第 64 步：执行安全、权限与隔离验收

---

## 一、步骤目标

对 Extension Kernel 的包安全、发布者信任、Permission、Scope、Runtime 隔离、Host API、Storage、Secret、UI 沙箱、Desktop Contribution、MCP、Workflow、Event/Hook 和生命周期执行最终安全验收。

本步骤目标：

> 证明第三方 Extension、未知 Publisher 包、恶意 `.amitiax`、受损 Runtime、越权 UI 和高风险 Desktop/Service 能力无法绕过 Amitia 的安全边界，并且所有授权、拒绝、审批、撤销、数据访问和副作用均可解释、可审计、可收窄。

---

## 二、安全模型范围

必须覆盖：

-供应链；
-包；
-Manifest；
-签名；
-Publisher；
-安装；
-更新；
-回滚；
-Runtime；
-RPC；
-Host API；
-Permission；
-Scope；
-Storage；
-Secret；
-Resource；
-MCP；
-Workflow；
-Event；
-Hook；
-UI；
-Electron；
-Desktop；
-Developer Mode；
-CLI；
-迁移；
-日志；
-诊断；
-卸载。

---

## 三、威胁主体

考虑：

-恶意第三方扩展；
-被入侵 Publisher；
-被篡改包；
-恶意 MCP Server；
-恶意 Service Binary；
-恶意 Web UI；
-恶意 Agent Skill；
-恶意 Workflow；
-本地低权限进程；
-用户误操作；
-旧数据污染；
-开发模式滥用；
-依赖冲突；
-供应链依赖；
-崩溃和故障导致的权限漂移。

---

## 四、包安全测试

覆盖：

-路径穿越；
-ZIP Bomb；
-嵌套归档；
-符号链接；
-硬链接；
-设备文件；
-大小写冲突；
-Unicode；
-保留文件名；
-超长路径；
-重复 Manifest；
-隐藏可执行；
-MIME 欺骗；
-文件替换；
-TOCTOU；
-安装时执行；
-跨平台权限位。

---

## 五、Manifest 安全

测试：

-未知字段；
-深度；
-大数组；
-远程 `$ref`；
-任意代码；
-绝对路径；
-Secret；
-Grant；
-角色 ID；
-任意 Runtime；
-高风险 Permission；
-假 Publisher；
-假 Signature；
-伪造 LegacyGo；
-动态 Contribution 越界。

---

## 六、签名与 Trust

测试：

-有效签名；
-内容篡改；
-Manifest 篡改；
-Key 替换；
-轮换；
-撤销；
-过期；
-未知 Publisher；
-用户 Trust；
-Blocked；
-Development；
-所有权转移；
-同版本重发；
-离线撤销缓存；
-信任库损坏。

---

## 七、Permission

建立权限矩阵并逐项测试：

-默认拒绝；
-Allow；
-Approval；
-Deny；
-条件；
-过期；
-撤销；
-后台任务；
-高风险；
-继承；
-子调用；
-Grant Generation；
-缓存失效。

P0：

-未授权 Tool 执行；
-撤销后继续执行；
-背景任务复用旧 Grant；
-UI 打开页面即获得全部权限；
-官方插件绕过权限。

---

## 八、Scope

测试：

-Global；
-Character；
-Conversation；
-Extension；
-Module；
-Invocation；
-Session；
-子调用收窄；
-角色切换；
-会话归属；
-后台 Schedule；
-Event；
-UI Session；
-MCP；
-Workflow；
-Storage。

P0：

-跨角色读写；
-跨会话；
-全局扩大；
-旧 Scope Snapshot 复用；
-前端当前角色替代真实 Scope。

---

## 九、Runtime 隔离

### JavaScript

-Node Builtin；
-fs；
-net；
-child_process；
-Electron；
-动态 require；
-原生模块；
-环境变量；
-其他 Extension；
-Host RPC；
-调试端口；
-内存；
-CPU；
-无限循环；
-事件循环阻塞。

### Task

-临时目录；
-Checkpoint；
-大输出；
-重启；
-网络；
-迁移限制。

### Service

-签名；
-进程树；
-网络监听；
-文件；
-环境；
-子进程；
-停止；
-Sandbox。

### WASM

-Import；
-WASI；
-Memory；
-Fuel；
-Trap；
-Host Function；
-大输出。

---

## 十、内部 RPC

测试：

-Nonce；
-Session；
-旧 Generation；
-消息伪造；
-帧注入；
-超大消息；
-Method Spoof；
-重复响应；
-未知 ID；
-流背压；
-日志污染；
-远程连接；
-重放；
-连接劫持。

---

## 十一、Host API

逐 Route 测试：

-身份；
-Permission；
-Scope；
-Schema；
-Deadline；
-Rate；
-Depth；
-Result；
-错误；
-脱敏；
-Resource Handle；
-子 Tool；
-Event；
-Schedule；
-Desktop；
-Message；
-Memory。

禁止存在万能 `host.call(any)`。

---

## 十二、Storage

测试：

-跨 Extension；
-跨 Module；
-跨角色；
-跨会话；
-CAS；
-事务；
-配额；
-大值；
-Schema；
-路径；
-备份；
-迁移；
-卸载；
-用户数据保护；
-Cache 清理。

---

## 十三、Secret

测试：

-创建；
-读取；
-Lease；
-过期；
-用途；
-共享；
-轮换；
-撤销；
-卸载；
-日志；
-错误；
-前端；
-诊断包；
-迁移；
-OAuth；
-环境注入；
-内存残留可行性。

P0：

-明文日志；
-前端回显；
-普通 Storage；
-长期 Lease；
-跨 Extension 读取。

---

## 十四、MCP

测试恶意 Server：

-无效 Tool Schema；
-超大 Schema；
-重复 Tool；
-恶意 Resource；
-Prompt 泄漏；
-Sampling 递归；
-Elicitation 欺骗；
-Roots 越界；
-日志洪泛；
-协议消息炸弹；
-重连风暴；
-Command 注入；
-OAuth Token 泄漏；
-Host Callback 越权。

---

## 十五、Workflow

测试：

-递归；
-循环；
-Depth；
-任意表达式；
-Tool 越权；
-后台旧 Permission；
-Scope；
-非幂等重试；
-结果未知；
-事件循环；
-大量并行；
-Delay/Approval；
-Sub Workflow；
-补偿。

---

## 十六、Event/Hook

测试：

-伪造系统事件；
-跨 Scope；
-无限重试；
-Depth；
-大 Payload；
-敏感字段；
-Filter 表达式；
-Hook 修改安全字段；
-Hook 超时；
-优先级抢占；
-重复订阅；
-Dead Letter 重放。

---

## 十七、UI 沙箱

测试：

-Node；
-Electron；
-Origin；
-CSP；
-postMessage；
-Preload；
-eval；
-远程脚本；
-fetch；
-Clipboard；
-文件；
-下载；
-导航；
-新窗口；
-Service Worker；
-Cookie；
-IndexedDB；
-跨 Extension；
-旧 Session；
-Bridge；
-Theme 注入；
-全局 CSS；
-Z-index；
-资源耗尽。

---

## 十八、Desktop

测试：

-动态 IPC；
-Shell；
-全局键盘监听；
-快捷键冲突；
-窗口安全；
-Always On Top；
-透明；
-外部 URL；
-文件；
-托盘；
-通知；
-系统协议；
-自启动；
-卸载残留。

---

## 十九、Developer Mode

测试：

-远程连接；
-未认证 CLI；
-工作区路径；
-热重载绕过校验；
-开发 Trust 外溢；
-正式版双运行；
-权限扩大；
-Source Map 泄漏；
-DevTools；
-构建脚本；
-远程模板；
-Secret 打包。

---

## 二十、CLI

测试：

-路径攻击；
-符号链接；
-Secret 文件；
-私钥；
-postinstall；
-远程模板；
-输出目录；
-签名；
-确定性；
-安装绕过；
-恶意项目配置。

---

## 二十一、迁移安全

测试：

-恶意旧路径；
-旧明文 Secret；
-Owner 错误；
-Scope 扩大；
-Enabled OR；
-假签名；
-旧 Plugin 假 Runtime；
-历史 Payload；
-数据覆盖；
-迁移工具远程访问；
-重复执行。

---

## 二十二、日志与诊断

检查所有：

-Backend；
-Electron；
-Runtime；
-MCP；
-Service；
-UI；
-CLI；
-Developer Console；
-诊断包；
-审计。

扫描：

-Token；
-Key；
-Authorization；
-Cookie；
-Secret；
-用户敏感内容；
-真实路径；
-系统 Prompt。

---

## 二十三、供应链依赖

检查：

-SDK；
-CLI；
-Plugin Host；
-Node Runtime；
-WASM Engine；
-JSON Schema；
-加密库；
-Electron；
-Go Dependencies；
-NPM Dependencies。

要求：

-锁定版本；
-License；
-已知漏洞扫描；
-依赖清单；
-更新策略；
-禁止运行时在线依赖。

---

## 二十四、模糊测试

至少对：

-Manifest；
-包布局；
-JSON-RPC；
-Host API Input；
-MCP 消息；
-Event Payload；
-Hook Result；
-UI Bridge；
-Workflow Definition；
-Schema UI；
-WASM Module；

执行 Fuzz。

---

## 二十五、渗透测试

建立恶意测试扩展集合：

```text
malicious-fs
malicious-network
malicious-rpc
malicious-ui
malicious-event-loop
malicious-secret
malicious-mcp
malicious-service
malicious-package
malicious-workflow
```

每个包有预期阻断点。

---

## 二十六、风险分级

```text
P0 critical
P1 high
P2 medium
P3 low
```

P0 必须清零。

P1 原则上清零；若接受必须有隔离和明确决策。

---

## 二十七、安全报告

必须输出：

-威胁模型；
-攻击面；
-测试矩阵；
-发现；
-修复；
-剩余风险；
-平台差异；
-Service 隔离真实能力；
-开发模式风险；
-第三方扩展安全说明；
-用户提示文案。

---

## 二十八、实施任务

1. 更新威胁模型。
2.执行包/Manifest/签名测试。
3.执行 Permission/Scope 测试。
4.执行各 Runtime 隔离测试。
5.执行 RPC/Host API 测试。
6.执行 Storage/Secret 测试。
7.执行 MCP/Workflow/Event/Hook 测试。
8.执行 UI/Desktop 测试。
9.执行 Developer/CLI 测试。
10.执行迁移安全测试。
11.扫描日志和诊断。
12.执行依赖漏洞扫描。
13.执行 Fuzz。
14.执行恶意扩展测试。
15.修复 P0/P1。
16.输出安全验收报告。

---

## 二十九、验收标准

1.包安全攻击被阻止。
2.签名和 Trust 不可伪造。
3.Permission 默认拒绝。
4.Scope 无跨角色/会话。
5.Runtime 无直接 Host 越权。
6.RPC 有认证和限制。
7.Host API 无万能入口。
8.Secret 无明文泄漏。
9.MCP/Workflow/Event 无递归和重放漏洞。
10.UI 无 Node/Electron 逃逸。
11.Desktop 无动态 IPC/Shell 后门。
12.Developer Mode 不扩大正式权限。
13.Fuzz 不崩溃。
14.P0 清零。
15.可进入第 65 步唯一入口切换。

---

## 三十、执行约束

> 安全验收必须针对恶意输入和恶意扩展，不得只验证正常开发者使用路径，也不得以“本地软件”或“官方扩展”为由跳过权限和隔离测试。

禁止：

-仅静态扫描；
-仅单元测试；
-忽略 Service 风险；
-忽略开发模式；
-忽略日志；
-以 Trust 替代 Permission；
-P0 未清零；
-对外宣称未实现的强沙箱能力。
