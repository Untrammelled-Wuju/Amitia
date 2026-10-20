# RAG 最新源码冲突适配（Phase 15，阶段性）

主干：U-Ai-source-20261016-084225.zip。旧实现来源：Phase 1–14 整合树。

本阶段通过“添加 Knowledge 独立模块 + 新版配置/服务/路由接线 + 增量 SQL”恢复数据域。**绝不整文件覆盖新版 Chat/Canonical Memory/WorldBook/DeviceMesh/Frontend 主链**。

关键兼容约束：
- 不恢复旧 memory/profile/episodic 体系，不替换 Canonical Memory。
- 首个 Knowledge 迁移改为仅新建 Knowledge 表，禁止照搬旧版 world_book 表重建。
- WorldBook 仍保留最新版所有字段及存储数据；空 character 规则不参与角色触发。
- 扩展工具 search_knowledge/read_knowledge 明确 knowledge.read 能力授权。
- 独立 Knowledge 及管理 API 可用不代表自动对话注入已闭环；Chat Agentic/Citation/Realtime Turn 适配新版 Tool Ledger 仍需后续阶段开发。

验证要求：go1.26.1 编译、迁移 fresh/upgrade、Web/Flutter 构建、跨 tenant 权限与端到端运行。当前环境 Go 1.23.2，不能宣称构建通过。

## 已完成的新版入口接线

- Web BuiltinRoute 和导航白名单添加 `/knowledge`，保持其他新版业务路由。
- Flutter BuiltinRouteCatalog、KnowledgeService Provider、AppRoutes 已添加 `/knowledge`；保留现有 UI Provider 扩展机制，不回退整套路由。
- Chat Prompt IR 增加独立 Knowledge 数据段，标记 untrusted/data_only；不把知识库混到 Canonical Memory。
- 自动召回只针对显式文档类请求，普通对话不调用 Knowledge；Agent 工具可以显式调用 search/read。
- 尚未完成：新版 Turn 的 Citation Snapshot 实时/历史完整闭环；可恢复 Agentic planner 过程事件/Tool Ledger、跨端 `[K1]` 渲染；迁移 fresh/upgrade 完整执行和两端编译。这些必须列为继续开发项，不得判定项目 100%。

- 迁移 `20261016002` 只复制空角色的世界书规则到 `world_book_orphans` 审计表，不删除、不改写最新的 `world_book`；运行时不触发孤儿规则。
