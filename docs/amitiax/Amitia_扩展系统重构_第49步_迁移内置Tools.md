# Amitia 扩展系统重构第 49 步实施文档

## 第 49 步：迁移系统内置 Tools

---

## 一、步骤目标

在第 1—48 步已经完成旧系统冻结、统一 Tool/Capability 模型、执行安全、权限、Scope、运行审计、Extension Kernel、`.amitiax`、Runtime 和 UI Contribution 之后，开始正式迁移旧系统中的内置 Tool。

本步骤目标是：

> 将 Amitia 当前所有内置 Tool、控制类 Tool、系统辅助 Tool、旧 Skill Handler 和内建插件 Tool，统一迁移为 `system/amitia-core` Extension 下的正式 Tool Contribution，并确保所有执行都通过 ToolRegistry、ExecutionSecurityKernel、Permission Broker、Scope Manager、Runtime Adapter 和统一审计。

迁移完成后，系统内置 Tool 与第三方 Tool 使用同一领域模型、同一执行入口和同一可用性判断。

---

## 二、迁移范围

至少包含：

-系统时间；
-角色信息读取；
-会话信息读取；
-记忆查询；
-记忆写入；
-消息发送；
-文件和资源；
-语音；
-图片；
-渠道；
-通知；
-桌面命令；
-系统诊断；
-Provider 调用；
-MCP 控制；
-Workflow 控制；
-旧 Skill 系统中的可执行 Handler；
-PluginManager 中直接暴露给模型的 Tool；
-内部控制 Tool；
-开发者诊断 Tool。

必须通过第 2 步调用链地图和第 3 步数据/资源清单重新核对，不得只迁移模型当前可见的 Tool。

---

## 三、目标建模

建立系统 Extension：

```text
ExtensionID: system/amitia-core
```

建议 Module：

```text
system/amitia-core#core-tools
system/amitia-core#memory-tools
system/amitia-core#message-tools
system/amitia-core#media-tools
system/amitia-core#desktop-tools
system/amitia-core#provider-tools
system/amitia-core#diagnostic-tools
```

每个 Tool 变为：

```text
ToolContributionDefinition
→ ToolDefinition
→ host_internal RuntimeBinding
```

---

## 四、稳定 Tool ID

格式：

```text
builtin/<namespace>/<tool-name>
```

或统一领域 ID：

```text
system/amitia-core#<module>/tool/<name>
```

模型名称与稳定 ID 分离。

迁移时必须建立：

```text
legacy_tool_id
→ canonical_tool_id
→ model_tool_name
```

映射表。

---

## 五、Tool 分类

迁移前将旧 Tool 分类：

### 1. 纯读取 Tool

例如时间、角色摘要、系统状态。

### 2. 状态写入 Tool

例如记忆写入、角色设置修改。

### 3. 外部副作用 Tool

例如消息发送、文件写入、网络请求。

### 4. 高风险桌面 Tool

例如窗口、剪贴板、系统操作。

### 5. 管理控制 Tool

例如启用 MCP、运行 Workflow、控制扩展。

### 6. 内部不可模型调用 Tool

仅系统编排使用。

---

## 六、Schema 迁移

每个 Tool 必须补齐：

-Input Schema；
-Output Schema；
-additionalProperties；
-字段长度；
-枚举；
-格式；
-默认值；
-错误；
-兼容版本。

禁止继续使用：

```text
map[string]any
```

作为无限制输入。

---

## 七、权限映射

每个旧 Tool 建立 Permission Matrix：

```text
Tool ID
Permission Requirement
Scope
Risk
Side Effect
Approval
```

示例：

```text
memory/read
memory/write
message/send
resource/read
resource/write
desktop/clipboard/read
desktop/clipboard/write
provider/invoke
extension/manage
```

旧 Tool 若无权限校验，迁移后必须默认收紧。

---

## 八、Scope 映射

每个 Tool 明确支持：

```text
global
character
conversation
invocation
```

例如：

-角色记忆 Tool 必须 Character Scope；
-会话 Tool 必须 Conversation Scope；
-系统诊断可 Global；
-消息发送必须绑定当前会话或明确目标 Scope。

不得继续从前端当前角色或全局变量隐式读取。

---

## 九、Runtime Adapter

内置 Tool 使用：

```text
HostInternalRuntimeAdapter
```

Handler 必须：

-接收 ToolInvocationContext；
-不自行做第二套权限；
-不读取全局当前角色；
-支持 Context Cancel；
-返回 ToolResult；
-记录 SideEffect；
-不直接写旧 Run 表。

---

## 十、旧 Skill Handler 迁移

旧 Skill 中实际为 Tool 的 Handler：

```text
Legacy Skill
→ Tool Contribution
```

迁移完成后：

-不再注册 SkillDefinition；
-不再由 SkillManager 执行；
-不再通过旧 Skill Enabled 控制；
-旧 API 只映射新 Tool 状态。

---

## 十一、模型暴露

模型 Tool 列表统一来自：

```text
ToolRegistry
→ EffectiveStateResolver
→ Model Tool Projection
```

不得保留：

-内置 Tool 单独列表；
-Plugin Tool 列表；
-MCP Tool 列表；
-Workflow Tool 列表；

在 Prompt Builder 中手动拼接。

---

## 十二、内部 Tool

内部控制能力仍可使用 ToolDefinition，但：

