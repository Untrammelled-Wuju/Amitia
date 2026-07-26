# Amitia 扩展系统重构第 57 步实施文档

## 第 57 步：实现 Plugin CLI

---

## 一、步骤目标

实现 Amitia Extension 官方命令行工具，为开发者提供创建、开发、校验、测试、构建、打包、签名、安装、发布检查和诊断能力。

建议命令名称：

```text
amitia-ext
```

本步骤目标：

> 将 Manifest v2、包结构、SDK、Schema、Runtime Contract、签名、完整性、License、安全检查和 `.amitiax` 打包流程固化到统一 CLI，避免开发者手工拼包和依赖宿主内部脚本。

---

## 二、CLI 命令总览

建议：

```text
amitia-ext init
amitia-ext dev
amitia-ext validate
amitia-ext lint
amitia-ext test
amitia-ext build
amitia-ext pack
amitia-ext sign
amitia-ext verify
amitia-ext inspect
amitia-ext install
amitia-ext uninstall
amitia-ext publish-check
amitia-ext migrate
amitia-ext doctor
amitia-ext sdk
```

---

## 三、`init`

用途：

-创建新 Extension；
-选择模板；
-生成 Manifest；
-生成 Module；
-安装 SDK；
-生成 Schema；
-生成测试；
-生成 License；
-生成配置。

模板：

```text
tool
agent-skill
workflow
mcp
schema-ui
web-ui
event-hook
provider
task
desktop
composite
```

---

## 四、初始化交互

询问：

-Extension ID；
-名称；
-版本；
-Publisher；
-License；
-Module 类型；
-Runtime；
-主要 Contribution；
-目标平台；
-SDK 版本；
-包管理器。

不能自动填入高风险 Permission。

---

## 五、生成目录

示例：

```text
my-extension/
├── manifest.ts
├── manifest.json
├── src/
├── modules/
├── schemas/
├── resources/
├── tests/
├── localization/
├── licenses/
├── package.json
├── tsconfig.json
└── amitia-ext.config.ts
```

`manifest.ts` 可用于构建时生成静态 `manifest.json`，运行包只保留 JSON。

---

## 六、`dev`

启动开发模式：

-构建监听；
-本地工作区挂载；
-开发签名；
-连接 Amitia Developer Host；
-热重载；
-日志；
-错误；
-状态；
-可选 DevTools。

`dev` 不把工作区复制进正式安装目录。

---

## 七、`validate`

执行：

-Manifest Schema；
-Semantic；
-包布局；
-路径；
-Entry；
-Contribution；
-Runtime；
-Permission；
-Scope；
-Dependency；
-UI Contract；
-Hook/Event；
-平台；
-资源；
-License；
-Integrity。

输出机器可读和人类可读报告。

---

## 八、`lint`

检查开发质量：

-使用未允许 Node Builtin；
-动态 require/import；
-原生模块；
-直接 fetch/fs/child_process；
-未处理 AbortSignal；
-未声明 Entry；
-日志疑似 Secret；
-过大 Bundle；
-未使用 SDK；
-Deprecated API；
-高风险 Permission 无理由；
-缺少输出 Schema；
-不稳定 ID。

---

## 九、`test`

运行：

-单元测试；
-SDK Mock；
-Contract Test；
-Manifest；
-Entry；
-Tool；
-Event；
-Hook；
-Task；
-UI；
-Cancel；
-Timeout；
-Storage；
-Permission；
-Scope；
-平台矩阵。

可支持：

```text
--host-version
--platform
--arch
--runtime-version
```

---

## 十、`build`

职责：

-编译 TypeScript；
-Bundle；
-生成类型；
-复制资源；
-生成 Source Map（开发）；
-剔除测试；
-生成 License 清单；
-检查依赖；
-生成构建报告。

生产构建禁止在线依赖。

---

## 十一、Bundle 规则

默认使用受支持构建器。

检查：

-Entry 存在；
-Chunk；
-动态 import；
-包内路径；
-Node Builtin；
-原生依赖；
-依赖 License；
-重复依赖；
-Bundle 大小。

---

## 十二、`pack`

生成 `.amitiax`：

1.清理临时目录。
2.构建。
3.生成静态 Manifest。
4.收集 Module。
5.收集 Resource。
6.生成 Integrity。
7.生成 License。
8.校验。
9.确定性归档。
10.输出 Hash。

---

## 十三、确定性打包

相同输入应产生稳定内容树。

需要固定：

-路径顺序；
-时间戳策略；
-文件权限；
-压缩参数；
-换行；
-JSON Canonicalization；
-排除文件。

Archive 二进制 Hash 是否完全稳定取决于格式实现，但 Content Tree 必须稳定。

---

## 十四、`sign`

使用开发者密钥对包签名。

要求：

-私钥不进项目；
-支持系统安全存储；
-支持外部签名器；
-显示载荷；
-确认 Extension ID/Version/Hash；
-生成签名记录。

---

## 十五、密钥管理

CLI 可提供：

```text
amitia-ext keys create
amitia-ext keys list
amitia-ext keys rotate
amitia-ext keys export-public
```

私钥导出必须明确风险和加密。

---

## 十六、`verify`

