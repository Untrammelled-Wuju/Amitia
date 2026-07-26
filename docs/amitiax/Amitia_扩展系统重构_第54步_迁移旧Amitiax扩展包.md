# Amitia 扩展系统重构第 54 步实施文档

## 第 54 步：迁移旧 `.amitiax` 扩展包

---

## 一、步骤目标

将旧 `.amitiax` v1、旧 Package 表、旧导入目录、旧 Skill/Workflow 包和历史安装记录迁移到 Manifest v2、ExtensionDefinition、ExtensionInstallation、Artifact、Resource Ownership 和唯一 Lifecycle Manager。

本步骤目标是：

> 在不执行旧包代码、不扩大权限、不覆盖用户数据、不保留旧 Parser 为主入口的前提下，将可识别旧包转换为正式 v2 Extension 或隔离为只读 Legacy Package。

---

## 二、旧包类型

旧 `.amitiax` 可能包含：

-单个 Skill；
-多个 Skill；
-Workflow；
-Instruction；
-MCP 声明；
-资源；
-旧 Plugin 类型标记；
-不存在真实 Runtime 的 Plugin 声明；
-旧 Manifest；
-旧校验和；
-用户导出包；
-损坏包；
-重复版本；
-无 Publisher；
-无签名；
-历史安装目录。

必须先分类。

---

## 三、迁移结果类型

每个旧包最终进入以下之一：

```text
converted_to_v2
converted_to_synthetic_extension
imported_disabled
legacy_read_only
quarantined
manual_review
invalid
duplicate
```

不得只有“成功/失败”二态。

---

## 四、迁移链路

```text
Legacy Package Artifact
→ Package Security
→ Legacy Parser
→ Canonical Migration DTO
→ Classification
→ v2 Domain Builder
→ Validation
→ Migration Plan
→ New Installation Transaction
```

禁止旧 PackageService 直接复制旧目录。

---

## 五、Artifact 发现

扫描来源：

-旧 Artifact 目录；
-旧安装目录；
-旧数据库 Package 记录；
-用户导入目录；
-缓存；
-备份；
-回滚目录。

扫描必须限定已知目录，不做全盘扫描。

---

## 六、Artifact 去重

使用：

-Archive Hash；
-Content Tree Hash；
-Legacy Package ID；
-Version；
-安装路径；
-数据库记录。

完全相同 Artifact 只迁一次。

---

## 七、旧 Manifest 解析

通过独立 Legacy Parser。

Parser 只输出：

-字段；
-类型；
-资源；
-声明；
-警告；
-未知内容；
-Hash；
-来源。

不执行、不连接、不安装。

---

## 八、类型分类

### Skill-only 包

转换为 Agent Skill Extension 或 Synthetic Extension。

### Workflow-only 包

转换为 Workflow Extension。

### Skill + Workflow

转换为多 Module v2 Extension。

### MCP 声明包

提取 MCP Contribution，并建立 Agent Skill/Workflow 依赖。

### 旧 Plugin 声明但无 Runtime

不能伪造 Plugin Runtime。

根据实际内容转换为 Tool/Agent Skill/Workflow/MCP，或标记 Invalid。

### 真实旧内置 Plugin 引用

仅官方已知映射可转换 LegacyGoRuntime。

第三方旧包不能声明 `legacy_go`。

---

## 九、Extension ID 生成

优先：

1.旧稳定 ID；
2.Publisher + Package Name；
3.用户本地 Synthetic ID；
4.内容 Hash 派生稳定后缀。

未知发布者的用户包：

```text
local.user/imported-<name>-<short-hash>
```

不得冒用官方命名空间。

---

## 十、版本迁移

合法 SemVer 保留。

非法版本：

-规范化；
-记录原值；
-无法规范化使用 `0.0.0-legacy.<hash>`；
-默认不允许自动更新；
-标记 Migration Warning。

---

## 十一、Publisher 与 Trust

旧包通常无签名。

迁移后：

```text
publisher=unknown 或 local.user
trust=development/user_imported
```

不得自动 Trusted。

旧包即使曾安装成功，也不等于可信 Publisher。

---

## 十二、Manifest v2 生成

转换器生成 Manifest v2 输入或直接生成 ExtensionDefinition。

推荐同时生成：

```text
migration-generated manifest v2
```

用于：

-导出；
-诊断；
-重打包；
-用户查看。

生成内容必须明确：

```text
generatedFromLegacy=true
```

---

## 十三、Module 拆分

按照内容建立：

```text
agent-skills
workflows
mcp
resources
ui-schema
```

无代码包使用：

```text
RuntimeType=static/workflow/mcp
```

不得默认生成 JavaScript Runtime。

---

## 十四、资源迁移

资源：

-路径；
-MIME；
-Hash；
-Owner；
-引用；
-大小；
-类型；
-是否用户修改。

损坏、缺失或越界资源：

-不复制；
-记录；
-依赖对象可能 Disabled；
-不得用空文件替代。

---

## 十五、完整性

旧包无文件清单时：

-由 Package Security 重新计算 Content Tree；
-生成新 Integrity 文件；
-标记 `legacy_integrity_generated`；
-不伪造原始 Publisher 签名。

---

## 十六、签名

迁移生成包不能继承不存在或无效的旧签名。

如果用户重新导出：

-使用本地开发/用户签名；
-或保持 unsigned；
-不得标记原 Publisher 已签名。

---

## 十七、Enabled 迁移

旧 Package Enabled、Skill Enabled、Workflow Enabled 可能冲突。

按第 19 步规则：

