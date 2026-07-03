# B-01 ~ B-20 Bug修复与关闭条件验证

> **V9审计反证 - 历史关闭记录失效：** 以下全部CLOSED状态已于2026-07-03审计V9.0发现反证后标记为失效(HISTORICAL_INVALID)。
> 失效原因：V9审计确认83项问题中对应原B/R编号的问题实际未修复或只有模拟/单元测试覆盖，缺乏生产链路接线。
> 原关闭证据基于旧代码基线(02dd1d79之前)，未绑定统一InteractionRun、Commit和Outbox链路。
> 修复步骤参照：Amitia-develop_6_168步V3.1审计问题详细修复实施文档_V2.1.md R02。
> 
> 状态重新定义为：OPEN | IN_PROGRESS | CODE_DONE | TESTED | ACCEPTED | REOPENED
> 关闭证据必须包含：提交、文件、迁移、命令、结果、回滚和残余风险。

## B-01 ~ B-20 缺陷状态（V9审计后重新评估）

| 编号 | 原状态 | V9审计状态 | V9关联 | 主修步骤 | Bug描述 |
|------|--------|-----------|--------|----------|---------|
| B-01 | HISTORICAL_INVALID | OPEN | P0-01,P0-02 | R07,R09 | 上下文重复/消息注入 |
| B-02 | HISTORICAL_INVALID | OPEN | P0-04 | R17 | 角色配置不完整 |
| B-03 | HISTORICAL_INVALID | OPEN | P0-03 | R20 | 心理表不存在 |
| B-04 | HISTORICAL_INVALID | OPEN | P0-01 | R06,R12 | 空闲时间scope错误 |
| B-05 | HISTORICAL_INVALID | OPEN | P1-34 | R18 | 用户画像scope |
| B-06 | HISTORICAL_INVALID | OPEN | P1-26 | R16,R38 | 记忆生成缺character_id |
| B-07 | HISTORICAL_INVALID | OPEN | P1-25 | R16 | 全量读取无检查点 |
| B-08 | HISTORICAL_INVALID | OPEN | P1-10 | R12,R37 | 主动消息scope |
| B-09 | HISTORICAL_INVALID | OPEN | P0-05 | R36,R37 | 主动消息未接入Runtime |
| B-10 | HISTORICAL_INVALID | OPEN | P0-01 | R06,R07 | 包级可变变量 |
| B-11 | HISTORICAL_INVALID | OPEN | P0-01 | R06,R07 | 包级forceVoiceFlag |
| B-12 | HISTORICAL_INVALID | OPEN | P1-33 | R15 | 服务容器重复实例 |
| B-13 | HISTORICAL_INVALID | OPEN | P1-33 | R15 | 情景详情UUID查询 |
| B-14 | HISTORICAL_INVALID | OPEN | P1-06 | R29,R31 | 记忆字段丢弃 |
| B-15 | HISTORICAL_INVALID | OPEN | P2-14 | R03 | retrieval_log字段错误 |
| B-16 | HISTORICAL_INVALID | OPEN | P0-03 | R20 | 生活状态源错误 |
| B-17 | HISTORICAL_INVALID | OPEN | P0-07 | R16,R38 | RandomBurst无scope过滤 |
| B-18 | HISTORICAL_INVALID | OPEN | P1-24 | R32,R39 | 主动消息存Prompt非内容 |
| B-19 | HISTORICAL_INVALID | OPEN | P2-14 | R03 | 前端两套PersonalityConfig |
| B-20 | HISTORICAL_INVALID | OPEN | P1-34 | R18 | 画像重复事实膨胀 |

## R-01 ~ R-20 风险状态（V9审计后重新评估）

| 编号 | 原状态 | V9审计状态 | V9关联 | 主修步骤 | 风险描述 |
|------|--------|-----------|--------|----------|---------|
| R-01 | HISTORICAL_INVALID | OPEN | P1-31 | R26 | 情绪劫持攻击 |
| R-02 | HISTORICAL_INVALID | OPEN | P1-37 | R29,R53 | 排他依赖崩溃 |
| R-03 | HISTORICAL_INVALID | OPEN | P1-24 | R32 | 跨渠道消息重复 |
| R-04 | HISTORICAL_INVALID | OPEN | P2-20 | R04,R57 | 调试追溯困难 |
| R-05 | HISTORICAL_INVALID | OPEN | P1-35 | R10,R47 | 迁移旧数据不可读 |
| R-06 | HISTORICAL_INVALID | OPEN | P0-01 | R06,R08 | 并发状态版本混乱 |
| R-07 | HISTORICAL_INVALID | OPEN | P1-32 | R25 | 心理状态Prompt超长 |
| R-08 | HISTORICAL_INVALID | OPEN | P1-37 | R29,R53 | LLM熔断降级 |
| R-09 | HISTORICAL_INVALID | OPEN | P1-20 | R37 | 主动消息频率过高 |
| R-10 | HISTORICAL_INVALID | OPEN | P1-06 | R29,R31 | 工具结果误作事实 |
| R-11 | HISTORICAL_INVALID | OPEN | P1-31 | R26 | 人格参数越界 |
| R-12 | HISTORICAL_INVALID | OPEN | P0-07 | R42,R44 | 删除未传播 |
| R-13 | HISTORICAL_INVALID | OPEN | P1-34 | R18 | 反思修改信念 |
| R-14 | HISTORICAL_INVALID | OPEN | P1-28 | R40 | 语音打断丢上下文 |
| R-15 | HISTORICAL_INVALID | OPEN | P1-10 | R12,R13 | 队列积压延迟 |
| R-16 | HISTORICAL_INVALID | OPEN | P1-33 | R15 | 缓存过时数据 |
| R-17 | HISTORICAL_INVALID | OPEN | P1-29 | R13,R52 | 优雅关闭丢交互 |
| R-18 | HISTORICAL_INVALID | OPEN | P0-01 | R06 | scope错位 |
| R-19 | HISTORICAL_INVALID | OPEN | P1-36 | R20,R22 | 心理状态漂移 |
| R-20 | HISTORICAL_INVALID | OPEN | P1-29 | R13 | Deadline不足 |
