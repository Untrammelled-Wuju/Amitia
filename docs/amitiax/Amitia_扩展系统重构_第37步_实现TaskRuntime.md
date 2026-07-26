# Amitia 扩展系统重构第 37 步实施文档

## 第 37 步：实现隔离 Task Runtime

---

## 一、步骤目标

实现面向一次性任务、数据迁移、批量导入、长计算和可恢复后台作业的独立 Task Runtime，避免阻塞或污染 Main Runtime。

目标：

```text
Task Definition
→ Task Plan
→ Ephemeral Runtime Instance
→ Checkpoint/Progress
→ Result Artifact
→ Cleanup
```

---

## 二、适用范围

-Extension 数据迁移；
-批量处理；
-资源索引；
-大型导入导出；
-长时间计算；
-一次性维护；
-可恢复后台任务；
-构建派生数据。

不适用：

-高频 Tool；
-同步 Hook；
-长期 Event Handler；
-UI；
-未经信任原生程序。

---

## 三、进程模型

默认：

```text
per_task
```

每个任务独立子进程。

可选小任务池在后续优化。

---

## 四、Task Definition

```go
type TaskDefinition struct {
    TaskID          string
    RuntimeType     string
    Entry           string
    InputSchema     json.RawMessage
    OutputSchema    json.RawMessage
    Checkpoint      bool
    Idempotent      bool
    Recoverable     bool
    ResourceLimits  RuntimeResourceLimits
    PermissionRequirements []PermissionRequirement
}
```

---

## 五、Task 状态

```text
created
queued
starting
running
checkpointing
paused
succeeded
failed
cancelled
timed_out
recovery_required
```

映射统一 Operation/Invocation。

---

## 六、启动流程

1.校验 Definition。
2.校验 Input。
3.创建 Operation。
4.固定 Scope/Permission/Dependency Snapshot。
5.创建临时工作区。
6.启动 Task Runtime。
7.认证 Session。
8.执行 Entry。
9.接收 Progress/Checkpoint。
10.校验结果。
11.提交 Result Artifact。
12.清理。

---

## 七、Task SDK

```ts
export default defineTask(async (input, context) => {
  context.progress.report({ current: 1, total: 10 });
  await context.checkpoint.save({ cursor: 1 });
  return { completed: true };
});
```

---

## 八、Progress

Progress：

-结构化；
-限流；
-可查询；
-关联 Operation；
-不得决定最终状态；
-不得包含 Secret。

---

## 九、Checkpoint

Checkpoint 适用于可恢复任务。

必须：

-有 Schema；
-版本；
-Hash；
-任务定义版本；
-原子写入；
-大小限制；
-不含 Secret 明文。

---

## 十、恢复

恢复前检查：

-Task Definition 相同；
-Entry Hash；
-Checkpoint Version；
-Extension Enabled；
-Owner；
-Scope；
-Permission；
-依赖；
-输入 Hash。

不满足则进入 manual/recovery_required。

---

## 十一、幂等

可恢复 Task 必须声明 Idempotent 或使用 Checkpoint 避免重复副作用。

未知结果的非幂等 Task 不自动重跑。

---

## 十二、数据迁移 Task

迁移 Task 额外限制：

-仅允许指定 Storage Namespace；
-禁止网络；
-禁止 Desktop；
-禁止其他 Extension；
-运行前 Snapshot；
-输出 Schema 验证；
-失败恢复。

---

## 十三、临时工作区

每个 Task：

```text
temp/tasks/<operation-id>/
```

受 Resource Ownership 管理。

Task 结束清理；需要保留结果则转为 Artifact。

---

## 十四、结果

小结果返回 JSON。

大结果必须转：

```text
Artifact Reference
```

不得通过 RPC 传输超大内容。

---

## 十五、取消

取消：

-发送 RPC Cancel；
-AbortSignal；
-等待 Grace；
-终止子进程；
-保存或丢弃 Checkpoint 按策略；
-清理临时资源。

---

## 十六、资源限制

Task 通常比 Main Runtime允许更高 CPU/内存，但：

-有上限；
-有总并发；
-有磁盘配额；
-有最长时间；
-有输出大小；
-有日志限制。

---

## 十七、调度

Task Runtime 由 Background Task Manager 或 Lifecycle Manager 启动。

禁止插件自行创建无限 Task。

---

## 十八、队列

按：

-优先级；
-Extension；
-资源；
-风险；
-用户触发；
-生命周期关键任务。

生命周期迁移 Task 可高优先级，但不能饿死其他任务。

---

## 十九、应用关闭

可恢复 Task：

-请求 Checkpoint；
-停止；
-下次恢复。

不可恢复 Task：

-取消或标记失败。

---

## 二十、审计

记录：

-Task；
-Entry；
-Input Hash；
-Checkpoint；
-Progress 摘要；
-资源；
-结果 Artifact；
-取消；
-恢复；
-副作用。

---

## 二十一、测试要求

覆盖：

-短任务；
-长任务；
-Progress；
-Checkpoint；
-恢复；
-Definition 变化；
-输入变化；
-取消；
-超时；
-进程崩溃；
-磁盘不足；
-大结果；
-迁移限制；
-非幂等未知结果；
-应用关闭；
-队列与并发。

---

## 二十二、实施任务

1. 定义 Task Definition。
2. 实现 Task Runtime Factory。
3. 实现 per_task 进程。
4. 实现 Task SDK。
5. 实现 Progress。
6. 实现 Checkpoint。
7. 实现 Result Artifact。
8. 实现 Cancel/Timeout。
9. 实现 Recovery。
10. 实现 Task Queue。
11. 接入 Lifecycle Migration。
12. 接入 Resource Ownership。
13. 接入 Audit。
14. 完成故障测试。

---

## 二十三、验收标准

1. Task 与 Main Runtime 分离。
2. 默认每任务独立进程。
3. Progress 和 Checkpoint 可用。
4. 恢复固定 Definition/Input。
5. 非幂等未知结果不自动重试。
6. 迁移 Task 权限受限。
7. 大结果使用 Artifact。
8. 临时资源可清理。
9. 应用关闭策略明确。
10. 可进入第 38 步内部 JSON-RPC。

---

## 二十四、执行约束

> Task Runtime 用于受控后台作业，不是让插件随意启动子进程的接口。

禁止：

-插件直接 spawn；
-无限 Task；
-迁移访问网络；
-Checkpoint 存 Secret；
-大结果塞 RPC；
-未知副作用自动重跑；
-Task 进程长期常驻冒充 Main Runtime。