-建立 Extension Enabled；
-Module Enabled；
-Contribution Override；
-Schedule Enabled；
-Scope Binding。

冲突默认更安全状态。

---

## 十八、Scope 迁移

旧角色、全局、会话设置：

-分别建 Scope Binding；
-角色不存在标记孤儿；
-不能迁为 Global；
-会话必须校验角色；
-无 Scope 信息时按类型决定是否要求用户选择。

---

## 十九、Permission 迁移

旧包无 Permission 声明：

-根据内容和 Contribution 推导 Requirements；
-不是自动 Grant；
-安装后默认 Disabled 或等待确认；
-网络、文件、桌面、MCP Command 等必须显式展示。

---

## 二十、依赖迁移

提取：

-Tool；
-MCP；
-Workflow；
-Provider；
-Host Feature。

缺失 Required Dependency：

-可安装为 Disabled；
-或阻止启用；
-生成报告。

---

## 二十一、安装记录迁移

旧已安装包转换：

-ExtensionInstallation；
-Installed Version；
-安装时间；
-来源；
-Enablement；
-Artifact；
-Module；
-Owner；
-迁移状态。

旧安装路径不作为新运行路径。

---

## 二十二、用户修改检测

比较：

-原 Artifact Hash；
-安装目录 Hash；
-当前文件 Hash。

若不一致：

```text
user_modified
```

处理：

-保留为用户 Fork；
-生成 Synthetic Extension；
-不被原包更新覆盖；
-显示差异。

---

## 二十三、重复和冲突

同 Extension ID 多个旧包：

-按 Version；
-Hash；
-安装状态；
-用户修改；
-来源；
-当前使用；

生成选择计划。

不得静默覆盖。

---

## 二十四、旧安装目录

迁移成功后：

-新系统只使用新 Artifact/版本目录；
-旧目录进入只读保留期；
-确认运行稳定后清理；
-清理通过 Resource Release Plan；
-不立即删除唯一原始数据。

---

## 二十五、回滚

迁移初期保留：

-旧 Artifact；
-旧数据库快照；
-旧安装目录只读副本；
-映射；
-迁移报告。

回滚只能用于数据恢复，不允许重新启用旧 Parser/PackageService 作为主运行链。

---

## 二十六、导出

迁移成功后可导出新 `.amitiax` v2：

-Manifest v2；
-Integrity；
-资源；
-用户修改；
-License；
-本地 Publisher/Unsigned 状态。

---

## 二十七、前端迁移向导

展示：

-发现的旧包；
-类型；
-可转换内容；
-未知内容；
-权限；
-Scope；
-依赖；
-用户修改；
-冲突；
-目标 Extension；
-默认 Disabled；
-保留/跳过/隔离。

用户可逐包确认。

---

## 二十八、批量策略

系统内置已知旧包可自动计划。

用户第三方包：

-默认逐包预览；
-可批量选择低风险无代码包；
-有 Command、网络或未知类型必须单独确认。

---

## 二十九、迁移报告

每个包记录：

-Source；
-Hash；
-Legacy Manifest；
-分类；
-目标 ID；
-Modules；
-Contributions；
-资源；
-权限；
-Scope；
-依赖；
-用户修改；
-警告；
-冲突；
-结果；
-新 Installation ID。

---

## 三十、兼容读取

迁移期间旧包详情 API 可通过 Migration Report 显示。

禁止新系统调用旧 Package Runtime。

---

## 三十一、测试要求

覆盖：

-Skill-only；
-Workflow-only；
-混合；
-MCP；
-旧 Plugin 假声明；
-官方已知 Plugin；
-无版本；
-非法版本；
-无签名；
-损坏；
-路径攻击；
-重复；
-用户修改；
-Enabled 冲突；
-Scope；
-依赖；
-安装记录；
-导出 v2；
-回滚数据；
-大量包；
-跨平台路径。

---

## 三十二、实施任务

1. 输出旧包全量清单。
2.实现 Artifact 发现和去重。
3.完善 Legacy Parser。
4.实现分类器。
5.实现稳定 ID 生成。
6.实现 v2 Manifest/Domain 转换。
7.实现 Module/Contribution 拆分。
8.迁移资源和 Integrity。
9.迁移 Enabled/Scope。
10.生成 Permission Requirements。
11.迁移 Dependencies。
12.迁移 Installation。
13.检测用户修改。
14.实现冲突计划。
15.实现前端迁移向导。
16.实现 v2 导出。
17.保留回滚资产。
18.冻结旧 PackageService。
19.完成全类型测试。

---

## 三十三、验收标准

1. 所有旧 `.amitiax` 已分类。
2.旧包只通过 Package Security/Legacy Parser 读取。
3.能转换的包生成 v2 领域对象。
4.假 Plugin 声明不会生成 Runtime。
5.无签名包不会自动 Trusted。
6.Permission 不自动 Grant。
7.Scope 不扩大。
8.用户修改转为 Fork。
9.新系统不使用旧安装目录运行。
10.重复和冲突可解释。
11.迁移后可导出 v2。
12.旧 PackageService 停止新安装。
13.关键测试通过。
14.可进入第 55 步扩展数据迁移。

---

## 三十四、执行约束

> 旧 `.amitiax` 迁移的目标是尽可能保存用户可识别的定义和资源，而不是维持旧包格式和旧运行逻辑的永久兼容。

禁止：

-旧包直接运行；
-假 Plugin 自动 Runtime；
-旧签名伪继承；
-未知权限自动 Grant；
-角色 Scope 变 Global；
-用户修改被覆盖；
-新系统从旧目录加载；
-旧 Parser 继续主导安装。
