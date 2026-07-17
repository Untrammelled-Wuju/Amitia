# Amitia 扩展生态全链路验收记录

## 1. 审计范围

本记录覆盖旧工具适配、Skill、官方 Plugin、Workshop 指令与工作流、AgentSkills、本地扩展包、权限与隔离、密钥与归档安全、并发和故障恢复、缓存与资源释放、Web/微信/QQ/主动消息、前端、OpenAPI、桌面端以及重启恢复。

状态仅使用：`PASS`、`FIXED`、`FAIL`、`BLOCKED_ENV`、`OUT_OF_SCOPE`。最终结论只能以实际命令、测试、接口或源码证据为依据。

## 2. 目标与验收标准

- 四条核心闭环均可安装、发现、激活、执行、审计、升级、回滚、卸载并在重启后恢复。
- 所有旧工具进入统一 Registry、Executor、权限和 LLM 工具调用链。
- 扩展包在独立数据目录实例间可导入导出，路径、归档、密钥和日志边界安全。
- 空库、旧库、历史库、中断迁移和重复迁移不破坏数据。
- 后端、前端、桌面端的测试、类型检查、构建、竞态检查和静态检查有明确结果。
- 更新桌面端核心后只由 Electron 拉起后端，并完成健康检查。

## 3. 架构清单

| 子系统 | 入口与关键实现 |
| --- | --- |
| 服务启动与关闭 | `backend/cmd/server/main.go`、`backend/cmd/server/services.go` |
| Skill 统一执行 | `backend/internal/extension/runtime.go`、`registry.go`、`executor.go`、`legacy_tool_adapter.go` |
| 权限 | `backend/internal/extension/permission.go`、`repository.go` |
| Plugin | `backend/internal/extension/plugin_manager.go`、`plugin_host.go`、`plugin_repository.go` |
| Workshop | `backend/internal/extension/workshop_service.go`、`workshop_installer.go`、`workflow_executor.go` |
| AgentSkills | `backend/internal/extension/agent_skill_parser.go`、`agent_skill_runtime.go`、`agent_skill_service.go` |
| 扩展包 | `backend/internal/extension/package_parser.go`、`package_installer.go`、`package_lifecycle.go` |
| LLM 工具循环 | `backend/internal/service/chat/compute.go`、`message_llm.go`、`message_pipeline.go` |
| HTTP API | `backend/internal/extension/router.go`、各 `*_handler.go` |
| 前端 | `front/src/pages/extension/` |
| 桌面端 | `desktop/src/main/`、`desktop/resources/core/AmitiaCore.exe` |

## 4. 旧工具枚举

统一 ID 为 `dev.amitia.skill.<name>`；普通工具允许 LLM 与手动触发，记忆读取工具仅允许手动触发。Schema 由原始工具定义转换，并禁止未声明字段。

| name | 原注册域 | capability | 副作用 | 幂等 |
| --- | --- | --- | --- | --- |
| `get_current_time` | 普通 | `runtime.time.read` | 否 | 是 |
| `create_schedule` | 普通 | `scheduler.own.manage` | 是 | 是 |
| `force_voice_reply` | 普通 | `notification.send` | 是 | 是 |
| `save_profile` | 普通 | `memory.candidate.write` | 是 | 是 |
| `save_episodic_memory` | 普通 | `memory.candidate.write` | 是 | 是 |
| `save_memory` | 记忆 | `memory.candidate.write` | 是 | 是 |
| `summarize_memories` | 记忆 | `memory.read` | 否 | 是 |
| `read_need_state` | 记忆 | `runtime.character.read` | 否 | 是 |
| `read_psyche_state` | 记忆 | `runtime.emotion.read` | 否 | 是 |

## 5. 初始验收矩阵

| 编号 | 项目 | 状态 | 当前证据 |
| --- | --- | --- | --- |
| A-01 | 统一 Runtime 在服务构造时初始化并在退出时关闭 | PASS | `services.go` 构造 `extension.NewRuntime`；`main.go` 延迟关闭 Runtime |
| A-02 | 9 个旧工具全部经适配器注册 | PASS | `tool.GetAll()` 与 `tool.GetMemoryTools()` 均由 `RegisterLegacyTools` 遍历 |
| A-03 | LLM 工具调用走 Registry 与 Executor | PASS | `message_llm.go` 调用 `ModelTools` 与 `ExecuteModelTool` |
| A-04 | Plugin 前后置钩子接入聊天主链 | PASS | `compute.go` 调用 BeforePrompt；`message_pipeline.go` 在提交后派发 AfterReply |
| A-05 | 新增远程市场、计费或自动发布 | OUT_OF_SCOPE | 验收规范明确禁止扩大范围 |
| A-06 | 权限系统策略并发读写 | FIXED | `permission.go` 增加 RWMutex；`TestSystemPolicyConcurrentGrantAndEvaluate` 与全仓 race 通过 |
| A-07 | 主动消息背压测试竞态 | FIXED | 用通道同步替代无同步切片与睡眠；目标包及全仓 race 通过 |
| A-08 | OpenAPI 覆盖全部扩展业务路由 | FIXED | 补齐包指标和操作详情；`TestOpenAPICoversExtensionBusinessRoutes` 通过 |
| A-09 | golangci-lint | BLOCKED_ENV | 本机未安装且仓库无本地可执行文件 |
| A-10 | QQ 实际收发 | BLOCKED_ENV | 运行时健康信息为 `qq=disconnected` |
| A-11 | 外部 Provider 真机工具往返 | BLOCKED_ENV | 无可复用的测试用户、角色与会话凭证；未绕过鉴权写入生产数据 |
| A-12 | 微信实际外发 | BLOCKED_ENV | 服务已连接，但没有指定测试接收方，未产生外部消息副作用 |

