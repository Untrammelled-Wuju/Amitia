# Amitia 扩展系统重构第 5 步实施文档

## 第 5 步：补齐现有系统基线测试

---

## 一、步骤目标

在第 2 步已经建立现有系统调用链地图、第 3 步已经建立数据表与资源归属清单、第 4 步已经完成“保留、重写、迁移、删除”范围划分的基础上，为当前 Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` 扩展包、Workshop 和扩展中心建立可重复执行的基线测试。

本步骤的目标不是提升旧系统质量，也不是为旧架构补齐完整测试覆盖，而是建立一套足以支撑后续重构的行为基线，确保未来能够判断：

1. 哪些行为在重构前真实存在；
2. 哪些行为必须保持兼容；
3. 哪些行为本身就是错误，不应被新系统继承；
4. 哪些数据迁移前后必须保持一致；
5. 哪些链路重构后出现了回归；
6. 哪些旧系统可以安全删除；
7. 哪些历史兼容层已经不再被调用；
8. 新 Extension Kernel 是否覆盖了现有最低可用能力。

本步骤完成后，项目必须具备：

> 一套能够验证旧系统现状、一套能够保护迁移过程、一套能够验收新 Extension Kernel 的基线测试集合。

---

## 二、执行原则

## 1. 测试现有真实行为，不测试理想设计

测试必须以当前源码实际行为为准。

不得根据未来设计预先修改预期结果。

例如：

- 当前 Agent Skill 被包装为 `SkillDefinition`，测试应记录这一真实链路；
- 当前 MCP Tool 经过 MCP Skill Runtime 注册，测试应记录这一真实链路；
- 当前 Plugin 只能通过 Go Factory 注册，测试应验证这一限制；
- 当前 `.amitiax` 只支持 Workflow/Instructions，测试应验证该限制；
- 当前多个 Enabled 状态共同决定 Tool 可见性，测试应覆盖该组合。

若现有行为本身错误，应：

1. 编写能够复现问题的测试；
2. 将其标记为“已知错误基线”；
3. 明确新系统不应保持该错误行为；
4. 不得为了让测试通过而修改业务代码。

---

## 2. 区分三类测试

### A. 行为基线测试

记录当前系统真实行为。

用途：

- 防止无意回归；
-帮助确认重构差异；
-建立迁移前后对照。

### B. 安全不变量测试

验证无论旧系统还是新系统都必须保持的安全规则。

例如：

- 路径穿越必须被拒绝；
-Secret 不得明文日志输出；
-超时必须终止执行；
-权限拒绝不得继续执行；
-卸载不得误删用户资产；
-禁用后不得继续调用高风险能力。

### C. 迁移验收测试

验证旧数据、旧配置和旧包迁移到新 Extension Kernel 后保持必要语义。

例如：

- 旧 Agent Skill 仍能读取；
-旧 MCP Server 配置不丢失；
-旧 Workflow 可被转换；
-旧权限映射正确；
-旧扩展包历史版本可追溯；
-迁移后不再写入旧表。

---

## 3. 测试不得扩展旧系统职责

本步骤允许：

- 新增测试代码；
-新增测试夹具；
-新增 Mock；
-新增 Fake Server；
-新增测试数据库；
-新增测试脚本；
-新增测试说明；
-新增只用于测试的辅助函数。

本步骤禁止：

- 为测试方便新增永久业务接口；
-为测试方便新增生产数据表；
-为测试方便修改现有权限；
-为测试方便放宽安全校验；
-为测试方便改变启动顺序；
-为测试方便暴露内部 Secret；
-为测试方便增加旧系统功能。

---

## 三、测试分层

## 1. 单元测试

用于验证：

- 纯函数；
-Parser；
-Schema；
-转换逻辑；
-权限判定；
-作用域判定；
-版本比较；
-Checksum；
-签名；
-路径安全；
-Tool Schema；
-Workflow Value；
-状态迁移。

要求：

- 不依赖网络；
-不依赖真实 MCP Server；
-不依赖 Electron；
-不依赖外部数据库；
-执行快速；
-失败定位明确。

---

## 2. 组件测试

用于验证一个完整子系统内部的多个对象协作。

例如：

- Extension Registry + Executor；
-Agent Skill Parser + Repository；
-MCP Manager + Fake Transport；
-PluginManager + Diagnostic Plugin；
-Package Parser + Installer；
-Workflow Compiler + Executor。

要求：

- 使用临时数据库；
-使用临时文件目录；
-使用 Fake Clock；
-使用 Mock Host；
-测试结束后清理全部资源。

---

## 3. 集成测试

用于验证跨子系统链路。

例如：

```text
MCP Server
→ Discovery
→ MCP Tool
→ Extension Registry
→ Model Tool
→ Execute
→ Audit
```

以及：

```text
Agent Skill
→ MCP Dependency
→ Server 创建
→ Tool Allowlist
→ Prompt 激活
```

要求：

- 覆盖真实 Repository；
-覆盖真实 Service；
-覆盖关键 API；
-不依赖公网；
-外部服务使用本地 Fake Server。

---

## 4. API 测试

用于验证前端实际调用的 HTTP 接口。

覆盖：

- 状态码；
-请求 Schema；
-响应 Schema；
-权限；
-作用域；
-错误码；
-幂等；
-分页；
-删除；
-恢复；
-冲突；
-不存在资源。

---

## 5. 前端组件测试

用于验证：

- 扩展中心页面；
-MCP 页面；
-Agent Skill 页面；
-Plugin 页面；
-Package 页面；
-Workshop；
-Schema Surface Renderer；
-安装与权限确认；
-状态展示；
-错误状态；
-路由跳转。

要求：

- 不以截图作为唯一判断；
-验证组件行为；
-验证 API 调用；
-验证按钮可用性；
-验证加载、错误和空状态。

---

## 6. Electron 桌面集成测试

当前 Amitia 只做桌面端，因此必须覆盖：

- 应用启动；
-前端加载；
-后端拉起；
-扩展中心访问；
-本地文件选择；
-`.amitiax` 导入；
-MCP stdio 子进程；
-退出时子进程清理；
-重启恢复；
-开发数据目录；
-多窗口或插件页面行为。

---

## 7. 迁移测试

用于验证旧数据向新结构转换。

当前步骤只建立测试框架和测试样本，不实施正式迁移。

迁移测试必须支持：

```text
旧数据库快照
→ 迁移器
→ 新数据库
→ 行为验证
→ 旧表只读验证
→ 新表写入验证
```

---

## 四、测试环境要求

## 1. Go Toolchain

项目当前声明的 Go 版本必须固定并可重复安装。

需要：

- 记录 `go.mod` 中的版本；
-在 CI 中使用明确版本；
-禁止依赖自动下载不可用 Toolchain；
-若当前环境无法安装，必须记录阻塞原因；
-不得伪报测试已通过。

建议提供：

```text
scripts/test/check-go-version.*
scripts/test/bootstrap-go.*
```

---

## 2. 数据库

测试必须使用隔离数据库。

要求：

- 每个测试套件独立数据库；
-支持自动迁移；
-支持事务回滚；
-支持 SQLite 临时库或独立测试库；
-不得连接用户生产数据库；
-不得复用真实 `amitiaData`；
-测试结束后清理。

需要固定：

- 数据库版本；
-字符集；
-外键行为；
-时间字段精度；
-JSON 行为；
-事务隔离。

---

## 3. 文件系统

所有扩展文件测试必须使用临时目录。

必须覆盖：

- Windows 路径；
-Unix 路径；
-相对路径；
-绝对路径；
-大小写差异；
-Unicode 路径；
-长路径；
-路径穿越；
-符号链接；
-压缩炸弹；
-重复文件名；
-特殊文件。

---

## 4. 时间与定时任务

Plugin Schedule、重连、超时、重试和回滚测试不得依赖真实等待。

应提供：

- Fake Clock；
-可控 Timer；
-可推进时间；
-固定时区；
-固定当前时间；
-可预测 Next Run。

---

## 5. 网络与外部协议

不得依赖公网。

必须提供本地 Fake MCP Server，支持：

- stdio；
-Streamable HTTP；
-initialize；
-tools/list；
-tools/call；
-resources/list；
-prompts/list；
-completion；
-tasks；
-sampling；
-elicitation；
-故障注入；
-断线；
-超时；
-重连；
-非法响应。

---

## 6. Electron

测试必须使用独立用户数据目录。

要求：

- 禁止读取真实用户配置；
-禁止连接真实模型；
-禁止发送真实消息；
-禁止启动真实外部工具；
-退出后检查后端和 MCP 子进程；
-测试失败后清理残留进程。

---

## 五、Extension Registry 与 Executor 基线测试

重点文件：

```text
backend/internal/extension/registry.go
backend/internal/extension/executor.go
backend/internal/extension/permission.go
backend/internal/extension/runtime.go
```

必须覆盖以下测试。

### 1. Registry 注册

验证：

- 正常注册；
-重复 ID；
-重复模型名称；
-不同 Source；
-相同 ID 不同 Scope；
-注册顺序；
-覆盖策略；
-移除；
-重新注册；
-并发注册。

### 2. Tool 可见性

验证：

- 全局启用；
-角色启用；
-会话作用域；
-禁用；
-权限未授权；
-来源不可用；
-MCP 未连接；
-Plugin 未启用；
-Workflow 无依赖；
-模型名称冲突。

### 3. Executor

验证：

- 参数 Schema；
-缺少必填参数；
-错误类型；
-超时；
-取消；
-Panic；
-返回错误；
-幂等；
-并发限制；
-结果标准化；
-副作用记录；
-审计写入；
-敏感字段脱敏。

### 4. Permission

验证：

- 允许；
-拒绝；
-仅本次；
-始终允许；
-不同角色；
-不同会话；
-权限撤销；
-高风险能力；
-缺少声明；
-Manifest 权限与运行调用不一致。

### 5. 关闭与恢复

验证：

- Runtime 关闭；
-重复关闭；
-Registry 清理；
-Worker 停止；
-恢复后能力重新注册；
-损坏状态隔离；
-恢复失败不影响主程序启动。

---

## 六、Legacy Tool 基线测试

重点文件：

```text
backend/internal/extension/legacy_tool_adapter.go
backend/internal/agent/tool/
```

必须覆盖：

- 每个内置工具是否被适配；
-Tool ID；
-模型名称；
-参数 Schema；
-描述；
-副作用；
-权限；
-Handler；
-错误转换；
-结果转换；
-重复注册；
-Host 缺失；
-工具不可用；
-调用是否经过统一 Executor。

必须生成基线快照：

```text
内置工具数量
每个工具 ID
每个模型名称
每个 Input Schema Hash
每个权限声明
每个副作用等级
```

该快照用于后续迁移对比。

---

## 七、Agent Skill 基线测试

重点文件：

```text
backend/internal/extension/agent_skill_parser.go
backend/internal/extension/agent_skill_service.go
backend/internal/extension/agent_skill_runtime.go
backend/internal/extension/agent_skill_handler.go
backend/internal/extension/agent_skill_repository.go
```

## 1. SKILL.md 解析

覆盖：

- 合法 Frontmatter；
-缺少 Frontmatter；
-字段缺失；
-未知字段；
-多语言；
-特殊字符；
-超长内容；
-非法编码；
-空文件；
-重复资源；
-不安全路径；
-脚本文件；
-资源文件；
-OpenAI 元数据；
-Amitia 元数据。

## 2. 导入

覆盖：

- ZIP；
-目录；
-重复导入；
-同版本；
-高版本；
-低版本；
-损坏 ZIP；
-Checksum 错误；
-超限文件；
-压缩炸弹；
-路径穿越；
-权限预览；
-MCP 依赖预览。

## 3. 作用域与启用

覆盖：

- 全局；
-角色；
-禁用；
-重新启用；
-角色删除；
-不存在角色；
-重复绑定；
-数据库与 Cache 不一致。

## 4. Prompt 激活

覆盖：

- 自动激活；
-显式激活；
-不相关消息；
-多个技能；
-Token 超限；
-资源渐进读取；
-指令顺序；
-缓存；
-重复轮次；
-激活记录；
-Hook 冲突。

## 5. 资源读取

覆盖：

- 正常读取；
-不存在资源；
-路径越权；
-超长资源；
-二进制资源；
-Asset；
-Reference；
-Script；
-并发读取；
-删除后读取；
-卸载后读取。

## 6. 删除

覆盖：

- Metadata；
-Artifact；
-Cache；
-Round State；
-角色绑定；
-MCP Dependency；
-激活记录；
-共享资源；
-删除失败补偿。

## 7. 伪 Skill 基线

必须明确测试并记录：

- Agent Skill 被注册为 `SkillDefinition`；
-是否存在 Handler；
-模型是否直接看到；
-内部激活工具如何调用；
-资源工具如何调用。

该测试未来应在新系统中被替代，而不是永久保留。

---

## 八、MCP 基线测试

重点目录：

```text
backend/internal/mcp/
backend/internal/mcpapi/
```

## 1. Server CRUD

覆盖：

- 创建；
-读取；
-更新；
-删除；
-重复名称；
-重复配置；
-Transport 切换；
-角色绑定；
-禁用；
-启用；
-Secret；
-Header；
-Environment；
-Command；
-Args；
-URL。

## 2. stdio

覆盖：

- 正常启动；
-命令不存在；
-权限不足；
-进程退出；
-输出非法 JSON；
-stderr；
-超时；
-取消；
-父进程退出；
-Windows 进程树；
-Unix 进程组；
-残留进程清理。

## 3. Streamable HTTP

覆盖：

- initialize；
-session；
-header；
-auth；
-超时；
-断线；
-非法状态码；
-重试；
-重连；
-服务端通知；
-并发调用。

## 4. OAuth

覆盖：

- 登录流程；
-Token 保存；
-Token 刷新；
-过期；
-撤销；
-State 校验；
-回调失败；
-Secret 日志脱敏；
-删除 Server 后 Token 处理。

## 5. Discovery

覆盖：

- tools/list；
-resources/list；
-prompts/list；
-resource templates；
-空列表；
-重复工具；
-工具更新；
-工具删除；
-Server 重连；
-非法 Schema；
-超大列表；
-部分失败。

## 6. Tool 注册

覆盖：

```text
Discovery
→ MCP Tool
→ Skill Runtime Adapter
→ Extension Registry
```

验证：

- Tool ID；
-模型名称；
-Input Schema；
-Enabled；
-Scope；
-角色；
-重复注册；
-重连注册；
-删除清理；
-禁用清理。

## 7. Tool 调用

覆盖：

- 正常结果；
-文本；
-结构化内容；
-资源引用；
-错误；
-超时；
-取消；
-长任务；
-敏感信息；
-副作用；
-审计；
-连接断开；
-重连后调用。

## 8. MCP Feature

覆盖：

- Resources；
-Prompts；
-Completion；
-Tasks；
-Sampling；
-Elicitation；
-Roots；
-Host Interaction。

## 9. Dependency Service

覆盖：

- Agent Skill 声明 MCP；
-复用现有 Server；
-创建新 Server；
-用户确认；
-stdio 风险；
-HTTP 风险；
-Tool Allowlist；
-角色作用域；
-卸载；
-共享 Server；
-引用计数；
-删除失败。

---

## 九、Plugin 基线测试

重点文件：

```text
backend/internal/extension/plugin_protocol.go
backend/internal/extension/plugin_registry.go
backend/internal/extension/plugin_manager.go
backend/internal/extension/plugin_host.go
backend/internal/extension/plugin_service.go
backend/internal/extension/plugin_repository.go
backend/internal/extension/plugin_surface.go
backend/internal/extension/plugin_builtin_diagnostic.go
```

## 1. Factory 与 Registry

覆盖：

- builtin 注册；
-非 builtin 拒绝；
-重复 Plugin ID；
-Manifest 无效；
-Factory 返回 nil；
-Factory Panic；
-加载顺序；
-禁用状态。

## 2. 生命周期

覆盖：

- Load；
-Enable；
-Disable；
-Unload；
-Reload；
-Start；
-Stop；
-重复调用；
-中途失败；
-状态迁移失败；
-恢复失败；
-部分加载。

## 3. Hook

覆盖：

- BeforePrompt；
-AfterReply；
-顺序；
-优先级；
-超时；
-Panic；
-并发限制；
-熔断；
-熔断恢复；
-禁用；
-作用域；
-返回内容；
-Token 增长；
-主聊天链降级。

## 4. Event

覆盖：

- 事件创建；
-持久化；
-投递；
-重试；
-重复投递；
-最大深度；
-不存在 Plugin；
-禁用；
-Handler 错误；
-Worker 关闭；
-崩溃恢复。

## 5. Schedule

覆盖：

- 创建；
-更新；
-删除；
-触发；
-重复触发；
-禁用；
-Plugin 删除；
-应用重启；
-Fake Clock；
-错误重试；
-Next Run。

## 6. State

覆盖：

- 读取；
-写入；
-CAS；
-版本冲突；
-并发；
-敏感数据；
-禁用后读取；
-删除后保留；
-迁移。

## 7. Host API

覆盖：

- 命名空间；
-权限；
-调用其他 Skill；
-状态；
-事件；
-定时任务；
-配置；
-运行快照；
-未声明能力；
-越权作用域；
-深度限制。

## 8. Surface

覆盖：

- Form；
-Action；
-Status；
-Table；
-字段类型；
-Secret；
-无效 Schema；
-未知组件；
-前端渲染；
-Action 提交；
-错误返回。

必须记录当前 Surface 只属于插件详情管理页，而非完整 UI 扩展系统。

---

## 十、Workflow 基线测试

重点文件：

```text
backend/internal/extension/workflow_compiler.go
backend/internal/extension/workflow_executor.go
backend/internal/extension/workflow_values.go
backend/internal/extension/workflow_*.go
```

## 1. 编译

覆盖：

- 合法定义；
-缺少节点；
-重复节点；
-循环；
-不存在依赖；
-Input Schema；
-Output Schema；
-类型错误；
-默认值；
-表达式；
-引用；
-条件；
-分支；
-并行；
-超限。

## 2. 执行

覆盖：

- 顺序执行；
-条件；
-失败；
-重试；
-超时；
-取消；
-并发；
-结果聚合；
-副作用；
-审计；
-部分完成；
-幂等；
-调用其他 Tool。

## 3. Host

覆盖：

- Schedule；
-Notification；
-Memory Candidate；
-Context Contribution；
-工具调用；
-Host 缺失；
-权限拒绝；
-角色作用域；
-会话作用域。

## 4. Package 与 Workshop 来源

覆盖：

- 包安装的 Workflow；
-Workshop 生成的 Workflow；
-用户直接创建；
-升级；
-回滚；
-卸载；
-所有权；
-历史记录。

---

## 十一、`.amitiax` v1 包基线测试

重点文件：

```text
backend/internal/extension/package_archive.go
backend/internal/extension/package_parser.go
backend/internal/extension/package_service.go
backend/internal/extension/package_installer.go
backend/internal/extension/package_lifecycle.go
backend/internal/extension/package_recovery.go
backend/internal/extension/schema/manifest.schema.json
```

## 1. Archive 安全

覆盖：

- 合法 ZIP；
-路径穿越；
-绝对路径；
-符号链接；
-压缩炸弹；
-超大文件；
-文件总量；
-重复路径；
-Unicode 冲突；
-大小写冲突；
-特殊文件；
-非法扩展名；
-嵌套 ZIP。

## 2. Manifest

覆盖：

- 合法 Workflow；
-合法 Instructions；
-Plugin Kind；
-未知 Kind；
-缺少字段；
-版本；
-兼容范围；
-依赖；
-权限；
-签名；
-发布者；
-错误 Schema；
-额外字段。

必须记录：

> Manifest Schema 中声明能力与 Parser 实际允许能力之间的差异。

## 3. Preview

覆盖：

- 权限预览；
-风险预览；
-依赖；
-版本冲突；
-签名；
-发布者；
-配置变化；
-Session 过期；
-重复 Preview。

## 4. Install

覆盖：

- Workflow；
-Instructions；
-重复安装；
-高版本；
-低版本；
-同版本；
-失败补偿；
-Artifact；
-Registry；
-Agent Skill；
-审计；
-配置；
-权限。

## 5. Upgrade

覆盖：

- 正常升级；
-权限增加；
-权限减少；
-配置迁移；
-依赖变化；
-失败回滚；
-旧版本保留；
-数据一致性。

## 6. Rollback

覆盖：

- 正常回滚；
-Artifact 缺失；
-Checksum 错误；
-数据库失败；
-Registry 恢复；
-Agent Skill 恢复；
-配置恢复；
-历史记录。

## 7. Uninstall

覆盖：

- 正常卸载；
-依赖引用；
-共享资源；
-Artifact；
-历史记录；
-权限；
-配置；
-Secret；
-失败补偿；
-用户资产。

## 8. Restore

覆盖：

- 应用重启；
-正常包；
-损坏包；
-Artifact 缺失；
-Registry 冲突；
-部分恢复；
-恢复隔离；
-审计。

---

## 十二、Workshop 基线测试

重点文件：

```text
backend/internal/extension/workshop_*.go
front/src/views/extensions/workshop/
```

必须覆盖：

- 创建 Session；
-Revision；
-草稿；
-AI 生成；
-手工修改；
-测试；
-编译；
-权限预览；
-安装；
-失败；
-恢复；
-删除 Session；
-已安装扩展；
-临时文件；
-Artifact；
-与 PackageService 的关系；
-与 Registry 的关系。

---

## 十三、前端扩展中心基线测试

重点目录：

```text
front/src/views/extensions/
front/src/views/mcp/
front/src/router/index.ts
front/src/navigation/app-nav.ts
front/src/components/SideNav.vue
```

## 1. 路由

验证：

- 每个扩展页面可访问；
-主导航入口；
-直接 URL；
-不存在资源；
-返回；
-刷新；
-权限；
-路由参数。

## 2. API 状态

验证：

- Loading；
-Empty；
-Error；
-Retry；
-分页；
-刷新；
-重复提交；
-错误码；
-网络断开。

## 3. 状态展示

验证：

- Package 已安装；
-Plugin 已启用；
-Agent Skill 已启用；
-MCP 已连接；
-Tool 已启用；
-权限已授权；
-连接异常；
-恢复中；
-卸载中。

必须记录前端状态与后端真实状态不一致的情况。

## 4. 关键交互

覆盖：

- 导入 `.amitiax`；
-安装；
-权限确认；
-升级；
-回滚；
-卸载；
-Agent Skill 导入；
-MCP 创建；
-MCP 连接；
-Tool 启停；
-Plugin 启停；
-Workflow 运行；
-Workshop 安装。

## 5. Schema Surface Renderer

覆盖所有现有组件类型和错误降级。

---

## 十四、Electron 桌面端基线测试

必须覆盖：

### 1. 启动

```text
Electron
→ 拉起 Go 后端
→ 前端加载
→ Extension Runtime 恢复
→ MCP Manager 恢复
```

### 2. 数据目录

验证：

- 使用隔离测试目录；
-目录自动创建；
-路径中包含空格；
-中文路径；
-只读目录；
-磁盘空间不足；
-目录损坏。

### 3. 本地包导入

验证：

- 文件选择器；
-取消选择；
-错误文件；
-大文件；
-重复导入；
-安装结果。

### 4. MCP stdio

验证：

- 子进程启动；
-窗口关闭；
-应用退出；
-异常退出；
-进程树清理；
-重启恢复。

### 5. 崩溃恢复

验证：

- 后端崩溃；
-Plugin Worker 崩溃；
-MCP 子进程崩溃；
-扩展恢复失败；
-损坏包；
-安全模式或隔离行为。

---

## 十五、已知错误基线

对当前已经确认的架构问题，应建立“已知错误基线”。

至少包括：

1. Agent Skill 被包装成伪 Skill；
2. MCP Tool 经 MCP Skill Runtime 注册；
3. Manifest 支持 Plugin，但包解析器不支持；
4. Plugin 只能 builtin；
5. Plugin Surface 不是完整 UI 扩展；
6. 多套 Enabled 状态；
7. MCP 与 Extension 分别维护生命周期；
8. Package 与 Plugin Runtime 未接通；
9. 扩展中心页面分散；
10. 启动装配层手动维护恢复顺序。

这些测试应使用以下标记：

```text
KNOWN_LEGACY_BEHAVIOR
KNOWN_ARCHITECTURE_DEFECT
MIGRATION_ONLY
DELETE_AFTER_KERNEL_SWITCH
```

不得把这些测试纳入新系统长期兼容要求。

---

## 十六、测试数据夹具

建议建立：

```text
backend/testdata/extensions/
├── agent-skills/
│   ├── valid/
│   ├── invalid/
│   ├── oversized/
│   ├── path-traversal/
│   └── mcp-dependent/
├── packages-v1/
│   ├── workflow-valid/
│   ├── instructions-valid/
│   ├── plugin-declared/
│   ├── signature-invalid/
│   └── archive-malicious/
├── workflows/
├── plugins/
├── mcp/
│   ├── stdio-server/
│   ├── http-server/
│   ├── invalid-server/
│   └── reconnect-server/
└── database-snapshots/
    ├── empty/
    ├── minimal/
    ├── full/
    ├── inconsistent/
    └── orphaned/
