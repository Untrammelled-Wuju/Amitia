# R15 证据：补齐生产Context Loader与必需数据契约
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. interaction/context_loaders_sqlite.go: 9个Context Loader全部实现(RuntimeProfile, Channel, Conversation, Psyche, Relationship, Belief, Life, Need, UnresolvedThread)
2. interaction/context_loader.go: ContextLoaderRegistry统一注册和并行加载
3. interaction/context.go: ContextSnapshot完整数据契约定义
4. cmd/server/services.go: newRuntimeContextLoaderRegistry注册全部9个Loader

## 测试结果
`
go test ./internal/interaction/... -count=1 -timeout 60s
ok  github.com/u-ai/backend/internal/interaction
`

## 验收标准
✅ RoleRuntimeProfile、MemoryGovernance、Belief、Psyche、Relationship、Need、Life、ConversationState和UnresolvedThread Loader全部注册
✅ 每个Loader声明required/optional、Deadline、fallback和版本
✅ required失败中止或进入保守路径
✅ Snapshot记录每段来源版本和加载耗时