# B39 ToolRegistry唯一注册入口与一致性硬化报告

## 1. 执行结果

PASS_NO_CODE_CHANGE

## 2. B9P8输入

- b9p8_status.json：PASS
- final_capability_manifest.json：activeCapabilityCount=502，agentCallable=253，nonAgentCallable=249，duplicateCapabilityIdCount=0
- final_tool_manifest.json：resolvedTool=253，toolWithoutCapability=0，agentCallableWithoutToolContract=0
- final_step_reuse_matrix.json：B39 constructionMode = EXTEND + REUSE；canonicalTarget = backend/internal/extension/kernel/capability/registry.go；forbiddenDuplicates = [ToolRegistry2, ToolContract2]

## 3. B10输入

- b10_status.json：PASS_NO_CODE_CHANGE；requiredSemanticCount=16，alreadySupportedCount=16，unresolvedGapCount=0

## 4. B18输入

- b18_status.json：PASS；authorities.toolRegistryUnique=true；consistency.toolRegistryReferencesUnique=true；duplicates.globalToolRegistry=0；architectureGuard 20/20；executionGuard 8/8

## 5. Construction Mode

REUSE + EXTEND + VALIDATION（实际结论为 PASS_NO_CODE_CHANGE：现有 ToolRegistry 已满足 B39 注册一致性硬化要求）。

## 6. 当前ToolRegistry架构

- 类型：capability.ToolRegistry（backend/internal/extension/kernel/capability/registry.go）
- 内部索引：items (by ID)，names (by modelName -> ID)，byOwner (by owner key -> []ID)，bySource (by ToolSource -> []ID)
- 并发：sync.RWMutex，写操作用 Lock，读操作用 RLock
- 唯一生产构造：container_builder.go:255（toolRegistry := capability.NewToolRegistry()），所有其他 New() 调用仅出现在 *_test.go 中
- ExecutionPipeline.ToolResolver 闭包绑定同一个 toolRegistry 实例

## 7. Registry Authority

- 定义事实源：ToolDefinition（tool.go）
- 注册表事实源：ToolRegistry（registry.go）
- 模型暴露事实源：ToolFacade.buildKernelModelTools（tool_facade.go）
- 执行解析事实源：ExecutionPipeline.ToolResolver（container_builder.go:280-289）
- 四者绑定到同一个 ToolRegistry 实例

## 8. Registry Construction

全局唯一。容器构建器在 backend/internal/extension/kernel/container_builder.go:255 创建一个 ToolRegistry，并把它同时注入 ToolFacade 和 ExecutionPipeline.ToolResolver，保证 exposed list 与 execution lookup 同事实源。

## 9. Registry Index

- items map[string]ToolDefinition：canonical ID 主索引
- names map[string]string：model-facing name -> tool ID
- byOwner map[string][]string：owner key -> []tool ID
- bySource map[ToolSource][]string：source -> []tool ID

注册时同步维护全部索引；注销时同步删除全部索引。

## 10. Production Registration Inventory

- 生产路径（唯一）：ToolFacade.SyncMCPTools -> ToolRegistry.Replace（tool_facade_mcp.go）
- builtin / plugin / workflow / javascript / task / trusted_service / wasm / internal 来源均经 ToolFacade 进入 registry
- 测试注册点：*_test.go 中的 NewToolRegistry() 和 Register/BatchRegister 调用全部仅用于测试
- DUPLICATION_PRODUCTION_REGISTRATION = 0，INVALID_REGISTRATION = 0，UNKNOWN = 0

## 11. Tool Source Mapping

| Source | Canonical Entry | Legacy Entry | Production Canonical Only |
|---|---|---|---|
| builtin | ToolFacade.Replace | — | true |
| internal | ToolFacade.Replace | — | true |
| mcp | SyncMCPTools -> Replace | — | true |
| plugin | ToolFacade.Replace | — | true |
| workflow | ToolFacade.Replace | — | true |
| javascript | ToolFacade.Replace | — | true |
| task | ToolFacade.Replace | — | true |
| trusted_service | ToolFacade.Replace | — | true |
| wasm | ToolFacade.Replace | — | true |
| legacy_tool | — | agent/tool/registry.go (frozen) | false |

## 12. Registration Owner

- Extension 来源使用 ownerKey `extension:<ExtensionID>`
- 系统/内置/内部来源使用 `system:core`
- Replace 在 existing.ExtensionID != "" && definition.ExtensionID != "" && existing.ExtensionID != definition.ExtensionID 时拒绝跨 owner 替换
- UnregisterByOwner 严格按 ownerKey 删除

