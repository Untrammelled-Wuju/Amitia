# Amitia 扩展系统重构第 62 步实施文档

## 第 62 步：执行新旧系统等价性验证

---

## 一、步骤目标

在第 1—61 步已完成 Extension Kernel、统一生命周期、`.amitiax` v2、多 Runtime、UI Contribution、正式迁移、SDK、CLI、开发模式、扩展中心和详情页后，进入最终切换前的系统等价性验证。

本步骤目标是：

> 对旧系统当前仍在生产链路中承担的有效能力，与新 Extension Kernel 中对应能力进行逐项、逐链路、逐副作用、逐平台验证，证明新系统在功能正确性、权限边界、作用域、运行结果、资源管理、用户数据、稳定性和可观测性上达到或优于旧系统，且不存在关键能力缺失、重复执行或语义漂移。

本步骤不是简单“页面能打开”验收，而是正式切换前的业务等价性证明。

---

## 二、验证范围

必须覆盖：

-内置 Tools；
-Agent Skills；
-MCP；
-Workflows；
-官方内置 Plugins；
-旧 `.amitiax`；
-扩展安装；
-启用/禁用；
-更新；
-回滚；
-卸载；
-权限；
-Scope；
-Storage；
-Secret；
-Event；
-Hook；
-Schedule；
-Background Task；
-UI Contribution；
-Desktop Contribution；
-扩展中心；
-扩展详情页；
-开发模式；
-迁移数据；
-运行历史；
-资源清理；
-应用启动/关闭/恢复。

---

## 三、等价性分类

验证结果分为：

```text
equivalent
improved
intentionally_changed
missing
regressed
not_applicable
blocked
```

### equivalent

新旧行为一致。

### improved

新系统功能更完整或更安全。

### intentionally_changed

基于新架构明确改变，需有决策记录。

### missing

新系统缺失旧有效能力。

### regressed

新系统行为错误或性能下降。

### blocked

受外部依赖或测试条件阻塞。

---

## 四、等价性基准

每项能力至少比较：

-入口；
-输入；
-输出；
-错误；
-副作用；
-幂等；
-超时；
-取消；
-并发；
-权限；
-Scope；
-资源；
-审计；
-恢复；
-前端显示；
-跨平台；
-用户数据；
-性能。

---

## 五、基线数据来源

使用第 5 步建立的旧系统基线测试和以下数据：

-旧系统调用链；
-运行日志；
-用户配置；
-真实 Extension；
-MCP Server；
-Workflow；
-Agent Skill；
-Plugin；
-历史 `.amitiax`；
-故障案例；
-性能指标；
-已知缺陷。

禁止临时以新系统行为反向定义“等价”。

---

## 六、Golden Case

为每类能力建立 Golden Case：

```text
输入
前置状态
权限
Scope
依赖
预期输出
预期副作用
预期审计
预期资源变化
预期错误
```

每个 Golden Case 同时在旧兼容环境和新系统执行，或使用旧基线记录对比。

---

## 七、内置 Tool 验证

逐 Tool 验证：

-稳定 ID 映射；
-模型名称；
-Input/Output Schema；
-权限；
-Scope；
-执行结果；
-副作用；
-错误；
-消息发送幂等；
-记忆读写；
-文件；
-Provider；
-桌面；
-取消；
-并发；
-审计。

P0 条件：

-消息重复；
-跨角色读写；
-绕过权限；
-Tool 不可用；
-结果格式破坏模型调用。

---

## 八、Agent Skill 验证

验证：

-SKILL.md 内容；
-资源；
-渐进加载；
-Token Budget；
-激活；
-Scope；
-依赖；
-Prompt 注入顺序；
-用户修改；
-角色专属；
-MCP 依赖；
-卸载/更新；
-不产生伪 Tool。

必须对比 Token 消耗和 Prompt 长度。

---

## 九、MCP 验证

覆盖：

-stdio；
-Streamable HTTP；
-OAuth；
-Headers；
-Secret；
-连接；
-手动断开；
-重连；
-Discovery；
-Tool Schema；
-Resources；
-Prompts；
-Tasks；
-Sampling；
-Elicitation；
-Roots；
-故障恢复；
-共享引用；
-应用重启。

重点验证：

