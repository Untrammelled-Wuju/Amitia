# 项目文件夹结构全量审计

审计时间：2026-10-14，Asia/Shanghai。源码基线：`22ed4860756c45fa9f0f1d9618bc8ae77cd7e0b4`。本轮只审计和生成报告，没有移动、删除、构建、启动、提交或推送项目。

## 结论与范围

发现 18 类需要整改的问题：9 类 P1、9 类 P2。另外列出 3 类需要明确政策边界的目录，避免把测试夹具、合法缓存和历史参考资料直接当成违规源码删除。

扫描了除 `.git` 元数据外的全部 28 个顶层目录，共 211712 个实际文件，累计 44637439292 字节，约 44.64 GB。Windows 长路径导致的 25 个初始读取失败已用长路径方式补查，最终文件读取错误为 0。目录联接和符号链接不递归跟随，避免重复计算和进入项目外部；Git 子模块另行标识，不把子模块源码误计为主仓库未跟踪文件。本轮没有全量读取文件正文，没有把凭据值写入报告。

依据是 `AGENTS.md` 的插件源码／产物目录规则、禁止产物与依赖进入 Git、桌面 `core` 白名单、Android Release 与源码唯一构建来源要求，以及源码、生成物、运行数据、第三方依赖分离的目录原则。没有规定必须把现有应用改名为 `apps/*`，所以不会仅因 `front`、`backend`、`mobile_app` 等名称判定违规。

逐文件清单、目录数量与 Git 状态保存在本地忽略目录：

- `artifacts/audits/folder-structure-20261014/inventory.json`
- `artifacts/audits/folder-structure-20261014/directory-inventory.csv`
- `artifacts/audits/folder-structure-20261014/unignored-files.csv`
- `artifacts/audits/folder-structure-20261014/ignored-active-sources.csv`

清单是审计时点快照，不包含本轮随后生成的报告和清单自身。

## 已确认问题

P1 表示影响源码完整性、构建来源或明确发布规则；P2 表示目录边界、忽略策略或维护流程存在实际问题。