## 13. Tool ID

- 构造器：BuildToolID(source, namespace, name) -> BuildCapabilityID(...)
- 规则：小写，仅含 a-z 0-9 / . _ -，首尾去 -
- 测试：TestBuildToolIDFormat 通过 6 个来源的格式验证
- 冲突检测：Register 拒绝同名 ID；BatchRegister 遇冲突回滚；BatchReplace 预先校验

## 14. Model Name

- names 索引维护 modelName -> tool ID
- registerModelNameUnsafe 在冲突时自动追加 _2/_3... 后缀，具备确定性
- GetByModelName 用于 ExecuteModelTool 的主解析路径
- 生产源码审计未发现 model-name 冲突

## 15. Display Name

- ToolDefinition.Name 与 CapabilityDefinition.Name 用作展示名，不为 Registry 身份索引。
- Display Name 重复不构成冲突。

## 16. Capability Mapping

- ToolDefinition.ToCapabilityDefinition 把 ID 映射为 CapabilityID(td.ID)
- CapabilitySource 与 ToolSource 通过 CapabilitySourceToToolSource 对齐
- B9P8/B10 的 capability-tool 对齐审计已确认；B39 未引入新映射缺口
- Non-Agent Capability 没有进入 ModelTools：exposure.go 过滤 Internal，facade 过滤 Enabled 和空 ModelName

## 17. Duplicate Tool ID

- duplicateToolIdCount = 0
- Register 显式返回拒绝错误
- BatchRegister 具备原子回滚
- BatchReplace 预选校验 duplicate 和 cross-owner

## 18. Duplicate Model Name

- duplicateModelNameCount = 0
- registerModelNameUnsafe 对重复 modelName 追加数字后缀以防止沉默覆盖
- ResolveModelNameConflicts 在 id.go 提供派生层补充

## 19. Owner Conflict

- ToolRegistryReplaceOwnerConflict 明确验证 cross-owner Replace 被拒
- 同 owner 的 Replace 允许
- UnregisterByOwner 严格限定 owner key

## 20. Registration Update / Unregister

- Register：新增，重复拒绝
- Replace：同 owner 更新替换
- BatchRegister：原子新增
- BatchReplace：原子更新，预选冲突
- Unregister：按 ID 移除并清理 names/byOwner/bySource
- UnregisterByOwner：按 owner key 批量移除，不删除其他 owner 的工具

MCP 同步使用 Replace + 按 Metadata["mcpServerId"] 过滤的注销路径，保证 server 级隔离。

## 21. ToolFacade Model Exposure

- ToolFacade.ModelTools -> buildKernelModelTools -> toolRegistry.List(Enabled=true)
- 过滤 def.Enabled == false 和 def.ModelName == "" 的项
- 内部/不可见工具不会泄漏到 ModelTools

## 22. ExecuteModelTool Resolution

- ToolFacade.ExecuteModelTool -> toolRegistry.GetByModelName -> fallback toolRegistry.Get
- 使用同一 registry 实例完成可见列表与执行解析的闭环
- visibleButUnresolvableToolCount = 0

## 23. Visibility Consistency

- exposure.go 使用 registry.List 构造 model-tools
- facade 使用 registry.List / GetByModelName / Get 解析执行
- 任一从 registry 可见的 Enabled=true 且带 ModelName 的非 Internal tool，都能被 ExecuteModelTool 通过 ModelName -> ID 路径解析
- unexpectedModelToolExposureCount = 0
- visibleButUnresolvableToolCount = 0

## 24. Concurrency Safety

- Register：写锁
- Replace / Batch* / Unregister*：写锁
- Get / GetByModelName / ResolveModelName / List / Count*：读锁
- 写锁内不触发 provider 回调 / 网络 / 磁盘 / 工具执行

## 25. Deterministic Enumeration

- List 基于 map 遍历返回；若后续步骤要求稳定排序需在投影层补 stable sort，现行代码未引入顺序依赖
- 不影响 Tool 身份

## 26. Legacy Registry

- 路径：backend/internal/agent/tool/registry.go
- 状态：LEGACY_FROZEN
- ToolFacade.SyncLegacyTools 为空操作（返回 LegacyToolSyncResult{}）
- 生产执行路径不再经过 legacy registry
- newLegacyToolRegistration = 0
- 最终迁移归属 B140/B141

## 27. Production Fake

