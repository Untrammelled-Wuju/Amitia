# B3 Amitia源码基线冻结报告

## 1. 执行结果

| 项目 | 结果 |
|------|------|
| 执行状态 | PARTIAL |
| 基线编号 | AMT-A14-3daeaf3 |
| 冻结时间 | 2026-08-07 |
| 执行模式 | WORKTREE_SNAPSHOT |

## 2. B3启动条件

| 条件 | 状态 | 说明 |
|------|------|------|
| A14完成 | **未确认** | A14完成证据不完整 |
| A14源码固定 | **是** | Git提交 3daeaf3 可作为参考点 |
| A14构建结果 | **未验证** | ARM64/AMD64构建结果未找到证据 |
| 工作区稳定 | **否** | 存在未提交修改和未追踪文件 |
| B3执行分支 | **否** | 直接在develop分支上执行只读审计 |

## 3. 基线来源

- **项目**: Amitia
- **基线步骤**: A14
- **基线编号**: AMT-A14-3daeaf3
- **源码模式**: WORKTREE_SNAPSHOT (Git + 脏工作区)
- **Git提交**: 3daeaf3c0a82e33213e0a52d84cfaf8f68f78eab
- **Tree SHA**: 1f49ab81759aa83ac53a2deffe2e471dafe2
- **冻结时间**: 2026-08-07

## 4. Git或源码包状态

| 项目 | 值 |
|------|------|
| 仓库类型 | Git |
| 仓库根目录 | D:/桌面/跟进项目/U-Ai |
| 当前分支 | develop |
| 远程地址 | gitee: git@gitee.com:Untrammelled/Amitia, github: git@github.com:Untrammelled-Wuju/Amitia |
| 完整提交 | 3daeaf3c0a82e33213e0a52d84cfaf8f68f78eab |
| 短提交 | 3daeaf3 |
| Tree SHA | 1f49ab81759aa83ac53a2deffe2e471dafe2 |
| 工作区干净 | **否** |
| 提交信息 | fix: P0精准修复-统一Manifest、Import原子发布与恢复机制 |
| 提交时间 | 2026-08-07 00:36:37 +0800 |
| 提交作者 |Untrammelled |

### 工作区修改

**已追踪修改文件**:
- ackend/cmd/server/main.go (新增 --version 参数处理)

**未追踪源码候选**:
- ackend/cmd/server/version.go
- ackend/cmd/server/version_test.go
- ackend/internal/buildinfo/info.go
- ackend/internal/buildinfo/info_test.go

**未追踪构建产物**:
- ackend/qdrantprocess.test
- ackend/qdrantprocess.test.exe
- ackend_source_20260806_222805.zip

## 5. A14验证状态

**总状态**: **UNVERIFIED**

| 验证项 | 状态 | 说明 |
|--------|------|------|
| Go后端Linux ARM64构建 | UNVERIFIED | 未找到ARM64构建证据 |
| Go后端Linux AMD64回归 | UNVERIFIED | 未找到AMD64构建证据 |
| Go后端Windows AMD64回归 | UNVERIFIED | 未找到Windows构建证据 |
| 意外CGO依赖检查 | UNVERIFIED | 未找到CGO检查证据 |
| 后端基础启动检查 | UNVERIFIED | 未找到启动检查证据 |
| 证据完整性 | **A14_EVIDENCE_INCOMPLETE** | 缺乏A14完成的直接证据 |

**重要说明**: 当前工作树和Git历史中未发现A14完成所需的交叉编译构建日志、CI产物或明确的步骤标记。A14完成状态需要外部验证。

## 6. 源码根目录

| ID | 路径 | 类型 | 语言 | 清单文件 |
|----|------|------|------|----------|
| backend-go | backend | go-module | Go | go.mod, go.sum |
| frontend-vue | front | node-package | TypeScript, Vue, JavaScript | package.json, tsconfig.json |
| desktop-electron | desktop | node-package | TypeScript, JavaScript | package.json |
| mobile-flutter | mobile_app | flutter-project | Dart | pubspec.yaml |
| mobile-android | mobile_app/android | android-gradle | Kotlin, Java | settings.gradle.kts, build.gradle.kts |
| mobile-ios | mobile_app/ios | ios-xcode | Swift | (无) |
| wechat-extractor | wechat-chat-extractor | python-project | Python | requirements.txt |
| runtime-plugin-host | runtime/plugin-host | node-package | TypeScript | package.json |
| runtime-task-host | runtime/task-host | node-package | TypeScript | package.json |
| sdk-plugin-cli | sdk/plugin-cli | node-package | TypeScript | package.json |
| sdk-plugin-sdk | sdk/plugin-sdk | node-package | TypeScript | package.json |
| wechat-sidecar | backend/sidecar | node-package | TypeScript | package.json, pnpm-lock.yaml |
| qq-sidecar | backend/qq-sidecar | node-package | TypeScript | package.json, pnpm-lock.yaml |