验证：

-包安全；
-Content Tree；
-Manifest；
-Schema；
-签名；
-Publisher；
-License；
-兼容；
-平台；
-风险。

不执行包。

---

## 十七、`inspect`

显示：

-Extension；
-Modules；
-Contributions；
-Runtimes；
-Permissions；
-Dependencies；
-Resources；
-UI；
-Entry；
-Signature；
-Hash；
-文件；
-平台；
-风险。

---

## 十八、`install`

仅用于本地开发或用户明确安装。

建议通过 Amitia 本地受认证 Developer API 提交包，仍由 Lifecycle Manager 安装。

CLI 不能直接复制到安装目录。

---

## 十九、`publish-check`

发布前检查：

-版本；
-签名；
-Publisher；
-兼容；
-README；
-License；
-Changelog；
-权限理由；
-截图；
-图标；
-平台；
-安全；
-Bundle；
-测试；
-不可逆 Migration；
-自动更新资格。

---

## 二十、`migrate`

辅助旧扩展项目迁移：

```text
v1 manifest → v2
old skill → agent skill/tool/workflow classification
old plugin metadata → contribution draft
old package layout → new layout
```

只生成报告和草稿，不自动决定高风险语义。

---

## 二十一、`doctor`

检查：

-Node；
-SDK；
-CLI；
-Amitia；
-Developer Host；
-平台工具；
-签名；
-文件权限；
-路径长度；
-端口/Pipe；
-构建器；
-依赖；
-缓存。

---

## 二十二、配置文件

建议：

```ts
export default defineAmitiaExtensionConfig({
  manifest: "./manifest.ts",
  outputDir: "./dist",
  packageDir: "./package",
  targets: ["windows-x64", "macos-arm64", "linux-x64"],
});
```

配置不能覆盖安全规则。

---

## 二十三、退出码

固定：

```text
0 success
1 validation/build failure
2 configuration error
3 environment error
4 signature/trust error
5 test failure
6 host connection error
7 internal CLI error
```

---

## 二十四、输出格式

支持：

```text
human
json
sarif
```

CI 推荐 JSON/SARIF。

---

## 二十五、缓存

缓存：

-依赖；
-构建；
-Schema；
-编译；
-测试；
-包检查。

缓存键包含 SDK/CLI/Host Contract Version。

---

## 二十六、CI 集成

提供示例：

-提交检查；
-多平台；
-测试；
-打包；
-签名；
-Artifact；
-发布检查。

CLI 不应要求 CI 中启动完整 Amitia UI。

---

## 二十七、安全

CLI 必须防：

-项目路径越界；
-符号链接；
-Secret 打包；
-`.env` 误入包；
-私钥误入包；
-恶意依赖脚本；
-构建阶段任意 postinstall；
-远程模板执行；
-输出目录覆盖用户文件。

---

## 二十八、模板来源

内置模板随 CLI 发布。

远程模板默认不执行代码；下载模板需签名和明确确认。

---

## 二十九、日志

支持：

```text
--verbose
--debug
--quiet
```

不得打印：

-私钥；
-Secret；
-Token；
-完整 Host Session。

---

## 三十、CLI 插件化

第一版禁止 CLI 自身第三方插件化，避免重新产生扩展链。

---

## 三十一、测试要求

覆盖：

-init 全模板；
-validate；
-lint；
-test；
-build；
-pack；
-sign；
-verify；
-inspect；
-install；
-publish-check；
-migrate；
-doctor；
-确定性内容树；
-路径攻击；
-Secret 排除；
-私钥；
-跨平台；
-CI；
-错误码；
-JSON 输出；
-大项目；
-并发构建。

---

## 三十二、实施任务

1. 建立 CLI 工程。
2. 实现 Config。
3. 实现 init/template。
4. 接入 Manifest Validator。
5. 实现 Linter。
6. 接入 Testing SDK。
7. 实现 Build/Bundler。
8. 实现 Integrity。
9. 实现 Pack。
10. 实现 Sign/Verify。
11. 实现 Inspect。
12. 实现 Developer Host Install。
13. 实现 Publish Check。
14. 实现 Migration Assistant。
15. 实现 Doctor。
16. 实现 CI 输出。
17. 建立跨平台安装包。
18. 编写文档和示例。

---

## 三十三、验收标准

1. 新扩展可由 CLI 初始化。
2. Manifest/包可完整校验。
3. Lint 可发现危险 API。
4.测试不依赖生产环境。
5. Build 不在线安装依赖。
6. Pack 生成正式 `.amitiax`。
7.签名不泄露私钥。
8. Install 仍走 Lifecycle Manager。
9. Publish Check 覆盖发布风险。
10.旧项目可生成迁移报告。
11.CI 可使用。
12.可进入第 58 步开发模式和热重载。

---

## 三十四、执行约束

> CLI 是开发和构建工具，不是绕过 Extension Kernel 的安装器、运行器或权限提升工具。

禁止：

-直接写安装目录；
-直接改数据库；
-打包 Secret；
-执行远程模板脚本；
-自动高风险权限；
-绕过签名/包安全；
-通过 CLI 启动未注册 Runtime；
-隐藏构建错误。
