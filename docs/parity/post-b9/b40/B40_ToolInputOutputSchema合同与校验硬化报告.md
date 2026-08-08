# B40 Tool Input / Output Schema合同与校验硬化报告

## 1. 执行结果

PASS_NO_CODE_CHANGE

## 2. B9P8输入

- b9p8_status.json = PASS
- final_capability_manifest.json = 502 active capability; 253 agent-callable
- final_tool_manifest.json = 253 resolved tool
- final_step_reuse_matrix.json 中 B40：primaryMode=EXTEND，canonicalTarget=backend/internal/extension/kernel/execution/pipeline.go，forbiddenDuplicates=[ExecutionPipeline2, NewExecutor]

## 3. B10输入

- b10_status.json = PASS_NO_CODE_CHANGE

## 4. B18输入

- b18_status.json = PASS；authorities 全集 UNIQUE 通过

## 5. B39输入

- b39_status.json = PASS_NO_CODE_CHANGE
- tool_registry_gap_matrix.json 把 richer I/O schema validation 记为 OUTSIDE_B39_SCOPE，futureStep=B40
- B40_input_manifest.json 给出 canonical ToolDefinition、字段名、253 agent-callable tools 等上下文

## 6. Construction Mode

REUSE + EXTEND。最终落地为 PASS_NO_CODE_CHANGE：现有 ToolDefinition/执行管道/Sanitizer 已构成 B40 所需的 Schema 合同与校验骨架，添加 JSON Schema 引擎被 B40 自身约束明确禁止（不得引入新依赖、不得实现完整引擎）。

## 7. 当前Tool Schema架构

- Canonical Schema 存储于 capability.ToolDefinition 的 InputSchema/OutputSchema（json.RawMessage）
- capability.UnifiedToolResult 统一承载 text/structured/binary_reference/resource_reference/stream/task_reference
- capability.ToolError 携带 Code/Message/Retryable/UserVisible/Details
- execution.InputValidator 执行 size-only 输入门控
- execution.ResultValidator 执行 status 补全、empty-content 转 failed、output size 门控
- execution.Sanitizer 对结果中的敏感 key 进行 redact
- ToolFacade + ToolExposureManager 基于同一个 ToolRegistry/同一个 ToolDefinition.Schema 做模型投影

## 8. Schema Authority

唯一权威是 capability.ToolDefinition。无第二 Schema 注册中心 / 第二 Tool Definition 系统 / 第二 Schema Runtime。

## 9. ToolDefinition

tool.go 中的 ToolDefinition 包含完整的静态描述（ID、Source、Permissions、Scope、Runtime）和契约部分（InputSchema、OutputSchema、ExecutionPolicy、ResultPolicy、ModelExposure、ToolVersion、State）。

## 10. Input Schema

- 表现形式：ToolDefinition.InputSchema json.RawMessage（整份 JSON Schema 原样保留）
- 再由 ToolFacade.schemaToParameters 反序列化为 agent/tool/Parameters，作为 OpenAI/Anthropic/Gemini 上的函数声明参数
- 含参数 Tool 的 Schema 由注册方（MCP/Plugin/Builtin）如实写入；无参数 Tool 可写空对象或不设

## 11. Required / Optional

- agent/tool/Parameters 具有 Required []string 字段，投影层保留必填语义
- Canonical ToolDefinition 本身不强制区分 required/optional，由原始 JSON Schema 文本承载

## 12. Primitive Types

- 支持 string / integer / number / boolean 等 JSON Schema 基本类型
- 因当前没有 structurally validate，类型 sanity 由实际 Tool Handler 自行把关

## 13. Object / Array

- InputSchema/OutputSchema 为原始 JSON Schema 文本，支持 object 与 array 的任意嵌套写法
- 实际结构性校验被延后

## 14. Enum / Nullable

- 同上，以原文保存；projection 透传；B40 不新增校验

## 15. Unknown Field Policy

- 当前 UNKNOWN_FIELDS = NOT_REQUIRED
- 结构性 unknown-field 拒绝不在 B40 范围

## 16. Registration Validation

- 当前不作结构性 Schema 校验
- 仅依赖 B39 冻结的身份唯一性约束
- 延后（需要 JSON Schema lib 支持）

## 17. Execution Input Validation

- execution.InputValidator（size only）
- InvocationValidator 校验 InvocationID/ToolID/UserID/Source 必填

## 18. Validation Error Mapping

- 入口错误统一映射到 capability.ErrorCodeInvalidInput
- 失败结果返回 ToolError，不调用 Tool Handler
- Sanitizer 确保错误详情不泄漏敏感 key（redact 为 [redacted]）

## 19. Output Schema

- ToolDefinition.OutputSchema json.RawMessage
- UnifiedToolResult.Structured 用于输出结构化结果
- UnifiedToolResult.Content 用于承载文本、资源引用、二进制引用等

## 20. Structured Output

