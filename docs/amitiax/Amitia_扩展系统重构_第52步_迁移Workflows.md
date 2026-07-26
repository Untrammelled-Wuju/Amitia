# Amitia 扩展系统重构第 52 步实施文档

## 第 52 步：迁移 Workflows

---

## 一、步骤目标

将当前 Workflow Definition、旧 Skill Workflow Wrapper、Schedule、Execution、Node Run、Approval、Delay、Sub-workflow 和 Tool Exposure 迁移到 Extension Kernel 的 Workflow Contribution、WorkflowRegistry、WorkflowExecutor、Scheduler 和统一运行模型。

目标：

> Workflow 成为独立声明式自动化对象，不再混入 Skill、Plugin 或 Tool Handler；执行只通过统一 WorkflowExecutor，Tool 暴露只通过 Workflow Adapter。

---

## 二、迁移范围

包括：

-系统内置 Workflow；
-用户创建 Workflow；
-扩展附带 Workflow；
-旧 Skill 包装 Workflow；
-Plugin Workflow；
-定时 Workflow；
-事件触发 Workflow；
-子 Workflow；
-运行中 Execution；
-暂停；
-等待；
-审批；
-补偿；
-旧节点日志；
-旧 Schedule；
-旧 Workflow Tool。

---

## 三、目标建模

系统 Workflow：

```text
system/amitia-core#workflows
```

用户 Workflow：

```text
Synthetic Extension
local.user/workflow-<stable-id>
```

扩展附带 Workflow：

```text
所属 Extension Module
→ workflow Contribution
```

---

## 四、稳定 ID

建立：

```text
legacy_workflow_id
legacy_skill_name
source_file
→ canonical_workflow_id
```

Workflow Tool ID 与 Workflow ID 分离。

---

## 五、Definition 迁移

迁移：

-Input Schema；
-Output Schema；
-Node；
-Edge；
-Condition；
-Tool Reference；
-Sub Workflow；
-Delay；
-Approval；
-Compensation；
-Retry；
-Timeout；
-Schedule；
-Event Trigger；
-版本；
-Owner；
-Scope；
-Permission；
-资源。

---

## 六、节点类型规范化

旧节点映射为统一类型：

```text
tool
condition
transform
delay
approval
sub_workflow
parallel
join
emit_event
wait_event
output
compensation
```

未知自定义节点：

-映射 Plugin Runtime Node（如已有正式协议）；
-或标记不可迁移；
-不得转换为任意代码节点。

---

## 七、Tool 引用

所有 Tool 节点引用稳定 Tool ID。

迁移时：

-旧 Skill 名映射；
-MCP Tool 映射；
-Plugin Tool 映射；
-内置 Tool 映射；
-缺失 Tool 报告。

不得保存 Handler。

---

## 八、Workflow Tool Exposure

若 Workflow 可被模型调用：

```text
Workflow Contribution
→ WorkflowToolAdapter
→ ToolDefinition
```

不再创建 SkillDefinition。

---

## 九、Schedule 迁移

拆分：

-Workflow Enabled；
-Schedule Enabled；
-Scope；
-Permission Snapshot Reference；
-Recurrence；
-Timezone；
-Missed Run；
-Overlap；
-Owner。

旧单一 Enabled 不再共用。

---

## 十、Event Trigger

旧事件触发器迁入：

```text
Event Subscription Contribution
→ WorkflowRuntimeAdapter
```

必须有：

-Event Type；
-Schema；
-Filter；
-Scope；
-幂等；
-Delivery；
-Depth。

---

## 十一、运行中 Execution

分类：

### 可安全恢复

-Delay；
-Approval；
-Wait Event；
-Paused；
-安全节点边界。

### 需要人工确认

-高风险 Tool 结果未知；
-旧节点状态不完整；
-副作用未记录；
-Definition 版本缺失。

### 不可恢复

-仅内存 Handler；
-旧临时上下文；
-Definition 已损坏。

---

## 十二、Definition Snapshot

迁移运行中 Execution 时固定：

-Workflow Definition Version；
-Definition Hash；
-Tool Dependency Snapshot；
-Scope Snapshot；
-Permission Snapshot；
-Input Hash。

没有快照的旧运行记录不得伪造。

---

## 十三、运行记录迁移

映射：

-Workflow Run → Operation；
-Node Run → Invocation；
-Retry → Attempt；
-Delay/Approval → Runtime Event/State；
-错误 → Runtime Error；
-副作用 → SideEffect。

---

## 十四、Approval

旧 Approval 状态：

-待审批；
-通过；
-拒绝；
-过期；
-取消。