| 编号 | 级别 | 位置 | 已确认事实与影响 | 整改方向 |
|---|---|---|---|---|
| F01 | P1 | `admin-system/`、`.gitignore:14`、`start-all-services.ps1:6` | 管理端整个目录被忽略，Git 中没有任何管理端文件；启动脚本仍使用它及其 `dist`。本地有实际 Vue 源码和构建配置，其他人仅克隆 Git 无法得到该应用源码。 | 保留当前应用目录，纳入源码、构建配置和锁文件，仅忽略依赖及输出。先审查本地配置，不能整目录直接强制加入 Git。 |
| F02 | P1 | `plugins/channels/wechat-ipad860/`、`.gitignore:80` | 整个正式渠道插件目录被忽略；含清单、运行时、UI、构建脚本、测试及原生依赖，共 681 个文件，但 Git 跟踪数量为 0。这里不是只有打包产物。 | 取消对整个插件源码树的排除；区分源码、第三方输入、测试与生成物。 |
| F03 | P1 | 根 `.gitignore:34`、`backend/.gitignore:14`、`runtime/build/` | 全局 `build/` 忽略规则同时匹配构建工具源码。实际存在但未入 Git 的 `build.py`、`build_fresh.py`、`validate.py` 位于 `runtime/build/runtime-package/android-arm64/`；已提交的 Runtime 包装脚本会调用其中的 `build.py`。`backend/internal/desktoppet/release/build/` 的新源码也会被自动隐藏。 | 把输出忽略规则限定到真实输出路径；保留构建工具源码，或将工具移动到 `runtime/tools/` 并统一调用。 |
| F04 | P1 | `plugins/channels/qq/package-sources/1.0.12/`、`plugins/channels/wechat/package-sources/1.0.12/`、`1.0.13/` | Git 跟踪了这些目录下 44 个解包文件，包括约 1.59 MB 和 6.16 MB 的编译 bundle。现有渠道构建脚本直接重打包该目录；Git 中没有对应渠道运行时的 `src` 和生成这些 bundle 的编译入口。可以重打包现有 bundle，不能据此认定已保留完整运行时源码构建链。 | 将宿主自有渠道运行时源码和编译入口集中到各插件目录；生成的版本目录作为构建输出隔离。第三方库按依赖或明确的 vendor 规则保留。 |
| F05 | P1 | `desktop/resources/core/` | 白名单外仍有 `sidecar/`、`qq-sidecar/`、备用核心 EXE、备份、待替换 EXE、未压缩 Node 和构建记录。目录混放已确认，但 `electron-builder.yml` 已使用过滤白名单，不能说这些文件必然进入当前安装包。 | 发布 staging 只保留核心和公共 Node 压缩包；渠道运行时归插件包，调试核心与构建记录归独立输出目录。 |
| F06 | P1 | Git 中的 `Plugin/Game/*.gamex`、根 `demo-extension.amitiax` | 两个游戏插件包和一个根目录插件包已被 Git 跟踪，违反构建产物不上传规则。忽略规则不会自动解除已经跟踪的文件。根插件包也没有统一放在 `plugin/`。 | 从索引移除生成包，统一存放到忽略的产物目录。若有必须版本化的测试二进制夹具，先明确例外和来源，不按普通发布包处理。 |
| F07 | P1 | `runtime/mobile_app/android/amitia-runtime/` | 这里有 3 个已提交文件：ARM64／x86_64 PRoot 二进制和元数据；实际 Gradle 使用的是根 `mobile_app/android/amitia-runtime/`。两个 ARM64 文件哈希不同，已形成另一套手机运行时资源副本。 | 手机资源以实际 Gradle 输入目录为唯一来源，退役多余副本；保留外部依赖输入的白名单与来源记录。 |
| F08 | P1 | `backend/server*.exe`、`backend/server-linux-arm64.codex*`、`start-all-services.ps1:11` | 后端源码根目录堆放多个诊断和历史可执行文件，其中 20 个未跟踪且未忽略。完整启动脚本优先挑选 `server.codex2.exe` 等历史文件，然后才回退到 `server.exe`，目录残留直接影响实际启动版本。 | 后端输出集中到按目标和提交区分的忽略目录，启动入口使用明确的当前构建，不以遗留文件是否存在选择版本。 |
| F09 | P1 | `mobile_app/android/amitia-accessibility/build/outputs/apk/debug/`、`mobile_app/android/app/build/intermediates/assets/debug/` | 本地仍存在 2 个 Debug 路径 APK，其中一个是 `amitia-accessibility-debug.apk`，另一个是 Debug 资产目录中的无障碍 APK。违反禁止保留 Debug APK 的明确规则；它们未被 Git 跟踪。本轮没有安装或生成 Debug APK。 | 在确认不是当前 Release 输入后，按 Debug 产物范围清理；保留正式 APK 与 Release 构建记录。 |
| F10 | P2 | `config/config.yml`、`backend/config/config.yml`、根和后端 `appsettings.json`、`AmitiaData/`、`backend/data/` | 已提交配置和本地运行配置边界不清；根配置存在非空本地凭据字段，两个默认配置的 `storage.dataDir` 也不同。数据相对路径随启动工作目录变化，不同入口容易形成不同数据根。这里不记录凭据值。 | 提交脱敏模板，运行配置与凭据归数据目录／环境变量；明确唯一数据根解析和各启动入口的工作目录。不能直接删除现有数据库。 |
| F11 | P2 | `mobile_app/android/` 的模块级 `generated/`、`intermediates/`、`outputs/`、`tmp/` 等 | 除当前 `mobile_app/build/` 和固定 APK 归档路径外，Android 源码树还遗留 834 个未忽略生成文件，覆盖宿主模块和 13 个 Flutter 插件输出目录。当前 Gradle 已重定向输出，旧布局仍留在源码层。 | 使用一致的构建输出根，忽略并隔离历史模块输出；固定 APK 路径仅作为规定的交付归档，不把其余生成目录放在源码层。 |
| F12 | P2 | `desktop/release/` | 88 个文件未跟踪且未忽略，包括安装程序、blockmap、`latest.yml` 和解包目录。根规则 `/desktop/release-*/` 没有覆盖实际 `release/`。 | 为实际正式输出目录添加精确忽略规则；发布前仍按规则清理该目录。 |
| F13 | P2 | `backend/artifacts/` | 共 77 个文件，约 1.70 GB；其中 36 个未忽略文件包含可执行文件和测试／发布结果。根 `/artifacts/` 规则不覆盖此目录。此类输出本轮之前也已多次在开发中产生。 | 统一输出位置或添加精确忽略；可复用的验证源码归 `scripts/`，生成日志和报告归输出目录。 |
| F14 | P2 | 根 `amitia-aidl-stage/` | 当前有 4 个未忽略 AIDL 生成副本，目录用于中文路径下的 ASCII staging，但散落在源码根。 | 可以保留这个技术 staging 机制，必须明确为缓存并忽略；若改路径，需保持同盘和 ASCII 约束，不能直接删掉构建功能。 |
| F15 | P2 | `runtime/__pycache__/`、`runtime/validation/__pycache__/`、`scripts/ios/__pycache__/`、`scripts/test/__pycache__/` | 前两个目录各有 1 个 `.pyc` 已进入 Git；后两个目录各有 1 个未忽略 `.pyc`。Python 缓存治理不完整。 | 全项目统一忽略 `__pycache__/` 与 `.pyc`，从索引移除已提交缓存，保留 Python 源码。 |
| F16 | P2 | `plugin/` 与 Git 路径 `Plugin/`，以及三个插件构建脚本 | Git 中已有大写 `Plugin/Game`，渠道默认输出小写 `plugin`，表情、生活、主动消息脚本默认输出大写 `Plugin`。Windows 上合并成同一目录，Linux 上会分成两套路径。 | 统一为规则规定的根 `plugin/`；修改脚本与 Git 路径时使用明确的大小写重命名流程。当前根 `plugin/` 本身没有完整插件源码，不将其误报为“源码放进产物目录”。 |
| F17 | P2 | `artifacts/stage-final10-r2/` 至 `r10/`、`ai-community-app/`、空旧业务目录、`backend/libs/unidbg-fetch-qsign-all.jar` | 9 套历史 staging 中仍有后端、网页、手机源码片段；`ai-community-app/` 仅剩 5 个缓存／构建文件；旧业务包、旧 sidecar 等空目录还在；旧签名服务 JAR 未在当前源码／脚本搜索中发现运行引用。`runtime/build/sidecar/.../build.py` 仍指向已空的 `backend/sidecar`。 | 区分可保留参考资料、必须归档的历史补丁、应退役的空目录和旧依赖；同步退役失效脚本。没有证据证明这些 staging 本轮被用于正式构建，不把它们直接判成违规构建来源。 |
| F18 | P2 | `scripts/audit/verify-source-hygiene.mjs`、`scripts/pack*.ps1`、`scripts/pack.py` | 卫生检查实际失败，命中 1113 条；其中包含 `.gitignore` 允许的本地数据库、备份和配置目录，因此不能把该数字当成 1113 个源码违规。检查又整体跳过 `core` 等敏感目录，漏掉已确认混放。多个源码归档脚本分别维护排除列表，规则不一致：例如 `pack.py` 没有排除所有手机缓存、Python `.venv` 和插件包；手机＋后端精简包还排除 Runtime／SDK 等模块，不应被表述为可完整重建的项目源码包。 | 分开校验 Git 源码、构建 staging 和最终归档；以统一分类／清单生成打包输入，并在归档后检查必需模块、依赖输入、运行数据和本地配置。 |