-不重复 Tool；
-不双连接；
-手动断开不自动恢复；
-Secret 不泄漏；
-Session 不持久复用。

---

## 十、Workflow 验证

覆盖：

-Tool Node；
-Condition；
-Transform；
-Parallel；
-Join；
-Delay；
-Approval；
-Sub Workflow；
-Event Trigger；
-Schedule；
-Compensation；
-Retry；
-取消；
-恢复；
-Tool Exposure；
-用户 Fork。

重点验证：

-旧 Schedule 与新 Schedule 不双触发；
-非幂等节点不自动重复；
-运行中 Definition 固定；
-角色 Scope 正确。

---

## 十一、官方 Plugin 验证

每个 Plugin 按迁移档案验证：

-Tool；
-Hook；
-Event；
-Schedule；
-Worker；
-State；
-Secret；
-Permission；
-Scope；
-Provider；
-UI；
-Desktop；
-生命周期；
-资源清理；
-运行性能。

必须证明旧 PluginManager 已不再承担实际业务主链。

---

## 十二、`.amitiax` 验证

覆盖：

-Manifest v2；
-多 Module；
-签名；
-Publisher；
-包安全；
-解析；
-安装；
-更新；
-回滚；
-卸载；
-旧包迁移；
-用户修改；
-平台文件；
-Resource；
-UI；
-Runtime。

---

## 十三、生命周期等价性

逐操作验证：

### Install

-文件；
-数据库；
-Definition；
-Resource；
-Permission Requirement；
-Scope；
-Runtime；
-Contribution；
-失败补偿。

### Enable/Disable

-Enabled 真值；
-Desired Runtime；
-Schedule；
-UI；
-MCP；
-运行中调用。

### Update/Rollback

-Generation；
-数据迁移；
-用户资产；
-权限变化；
-Scope；
-失败恢复。

### Uninstall

-资源 Release Plan；
-用户数据；
-Secret；
-共享资源；
-历史保留。

---

## 十四、权限等价性

对比旧行为，但新系统必须更严格。

验证：

-旧默认放行；
-新 Requirement；
-用户 Grant；
-拒绝；
-Approval；
-撤销；
-条件；
-高风险；
-后台任务；
-桌面；
-MCP；
-Provider；
-UI。

出现新系统无法完成旧操作时，判断：

-旧操作本身越权；
-新权限缺失；
-迁移 Grant 缺失；
-产品行为变更。

---

## 十五、Scope 等价性

重点：

-Global；
-Character；
-Conversation；
-Schedule；
-Event；
-UI；
-Tool；
-Agent Skill；
-MCP；
-Workflow。

P0：

-角色数据串用；
-会话跨角色；
-当前前端角色替代固定 Scope；
-Global 扩大；
-后台任务使用旧 Scope。

---

## 十六、Storage/Secret 等价性

验证：

-配置读取；
-状态；
-用户数据；
-Cache；
-CAS；
-配额；
-迁移；
-备份；
-Secret；
-OAuth；
-轮换；
-卸载；
-用户数据保留。

必须确保新系统没有因隔离改变业务数据语义。

---

## 十七、Event/Hook 等价性

验证：

-事件类型；
-触发次数；
-顺序；
-Filter；
-重试；
-幂等；
-Dead Letter；
-Depth；
-Hook 排序；
-修改字段；
-失败策略；
-超时；
-Circuit。

重点排查双投递和顺序变化。

---

## 十八、UI 等价性

验证：

-扩展页面；
-Schema UI；
-Web UI；
-Chat Slot；
-Message Action；
-Composer；
-Custom Message；
-Desktop Menu；
-Tray；
-Shortcut；
-Window；
-主题；
-Locale；
-无障碍；
-卸载回退。

新 UI 可重构，但核心功能不可缺失。

---

## 十九、数据等价性

对账：

-对象数量；
-ID 映射；
-Enabled；
-Scope；
-Permission；
-Storage；
-Secret Reference；
-Schedule；
-资源；
-用户偏好；
-运行历史；
-Owner；
-依赖。

---

## 二十、运行历史等价性

新历史至少应支持：

-旧运行查询；
-新运行追踪；
-错误；
-副作用；
-审计；
-跨 Tool/Workflow/MCP/Plugin Trace。

旧历史可标记 Legacy，但不能丢失关键高风险记录。

---