```

前端建议建立：

```text
front/src/test/fixtures/extensions/
```

Electron 建议建立：

```text
electron/test/fixtures/extensions/
```

所有夹具必须：

- 不包含真实 Secret；
-不包含真实用户聊天；
-不包含生产数据库；
-可公开提交；
-可重复生成；
-有用途说明。

---

## 十七、测试命名规范

建议统一：

```text
TestLegacy_<Subsystem>_<Behavior>
TestSecurity_<Subsystem>_<Invariant>
TestMigration_<Source>_To_<Target>
TestIntegration_<Start>_To_<End>
```

示例：

```text
TestLegacy_MCPTool_DiscoveryRegistersSkill
TestLegacy_AgentSkill_RegistersInstructionSkill
TestSecurity_PackageArchive_RejectsPathTraversal
TestSecurity_PluginHost_RejectsUndeclaredCapability
TestMigration_AgentSkillV1_ToContributionV2
TestIntegration_MCPServer_ToModelToolCall
```

前端：

```text
<Feature>.legacy.spec.ts
<Feature>.security.spec.ts
<Feature>.migration.spec.ts
```

---

## 十八、测试标签与执行分组

建议划分：

```text
unit
component
integration
api
frontend
electron
security
migration
legacy
slow
mcp
package
plugin
agent-skill
workflow
```

CI 至少分为：

### 快速检查

- Unit；
-Schema；
-Parser；
-Permission；
-Archive；
-前端类型检查。

### 后端集成

- 数据库；
-Extension；
-Agent Skill；
-Plugin；
-Workflow；
-Package。

### MCP 集成

- stdio；
-HTTP；
-OAuth Mock；
-Discovery；
-Tool Call；
-Reconnect。

### 桌面端

- Electron 启动；
-包导入；
-子进程清理；
-退出恢复。

### 安全

- Archive；
-权限；
-Secret；
-路径；
-命令；
-越权；
-资源泄漏。

---

## 十九、覆盖率要求

本步骤不追求整体高覆盖率，但关键链路必须覆盖。

最低要求：

| 模块 | 关键路径覆盖 |
|---|---:|
| Package Archive/Security | 90% |
| Package Parser | 85% |
| Permission | 90% |
| Executor | 85% |
| Agent Skill Parser | 85% |
| MCP Transport/Client 核心 | 80% |
| Plugin Lifecycle 核心 | 80% |
| Workflow Compiler | 80% |
| Migration Helpers | 90% |

不要求旧前端页面达到统一行覆盖率，但关键交互必须有行为测试。

不得为了覆盖率编写无意义断言。

---

## 二十、失败与不稳定测试处理

### 1. 不得静默跳过

测试因环境无法运行时，必须记录：

- 缺少什么；
-为什么缺少；
-影响哪些链路；
-如何补齐；
-是否阻塞后续步骤。

### 2. Flaky 测试

必须：

- 标记原因；
-禁止简单重复执行掩盖；
-使用 Fake Clock；
-移除公网依赖；
-固定随机种子；
-清理共享状态；
-隔离端口；
-隔离临时目录。

### 3. 已知失败

允许暂时保留，但必须：

- 有 Issue/文档编号；
-有明确原因；
-有目标步骤；
-不得计入“测试通过”。

---

## 二十一、CI 与脚本

建议新增：

```text
scripts/test/
├── run-extension-unit.*
├── run-extension-integration.*
├── run-mcp-integration.*
├── run-extension-security.*
├── run-extension-migration.*
├── run-extension-frontend.*
├── run-extension-electron.*
└── cleanup-extension-test-processes.*
```

CI 至少执行：

```text
扩展核心单元测试
扩展数据库集成测试
MCP Fake Server 集成测试
包安全测试
前端类型检查与组件测试
Electron 最小启动测试
```

CI 必须在失败时保留：

- 测试日志；
-临时数据库副本；
-进程列表；
-应用日志；
-MCP Server 日志；
-截图或前端错误信息。

不得保留 Secret。

---

## 二十二、实施任务

### Task 1：建立测试基线目录与命名规范

创建测试目录、标签、夹具目录和统一命名规则。

### Task 2：建立测试环境

完成：

- 固定 Go Toolchain；
-临时数据库；
-临时数据目录；
-Fake Clock；
-Fake MCP Server；
-Mock Host；
-Electron 测试目录。

### Task 3：补齐 Extension Registry 与 Executor 测试

覆盖注册、可见性、执行、权限、作用域、超时、审计和关闭。

### Task 4：补齐 Legacy Tool 测试

生成内置工具基线快照。

### Task 5：补齐 Agent Skill 测试

覆盖解析、导入、启用、Prompt、资源、删除和 MCP 依赖。

### Task 6：补齐 MCP 测试

覆盖 Server、Transport、OAuth、Discovery、Tool 注册、Tool 调用、重连和 Features。

### Task 7：补齐 Plugin 测试

覆盖 builtin 注册、生命周期、Hook、Event、Schedule、State、Host API 和 Surface。

### Task 8：补齐 Workflow 测试

覆盖编译、执行、Host、权限、来源和所有权。

### Task 9：补齐 `.amitiax` v1 测试

覆盖 Archive、Manifest、Preview、Install、Upgrade、Rollback、Uninstall、Restore。

### Task 10：补齐 Workshop 测试

覆盖 Session、Revision、测试、安装和资源清理。

### Task 11：补齐前端扩展中心测试

覆盖路由、API、状态、关键交互和错误降级。

### Task 12：补齐 Electron 测试

覆盖启动、数据目录、本地包、MCP 子进程、退出和恢复。

### Task 13：建立已知错误基线

将历史架构问题明确标记，避免新系统错误继承。

### Task 14：建立 CI 分组

完成快速、集成、MCP、桌面、安全和迁移测试分组。

### Task 15：输出测试覆盖与阻塞报告

明确：

- 已通过；
-已知失败；
-环境阻塞；
-未覆盖；
-后续补齐步骤。

---

## 二十三、建议文档结构

建议新增：

```text
docs/extension-kernel/
├── 05-legacy-baseline-tests.md
├── testing/
│   ├── strategy.md
│   ├── environment.md
│   ├── fixtures.md
│   ├── naming.md
│   ├── ci-groups.md
│   ├── known-legacy-behaviors.md
│   ├── known-failures.md
│   └── coverage-report.md
└── reports/
    ├── extension-test-results.md
    ├── mcp-test-results.md
    ├── frontend-test-results.md
    ├── electron-test-results.md
    └── blocked-tests.md