```text
Exposure.InternalOnly = true
ModelVisible = false
```

禁止为了方便让所有内部 Tool 对模型可见。

---

## 十三、错误统一

旧错误映射为：

```text
invalid_input
permission_denied
scope_denied
not_found
conflict
timeout
cancelled
dependency_unavailable
provider_unavailable
side_effect_failed
internal_error
```

错误消息不得泄露内部路径、SQL、Token 或堆栈。

---

## 十四、副作用记录

写操作必须记录：

-目标；
-类型；
-结果；
-可逆性；
-外部 ID；
-前状态 Hash；
-后状态 Hash；
-失败阶段。

例如：

-发送消息；
-写记忆；
-保存文件；
-修改角色；
-启动扩展；
-调用 Provider。

---

## 十五、幂等性

为每个 Tool 明确：

```text
idempotent
conditionally_idempotent
non_idempotent
```

非幂等 Tool 不自动重试。

消息发送必须使用 Idempotency Key，防止重复推送。

---

## 十六、超时与取消

所有 Handler 必须接受 `context.Context`。

迁移时排查：

-无 Context 的数据库操作；
-无取消网络调用；
-阻塞 TTS；
-文件操作；
-Provider 调用；
-桌面 IPC。

无法取消的 Legacy 调用必须记录并逐步替换。

---

## 十七、并发和锁

每个 Tool 声明：

-最大并发；
-每角色；
-每会话；
-全局；
-串行要求；
-资源锁。

避免记忆写入、角色修改和消息发送并发冲突。

---

## 十八、兼容层

旧调用：

```text
LegacySkillManager.Execute(skillName)
```

转换为：

```text
ToolExecutionRequest(canonicalToolID)
```

兼容层只允许单向调用，不再执行旧 Handler。

---

## 十九、迁移顺序

建议：

1. 纯读取 Tool；
2. 状态查询 Tool；
3. 记忆 Tool；
4. 消息 Tool；
5. Provider Tool；
6. 文件与资源 Tool；
7. 桌面 Tool；
8. 扩展管理 Tool；
9. 诊断 Tool；
10. 删除旧注册入口。

---

## 二十、双运行防护

迁移一个 Tool 后：

-旧注册立即标记 Frozen；
-模型只看到新 Tool；
-旧调用映射新 Tool；
-旧 Handler 不再直接执行；
-禁止新旧 Tool 同时写副作用。

---

## 二十一、回归基线

对每个 Tool 比较：

-输入；
-输出；
-权限；
-Scope；
-副作用；
-错误；
-性能；
-Token 暴露；
-模型名称；
-渠道行为；
-取消；
-幂等。

行为变化必须写迁移说明。

---

## 二十二、前端

Tool 管理页面统一展示：

-来源：system/amitia-core；
-Module；
-Enabled；
-Scope；
-Permission；
-风险；
-模型可见；
-可执行；
-运行记录；
-错误；
-版本。

不再将内置 Tool 隐藏在旧 Skill 页面。

---

## 二十三、数据库

迁移目标：

```text
extension_definitions
extension_modules
extension_contributions
tool_definitions
runtime_bindings
contribution_enablement_overrides
legacy_id_mappings
```

旧 Tool/Skill 表停止新写。

---

## 二十四、测试要求

覆盖：

-每个内置 Tool；
-Schema；
-Permission；
-Scope；
-模型暴露；
-副作用；
-幂等；
-消息重复；
-Context Cancel；
-Timeout；
-并发；
-旧 ID；
-前端；
-审计；
-性能；
-启动重建；
-Extension Disable 不允许禁用系统关键 Tool 的策略。

---

## 二十五、实施任务

1. 输出内置 Tool 全量清单。
2. 建立稳定 ID 映射。
3. 建模 system/amitia-core。
4. 补齐 Schema。
5. 建立 Permission Matrix。
6. 建立 Scope Matrix。
7. 实现 HostInternalRuntimeAdapter。
8. 分批迁移 Handler。
9. 接入统一 ToolRegistry。
10. 接入 ExecutionSecurityKernel。
11. 接入 SideEffect/Audit。
12. 迁移模型 Projection。
13. 建立兼容 Adapter。
14. 冻结旧 Tool 注册。
15. 重构前端 Tool 页面。
16. 完成全量回归和差异报告。

---

## 二十六、验收标准

1. 所有内置 Tool 有正式 ContributionDefinition。
2. Tool 使用稳定 ID。
3. 模型列表只来自 ToolRegistry。
4. 所有 Tool 经过 ExecutionSecurityKernel。
5. Permission 和 Scope 明确。
6. 写操作有 SideEffect。
7. 非幂等 Tool 不自动重试。
8. 消息发送有幂等防重。
9. 旧 Skill Handler 不再执行。
10. 内置 Tool 与第三方 Tool 使用同一模型。
11. 旧表停止新写。
12. 关键回归通过。
13. 可进入第 50 步 Agent Skill 迁移。

---

## 二十七、执行约束

> 本步骤只迁移内置 Tool，不允许继续保留“系统 Tool 走旧逻辑、第三方 Tool 走新逻辑”的双轨结构。

禁止：

-模型 Prompt 手动拼 Tool；
-旧 Handler 直接执行；
-Tool 隐式读取当前角色；
-无 Schema；
-无权限写操作；
-消息发送无幂等；
-系统 Tool 绕过审计；
-新旧双注册。