## 6. 基线构建与测试

`PASS`：

- `go test ./...`：通过。
- `go test -race ./...`：首次发现并修复背压测试竞态；修复后全仓通过。
- `go vet ./...`：通过。
- `go fmt ./...`：完成；`git diff --check` 通过。
- `golangci-lint`：`BLOCKED_ENV`，工具未安装。
- 前端 `typecheck`、3 个 Vitest、生产构建：通过。
- 桌面端 `typecheck`、16 个 Vitest、生产构建：通过。
- 前端和桌面构建存在第三方 PURE 注解、动态导入和大 chunk 警告，不影响构建退出码。

## 7. 数据库迁移

`PASS`：`go test ./internal/migration -count=1` 覆盖迁移前 SQLite/WAL/SHM 备份、备份失败阻断、空库与已有库、事务回滚、重复列、消息序列冲突修复、延迟索引重试和当前历史脚本。桌面核心在真实数据目录连续启动成功，数据库健康为 `ok`，验证重复迁移可重入。

## 8. 启停与 Worker

`PASS`：服务只由 Electron 拉起；核心、Qdrant、SurrealDB 父子关系正确，HTTP 健康为 ready。关闭主窗口按产品设计进入托盘而非退出；`before-quit` 调用 `stopCore`。全仓测试覆盖 outbox、delivery、proactive、reconciliation、队列与取消路径。最终重启前清理项目占用，并验证 18899 由核心监听、3000 未监听。

## 9. Skill Registry 与 Executor

`PASS`：`TestManifestAndRegistryValidation`、`TestIncompatibleSkillCannotEnable`、`TestExecutorSchemaPermissionDisableAndTrigger`、`TestOutputValidationTimeoutCancelPanicAndIdempotency`、`TestRunPersistenceRedactionAndScope` 通过。Executor 有 64 个并发槽位，超时和取消均返回标准错误，panic 被恢复，输入输出均校验 Schema。

## 10. 权限模型

`FIXED`：deny、allow_once、session、character、always 由 Repository 按 ExecutionScope 解析；显式 deny 覆盖系统策略，模型预览不消耗 allow_once，角色和会话隔离测试通过。本轮为系统策略表补充 RWMutex 和并发回归测试。

## 11. LLM 工具调用闭环

`PASS`：`ModelTools` 仅曝光可用且获授权的 LLM 工具；`message_llm.go` 执行模型工具循环并回填工具结果；`TestModelToolPathUsesRuntimeAndHonorsDisable` 验证禁用后不再执行；错误以 RFC 9457 直达。真实外部 Provider 调用为 `BLOCKED_ENV`，原因见矩阵 A-11。

## 12. 官方 Plugin 生命周期

`PASS`：内置官方诊断 Plugin 覆盖 load/enable/disable/unload、before_prompt/after_reply/event/schedule、状态、声明式 Surface 和注册 Skill。`TestPluginRuntimeLifecycleStateAndSurface`、`TestPluginEventPersistenceKeepsValidRedactedJSONAndRoleIsolation` 通过；Runtime Start/Close 管理 PluginManager 生命周期。

## 13. Plugin 故障与熔断

`PASS`：每个钩子独立并发限制、超时和 circuit；`TestPluginCircuitTransitions` 覆盖 closed/open/half-open/recovery，`TestPluginContributionValidation` 覆盖贡献大小与结构边界。

## 14. Workshop 指令

`PASS`：Workshop 指令生成进入 AgentSkills 安装器；资源索引、激活、读取、恢复由 `TestAgentSkillInstallActivateResourceAndRestore` 覆盖，AgentSkills 包生命周期覆盖导出、独立数据库导入、升级、回滚和卸载。

## 15. Workshop 工作流

`PASS`：`TestWorkshopEndToEndInstallExecuteAndRestore` 覆盖生成后安装、执行、升级和 Registry 恢复；编译器覆盖 http、condition、transform、template、call_skill、schedule、notification、memory_candidate、context_contribution 的静态策略，值解析、循环依赖、超时、输出 Schema、受控 live 限制均通过。

## 16. Workshop 三类场景

`PASS`：指令型由 Workshop→AgentSkills 流程覆盖；工作流型由 Workshop E2E 覆盖；混合型由 workflow `call_skill`、AgentSkills 工具映射与包依赖测试联合覆盖。

## 17. AgentSkills 解析