```

---

## 二十四、本步骤不做的事情

本步骤明确不做：

- 不修复旧架构；
-不修改领域模型；
-不实现 Tool/Capability 新类型；
-不实现 Extension Kernel；
-不迁移数据库；
-不修改 `.amitiax` Manifest；
-不删除旧 Adapter；
-不重写 Plugin；
-不重写 MCP；
-不重写 Agent Skill；
-不重建扩展中心；
-不改变当前功能；
-不引入真实外部 MCP 服务；
-不使用真实用户数据。

---

## 二十五、验收产物

完成后必须提交：

### 1. 测试策略主文档

```text
docs/extension-kernel/05-legacy-baseline-tests.md
```

### 2. 测试环境

包含：

- 固定 Toolchain；
-临时数据库；
-临时文件目录；
-Fake Clock；
-Fake MCP Server；
-Mock Host；
-Electron 测试配置。

### 3. 后端基线测试

至少覆盖：

- Extension Registry；
-Executor；
-Permission；
-Legacy Tool；
-Agent Skill；
-MCP；
-Plugin；
-Workflow；
-Package；
-Workshop。

### 4. 前端基线测试

至少覆盖：

- 路由；
-关键页面；
-关键状态；
-安装；
-启停；
-错误降级；
-Schema Surface。

### 5. Electron 基线测试

至少覆盖：

- 启动；
-数据目录；
-包导入；
-MCP 子进程；
-退出清理；
-重启恢复。

### 6. 安全测试

至少覆盖：

- Archive；
-路径；
-签名；
-Secret；
-权限；
-作用域；
-超时；
-资源清理。

### 7. 基线快照

至少包含：

- 内置 Tool 清单；
-MCP Tool Schema；
-Agent Skill Metadata；
-`.amitiax` v1 Manifest；
-旧 API 响应；
-主要数据表结构。

### 8. 已知错误基线清单

每项必须说明：

- 当前行为；
-源码证据；
-对应测试；
-新系统是否保持；
-删除步骤。

### 9. 测试报告

必须区分：

- 通过；
-失败；
-已知失败；
-环境阻塞；
-未执行；
-未覆盖。

---

## 二十六、验收标准

本步骤通过必须同时满足以下条件：

1. 已建立独立测试环境，不使用用户真实数据。
2. 已固定 Toolchain、数据库、文件目录和时间。
3. Extension Registry、Executor 和 Permission 已有基线测试。
4. Legacy Tool 已生成稳定快照。
5. Agent Skill 导入、激活、资源和删除链已覆盖。
6. MCP Server、Transport、Discovery、Tool 注册和 Tool 调用已覆盖。
7. Plugin 生命周期、Hook、Event、Schedule 和 State 已覆盖。
8. Workflow 编译和执行已覆盖。
9. `.amitiax` v1 安装、升级、回滚、卸载和恢复已覆盖。
10. 前端扩展中心关键交互已覆盖。
11. Electron 启动与 MCP 子进程清理已覆盖。
12. 安全不变量测试已建立。
13. 已知架构问题已建立错误基线。
14. 所有无法执行的测试均已如实记录。
15. 测试代码未改变任何旧系统行为。
16. 后续第 6 步可以在测试保护下解除 Skill 概念过载。

---

## 二十七、退出条件

只有满足以下条件后，才能进入第 6 步“解除 Skill 概念过载”：

- 核心调用链已有自动化测试；
-关键数据写入已有测试；
-关键权限和作用域已有测试；
-包安全已有测试；
-MCP Fake Server 可稳定运行；
-Plugin 生命周期已有测试；
-Agent Skill Prompt 链已有测试；
-旧 `.amitiax` 生命周期已有测试；
-Electron 最小链路已有测试；
-已知错误已明确标记；
-测试失败不再依赖人工判断；
-后续重构可以通过测试识别行为差异。

---

## 二十八、执行约束

执行本步骤时必须遵守：

> 测试的任务是记录和保护真实行为，不是替旧系统辩护，也不是让错误架构永久兼容。

对于每一项测试，必须明确它属于：

```text
长期安全不变量
旧系统行为基线
迁移验收
已知错误基线
未来删除验证
```

任何无法归类的测试不得直接加入长期测试套件。

本步骤完成后，旧系统仍然保持冻结状态，不得因为测试补齐而恢复旧系统功能开发。
