# Phase 17：Citation Snapshot 与新版运行时适配记录

基线：2026-10-16 完整源码 + Phase 15/16，工作树 `/mnt/data/amitia_latest_rag_adapt_work/U-Ai`。

## 已落地

1. Knowledge 的检索证据只有在真正写入 Prompt 的 `[K数字]` 位置时才进入本轮引用账本；保留来源、revision、unit、locator、checksum 和限长正文，不能从文件路径重新读取绕过权限。
2. 最终回复只冻结 Markdown 正文实际引用的 Knowledge/Web 证据；忽略反引号代码片段，缺失证据的编号不能生成快照。Web `[数字]` 和 Knowledge `[K数字]` 独立编号。
3. `Message` 新增 `citation_snapshot_json` 和只读 JSON 映射，GORM 写入时序列化、历史读取时恢复；旧消息空字段与损坏 JSON 均不使聊天历史读取失败。
4. `ComputeResult → messageCommitPlan → messages` 复用当前数据库提交事务；新的 `text.completed` 事件携带相同的引用快照，断线 Replay/历史读取遵循同一数据源，不另外发第二条事件流。
5. Web/Electron CitationSource 对快照优先取值；支持 `[K1]` 格式，历史引用不能因为检索索引重建而失效。旧消息仍兼容 Web Tool Result 引用。
6. Flutter `MessageDto → ChatMessage → AmrpMessage` 保存快照，并同步支持 `[K1]` 的点击渲染；保留旧数字引用语义。
7. `messages` Fresh Baseline 与历史迁移 `20260927001` 均覆盖引用字段，迁移代码原版本未改动或重复注册。

## 已做的验证

- 所有本轮变更 Go 源码经 gofmt 与标准 Go 解析器检查。
- `messages` 基线 SQLite DDL 建表，字段默认值 `[]` 验证通过。
- 变更 TS/Vue script 的 TypeScript 语法解析通过。
- 新增 Go 测试覆盖最终实际引用过滤、代码片段引用屏蔽、冻结内容上限、历史 round-trip 与损坏旧 JSON 的容错；测试执行受到项目 Go 1.26.1 toolchain 和依赖不可用的环境限制，尚不能认定通过。
- 由于环境无 Dart SDK，Flutter 仅进行静态结构核对，未执行 Flutter analyze/build。

## 未完成或后续重点

- 真机/Electron 的最新流消费端收到 `text.completed` 时是否即时应用 `citationSnapshot` 仍需真实联调验收；已有历史 API 不代替流验证。
- 完整 Go 类型编译、迁移升级/恢复、长历史对话压测与多端 UI 回归仍需在有依赖的构建环境运行。
- 未修改新版 Canonical Memory、Tool Ledger 与 Conversation Runtime 的核心状态机。