## 二十一、性能对比

至少比较：

-应用启动；
-扩展加载；
-Tool 调用；
-MCP 连接；
-Workflow；
-Agent Skill Prompt；
-UI 首屏；
-扩展中心；
-详情页；
-内存；
-CPU；
-磁盘；
-包安装；
-更新；
-应用关闭。

定义允许退化阈值，关键链路默认不得显著退化。

---

## 二十二、稳定性对比

测试：

-长时间运行；
-频繁启停；
-频繁角色切换；
-高并发 Tool；
-MCP 重连；
-Workflow 大量任务；
-UI 多扩展；
-应用反复重启；
-系统休眠/唤醒；
-网络断开；
-磁盘不足；
-Runtime Crash；
-Renderer Crash。

---

## 二十三、灰度验证

建议使用：

```text
shadow compare
read-only compare
test profile
development profile
internal dogfood
```

禁止生产副作用双执行。

对于写操作，只能：

-对比 Plan；
-对比预期；
-在隔离测试数据执行；
-不能旧新同时发送消息。

---

## 二十四、差异处理

每个差异必须：

1.分类。
2.确定预期。
3.记录 Owner。
4.确定修复或接受。
5.补充测试。
6.更新迁移文档。
7.关闭后复验。

---

## 二十五、P0 阻塞条件

以下存在不得进入第 65 步：

-消息重复或丢失；
-角色/会话数据串用；
-权限绕过；
-Secret 泄漏；
-双 Schedule；
-双 MCP；
-双 Event；
-旧 Tool 仍执行；
-卸载删除用户资产；
-更新无法回滚；
-Registry 无法重建；
-应用无法稳定启动/关闭；
-关键官方 Plugin 缺失；
-数据迁移对账不一致。

---

## 二十六、验证矩阵

建议文件：

```text
docs/extension-kernel/validation/equivalence-matrix.csv
```

字段：

```text
Capability
Legacy ID
Canonical ID
Scenario
Platform
Expected
Actual
Status
Difference
Risk
Owner
Evidence
```

---

## 二十七、自动化

将 Golden Case 纳入：

-单元测试；
-集成测试；
-E2E；
-跨平台；
-故障注入；
-性能基线；
-安全测试。

---

## 二十八、证据

每项验收保存：

-测试日志；
-Trace；
-Screenshot；
-数据库对账摘要；
-性能报告；
-资源报告；
-审计；
-包 Hash；
-环境版本。

避免只写“测试通过”。

---

## 二十九、实施任务

1. 建立等价性矩阵。
2.导入第 5 步基线。
3.建立 Golden Case。
4.验证内置 Tool。
5.验证 Agent Skill。
6.验证 MCP。
7.验证 Workflow。
8.验证官方 Plugin。
9.验证 `.amitiax`。
10.验证 Lifecycle。
11.验证 Permission/Scope。
12.验证 Storage/Secret。
13.验证 Event/Hook。
14.验证 UI/Desktop。
15.完成数据对账。
16.完成性能和稳定性对比。
17.修复所有 P0/P1 差异。
18.输出等价性验收报告。

---

## 三十、验收产物

必须提交：

-等价性矩阵；
-Golden Case；
-自动化测试；
-数据对账；
-性能报告；
-稳定性报告；
-差异清单；
-修复记录；
-接受的语义变更；
-P0/P1 清零证明；
-最终签字报告。

---

## 三十一、验收标准

1. 旧有效能力全部有新系统对应项。
2.关键行为等价或明确改进。
3.所有差异有分类和决策。
4.Tool、MCP、Workflow、Plugin 无双执行。
5.Permission 和 Scope 无越权。
6.Secret 无泄漏。
7.用户数据完整。
8.UI 核心功能完整。
9.性能无不可接受退化。
10.P0 清零。
11.P1 有明确关闭或接受。
12.可进入第 63 步桌面稳定性验收。

---

## 三十二、执行约束

> 等价性验证必须以旧系统真实基线和新系统正式链路为依据，不得通过修改预期、跳过高风险场景或保留旧主链来“证明通过”。

禁止：

-生产写副作用双执行；
-只测页面；
-只测成功路径；
-忽略 Scope；
-忽略历史数据；
-把旧越权行为当必须保留；
-无证据签字；
-P0 未清零进入切换。
