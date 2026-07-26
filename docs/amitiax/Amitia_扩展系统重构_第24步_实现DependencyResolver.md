# Amitia 扩展系统重构第 24 步实施文档

## 第 24 步：实现统一 Dependency Resolver

---

## 一、步骤目标

建立 Extension Kernel 唯一的依赖解析器，统一 Extension、Module、Contribution、Tool、Workflow、MCP Server、Provider、Runtime、Host Feature、Platform 与版本依赖。

目标：

```text
DependencyDefinition
→ Candidate Discovery
→ Version Constraint Solving
→ Conflict Detection
→ Resolution Plan
→ Dependency Snapshot
→ Lifecycle/Runtime/Execution 使用
```

禁止 Package、Agent Skill、Workflow、MCP 和 Plugin 各自维护依赖逻辑。

---

## 二、职责

Dependency Resolver 负责：

- 构建依赖图；
-发现候选；
-解析 Required/Optional；
-解析版本范围；
-检查平台和 Host Feature；
-检测循环；
-检测冲突；
-生成安装期、启用期、启动期、执行期依赖结果；
-生成依赖快照；
-监听依赖丢失；
-提供受影响对象；
-为 Lifecycle Plan 提供子计划；
-为 EffectiveStateResolver 提供 DependencyReady；
-为 Registry 提供注册顺序。

不负责：

- 自动下载；
-自动安装；
-自动启用；
-授权；
-Scope；
-Runtime 启停；
-执行 Tool。

---

## 三、依赖模型

建议：

```go
type DependencyRequest struct {
    SourceID       string
    Type           DependencyType
    Target         DependencyTarget
    VersionRange   string
    Required       bool
    Scope          DependencyScope
    Resolution     DependencyResolutionPolicy
}
```

结果：

```go
type DependencyResolution struct {
    Request        DependencyRequest
    Status         string
    Selected       *DependencyCandidate
    Candidates     []DependencyCandidate
    Conflicts      []DependencyConflict
    Warnings       []DependencyWarning
}
```

---

## 四、依赖阶段

支持：

```text
install
enable
start
execute
build
development
```

不同阶段规则不同：

- install：是否需要安装目标；
-enable：启用前必须满足；
-start：Runtime 启动前满足；
-execute：单次调用前满足；
-build/development：SDK 与开发环境使用。

---

## 五、依赖目标

统一目标：

```text
extension
module
contribution
tool
workflow
mcp_server
provider
runtime
host_feature
platform
architecture
```

---

## 六、版本范围

必须支持标准 SemVer 范围：

```text
=1.2.3
>=1.2.0
<2.0.0
^1.4.0
~1.4.2
1.x
```

禁止自定义模糊字符串。

同一依赖的多个约束需要求交集。

---

## 七、候选来源

候选来源：

- 已安装 Extension；
-系统内置 Extension；
-Synthetic Extension；
-当前待安装 Plan；
-本地可信 Artifact；
-未来扩展中心索引。

本步骤不自动远程下载。

---

## 八、解析策略

建议：

```text
exact
highest_compatible
lowest_compatible
installed_preferred
system_preferred
user_selected
isolated
```

默认：

```text
installed_preferred + highest_compatible
```

但共享 Runtime 或全局 Provider 需要更严格规则。

---

## 九、Required 与 Optional

Required 缺失：

- install/enable/start 阶段阻塞；
-execute 阶段返回 dependency_missing。

Optional 缺失：

- 允许继续；
-相关 Contribution 可降级；
-必须产生 Warning；
-不得伪装为完整可用。

---

## 十、依赖图

建议：

```go
type DependencyGraph struct {
    Nodes map[string]DependencyNode
    Edges []DependencyEdge
    Hash  string
}
```

边包含：

-来源；
-目标；
-阶段；
-Required；
-版本；
-条件；
-Owner。

---

## 十一、循环检测

必须检测：

- Extension 循环；
-Module 循环；
-Workflow 子调用循环；
-Provider 循环；
-Runtime 循环；
-Tool 依赖循环；
-混合类型循环。

允许的软循环必须显式声明且不能影响启动顺序。

默认阻止 Required 循环。

---

## 十二、冲突类型

```text
version_conflict
missing_dependency
dependency_cycle
platform_conflict
architecture_conflict
host_feature_missing
exclusive_provider_conflict
runtime_conflict
owner_conflict
scope_incompatible
duplicate_capability
dependency_disabled
dependency_quarantined
```

---

## 十三、安装依赖计划

Resolver 只生成：

```text
Dependency Install Plan
```

包含：

-需要安装；
-需要更新；
-需要用户选择；
-已满足；
-冲突；
-风险；
-权限变化。

真正执行由 Lifecycle Manager。

---

## 十四、共享依赖

共享对象如：

- MCP Server；
-Provider；
-Service Runtime；
-系统 Tool；
-用户资源。

必须使用 Resource Reference 图。

单一 Extension 卸载不能删除仍被引用的依赖。

---

## 十五、隔离依赖

某些 Runtime 可允许版本隔离：

- JavaScript 包依赖；
-WASM 模块；
-私有 Worker。

隔离范围必须由 Runtime 类型声明。

不可把宿主级 Provider 当作可任意隔离依赖。

---

## 十六、Provider 选择

当多个 Provider 满足同一接口：

- 根据优先级；
-用户选择；
-信任；
-版本；
-平台；
-作用域；
-健康；
-成本策略。

