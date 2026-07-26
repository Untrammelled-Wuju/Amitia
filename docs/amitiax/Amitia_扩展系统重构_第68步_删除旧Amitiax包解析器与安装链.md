# Amitia 扩展系统重构第 68 步实施文档

## 第 68 步：删除旧 `.amitiax` 包解析器与安装链

---

## 一、步骤目标

在所有旧 `.amitiax` 已完成分类、迁移或隔离，正式导入只使用 Manifest v2 Parser、Package Security 和 Lifecycle Manager 后，删除旧包解析器、旧安装器、旧更新/卸载链和旧包目录运行逻辑。

本步骤目标：

> 确保任何 `.amitiax` 文件都只能经过 Package Security、Sealed Staging、Manifest v2/Legacy Migration Parser、Domain Builder 和 Lifecycle Plan，彻底取消旧 PackageService 直接解压、复制、注册和运行能力。

---

## 二、删除前置条件

必须满足：

-第 54 步旧包迁移完成；
-旧包全量分类；
-旧 v1 需要保留的包已生成 v2 或 Legacy Read-only；
-新安装事务通过；
-更新/回滚通过；
-旧 PackageService 无生产调用；
-旧安装目录不被 Runtime 使用；
-旧包 API 已映射；
-回滚资产完整；
-旧 Parser 调用统计为零或仅迁移工具。

---

## 三、删除对象

至少包括：

```text
LegacyPackageParser (runtime path)
OldAmitiaxParser
PackageService.Install
PackageService.Update
PackageService.Uninstall
PackageExtractor
PackageRegistry
PackageEnabled
PackageRuntimeLoader
PackageSkillImporter
PackageWorkflowImporter
PackagePluginImporter
PackageMCPImporter
OldChecksumVerifier
OldManifestTypes
OldInstallDirectoryResolver
```

迁移专用 Legacy Parser 可保留为离线工具，但不能处于生产安装入口。

---

## 四、保留对象

允许保留：

-只读 Legacy Parser Library；
-迁移 CLI；
-旧 Manifest DTO；
-旧包诊断；
-旧 Artifact；
-旧 ID Mapping；
-历史安装记录；
-旧包导出工具。

保留代码必须置于：

```text
migration/legacy_amitiax
```

并禁止被生产 Lifecycle 引用。

---

## 五、旧解压逻辑删除

删除所有：

-直接 `zip.OpenReader` 安装；
-直接解压目标目录；
-不检查链接；
-不检查路径；
-不限制大小；
-安装后立即运行；
-覆盖 current；
-信任归档权限位。

统一使用 Package Security。

---

## 六、旧 Manifest 类型删除

生产代码只接受：

```text
ManifestV2DTO
ExtensionDefinition
```

旧字段转换只存在迁移 Adapter。

禁止在 v2 DTO 中继续加入旧兼容字段。

---

## 七、旧安装目录

确认无 Runtime 从以下读取：

-旧 Package 目录；
-旧 Skill 导入目录；
-旧 Workflow 目录；
-旧 Plugin 目录；
-旧临时解压目录。

新 Runtime 只读取新 Artifact/Version View。

---

## 八、旧安装状态

旧：

```text
package_installed
package_enabled
package_status
```

不再参与任何业务判断。

查询可通过 ID Mapping 返回新 Installation。

---

## 九、旧更新和回滚

删除旧：

-原地覆盖；
-备份目录重命名；
-只更新文件；
-无 Definition Diff；
-无 Permission Diff；
-无数据迁移；
-无 Resource Plan。

全部使用新 Lifecycle Update/Rollback。

---

## 十、旧卸载

删除旧：

-按目录递归删除；
-按 Package ID 删除 Skill；
-删除共享 MCP；
-删除用户 Workflow；
-删除未知资源。

新卸载使用 Resource Release Plan。

---

## 十一、旧前端导入

删除：

-旧扩展包上传 API；
-旧 Package Preview；
-导入即安装；
-旧包卡片；
-旧安装进度。