## 关键证据补充

### `core` 中的全部白名单外文件

- `.amitiacore-build.json`
- `.release-runtime-assets.json`
- `AmitiaCore-test.exe`
- `AmitiaCore.exe.bak`
- `AmitiaCore.pending.exe`
- `node/node.exe`
- `qq-sidecar/bundle.mjs`
- `qq-sidecar/launcher.mjs`
- `sidecar/bundle.mjs`
- `sidecar/launcher.mjs`

构建元数据由现有脚本主动生成。与其每次人工删除，更合理的方式是把它们移到 staging 外，并让包装器从指定位置读取；发布资源过滤仍应保留。

### 已跟踪的非源码文件

- `Plugin/Game/Minecraft_1.21.11_Amitiax_V1_1.0.0.gamex`
- `Plugin/Game/Minecraft_Amitiax_V1_0.7.0-dev.gamex`
- `demo-extension.amitiax`
- `runtime/__pycache__/__init__.cpython-312.pyc`
- `runtime/validation/__pycache__/__init__.cpython-312.pyc`

合法第三方输入，例如 Node／数据库压缩包和手机原生 JNI 输入，没有仅因是二进制就一概判为违规。F07 的问题是重复且不一致的输入树，而不是手机必须禁止使用原生库。

### 两份 PRoot 的差异

- 实际手机输入 ARM64 SHA-256：`6eae92a445afd3ea6e5e08d1b4b737ab78ae2341010009ab777e31f35fcee957`。
- `runtime/mobile_app` 副本 ARM64 SHA-256：`b1403a384b92d09b4a01d1130c4e227302d00c186488bd245692882d76baea4e`。

### 两个 Debug APK

- `mobile_app/android/amitia-accessibility/build/outputs/apk/debug/amitia-accessibility-debug.apk`
- `mobile_app/android/app/build/intermediates/assets/debug/mergeDebugAssets/accessibility/amitia-accessibility.apk`

### 历史空目录与异常目录