迁移后由统一 Approval Service 或 Workflow Executor 状态管理。

---

## 十五、Delay 与 Timer

旧 Timer 不恢复为内存 Timer。

迁移为持久：

```text
resume_at
scheduler resource
```

---

## 十六、并行节点

旧并行执行若无明确 Join 状态：

-保留已完成分支；
-未知分支标记；
-不自动重复执行非幂等节点；
-必要时 manual recovery。

---

## 十七、补偿

旧 Workflow 若没有补偿信息：

-迁移 Definition；
-运行中高风险实例不自动恢复；
-后续新运行使用新补偿模型。

---

## 十八、用户修改

用户编辑扩展附带 Workflow：

-转为用户 Fork；
-新 Synthetic Extension；
-保留来源；
-扩展更新不覆盖。

---

## 十九、Enabled/Scope

旧 Global/Character Workflow：

-Workflow Enabled；
-Scope Binding。

角色 Workflow 不迁 Global。

Schedule 使用固定 Scope，不读取当前前端角色。

---

## 二十、Permission

每个 Tool 节点执行时重新经过 ExecutionSecurityKernel。

Workflow 本体不能预先绕过 Tool Permission。

后台 Schedule 使用 Permission Snapshot Reference，并在运行时重新验证撤销。

---

## 二十一、依赖

Workflow Dependency：

-Tool；
-Sub Workflow；
-Provider；
-Event；
-Host Feature。

由 Dependency Resolver 处理。

---

## 二十二、前端

统一展示：

-Extension；
-Owner；
-版本；
-Enabled；
-Scope；
-节点；
-依赖；
-Tool Exposure；
-Schedule；
-Event Trigger；
-运行；
-恢复；
-用户 Fork；
-错误；
-审计。

---

## 二十三、兼容 API

旧：

```text
RunWorkflow
EnableWorkflow
DeleteWorkflow
CreateSchedule
```

映射：

-Run → WorkflowExecutor；
-Enable → EnablementService；
-Delete → Lifecycle/Contribution Delete；
-Schedule → Scheduler Service。

---

## 二十四、迁移批次

1.系统内置；
2.用户 Definition；
3.Skill Wrapper；
4.Plugin Workflow；
5.Schedule；
6.Event Trigger；
7.历史记录；
8.运行中实例；
9.用户 Fork；
10.旧表冻结。

---

## 二十五、双执行防护

迁移后：

-旧 Scheduler 停止；
-旧 Event Trigger 停止；
-旧 Workflow Executor 拒绝新运行；
-旧 API 映射新 Executor；
-防同一 Schedule 双触发；
-防同一 Event 双投递。

---

## 二十六、测试要求

覆盖：

-Definition；
-Tool；
-Sub Workflow；
-Condition；
-Transform；
-Delay；
-Approval；
-Parallel；
-Compensation；
-Schedule；
-Event；
-Tool Exposure；
-Scope；
-Permission；
-用户 Fork；
-运行记录；
-恢复；
-未知结果；
-旧 API；
-双执行；
-性能。

---

## 二十七、实施任务

1. 输出 Workflow 全量清单。
2. 建立稳定 ID。
3. 建立 System/Synthetic Extension。
4.迁移 Definition。
5.迁移节点和 Tool 引用。
6.迁移 Tool Exposure。
7.迁移 Schedule。
8.迁移 Event Trigger。
9.迁移 Scope/Enabled。
10.迁移运行记录。
11.分类运行中实例。
12.实现用户 Fork。
13.接入 WorkflowRegistry/Executor。
14.冻结旧 Scheduler/Executor。
15.改造前端/API。
16.完成恢复和双执行测试。

---

## 二十八、验收标准

1. 所有 Workflow 有正式 ContributionDefinition。
2. Workflow 不再作为 Skill。
3.执行只通过 WorkflowExecutor。
4.Tool Exposure 通过 Adapter。
5.Schedule 与 Workflow Enabled 分离。
6.Event Trigger 进入 Event Bus。
7.运行记录统一。
8.高风险未知结果不自动重跑。
9.用户修改不被覆盖。
10.旧 Scheduler 不再触发。
11.关键测试通过。
12.可进入第 53 步官方内置 Plugin 迁移。

---

## 二十九、执行约束

> Workflow 是声明式编排定义，不是可任意执行代码的容器，也不是 Skill 的一种运行模式。

禁止：

-任意代码节点；
-旧 Executor 新运行；
-旧 Scheduler 双触发；
-Tool Handler 嵌入 Definition；
-当前前端角色作为 Schedule Scope；
-未知副作用自动恢复；
-新旧双写。
