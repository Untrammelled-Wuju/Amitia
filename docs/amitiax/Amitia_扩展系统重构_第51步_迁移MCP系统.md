# Amitia 扩展系统重构第 51 步实施文档

## 第 51 步：迁移 MCP 系统

---

## 一、步骤目标

将当前 MCP Client、Server 配置、Tool Discovery、Resources、Prompts、OAuth、Agent Skill 依赖和旧 MCP Tool Adapter 迁移到 Extension Kernel 的 MCP Contribution、MCP Runtime、Dependency Resolver、Secret Broker、ToolRegistry 和统一生命周期体系。

目标：

> 保留成熟 MCP 协议能力，删除 MCP 作为独立扩展子系统的生命周期、Enabled、Scope、运行记录和 Tool 注册逻辑。

---

## 二、迁移范围

包括：

-手动配置 MCP Server；
-内置 MCP；
-Agent Skill 声明 MCP；
-扩展包附带 MCP；
-stdio；
-Streamable HTTP；
-OAuth；
-Headers；
-Environment；
-Tool Discovery；
-Resources；
-Prompts；
-Completion；
-Tasks；
-Sampling；
-Elicitation；
-Roots；
-Connection Recovery；
-旧 MCP Skill Adapter；
-旧 MCP Tool Registry；
-旧连接状态；
-旧运行记录；
-旧 Secret。

---

## 三、目标建模

### 手动 MCP

使用 Synthetic Extension：

```text
local.user/mcp-<stable-id>
```

Module：

```text
mcp
```

Contribution：

```text
mcp_server
```

Runtime：

```text
mcp
```

### 扩展附带 MCP

属于扩展 Module。

### Agent Skill 依赖 MCP

只建立 Dependency Reference。

---

## 四、稳定 Server ID

必须建立：

```text
legacy_server_id
legacy_name
transport
command/url
→ canonical_contribution_id
```

禁止使用 Session ID、进程 ID 或连接 ID。

---

## 五、Server Definition

迁移字段：

-Transport；
-Command；
-Args；
-URL；
-Headers Reference；
-Environment Reference；
-OAuth；
-Tool Allowlist/Denylist；
-Resource/Prompt 能力；
-Enabled；
-Scope；
-Owner；
-平台；
-自动连接策略；
-重连策略。

---

## 六、Secret 迁移

以下必须迁入 Secret Broker：

-API Key；
-Authorization Header；
-OAuth Access Token；
-Refresh Token；
-Client Secret；
-Environment Secret；
-Certificate；
-Private Key。

旧明文配置：

1.读取；
2.写入 Secret Broker；
3.保存 Reference；
4.验证；
5.清除旧明文；
6.审计；
7.失败则默认禁用 Server。

---

## 七、Command 安全

stdio Command 迁移：

-可执行路径来源；
-Args 数组；
-工作目录；
-平台；
-Environment；
-包内/外部；
-Trust；
-权限。

禁止保留 Shell 字符串。

---

## 八、Connection State

旧：

```text
enabled
connected
active
available
```

拆分：

-Server Enabled；
-Desired Connection；
-Actual Connection；
-Protocol Ready；
-Health；
-Circuit；
-Tool Availability。

---

## 九、Runtime 迁移

所有连接由：

```text
MCPRuntimeFactory
→ RuntimeSupervisor
→ MCPConnectionSupervisor
```

管理。

旧 MCP Manager 不再直接在启动时重连。

---

## 十、Discovery 迁移

统一：

```text
MCP Discovery
→ Descriptor Snapshot
→ Dynamic Contribution Diff
→ Contribution Registry
→ ToolDefinition
```

重连：

-稳定 Tool ID；
-原子替换；
-不重复；
-移除已不存在 Tool；
-保留历史运行记录；
-更新 Generation。

---

## 十一、MCP Tool ID

建议：

```text
mcp/<server-stable-id>/<tool-name>
```

或父 Contribution 子 ID。

必须保留旧模型名称别名，防 Prompt 行为突变。

---

## 十二、Tool Schema

Discovery 获取的 Schema：

-规范化；
-大小限制；
-JSON Schema 安全子集；
-缺失字段修复策略；
-Hash；
-版本；
-不可执行时原因。

恶意或无效 Schema 的 Tool 不注册为可执行。

---

## 十三、Resources 与 Prompts

不得继续塞入 Tool/Skill。

分别建模为：

-Resource Capability/Contribution；
-Prompt Resource；
-Completion Provider。

访问经过 Host API、Permission、Scope 和审计。

---

## 十四、Sampling/Elicitation/Roots

Host Callback 迁移到统一 Host API/Permission：

-不默认传完整 Prompt；
-不默认开放文件系统；
-用户交互可见；
-有 Scope；
-有 Deadline；
-有审计；
-防递归。

---

## 十五、OAuth

迁移：