选择结果必须进入 Dependency Snapshot。

---

## 十七、Dependency Snapshot

执行或 Runtime 启动前固定：

```go
type DependencySnapshot struct {
    SnapshotID    string
    SourceID      string
    Resolutions   []DependencyResolutionRef
    GraphHash     string
    Generation    int64
    CreatedAt     time.Time
}
```

运行中不因依赖更新自动漂移。

---

## 十八、依赖丢失

依赖发生：

- Disabled；
-Uninstalled；
-Crashed；
-Quarantined；
-Version changed；
-Scope/Permission lost。

Resolver 发布：

```text
DependencyLost
```

影响：

- EffectiveState；
-Runtime Desired State；
-Contribution Active；
-Schedule；
-执行前检查。

---

## 十九、依赖恢复

依赖恢复后：

- 重新解析；
-Generation 更新；
-Registry 重新激活；
-Runtime 可按 Lifecycle Reconcile 启动；
-不得重复注册。

---

## 二十、Tool 执行依赖

执行前检查：

- Tool Definition 依赖；
-Runtime；
-MCP Server；
-Provider；
-Host Feature；
-平台。

不在执行时自动安装依赖。

---

## 二十一、Agent Skill 依赖

Agent Skill 依赖：

- Tool；
-MCP Server；
-Host Feature。

缺失时不进入完整激活候选，Optional 依赖允许降级。

---

## 二十二、Workflow 依赖

编译时解析：

- Tool；
-Sub Workflow；
-Provider；
-Event Type。

执行时使用固定 Snapshot，并在每个高风险节点前确认关键依赖仍存在。

---

## 二十三、MCP 依赖

Agent Skill 或 Extension 对 MCP 的依赖不能直接等于“创建 Server”。

Resolver 输出：

```text
reuse existing
create synthetic/user server
install extension-owned server
manual configuration required
```

由 Lifecycle Plan 执行。

---

## 二十四、Dependency Resolver 接口

```go
type DependencyResolver interface {
    Resolve(
        ctx context.Context,
        request DependencyResolveRequest,
    ) DependencyResolveResult

    BuildGraph(
        ctx context.Context,
        roots []string,
    ) DependencyGraph

    Snapshot(
        ctx context.Context,
        sourceID string,
    ) (DependencySnapshot, error)

    AffectedBy(
        ctx context.Context,
        targetID string,
    ) ([]DependencyAffectedSubject, error)
}
```

---

## 二十五、缓存与 Generation

缓存键：

```text
source_definition_hash
installation_generation
dependency_generation
platform
host_version
```

依赖变化必须精确失效。

---

## 二十六、持久化

建议：

```text
extension_dependency_definitions
extension_dependency_resolutions
extension_dependency_snapshots
extension_dependency_conflicts
extension_dependency_generations
```

---

## 二十七、生命周期接入

Install/Update/Enable/Disable/Uninstall Plan 必须调用 Resolver。

禁止 Lifecycle Manager 自己临时遍历字符串依赖。

---

## 二十八、Registry 接入

Contribution Registry 使用 Resolver 输出注册顺序和激活条件。

---

## 二十九、Runtime 接入

Runtime Supervisor 仅启动 Dependency Snapshot 满足的 Runtime。

---

## 三十、前端

展示：

- Required/Optional；
-目标；
-版本；
-当前选中；
-冲突；
-缺失；
-受影响 Extension；
-共享引用；
-用户选择。

---

## 三十一、测试要求

覆盖：

- SemVer；
-多约束求交；
-Required/Optional；
-循环；
-共享依赖；
-隔离依赖；
-Provider 多候选；
-平台；
-Host Feature；
-依赖丢失与恢复；
-Snapshot；
-并发更新；
-安装子计划；
-卸载阻塞；
-大图性能。

---

## 三十二、实施任务

1. 定义 Dependency DTO。
2. 实现 SemVer Range。
3. 建立 Candidate Provider。
4. 实现图构建。
5. 实现循环检测。
6. 实现约束求解。
7. 实现冲突检测。
8. 实现 Provider 选择。
9. 实现共享资源引用检查。
10. 实现 Snapshot。
11. 实现 Dependency Generation。
12. 接入 Lifecycle Manager。
13. 接入 Contribution Registry。
14. 接入 Runtime Supervisor。
15. 接入 EffectiveStateResolver。
16. 迁移 MCP/Agent Skill/Workflow 旧依赖。
17. 改造前端。
18. 完成测试与报告。

---

## 三十三、验收标准

1. 所有依赖使用统一 Resolver。
2. Required 与 Optional 明确。
3. SemVer 约束稳定。
4. Required 循环可阻止。
5. 共享资源不会被单方删除。
6. Resolver 不自动安装。
7. Lifecycle 使用依赖计划。
8. Registry 使用依赖顺序。
9. Runtime 使用 Dependency Snapshot。
10. 依赖丢失可使 Contribution 不可用。
11. 旧依赖系统停止新增写入。
12. 可进入第 25 步 Runtime Supervisor。

---

## 三十四、执行约束

> Dependency Resolver 只解析和计划依赖，不执行安装、授权、连接或启动。

禁止：

- 自动下载；
-自动 Grant；
-将 Optional 当 Required；
-依赖丢失时自动删除 Source；
-卸载时忽略 Dependents；
-使用当前 Runtime 状态替代 Definition 依赖；
-新旧 Resolver 双写。
