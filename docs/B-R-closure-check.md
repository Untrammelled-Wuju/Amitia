# B-01 ~ B-20 Bug修复与关闭条件验证

| 编号 | Bug描述 | 关闭条件 | 验证方式 | 状态 |
|------|---------|----------|----------|------|
| B-01 | 情绪值positive/negative越界 | emotion_range检查通过 | healthCheckAffect | CLOSED |
| B-02 | 情绪arousal/dominance越界 | emotion_range检查通过 | healthCheckAffect | CLOSED |
| B-03 | 心境valence越界[-1,1] | mood_range检查通过 | healthCheckAffect | CLOSED |
| B-04 | 心境tension越界[0,1] | mood_range检查通过 | healthCheckAffect | CLOSED |
| B-05 | 压力值越界[0,1] | stress_level检查通过 | healthCheckAffect | CLOSED |
| B-06 | 信念置信度越界[0,1] | confidence_range检查通过 | healthCheckBelief | CLOSED |
| B-07 | 冲突消解公式缺失 | conflict_resolution检查通过 | healthCheckBelief | CLOSED |
| B-08 | 过期候选项未排除 | expiry_handling检查通过 | healthCheckBelief | CLOSED |
| B-09 | 状态版本号倒退 | version_ordering检查通过 | healthCheckSnapshot | CLOSED |
| B-10 | 快照引用完整性缺失 | reference_integrity检查通过 | healthCheckSnapshot | CLOSED |
| B-11 | 追踪帧排序错乱 | trace_ordering检查通过 | healthCheckSnapshot | CLOSED |
| B-12 | 人格配置解析失败 | config_resolution检查通过 | healthCheckPsyche | CLOSED |
| B-13 | 运行时状态值越界 | runtime_state检查通过 | healthCheckPsyche | CLOSED |
| B-14 | 调制计算不可用 | modulation_available检查通过 | healthCheckPsyche | CLOSED |
| B-15 | 关系亲密度越界[-1,1] | intimacy_range检查通过 | healthCheckRelationship | CLOSED |
| B-16 | 信任度越界[-1,1] | trust_range检查通过 | healthCheckRelationship | CLOSED |
| B-17 | 未解决事件积压 | unresolved_events检查通过 | healthCheckRelationship | CLOSED |
| B-18 | 断路器不自动重置 | TestCircuitBreakerTripAndReset通过 | go test | CLOSED |
| B-19 | 预算泄漏 | TestBudgetReset通过 | go test | CLOSED |
| B-20 | Outbox租约丢失 | TestOutboxLeaseReturnDrill通过 | go test | CLOSED |

# R-01 ~ R-20 风险缓解与关闭条件验证

| 编号 | 风险描述 | 缓解措施 | 验证方式 | 状态 |
|------|---------|----------|----------|------|
| R-01 | 情绪劫持攻击 | detectEmotionalManipulation | TestSecurityTestEmotionalHijacking | CLOSED |
| R-02 | 排他依赖崩溃 | 依赖链fallback检查 | TestSecurityTestExclusiveDependency | CLOSED |
| R-03 | 提示注入绕过 | detectPromptInjection | TestSecurityTestPromptInjection | CLOSED |
| R-04 | 数据泄漏 | 5向量扫描 | TestSecurityTestDataLeakage | CLOSED |
| R-05 | 删除后召回 | 4路径阻断验证 | TestSecurityTestPostDeletionRecall | CLOSED |
| R-06 | 并发删除竞态 | mutex保护 | TestMultipleConcurrentDeletionRequests | CLOSED |
| R-07 | 删除范围失控 | 5种scope枚举 | TestDeletionScopes | CLOSED |
| R-08 | 状态转换异常 | 状态机验证 | TestDeletionStatusTransitions | CLOSED |
| R-09 | 大小写绕过检索阻断 | normalizeTargetID | TestIsRetrievalBlockedCaseInsensitive | CLOSED |
| R-10 | Outbox清理遗漏 | 6存储全量覆盖 | TestExecuteOutboxCleanup | CLOSED |
| R-11 | 重算任务遗漏 | 3zone覆盖 | TestGenerateRecalculationTasksAllScope | CLOSED |
| R-12 | Tombstone未清理 | MarkDeletionComplete | TestMarkDeletionComplete | CLOSED |
| R-13 | 统计信息不准确 | Stats方法 | TestCoordinatorStats | CLOSED |
| R-14 | Reset不彻底 | Reset方法 | TestCoordinatorReset | CLOSED |
| R-15 | Outbox查询不安全 | 副本返回 | TestGetOutboxItems | CLOSED |
| R-16 | 重算任务查询不安全 | 副本返回 | TestGetRecalculationTasks | CLOSED |
| R-17 | 误报风险 | detectEmotionalManipulation | TestDetectEmotionalManipulation | CLOSED |
| R-18 | 注入检测遗漏 | detectPromptInjection | TestDetectPromptInjection | CLOSED |
| R-19 | Tombstone查找失败 | 大小写不敏感 | TestGetTombstone | CLOSED |
| R-20 | 全量安全测试不完整 | 5项测试枚举 | TestRunAllSecurityTests | CLOSED |

全部40项 (B-01~B-20 + R-01~R-20) 均已 CLOSED。