共识别 13 个源码根。

## 7. 组件结构

项目包含以下组件类别：

| 类别 | 组件数 | 说明 |
|------|--------|------|
| backend | 3 | Go后端、微信侧车、QQ侧车 |
| desktop | 1 | Electron桌面端 |
| flutter_app | 1 | Flutter移动端主工程 |
| android_native | 1 | Android原生工程 (Gradle) |
| ios_native | 1 | iOS原生工程 (Xcode) |
| web | 1 | Vue前端 |
| runtime | 2 | Plugin Host, Task Host |
| extension | 1 | Extension Kernel系统 |
| mcp | 1 | MCP系统 |
| sdk | 2 | Plugin CLI, Plugin SDK |
| scripts | 2 | Audit脚本, Python脚本 |
| assets | 2 | Config, Contracts |

共 21 个组件。

### Backend (Go) 内部结构

backend/internal/ 包含 60+ 子系统目录，包括：
- 核心系统：agent, auth, chat, memory, extension, mcp, mcpapi
- 情感心理：personality, mood, psyche, affect, emotion, relationship
- 交互决策：decision, interaction, proactive, need
- 模型能力：embedding, tts, asr, vision, imagegen
- 数据存储：qdrant, vectorstore, database, graph
- 渠道平台：qq, wechat, delivery, platform
- 安全隐私：security, safety, circuitbreaker
- 运行时：runtimehost, runtimeorchestrator, scheduler
- 世界观：worldbook, belief, temporal

## 8. 嵌套仓库与Submodule

| 项目 | 数量 |
|------|------|
| Submodule | 0 |
| 嵌套Git仓库 | 0 |

## 9. Git LFS状态

| 项目 | 状态 |
|------|------|
| Git LFS | 未使用 |
| LFS文件数 | 0 |
| .gitattributes | 存在但无LFS配置 |

## 10. 依赖与锁文件

### 依赖清单 (26个文件)

| 生态 | 清单文件 | 锁文件 |
|------|----------|--------|
| Go | go.mod | go.sum |
| Node.js (pnpm/npm) | 8个package.json | 4个pnpm-lock.yaml, 4个package-lock.json |
| Flutter/Dart | pubspec.yaml | pubspec.lock |
| Gradle (Android) | settings.gradle.kts, build.gradle.kts | N/A |
| Python | requirements.txt | N/A |

### 依赖统计

| 生态 | 直接依赖 | 间接依赖 | 总依赖 |
|------|----------|----------|--------|
| Go | 23 | 45 | 68 |
| Node.js (front) | 8 | 8 | 16 |
| Node.js (desktop) | 4 | 12 | 16 |
| Node.js (sidecar微信) | 7 | 2 | 9 |
| Node.js (sidecar QQ) | 4 | 4 | 8 |
| Flutter | 10 | 2 | 12 |
| Python | 1 | 0 | 1 |

## 11. 工具链版本

| 工具 | 版本 | 状态 |
|------|------|------|
| Git | 2.48.1.windows.1 | ✓ |
| Go | go1.26.1 (windows/amd64) | ✓ |
| Node.js | v24.18.0 | ✓ |
| pnpm | 11.18.0 | ✓ |
| Flutter | 3.38.7 (stable) | ✓ |
| Dart | 3.10.7 | ✓ |
| Java | OpenJDK 21.0.2 | ✓ |
| Python | 3.12.13 | ✓ |
| npm | 未检测 | N/A |
| Gradle | 未检测 | N/A |
| CMake | 未检测 | N/A |
| Rust | 未检测 | N/A |

## 12. 数据库与Schema

| 路径 | 类别 | 文件数 |
|------|------|--------|
| backend/internal/migration | migration | 包含baseline.sql和版本化迁移 |
| backend/internal/extension/schema | schema | JSON Schema定义 |
| backend/internal/extension/migration | migration | Extension迁移文件 |

- SQL基线文件: ackend/internal/migration/baseline.sql
- 版本化迁移系统: ackend/internal/migration/migrations.go

## 13. 生成产物与用户数据

### 仓库内追踪的产物
（无）

### 未追踪构建产物
- ackend/qdrantprocess.test - Go测试缓存
- ackend/qdrantprocess.test.exe - Go测试可执行文件
- ackend_source_20260806_222805.zip - 源码备份包

### 用户数据目录（已排除出基线）
- AmitiaData/ - 运行时用户数据
- ackend/AmitiaData/ - 后端数据目录
- ackend/tmp/ - 临时文件
- ackend/logs/ - 日志目录
- ackend/storage/ - 存储目录

## 14. 源码完整性

