# Amitia 扩展系统重构第 30 步实施文档

## 第 30 步：定义 `.amitiax` 多模块包结构

---

## 一、步骤目标

在 Manifest v2 基础上定义 `.amitiax` 归档内部唯一目录规范，使一个扩展包可同时承载 JavaScript Runtime、Task Runtime、MCP、Agent Skill、Workflow、UI、WASM、Service、资源、迁移和平台文件。

目标：

> 建立跨 Windows、macOS、Linux 一致、可验证、可签名、可增量更新、可按 Module 安装与加载的多模块包布局。

---

## 二、推荐根目录

```text
/
├── manifest.json
├── integrity/
│   ├── files.json
│   └── content-tree.json
├── modules/
│   ├── main/
│   ├── tasks/
│   ├── ui/
│   ├── mcp/
│   ├── workflows/
│   ├── skills/
│   ├── wasm/
│   └── service/
├── resources/
├── assets/
├── migrations/
├── licenses/
├── docs/
└── signatures/
```

---

## 三、根目录规则

根目录只允许标准目录和 Manifest 声明的扩展目录。

禁止：

-隐藏可执行入口；
-重复 Manifest；
-根目录任意脚本自动执行；
-大小写冲突；
-平台保留名称；
-路径穿越；
-符号链接；
-硬链接；
-设备文件。

---

## 四、Module 目录

每个 Module：

```text
modules/<module-id>/
├── module.json            # 可选构建元数据，不是运行真值
├── dist/
├── resources/
├── schemas/
├── assets/
└── licenses/
```

最终运行入口仍由 Manifest 指定。

---

## 五、JavaScript Module

```text
modules/main/
├── dist/
│   ├── index.js
│   ├── chunks/
│   └── source-map/
├── package-lock.snapshot.json
└── licenses/
```

生产包不得依赖运行时在线 `npm install`。

依赖应在构建阶段打包或生成受控依赖快照。

---

## 六、Task Module

```text
modules/tasks/
├── dist/
│   ├── cleanup.js
│   └── migration.js
└── schemas/
```

Task Entry 必须在 Manifest 声明。

---

## 七、Agent Skill

```text
modules/skills/<skill-id>/
├── SKILL.md
├── references/
├── templates/
├── examples/
├── schemas/
└── assets/
```

Agent Skill 目录不允许可执行脚本作为默认行为。

---

## 八、Workflow

```text
modules/workflows/
├── definitions/
│   ├── daily-summary.json
│   └── maintenance.json
└── schemas/
```

Workflow 文件只包含声明式定义。

---

## 九、MCP

```text
modules/mcp/<server-id>/
├── config/
├── bin/
│   ├── windows-x64/
│   ├── windows-arm64/
│   ├── macos-x64/
│   ├── macos-arm64/
│   ├── linux-x64/
│   └── linux-arm64/
├── resources/
└── licenses/
```

若使用外部 Command，则包内不提供 bin，但必须声明外部依赖和用户配置。

---

## 十、UI

```text
modules/ui/<ui-id>/
├── dist/
│   ├── index.html
│   ├── assets/
│   └── chunks/
├── schemas/
└── localization/
```

UI 运行在受限 Sandbox，不允许直接使用 Electron/Node。

---

## 十一、WASM

```text
modules/wasm/<module-id>/
├── module.wasm
├── interface.wit
├── resources/
└── licenses/
```

WASM 导入接口必须与 Manifest 声明匹配。

---

## 十二、Service

```text
modules/service/<service-id>/
├── bin/<platform-arch>/
├── config/
├── protocols/
└── licenses/
```

Service 只适用于受信任发布者和高风险确认。

---

## 十三、Resources 与 Assets

### resources

供运行时或 Agent 使用的语义资源。

### assets

图标、图片、字体引用、静态文件。

资源必须：

-有 Hash；
-有 MIME；
-有 Owner；
-有大小限制；
-路径可追踪；
-不包含未声明可执行文件。

---

## 十四、Migrations

```text
migrations/
├── data/
├── state/
└── definitions/
```

迁移文件必须在 Manifest 中按 Migration ID 声明。

---

## 十五、Integrity

`integrity/files.json` 应列出：

