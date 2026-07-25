# 扩展系统冻结范围 PR 评审模板

> 本模板适用于 Amitia 扩展系统重构第 1 步「冻结现有扩展系统功能开发」期间，涉及旧扩展系统冻结范围的所有 Pull Request。
> 冻结开始日期：2026-07-25
> 冻结总说明：[`docs/extension-kernel/01-system-freeze.md`](../docs/extension-kernel/01-system-freeze.md)
> 旧系统变更策略：[`docs/extension-kernel/legacy-change-policy.md`](../docs/extension-kernel/legacy-change-policy.md)
> 实施依据：[`.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`](../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md)

---

## 一、适用范围

本模板适用于涉及以下任意路径或对象的 PR：

- `backend/internal/extension/**`
- `backend/internal/mcp/**`
- `front/src/views/extensions/**`
- `front/src/views/mcp/**`
- `front/src/views/creative-workshop/**`
- `backend/internal/extension/schema/manifest.schema.json`
- 任何涉及旧扩展系统数据库表、Registry、路由的改动
- 任何涉及 Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` 旧扩展包、扩展中心与创意工坊的改动

提交者在本模板外的 PR 描述中若已包含等效信息，可在评审时引用，但仍须保证本模板每一项均被覆盖。

---

## 二、提交者必填说明

提交者必须在 PR 描述中如实填写以下 8 项。若某项不适用，须明确写「不适用」并说明原因，禁止留空。

### 1. 修改原因

- [ ] 已说明本次修改的业务或技术原因
- 修改原因：

### 2. 修改分类（缺陷 / 安全 / 测试 / 迁移，可多选）

- [ ] 阻塞性缺陷修复（`fix`）
- [ ] 安全修复（`security`）
- [ ] 回归测试补充（`test`）
- [ ] 迁移辅助能力（`migration`）
- [ ] 其他（须说明为何不属于以上四类且仍属允许范围）：

### 3. 是否新增字段

- [ ] 否
- [ ] 是，字段位置与用途：

### 4. 是否新增数据表

- [ ] 否
- [ ] 是，表名与用途，并标记 `permanent` / `migration-only` / `temporary`：

### 5. 是否改变已有行为

- [ ] 否
- [ ] 是，行为变化与影响范围：

### 6. 是否增加兼容层

- [ ] 否
- [ ] 是，兼容层位置、退出条件与删除计划：

### 7. 后续是否需要删除

- [ ] 否
- [ ] 是，删除时机与对应迁移步骤编号：

### 8. 对新 Extension Kernel 的影响

- [ ] 无影响
- [ ] 有影响，影响说明：

---

## 三、必查项（13 项）

评审者必须逐项确认。任意一项被勾选为「是」时，须在 PR 中给出明确说明，否则按违规处理。

- [ ] 是否增加新产品能力
- [ ] 是否扩大旧架构职责
- [ ] 是否增加永久兼容层
- [ ] 是否增加重复状态
- [ ] 是否新增数据库表
- [ ] 是否新增 Registry
- [ ] 是否新增权限判断
- [ ] 是否新增独立生命周期
- [ ] 是否直接修改 Manifest v1
- [ ] 是否增加新 UI 入口
- [ ] 是否影响后续迁移
- [ ] 是否包含测试
- [ ] 是否有回滚方式

---

## 四、直接拒绝条件（9 项）

出现以下任意情况，PR 应直接拒绝，不再继续评审：

1. 在旧 Plugin 上增加第三方插件能力
2. 在旧 `.amitiax` 中增加代码运行时
3. 新建另一套 Tool/Skill Registry
4. 新增重复权限系统
5. 新增与现有表功能重复的数据表
6. 通过双写维持新旧系统长期并行
7. 修改旧系统但未提供必要测试
8. 修改核心执行链但未说明迁移影响
9. 将前端页面状态继续绑定到旧系统内部实现

---

## 五、提交类型约束提醒

### 1. 允许的提交类型

```text
fix(extension):
fix(mcp):
test(extension):
test(mcp):
docs(extension):
refactor(extension-foundation):
migration(extension):
security(extension):
```

### 2. 禁止的提交类型

```text
feat(skill):
feat(plugin):
feat(mcp):
feat(workflow):
feat(package):
```

例外：除非该功能明确属于新的 Extension Kernel，且不修改旧系统职责。提交者若主张适用例外，必须在 PR 描述中给出依据与对应新系统规划文档引用。

---

## 六、评审流程提示

1. 提交者按第二节填写 8 项必填说明。
2. 评审者按第三节逐项核对 13 项必查项。
3. 评审者按第四节核对 9 项直接拒绝条件，命中任意一项即拒绝。
4. 评审者按第五节核对提交类型是否符合约束。
5. 任意一项不满足时，要求提交者修订后重新提交。
6. 通过后须在 PR 中保留评审记录，便于后续迁移步骤追溯。

完整变更策略见 [`docs/extension-kernel/legacy-change-policy.md`](../docs/extension-kernel/legacy-change-policy.md)。