- 当前 OutputSchema 主要起描述作用
- ResultValidator 校验 status/empty/size，不结构性校验输出形状

## 21. ToolResult关系

- capability.UnifiedToolResult 保持单一
- 无 StructuredToolResult2 / ToolArtifactResult2

## 22. Result Validation

- ResultValidator.Validate 仅在 status / empty-content / size 语义上兜底
- 完整的结构校验归属延后

## 23. Model Tool Projection

- ToolFacade.buildKernelModelTools 基于 toolRegistry.List(Enabled=true) 抓取定义
- schemaToParameters 把 InputSchema 投射为 OpenAI-style Parameters
- identity provenance 与 authority 完全来自 ToolRegistry

## 24. OpenAI Projection

- agent/tool/Parameters -> agent/tool/Function{Name,Description,Parameters}
- schemaToParameters 解析 def.InputSchema 作为必要前提；解析失败则跳过该 Tool

## 25. Anthropic Projection

- llm_client 走同一 service.layer，复用同一份 tool.Tool DTO；B40 未新增独立 Anthropic Schema Authority

## 26. Gemini Projection

- 同上，复用同一份投影

## 27. MCP Schema Mapping

- MCPToolAdapter.AdaptTool 把 MCPToolDescriptor.InputSchema/OutputSchema 原样拷贝到 ToolDefinition
- Annotations.readOnlyHint/destructiveHint/openWorldHint 映射为 RiskLevel/SideEffectLevel
- 无可绕过 Canonical ToolDefinition 的第二通道

## 28. Plugin / Workflow Mapping

- 走 ToolFacade.Replace 进入统一 ToolRegistry
- 其 InputSchema/OutputSchema 等同地作为 Canonical 权威

## 29. Platform / Browser / Media Future Mapping

- 当前尚无独立 Provider 提供 Tool；若未来新增，仍须走 ToolDefinition + 同一 ToolRegistry

## 30. ResourceURI字段

- ToolContent 含 URI 字段；B13 资源解析器负责 ResourceURI 语义
- B40 Schema Validator 不复制 URI 解析逻辑

## 31. Secret安全

- rawSecretsInSchema = 0
- Sanitizer 提供敏感 key 的运行时 redact
- Validation Error 不暴露 secret 值

## 32. B39 Registry Regression

- duplicate Tool ID guard / duplicate Model Name guard / registration owner guard / legacy no-op 均未回归
- B40 未改动 registry.go，保持零回归

## 33. Duplicate System Validation

- ToolSchemaRegistry2 / ToolDefinitionV2 / SchemaRuntime2 / SchemaValidator2 / ToolResult2 / ExecutionPipeline2 = 0

## 34. 实际代码修改

没有源码修改。现有 Tool Schema、Input Validation 和 Result Validation 已经满足 B40 要求，无需新增第二套 Schema 系统。

## 35. Backward Compatibility

PASS_NO_CODE_CHANGE，全部现有 Tool 注册/投影/执行保持兼容。

## 36. B41输入

- Canonical ToolDefinition + ToolInvocation + RuntimeBinding + PermissionRequirement 已是完整 Tool Context 骨架
- 缺失的 context 语义留作 B41 专注点

## 37. Future Stream输入

- result.go 已预留 ToolContentType.Stream，但没有代码真正发出流式负载；Schema 留作 streaming 步骤补齐

## 38. B141输入

- Canonical schema authority = ToolDefinition.Input/OutputSchema
- 非运行时权威的遗留投影 = agent/tool/Parameters

## 39. Tests

- capability 现有 registry_test.go 18 用例保持覆盖
- execution pipeline 现有 pipeline_test.go 保持覆盖
- race：无需重跑（零源码修改）
- gofmt：零修改

## 40. Source Scope

- Modified files / Unexpected files / go.mod / go.sum / DB 全零修改

## 41. 阻断项

无

## 42. 最终结论

1. Amitia Tool Schema 继续由现有 ToolDefinition 体系作为唯一 Authority
2. 没有建立 ToolSchemaRegistry2 或 SchemaRuntime2
3. 所有 253 个 Agent-callable Tool 的 Input Schema 状态明确（VALID_SCHEMA）
4. 无参数 Tool 使用标准 Empty Schema 而非 Missing
5. 模型看到的 Tool Schema 与 Execution Pipeline 使用的 Schema 来自同一 ToolDefinition 事实源
6. Invalid Input 在 InvocationValidator/InputValidator 阶段被拦截，不会调用 Tool Handler
7. 当前 Structured Output 结构性校验缺少引擎，已作为 deferred gap 记录
8. ToolResult 保持唯一，无 StructuredToolResult2
9. MCP / Plugin / Workflow 已经映射到同一 Canonical Schema 合同
10. ResourceURI 仍然是资源参数的便携语义
11. Schema 及其 Validation Error 均不含 Secret
12. B39 冻结的 Tool ID、Model Name、Owner 规则零回归
13. 允许进入 B41 Tool Context 硬化