统一 Extension Center 本地导入。

---

## 十二、文件关联和拖拽

所有 `.amitiax` 打开/拖拽：

```text
Extension Center Preview
```

不能调用旧 Parser。

---

## 十三、CLI

`amitia-ext install` 提交包到新 Developer/Local API。

旧 CLI 或脚本直接复制目录必须删除。

---

## 十四、迁移 Parser 隔离

Legacy Parser 运行约束：

-只读；
-离线；
-不安装；
-不执行；
-不联网；
-输出 Canonical Migration DTO；
-调用必须显式 `migration mode`；
-有安全限制。

---

## 十五、依赖清理

删除旧归档和解析依赖中不再使用的库。

若新 Package Security 仍使用同一库，不误删。

---

## 十六、旧缓存清理

清理：

-解析缓存；
-旧 Manifest Cache；
-旧 Extract Cache；
-旧 Checksum Cache；
-旧 Package Index。

保留期和删除通过 Resource Ownership。

---

## 十七、旧目录迁移清理

确认用户数据已迁后：

-生成 Dry Run；
-列出文件；
-Owner；
-Hash；
-保留；
-删除；
-用户修改。

不得直接清空。

---

## 十八、静态检查

CI 禁止生产包 import：

```text
legacy_amitiax
old_package_service
old_manifest
```

仅 Migration Tool 白名单。

---

## 十九、运行时检查

应用启动检查：

-旧 Parser 未注册；
-旧 Install Route 不存在；
-旧 Package Worker 不存在；
-旧目录无 Active Lock；
-旧表无写。

---

## 二十、删除顺序

1.删除旧前端入口。
2.删除旧 API。
3.删除旧安装/更新/卸载。
4.删除旧 Runtime Loader。
5.隔离 Legacy Parser。
6.删除旧 Manifest 生产类型。
7.删除旧目录解析。
8.删除旧缓存。
9.清理依赖。
10.增加 CI。
11.执行旧包回归和新包回归。

---

## 二十一、回滚

如发现某旧包无法迁移：

-恢复迁移工具；
-补充转换器；
-将包标记 Legacy Read-only；
-不得恢复旧生产安装链。

---

## 二十二、测试要求

覆盖：

-v2 安装；
-旧包 Preview；
-旧包 Migration；
-恶意包；
-更新；
-回滚；
-卸载；
-拖拽；
-文件打开；
-CLI；
-旧 API；
-旧目录；
-用户修改；
-共享资源；
-应用启动；
-无旧 Parser 主链；
-三平台。

---

## 二十三、实施任务

1.输出旧 Package/Parser 删除清单。
2.确认旧调用为零。
3.迁移/隔离 Legacy Parser。
4.删除旧前端/API。
5.删除旧安装/更新/卸载。
6.删除旧 Runtime Loader。
7.删除生产旧 Manifest 类型。
8.清理旧缓存和依赖。
9.生成旧目录 Release Plan。
10.增加 CI 禁止导入。
11.执行包系统全量回归。
12.输出删除报告。

---

## 二十四、验收标准

1.生产安装只使用新 Package Security。
2.生产解析只使用 Manifest v2 Parser。
3.Legacy Parser 仅迁移只读。
4.旧 PackageService 不存在。
5.旧安装目录不被运行。
6.更新/回滚/卸载只走 Lifecycle。
7.旧前端导入不存在。
8.旧目录清理有 Release Plan。
9.CI 阻止重新引入。
10.关键测试通过。
11.可进入第 69 步删除重复表和状态模型。

---

## 二十五、执行约束

> 删除旧包解析器时必须保留用户旧包的可迁移读取能力，但绝不能保留可安装、可执行、可覆盖目录的旧生产链。

禁止：

-旧 Parser 生产注册；
-直接解压安装；
-恢复旧 PackageService；
-v2 Parser 内塞 v1 分支；
-旧目录运行；
-旧卸载递归删除；
-无 Dry Run 清理用户文件。