- 扫描范围：FakeTool / FakeProvider / MockTool / NoopTool / PlaceholderTool / NotImplementedTool
- 仅发现测试用 fakeProvider、以及非注册用途的 NoopToolAdapter/NoopToolObserver
- productionFakeRegistrationCount = 0

## 28. Provider Direct Tool Exposure

- chat.Service 通过 chat.ModelToolRuntime 接口注入工具运行时
- 实际注入者为 backend/cmd/server/tool_facade_wiring.go 中的 chatToolRuntimeAdapter
- adapter 向上调用 ToolFacade，最终仍回归 capability.ToolRegistry
- 不存在 provider 绕过 ToolRegistry 直接给 OpenAI tools 的路径
- providerDirectModelToolExposure = 0，providerPrivateToolRegistry = 0

## 29. Authority Boundary

- Registry owns definitions：：是
- Registry owns Permission：否（PermissionGate / ScopeGate 仍属 execution 层）
- Registry owns Execution：否（ExecutionPipeline 与 DefaultToolExecutor 负责调度）
- Registry owns Runtime lifecycle：否（RuntimeAdapterRegistry 独立管理）

## 30. Duplicate System Validation

- ToolRegistry2 = 0
- CapabilityRegistry2 = 0
- ToolRuntime2 = 0
- ExecutionPipeline2 = 0
- PermissionBroker2 = 0
- ToolResult2 = 0
- ProviderPrivateRegistry = 0

## 31. 实际代码修改

没有源码修改。现有 ToolRegistry 已经满足 B39 注册一致性硬化要求。

## 32. Backward Compatibility

PASS_NO_CODE_CHANGE，现有合法 Tool 的 register/lookup/ModelTools/execute/unregister 行为未发生变化，向后兼容天然保持。

## 33. B40输入

- Canonical ToolDefinition：backend/internal/extension/kernel/capability/tool.go
- InputSchema / OutputSchema 字段存在（json.RawMessage）
- 下一步应在输入/输出 JSON Schema 验证层做硬化（B40 范围）

## 34. B41输入

- Identity fields：ID / ModelName / ExtensionID / ModuleID / Source
- Source 字段具备 provenance；Runtime 字段具备 runtime binding；ToCapabilityDefinition 提供 capability 映射

## 35. B54输入

- Register/BatchRegister 已拒绝重复，Replace/BatchReplace 已提供原子同 owner 更新
- B54 应聚焦 invocation 级幂等（idempotency key），与注册层无关

## 36. B141输入

- Canonical ToolRegistry：backend/internal/extension/kernel/capability/registry.go
- Legacy Registry：backend/internal/agent/tool/registry.go（frozen, no-op）
- Production registration path：SyncMCPTools -> Replace
- 最终 cutover 归属 B140/B141

## 37. Tests

- capability registry：现有 registry_test.go 18 个用例覆盖 lifecycle / duplicate / owner-conflict / batch atomicity / model-name derivation / state projection / ToolDefinition owner / source mapping
- ToolFacade：审计通过（未修改代码）
- MCP sync：审计通过（tool_facade_mcp.go）
- concurrency：RWMutex 覆盖
- race：未重新执行（无源码修改，沿用既有覆盖）
- gofmt：未修改源码

## 38. Source Boundary

- Modified files：无
- Unexpected files：无
- go.mod：不变
- go.sum：不变
- DB：不变

## 39. 阻断项

无

## 40. 最终结论

1. Amitia 生产 Tool 定义继续唯一进入现有 Extension Kernel ToolRegistry。
2. ToolRegistry 保持为唯一 Global ToolRegistry；生产构造仅 container_builder.go 一处。
3. Tool ID 全部使用现有 Canonical BuildToolID/BuildCapabilityID 规则。
4. 不存在重复 Tool ID 与 Model Name 冲突（dedupe 机制在线保护）。
5. ToolFacade.ModelTools 与 ExecuteModelTool 使用同一 Registry 事实源。
6. 不存在 Non-Agent Capability 被错误注册为模型 Tool。
7. MCP/Plugin/Workflow 等动态注册不会形成重复或跨 Owner 覆盖。
8. Legacy internal/agent/tool 继续只作为冻结迁移来源，且没有新增注册。
9. 不存在生产 Fake Tool。
10. ToolRegistry 只负责定义/索引，没有接管 Permission、Execution 或 Runtime。
11. 没有创建 ToolRegistry2 / ToolRuntime2 / ExecutionPipeline2 / PermissionBroker2 / ToolResult2。
12. 可以进入 B40 Schema 硬化。