已确认为空的旧业务目录包括 `backend/internal/companion`、`deepsearch`、`emote`、`episodic`、`lifecontext`、`memory`、`memoryaccess`、`proactive`、`profile`、`qdrant`、`qq`、`user`，以及 `backend/sidecar/src`、`backend/qq-sidecar/{data,scripts,src}`、`backend/cmd/verify-amitiax`。

网页树内还有一个名称包含盘符和路径字符替代符的空诊断目录，以及网页和桌面树内的 `NVIDIA Corporation/umdlogs` 空目录。它们属于本地残留，不是 Git 中的旧业务实现。

## 需要明确边界的三类目录

1. `testplugins/` 保存完整示例和一致性测试插件，CI 与 `tests/gamehost_external` 真实依赖它，符合常见测试夹具布局；但项目“插件完整源码统一放 plugins”没有明确测试例外。可选择将其放入 `plugins/examples`／`plugins/fixtures`，或明确允许 `testplugins`。不能直接删除。
2. `plugins/{emote,lifestyle,proactive}/legacy/` 和渠道 `package-sources/*/assets/legacy-source/` 保存历史源码与来源清单；它们可能服务于界面复用和溯源。需要明确“只读参考资料”与“可执行源码”的边界。仅存在 legacy 目录不能证明旧功能仍在运行。
3. 根 `README.md` 的目录示意未覆盖手机、桌面、插件、SDK 和 Runtime；文档／发布报告散落在根、`docs`、`release-reports` 和 `artifacts`。这属于说明和文档组织欠缺，没有现行规则禁止这些名称，列为治理建议而非硬性违规。

## 未判为违规的目录与例外

- `.github`、`front/src`、`backend/internal`／`pkg`／`cmd`、`mobile_app/lib`／`test`、`desktop/src`、`contracts`、`sdk`、`tests` 是合理的应用或共享源码边界。
- `backend/third_party/ish` 是正式 Git 子模块；手机 `third_party` 是明确第三方源码，不当作重复第一方系统删除。
- 公共 Go 游戏 SDK 与后端协议存在复制，但 `scripts/sync_game_plugin_go_sdk.py` 明确声明权威来源和同步规则，CI 检查漂移，不能仅凭重复路径判为违规。
- `AmitiaData`、`admin-data`、`logs`、各模块 `node_modules`、`.pub-cache`、`.dart_tool`、`.gradle` 等作为忽略的本地数据／缓存，存在本身不违反 Git 上传规则。卫生脚本对此应正确分类。
- `desktop/resources/qdrant` 只保留数据库压缩包与配置，`resources/surrealdb` 保留压缩包，当前检查未发现 Qdrant 运行存储被放入 `core`。
- Flutter 的 Gradle wrapper 由当前 Flutter 工具 `injectGradleWrapperIfNeeded` 补充，其默认忽略规则不单独判为缺失源码。
- `runtime/out/originos-island-reference` 是被忽略的参考仓库，不是当前 Git 主仓库源码；扫描未递归其 Git 元数据。
- root `plugin/` 有 21 个插件产物文件，未发现完整插件源树误放其中。

## 建议的整改顺序与目标结构

保持现有应用根目录，不进行无必要的全仓重命名。行业通行的做法是按用途明确边界，并使 Git、构建和发布采用同一套分类。

1. 先恢复正式源码可追踪性：管理端、Ipad860、被忽略的 Runtime 构建源码，以及渠道 bundle 的源码构建链。
2. 统一 `plugin/` 大小写和插件源码／产物路径，解除生成包、缓存和重复原生资源的 Git 跟踪。
3. 统一构建输出与 staging，收敛启动脚本使用的核心二进制，处理 `core` 白名单污染和 Debug APK。
4. 脱离运行配置与源码模板，明确数据目录；数据库、备份和安装状态须保留，不按普通缓存清理。
5. 最后归档历史源码片段和空目录，明确测试例外，并统一卫生检查、源码打包及发布验收规则。

建议边界：`plugins/` 为正式插件源码，`plugin/` 为忽略的插件交付产物，`artifacts/` 为忽略的构建／验证输出，应用自身标准 `build`／`dist` 和正式规定的 `desktop/release` 可保留但必须忽略；`desktop/resources` 只作为经白名单验证的发布 staging；`runtime/scripts`／构建工具为源码，`runtime/out` 为输出；`docs/audits` 为审计结论，原始海量清单归本地 `artifacts/audits`。

每一类整改都需要同步实际路径引用和相应定向验证。本轮没有执行整改，也没有将本轮报告推送 Git。
