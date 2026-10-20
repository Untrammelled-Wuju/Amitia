# Amitia RAG Phase 16：新版会话运行时融合记录

## 本轮代码基线

以 `U-Ai-source-20261016-084225.zip` 为新主干，以 Phase 15 已完成的选择性整合作为唯一连续工作树。本轮不覆盖新版 Canonical Memory、Extension Kernel、Tool Ledger、数据库迁移及其他业务代码。

## 已实施

- 从已交付的旧版调查器移植有界 Agentic Investigation 至新版 Chat orchestration；沿用当前 KnowledgePort 的 Search/Read，模型仅提出下一步动作，不可绕开确定性证据充分性门禁。
- 调查动作限制为 search、read、finish_sufficient、finish_insufficient；默认最多三轮、两次补充搜索、六次原文读取、四万八千字符读取预算，包含总时限、重复动作/无进展终止、来源白名单、取消支持。
- 明确页码/行号及原文核对请求会从 Hybrid Retrieval 定位到 Canonical Reader；跨资料比较按不同 source 做有界读取。原文 Evidence 优先进入模型上下文，避免被先前 chunk 列表挤掉。
- `turn.started` 移至聊天预处理检索之前；中间预处理故障会收口 Turn；被表达计划阻止的回合不启动多余 Turn。
- 将检索、原文精读、调查轮次映射到同一个 Turn 的持久化 `agent_task` block；支持状态变更、快照/重放与中断状态恢复，不新增消息流。
- Retrieval Policy 支持新策略、旧策略回滚和 Shadow 遥测；普通陪伴交流优先跳过知识检索，默认不让 Knowledge 覆盖 Canonical Memory。
- 复用 AgentUIEvent V1，补全现有协议 Schema 对检索/精读/调查事件类型的枚举。
- 增加精确 locator/tenant/revision 的自动化回归、Agentic 防越权及无进展收口、持久化 Agent Task round-trip 回归。

## 本轮仍未验收的项目

- 当前环境只有 Go 1.23.2，项目要求 1.26.1；定向 `go test ./internal/chat` 因 toolchain 版本检查而未进入编译与测试执行阶段。
- 未执行真实数据库迁移、Qdrant、SurrealDB、Electron 和 Flutter 联调；前端解析与完整 UI 行为仍需在项目完整构建环境里确认。
- Phase 17 的 Citation Snapshot（实时与历史一致性）及完整客户端细节仍未进入本轮完成范围。

## 补充验收标准

1. 检索发生在同一个 Turn 内；持久化任务能在历史 API、重连和 Snapshot 中复原。
2. 普通寒暄与陪伴请求不得无条件产生 Knowledge 检索；显式 PDF 第 37 页请求应定位到该页原文。
3. 任意 planner 给出的未知 source、revision 或文件路径均不得扩大原有 tenant/证据来源集合。
4. Canonical Read 失败不应退化成未经授权的文件系统读取；检索失败应是显式 degraded 状态。
5. Agentic 轮次、搜索次数、读取字符及最长时间均受硬预算约束；取消后不再追加搜索任务。
6. 只有实现了完整 Go、数据库及双端真实联调后，才可将这一阶段认定为生产级验收完成。