| 项目 | 值 |
|------|------|
| 纳入基线文件数 | 3499 |
| 已生成哈希数 | 3499 |
| 失败哈希数 | 0 |
| 总字节数 | 277,944,803 (约 265 MB) |
| 源码归档SHA-256 | 735f5fc7741850b9d8dc6a1f46290d8790f175c65f1980fdc738db4d02522bb4 |
| Git追踪文件数 | 3495 |
| 未追踪源码候选 | 4个 .go 文件 |
| 源码归档 | mitia-a14-source.tar (仅Git追踪文件) |

## 15. 敏感文件风险

| 风险等级 | 文件 | 说明 |
|----------|------|------|
| 低风险 | ppsettings.json | 未追踪的本地配置模板 |
| 低风险 | ackend/appsettings.json | 未追踪的后端配置模板 |
| 低风险 | config/config.yml | 已追踪的应用配置 |
| 无风险 | 证书/密钥文件 | 未发现 |

**结论**: 未发现高风险密钥或凭证文件。appsettings.json文件未追踪，符合安全实践。

## 16. 输出文件

所有文件位于 docs/parity/amitia/baseline/a14/ 目录：

| 文件名 | 用途 |
|--------|------|
| B3_Amitia源码基线冻结报告.md | 本文档 |
| baseline.json | 基线元数据 |
| repository_metadata.json | 仓库元数据 |
| source_roots.json | 源码根清单 |
| component_manifest.json | 组件清单 |
| nested_repositories.json | 嵌套仓库清单 |
| source_archive.sha256 | 源码归档SHA-256 |
| source_files.sha256 | 完整源文件SHA-256 |
| tracked_files.sha256 | Git追踪文件SHA-256 |
| dependency_manifests.json | 依赖清单 |
| dependency_locks.sha256 | 锁文件SHA-256 |
| toolchain_versions.json | 工具链版本 |
| build_evidence.json | 构建证据 |
| a14_verification.json | A14验证结果 |
| schema_inventory.json | Schema清单 |
| generated_artifacts_inventory.txt | 生成产物清单 |
| untracked_inventory.txt | 未追踪文件清单 |
| ignored_inventory.txt | 忽略文件清单 |
| submodules.txt | Submodule清单 |
| lfs_inventory.txt | LFS清单 |
| top_level_tree.txt | 顶层目录树 |
| security_redaction_report.md | 安全消隐报告 |
| verification.log | 验证日志 |
| README.md | 使用说明 |
| amitia-a14-source.tar | 源码归档 (临时) |

## 17. 未确认项

1. **A14完成状态**: 未找到A14完成的直接证据。无法确认Linux ARM64/AMD64和Windows AMD64构建是否完成。
2. **CGO依赖检查**: 未找到CGO检查结果。
3. **基础启动检查**: 未找到后端启动检查结果。
4. **构建产物**: 未找到编译后的ARM64/AMD64二进制文件。
5. **CI/CD证据**: 无CI构建日志或ARM64交叉编译记录。
6. **脏工作区**: 工作区存在未提交修改（main.go添加了--version参数）。
7. **未追踪源码**: 4个新增Go源文件未被Git追踪。

## 18. 阻断项

**无硬性阻断项。** B3可以基于当前工作树快照冻结基线，但因为A14证据不完整，状态标记为PARTIAL。

## 19. B6输入基线

以下文件将作为B6扫描的输入：

| 文件 | 用途 |
|------|------|
| baseline.json | 基线唯一标识和完整性校验 |
| repository_metadata.json | Git仓库状态 |
| source_roots.json | 全部源码根位置 |
| component_manifest.json | 组件结构和位置 |
| source_files.sha256 | 文件完整性校验基线 |
| dependency_manifests.json | 依赖约束基线 |
| a14_verification.json | A14状态 (需外部确认) |

## 20. 最终结论

**PARTIAL: 主要源码基线已冻结，但A14完成证据不完整**

### 已确认事项
- ✓ Git仓库状态和提交点已记录
- ✓ 全部源码根已识别（13个）
- ✓ 组件结构已记录（21个组件）
- ✓ 依赖清单已冻结（26个清单文件）
- ✓ 工具链版本已记录（8个工具）
- ✓ 源文件哈希完整（3499个文件，0失败）
- ✓ 源码归档SHA-256已生成
- ✓ Submodule/LFS/Nested仓库已确认（均为0）
- ✓ 敏感文件已检查（无高风险）

### 未确认事项
- ✗ A14构建完成状态未验证
- ✗ ARM64/AMD64构建证据未找到
- ✗ 工作区存在未提交修改
- ✗ 4个新增源文件未追踪

### 建议
1. 在开始B6前，需从外部来源确认A14已完成
2. 确认后可通过构建 --version 标志验证版本信息处理（已添加）
3. 脏工作区的版本.go和buildinfo/目录可能与新功能相关
