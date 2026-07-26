# Amitia 扩展系统重构第 32 步实施文档

## 第 32 步：重写 `.amitiax` 安装事务

---

## 一、步骤目标

在包安全、Manifest v2、布局和 Parser 基础上，重写 `.amitiax` 安装事务，使安装成为 Lifecycle Manager 控制的可计划、原子、可补偿、可恢复操作。

目标：

```text
Package Source
→ Security Verify
→ Sealed Staging
→ Parse
→ Dependency/Permission/Scope Preview
→ Lifecycle Plan
→ Snapshot
→ Artifact Commit
→ Definition/Installation Commit
→ Resource Registration
→ Runtime Reconcile
→ Contribution Activation
→ Finalize
```

---

## 二、职责边界

安装事务负责：

-安装阶段 Step Handler；
-Artifact 提交；
-Definition 保存；
-Installation 创建；
-Module 创建；
-Owner 注册；
-失败补偿；
-Recovery Journal；
-Postcondition。

它不独立接受前端请求，入口属于 Lifecycle Manager。

---

## 三、安装状态

```text
planned
awaiting_confirmation
staging
verified
parsed
committing
registering
activating
installed
failed
compensating
recovery_required
```

最终 InstallationState 仍使用领域模型。

---

## 四、事务边界

数据库内部可使用 ACID。

文件与 Runtime 采用：

```text
Durable Steps + Journal + Compensation
```

不得声称整个安装是单一数据库事务。

---

## 五、安装目录

```text
artifacts/<extension>/<version>/<hash>.amitiax
extensions/<extension>/versions/<version>/<definition-hash>/
extensions/<extension>/current
```

`current` 使用原子引用切换。

---

## 六、预检

安装前：

-包安全；
-签名；
-Publisher；
-Manifest；
-兼容性；
-依赖；
-权限；
-Scope；
-磁盘；
-冲突；
-已有安装；
-用户资产；
-平台；
-开发模式。

---

## 七、安装计划

必须展示：

-Extension/Version；
-Modules；
-Contributions；
-Runtimes；
-权限；
-Scope；
-依赖；
-文件；
-存储；
-Secret 需求；
-MCP；
-后台任务；
-UI；
-风险；
-数据保留。

---

## 八、Artifact Commit

步骤：

1. 将验证包复制到目标临时目录。
2. fsync 文件。
3. 校验 Hash。
4. 原子 rename 到不可变 Artifact。
5. 校验内容。
6.登记 Resource Ownership。

跨文件系统时使用 copy + fsync + verify + swap。

---

## 九、安装视图

从 Sealed Staging 或 Artifact 构建只读版本目录。

不得让 Runtime 使用 Staging。

---

## 十、数据库 Commit

建议：

-保存 Definition Version；
-创建 Installation pending；
-创建 Installed Modules；
-保存 Contribution Definition；
-保存 Runtime Definition；
-保存 Dependencies；
-保存 Permission Requirements；
-保存资源引用；
-写 Outbox。

使用唯一约束和 Generation。

---

## 十一、注册阶段

数据库提交后：

```text
Contribution Registry Register Batch
→ Runtime Desired State
→ Runtime Reconcile
→ Activate Eligible
```

若 Runtime 失败，安装可成功但启用部分失败，结果必须明确。

---

## 十二、默认启用

包可声明 `enabledByDefault`，但宿主策略决定。

未知发布者或高风险 Runtime 默认安装后禁用。

---

## 十三、依赖安装

不允许 Parser 或安装 Step 自动下载。

若依赖缺失：

-生成子 Plan；
-用户确认；
-按拓扑安装；
-父安装等待；
-失败补偿。

---

## 十四、权限

安装只存 Permission Requirements。

Grant 由用户确认和 Permission Broker 管理。

---

## 十五、Scope

安装可创建用户已确认的 Scope Binding。

Manifest 默认 Scope 不自动绑定具体角色。

---

## 十六、重复安装

同 Extension + Version + Hash：

-可视为幂等；
-验证现有安装；
-不重复写；
-可进入 Repair。

同版本不同 Hash：

```text
version_republish_conflict
```

拒绝。

---

## 十七、覆盖安装

禁止直接覆盖。

应使用 Update 或 Repair。

---

## 十八、失败补偿

按 Journal：

-停止新 Runtime；
-注销新 Contribution；
-删除 Installation pending；
-移除新 Owner 引用；
-删除未引用安装视图；
-保留原始 Artifact 或按策略清理；
-清理 Staging；
-保留报告与审计。

---

## 十九、崩溃恢复

启动时：

-扫描 pending Operation；
-验证 Artifact；
-验证数据库；
-验证 Registry；
-验证 Runtime；
-决定继续、补偿或人工修复。

---

## 二十、取消

允许取消阶段：

-下载/读取；
-Security；
-Parse；
-Plan；
-确认前。

进入 Artifact/DB Commit 后只能在安全 Step 边界取消并补偿。

---

## 二十一、Postcondition

成功必须满足：

-Artifact Hash 正确；
-Definition 可读取；
-Installation=installed；
-Module 完整；
-Owner 完整；
-Registry 可重建；
-无 Staging；
-当前版本引用正确；
-审计完成。

---

## 二十二、旧导入迁移

旧 `.amitiax` 仍先进入 Package Security 和 Legacy Parser，再构建新 Lifecycle Plan。

禁止旧 PackageService 直接复制目录。

---

## 二十三、测试要求

覆盖：

-最小包；
-多 Module；
-未知发布者；
-权限确认；
-依赖子计划；
-磁盘不足；
-跨盘；
-Artifact 冲突；
-DB 失败；
-Registry 失败；
-Runtime 失败；
-取消；
-每个 Step 崩溃；
-重复安装；
-同版本不同 Hash；
-跨平台文件锁；
-恢复；
-资源泄漏。

---

## 二十四、实施任务

1. 定义安装 Step。
2. 接入 Lifecycle Planner。
3. 实现 Artifact Committer。
4. 实现安装视图构建。
5. 实现 Definition/Installation DB Step。
6. 接入 Resource Ownership。
7. 接入 Dependency/Permission/Scope。
8. 接入 Contribution Registry。
9. 接入 Runtime Supervisor。
10. 实现补偿。
11. 实现 Recovery。
12. 实现 Postcondition。
13. 迁移旧 PackageService。
14. 改造前端安装进度。
15. 完成故障注入。

---

## 二十五、验收标准

1. 安装只有 Lifecycle Manager 入口。
2. 安装先 Plan。
3. Artifact 不可变。
4. 同版本不同 Hash 拒绝。
5. 数据库和文件有补偿。
6. Parser 不写最终目录。
7. 安装不自动 Grant。
8. 依赖通过子 Plan。
9. Runtime 失败可解释。
10. 崩溃可恢复。
11. 旧 PackageService 不再主导安装。
12. 可进入第 33 步签名与发布者信任。

---

## 二十六、执行约束

> 安装事务的成功标准是定义、安装事实、资源和注册状态一致，而不是“文件已解压”。

禁止：

-直接解压 current；
-安装阶段执行代码；
-自动 Grant；
-覆盖同版本；
-失败后遗留可运行 Runtime；
-新旧安装双写；
-前端直接复制包。
