# 心理模拟系统风险登记册

## R-01 ~ R-20 风险评估与缓解措施

| 编号 | 风险描述 | 等级 | 缓解措施 | 当前状态 |
|------|---------|------|----------|----------|
| R-01 | 心理状态引擎与现有chat路径冲突 | 高 | 影子运行对比，逐步灰度切换 | MITIGATED |
| R-02 | 多存储最终一致性导致用户可见异常 | 高 | Runtime Reconciliation巡检 + 自动补偿 | MITIGATED |
| R-03 | 跨渠道消息重复或丢失 | 高 | DedupManager + Outbox + 输出租约 | MITIGATED |
| R-04 | 调试和追溯困难 | 中 | TraceMiddleware + 请求ID + 因果链导出 | MITIGATED |
| R-05 | 迁移导致旧数据不可读 | 高 | 幂等迁移 + 备份恢复 + 待确认清单 | MITIGATED |
| R-06 | 并发写入状态版本混乱 | 高 | 乐观锁 + 角色级串行提交 + CommitValidator | MITIGATED |
| R-07 | 心理状态累积导致Prompt超长 | 中 | TokenBudgetManager + 优先级裁剪 | MITIGATED |
| R-08 | LLM故障时系统不可用 | 高 | CircuitBreaker熔断 + 降级路径 | MITIGATED |
| R-09 | 主动消息频率过高打扰用户 | 中 | 动机/抑制评分 + 每日预算 + 冷却降频 | MITIGATED |
| R-10 | 工具执行结果被错误地作为确定事实 | 高 | ToolCallResult.Confidence + UNKNOWN状态 + 确认窗口 | MITIGATED |
| R-11 | 人格参数越界导致异常行为 | 中 | SafetyGovernor + 钳制函数 + 安全上限 | MITIGATED |
| R-12 | 删除请求未传播到派生存储 | 高 | DataLifecycleCoordinator + DeletionTombstone + Outbox清理 | MITIGATED |
| R-13 | 反思系统修改不应修改的信念 | 中 | ReflectionSupervisor审批 + 版本回滚 | MITIGATED |
| R-14 | 语音打断导致上下文丢失 | 中 | VoiceSession + 打断轮次取消 + 快照延续 | MITIGATED |
| R-15 | 队列积压导致实时回复延迟 | 中 | 优先级队列P0-P5 + 背压控制器 + 主动消息驱逐 | MITIGATED |
| R-16 | 缓存使用过时数据 | 中 | 缓存键含scope/version + TTL分级 + LRU淘汰 | MITIGATED |
| R-17 | 优雅关闭导致正在进行的交互丢失 | 中 | LifecycleComponent + Drain + ShutdownSequence | MITIGATED |
| R-18 | 情境记忆和用户画像作用域错位 | 中 | InteractionScope统一解析 + character_id双层校验 | MITIGATED |
| R-19 | 长期运行后心理状态漂移 | 低 | 纵向仿真 + Reconciliation巡检 + 版本回滚 | MITIGATED |
| R-20 | 全链路Deadline不足导致部分提交 | 中 | Deadline传播器 + 子预算分配 + 超预算降级 | MITIGATED |

最后更新: 2026-07-01
