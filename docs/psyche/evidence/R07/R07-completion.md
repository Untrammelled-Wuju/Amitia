# R07 证据文件 - 稳定RequestID和并发幂等结果复用

**完成时间:** 2026-07-03 12:58
**关联审计问题:** P0-01, P0-02, P1-20, P1-21, P2-10
**对应原168步:** 第8、15、30、39、159步

## 修改清单

### 1. orchestrator.go - handleIdempotentHit扩展
- 新增 ErrOrchestratorProcessing 错误哨兵
- handleIdempotentHit 增加 Processing 等活跃状态处理：返回 ErrOrchestratorProcessing
- Completed/Committed/Delivered 状态返回原结果（nil error）
- Failed/Cancelled/Superseded 状态返回 ErrOrchestratorDuplicate

### 2. webchat_handler.go - Processing状态适配
- 添加 "errors" 导入
- 在 err != nil 检查前增加 ErrOrchestratorProcessing 判断
- Processing 状态返回 {"status": "processing", ...} 而非 InternalError

### 3. stream_handler.go - Processing状态适配
- 添加 "errors" 导入
- 同样增加 ErrOrchestratorProcessing 判断
- Processing 状态返回 500 但携带 "请求处理中" 消息

### 4. 主动任务UnixNano → UUID修复（5处）
- proactive/executor.go: sendToWeb和sendToQQ两处 msgID 改用 uuid.New()
- companion/delayed_reply_service.go: msgID 改用 uuid.New()
- companion/random_burst.go: msgID 改用 uuid.New()
- agent/tool/schedule.go: id 改用 uuid.New()
- companion/proactive_unified_dispatch.go: proactiveRequestID 改用 uuid.New()

### 5. orchestrator_runtime_test.go - 测试更新
- TestOrchestratorDuplicateRequestIDDoesNotReprocess: 更新为期望幂等返回 nil error

## 测试结果
- 编译: PASS
- 互动模块全部测试: PASS (ok github.com/u-ai/backend/internal/interaction 10.047s)

## 验收标准
1. ✅ 已完成请求幂等返回原结果（nil error）
2. ✅ Processing状态返回可查询状态（ErrOrchestratorProcessing）
3. ✅ 主动任务不再使用UnixNano作幂等ID
4. ✅ request_id回传客户端
5. ✅ 所有互动模块测试通过