-Provider 元数据；
-Client；
-Token Reference；
-回调；
-刷新；
-撤销；
-用户状态。

OAuth Session 不作为 Extension Definition。

---

## 十六、Agent Skill MCP 依赖

旧 Skill 自动安装逻辑删除。

新链路：

```text
Agent Skill Dependency
→ Dependency Resolver
→ Existing Server / Synthetic Server / Install Plan
→ User Confirmation
```

---

## 十七、Scope

旧 MCP Server 和 Tool Scope：

-Server Enabled；
-Server Scope Binding；
-Tool Override；
-Tool Scope Rule。

不能混成一个 `scope_enabled`。

---

## 十八、Permission

至少分层：

1.连接/启动 Server；
2.Server Transport 网络/进程；
3.Tool 调用权限；
4.Host Callback 权限；
5.Resource/Prompt 读取；
6.Secret 使用。

---

## 十九、手动断开

迁移后：

```text
desired_state=disconnected
```

保持 Enabled。

下次启动不自动连接，除非用户重新连接。

---

## 二十、运行记录

旧 MCP Operation 映射：

-Operation；
-Invocation；
-Attempt；
-Runtime Event；
-Protocol Event；
-Error；
-SideEffect。

旧 Session 只保留历史，不恢复。

---

## 二十一、Owner 与共享

同一 MCP 被多个 Skill/Extension 使用：

-单一 Server Owner；
-多个 Resource Reference；
-卸载一个引用者不删除 Server；
-Owner 转移需明确；
-用户手动 Server 默认 Owner=user。

---

## 二十二、迁移批次

1.手动 stdio；
2.手动 HTTP；
3.OAuth；
4.内置 MCP；
5.Agent Skill 依赖；
6.扩展附带 MCP；
7.Tool Discovery；
8.Resources/Prompts；
9.Host Callback；
10.旧表冻结。

---

## 二十三、前端

统一 MCP 页面展示：

-所属 Extension；
-Owner；
-Enabled；
-Desired；
-Actual；
-Transport；
-Secret；
-Scope；
-Permission；
-Tool；
-Resources；
-Prompts；
-Health；
-Circuit；
-运行记录；
-依赖引用。

---

## 二十四、兼容 API

旧：

```text
ConnectServer
DisconnectServer
EnableServer
ListTools
DeleteServer
```

映射：

-Connect/Disconnect → Runtime Desired State；
-Enable → EnablementService；
-ListTools → Contribution Registry；
-Delete → Lifecycle/Resource Release。

---

## 二十五、双连接防护

迁移期间：

-旧 Manager 自动重连关闭；
-新 Supervisor 成为连接 Owner；
-进程锁；
-Connection Generation；
-旧连接清理；
-防同 Server 双进程。

---

## 二十六、测试要求

覆盖：

-stdio；
-HTTP；
-OAuth；
-Secret；
-Command；
-重连；
-手动断开；
-Discovery；
-Tool Diff；
-Schema；
-Resources；
-Prompts；
-Sampling；
-Elicitation；
-Roots；
-Agent Skill 依赖；
-共享；
-删除；
-更新；
-应用重启；
-旧 API；
-双连接；
-跨平台；
-性能。

---

## 二十七、实施任务

1. 输出 MCP 全量清单。
2. 建立稳定 ID。
3. 建立 Synthetic/Extension Contribution。
4.迁移 Server Definition。
5.迁移 Secret。
6.迁移 Enabled/Scope。
7.接入 MCP Runtime。
8.接入 Runtime Supervisor。
9.重写 Discovery 注册链。
10.迁移 Resources/Prompts。
11.迁移 Host Callback。
12.迁移 Agent Skill 依赖。
13.迁移运行记录。
14.冻结旧自动重连。
15.改造前端/API。
16.完成双连接和回归测试。

---

## 二十八、验收标准

1. MCP 成为正式 Contribution/Runtime。
2.手动 MCP 使用 Synthetic Extension。
3.所有 Secret 进入 Broker。
4.旧 Session 不恢复。
5.连接由 Runtime Supervisor 管理。
6.Tool 由动态 Contribution 注册。
7.重连不重复 Tool。
8.手动断开不等于 Disabled。
9.Agent Skill 不自动安装 MCP。
10.共享 MCP 有引用图。
11.旧 Manager 不再自动连接。
12.关键测试通过。
13.可进入第 52 步 Workflow 迁移。

---

## 二十九、执行约束

> MCP 协议能力保留，但 MCP 不再拥有独立于 Extension Kernel 的安装、启用、Scope、权限、Tool 注册和生命周期系统。

禁止：

-旧 Manager 自动重连；
-明文 Secret；
-Shell Command；
-Session ID 稳定化；
-MCP Tool 注册 Skill；
-Agent Skill Loader 安装 MCP；
-连接状态回写 Enabled；
-新旧双连接。