`PASS`：解析矩阵覆盖 BOM、frontmatter、必填字段、名称、重复键、alias/tag、多文档、长度、明文密钥；目录限制、危险 SVG、ZIP traversal/absolute/drive/multiple roots、资源和 OpenAI 元数据映射均通过。脚本仅索引为不可执行资源。

## 18. AgentSkills 运行时

`PASS`：聊天输入可显式选择 AgentSkill；运行时按 catalog→激活正文→受控资源渐进加载，轮次结束持久化 activation trace，工具映射经统一 Registry，缓存和 token 指标有测试覆盖，重建 Runtime 后恢复成功。

## 19. 本地扩展包格式

`PASS`：fixture 覆盖合法 workflow/AgentSkills、checksum 缺失/不匹配/未列文件、签名可信与不可信、traversal、盘符、大小写/Unicode 冲突、可执行文件、嵌套归档、目录层级、文件数量、大小和压缩比限制。签名信任不自动授予 capability。

## 20. 本地扩展包生命周期

`PASS`：`TestPackageWorkflowLifecycle` 与 `TestPackageAgentSkillsLifecycle` 使用独立 SQLite 实例验证 preview→install disabled→enable→execute→export→cross-instance import→upgrade→compare→rollback→uninstall；依赖阻断、配置迁移、一次性会话和操作审计测试通过。

## 21. 用户与上下文隔离

`PASS`：Package preview session 绑定用户；Plugin 事件/状态按角色隔离；Skill run detail 禁止跨角色，manual execution 禁止跨会话，AgentSkills global/character scope 和 permission grant scope 均有测试。

## 22. 密钥与敏感信息

`PASS`：配置加密跨 Repository 重启，读取仅返回 placeholder；run、Plugin event、Workshop report、包导出均做脱敏/拒绝；服务端 secret scan 在导出前执行。`git grep` 的受跟踪文件密钥/私钥模式扫描通过，本文不记录任何真实密钥值。

## 23. 路径与归档安全

`PASS`：包读取拒绝绝对路径、盘符、反斜杠越界、非规范化路径、Windows 保留名、链接/特殊文件、Unicode NFC 与大小写冲突；逐文件、总展开量、压缩比、文件数和深度均有限制。AgentSkills ZIP 有独立路径验证。

## 24. 故障注入

`PASS`：数据库安装失败不污染 Registry，Registry 失败补偿数据库；损坏/越界归档、checksum、签名、超时、取消、panic、输出不匹配均有负向测试。真实磁盘写满未执行，避免破坏宿主，相关路径由失败注入覆盖。

## 25. 并发与竞态

`FIXED`：首次全仓 race 定位到背压测试共享切片；改为 channel 后目标包和 `go test -race ./...` 全部通过。系统权限策略新增 RWMutex，并发 grant/evaluate 测试通过。安装/升级使用事务、乐观锁和一次性 preview session 防止重复消费。

## 26. 性能与资源释放

`PASS`：Skill Executor 64 槽、Plugin 每钩子并发限制、Workflow 步数/时长/响应/深度限制均通过；归档 entry 显式关闭，临时测试目录由 `t.TempDir` 清理，Runtime Close 停止 PluginManager。未发现 race 或测试超时后的泄漏。

## 27. 缓存与重启恢复

`PASS`：Workshop 版本更新使旧权限 checksum 失效；AgentSkills enable/disable 驱逐 prompt cache；Runtime Restore 重建 Workshop/AgentSkills/Package/Plugin 状态。真实桌面核心在已有数据目录启动健康。

## 28. Web、微信、QQ 与主动消息

`PASS`（代码与自动测试）：Web、微信、QQ 共用聊天计算、统一工具循环、reply commit、outbox/delivery 与 Plugin after_reply；delivery/proactive 测试覆盖 channel scope、去重、重试、backpressure、用户输入抢占。真实 QQ 和微信外发分别为 `BLOCKED_ENV`，见 A-10/A-12。

## 29. 前端、OpenAPI 与错误语义

`FIXED`：前端扩展中心含 Packages、Skills、AgentSkills、Plugins、Workshop、Runs 页面，类型检查/测试/构建通过。OpenAPI 为 3.1.0；自动测试对照 Gin 的全部扩展业务路由与文档 path/method，补齐 2 个缺口后通过。实时无认证请求返回 401 与 `application/problem+json`。

## 30. 桌面端核心与 Electron

`PASS`：新 Go 核心构建 SHA-256 与安装后的 `desktop/resources/core/AmitiaCore.exe` 一致，且不同于旧核心；Sidecar 未修改。只启动 Electron 后自动出现 Electron→AmitiaCore→Qdrant/SurrealDB 进程树，18899 健康 ready，3000 未监听。窗口关闭按设计驻留托盘；完整重启使用项目进程清理后再次由 Electron 拉起。

## 31. 最终结论

结论：`PASS_WITH_BLOCKED_ENV`。发现的 3 个缺陷均已修复并由回归测试锁定；当前无已知 `FAIL`。阻塞项仅为本机缺少 golangci-lint、QQ 未连接、没有可安全复用的外部 Provider 会话 fixture、未指定微信测试接收方；这些均未伪报为通过。远程市场、计费和自动发布保持 `OUT_OF_SCOPE`。