-路径；
-大小；
-Hash；
-类型；
-可执行标记；
-平台；
-Module；
-是否可选。

内容树用于签名绑定。

---

## 十六、Signatures

```text
signatures/
├── package.sig
├── publisher.json
└── certificate-chain.json
```

签名文件本身不进入被签名内容树时，必须有明确封装规则。

---

## 十七、Licenses

必须收集：

-扩展许可证；
-第三方依赖许可证；
-原生二进制许可证；
-模型或资源许可证。

Extension Center 可展示。

---

## 十八、本地化

推荐：

```text
localization/
├── zh-CN.json
├── en-US.json
└── ja-JP.json
```

Manifest 可内嵌基础名称，也可引用本地化资源。

---

## 十九、平台选择

安装时不删除其他平台文件作为定义真值。

可选择：

-完整保留；
-安装后派生平台视图；
-缓存当前平台解包内容。

更新和回滚仍需要原始 Artifact。

---

## 二十、可执行文件

任何可执行文件必须：

-Manifest 声明；
-Integrity 声明；
-平台/架构声明；
-权限声明；
-签名绑定；
-安装时不执行；
-运行时由 Supervisor 启动。

---

## 二十一、文件权限

归档不可信任原始权限位。

安装时由宿主根据类型重建：

-普通资源只读；
-可执行入口可执行；
-Secret 不存包内；
-临时文件受限；
-用户数据目录分离。

---

## 二十二、大小与数量限制

按：

-包总大小；
-文件数量；
-单文件；
-Module；
-二进制；
-Source Map；
-资源；
-嵌套深度。

---

## 二十三、安装布局

原始 Artifact：

```text
artifacts/<extension-id>/<version>/<package-hash>.amitiax
```

不可变安装视图：

```text
extensions/<extension-id>/versions/<version>/
```

当前版本使用引用或原子指针：

```text
extensions/<extension-id>/current
```

不得原地覆盖当前目录。

---

## 二十四、运行数据分离

扩展包目录不得存用户数据。

数据：

```text
data/extensions/<extension-id>/
```

Secret：

```text
Secret Broker
```

缓存：

```text
cache/extensions/<extension-id>/
```

临时：

```text
temp/extensions/<extension-id>/<operation-id>/
```

---

## 二十五、开发包结构

开发模式可含：

```text
src/
tests/
source-maps/
dev/
```

生产发布默认剔除。

开发包必须标记 Development Revision。

---

## 二十六、构建产物

CLI 后续应生成：

-Manifest；
-Integrity；
-License；
-Bundle；
-Signature；
-平台矩阵报告；
-大小报告；
-依赖报告。

---

## 二十七、测试要求

覆盖：

-最小包；
-多 Module；
-全部目录；
-大小写冲突；
-Windows 保留名；
-路径过长；
-符号链接；
-平台二进制；
-可执行权限；
-用户数据分离；
-完整性；
-原子 current；
-开发包；
-大包；
-大量文件。

---

## 二十八、实施任务

1. 锁定根目录规范。
2. 定义各 Module 布局。
3. 定义 Integrity 文件格式。
4. 定义 Signature 目录。
5. 定义平台/架构目录。
6. 定义安装视图和 Artifact 存储。
7. 定义数据/缓存/临时分离。
8. 实现路径和文件类型 Validator。
9. 实现包布局检查器。
10. 建立示例包。
11. 建立跨平台测试。
12. 输出旧包布局迁移映射。

---

## 二十九、验收标准

1. 包布局唯一且文档化。
2. 多 Module 可共存。
3. 原始 Artifact 与安装视图分离。
4. 用户数据不写包目录。
5. 所有可执行文件显式声明。
6. 平台文件可验证。
7. Integrity 可覆盖全部内容。
8. 路径跨平台安全。
9. 开发包与生产包区分。
10. 可进入第 31 步重写 Parser/Validator。

---

## 三十、执行约束

> `.amitiax` 是不可变分发包，不是用户数据目录，也不是运行时临时目录。

禁止：

-在线安装依赖；
-运行时修改包文件；
-包内保存 Token；
-隐藏入口；
-安装时执行；
-原地覆盖 current；
-使用软链接逃逸；
-依赖平台原始权限位。
