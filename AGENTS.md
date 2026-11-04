禁止使用cmd和powershell工具进行批量替换。
构建手机端时必须从后端源码重新构建 Runtime Package，并使用同一份后端源码重新构建手机端所需后端文件，禁止使用旧 Runtime Package 或旧后端产物。
任何修改必须先给出行业标准方案；如果现有设计不符合行业标准方案，必须先给出行业标准方案，等待用户确认无误后，再按照确认后的行业标准方案进行修改或重构。
做功能时优先参考行业标杆项目或者高星开源项目的实现方式。
用户未要求使用skill时，禁止使用任何skill。
项目中不使用系统Node环境，必须使用nodeexe或Linux版本可直接运行的Node二进制文件。
电脑中已经安装powershell7,必须使用powershell时优先使用powershell7。
Go 安装路径：C:\Code\Go\bin\go.exe
Git 安装路径：C:\Code\Git\Git\bin\git.exe
必须中文回复。
项目中不写注释。
纯前端代码修改无需重启完整服务，使用前端热更新生效，禁止因纯前端代码修改重启完整服务。
涉及后端或其他需要重启才能生效的修改时，完成后必须重启完整服务（前端、后端）。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。

禁止在有xx系统的情况下，重复造第二套相同的系统，优先对原系统进行修改，如果修改难度超过重构难度那就进行局部重构，遇到已有两套系统的情况优先进行询问，或者遇到无法抉择的情况也优先进行询问。

开发一个功能时，如果了解到的信息不够支持进行开发，优先询问用户，当拿到充足的信息后，有100%把握一次性完成开发后再进行开发。

用户提出的任何修改需求需要先结合项目进行详细的分析后再进行修改。
除非用户明确指定本次修改属于管理端，否则默认禁止改动管理端任何代码、配置、端口、数据库和发布流程；当需求不可避免地涉及管理端改动时，必须先向用户说明原因并询问，获得明确授权后才能修改。
任何的修改不要使用3000端口(CCX)，项目中使用的端口应避开3000。
禁止拉取git。
项目启动端口严格使用项目内端口，禁止乱改端口。
启动=启动完整项目。
重启=重启完整项目。

单版本策略：

当前版本视为系统首个版本，只支持与当前源码同时构建和发布的 Web、Electron、Android、iOS、后端、Runtime Package、插件包、协议及数据格式。
禁止新增或保留任何旧版本兼容，包括旧字段、旧路由、旧端口、旧协议分支、旧消息格式、旧配置、旧本地存储、旧数据库结构升级、旧插件包迁移、旧运行包、legacy 别名、回退端点、双写、双读、自动归一化和兼容适配器。
旧版本输入必须直接失败并原样返回完整原始错误，禁止静默转换、降级、回退或尝试修复。
删除功能或协议时，同步删除其兼容代码、运行时分支、脚本、测试和文档引用；不得以保留兼容层作为过渡方案。
仅允许当前版本内部的数据迁移、数据库迁移、扩展更新和错误恢复机制；这些机制不得读取、升级或转换任何历史版本格式。

错误处理强制规则：
所有端、所有模块和所有错误处理路径必须原样保留并返回原始报错信息，包括错误对象、错误链、cause、message、msg、error、detail、internalMessage、reason 和堆栈信息。
禁止静默捕获、空 catch、丢弃 error、错误参数未使用、将原始错误替换为固定文案、按错误码或状态码改写原始信息、截断原始错误、无 cause 重新抛错，以及仅记录日志却向调用方返回固定错误。
API、函数、任务、流式响应、RPC、原生桥接、插件、脚本和构建工具必须向上游调用方传播原始错误；无法直接返回错误对象时，返回的失败文本或失败对象必须包含完整原始错误内容。
允许在原始报错之外附加面向用户的提示、解释、处理建议和上下文，但附加信息不得替代、覆盖、截断或隐藏原始报错。
用户界面可以向用户展示易懂的错误解释，但同时必须展示可查看或复制的完整原始报错信息；禁止只展示固定提示而不展示原始错误。
任何错误处理修改完成后必须执行交叉审计，确认不存在吞错、固定错误覆盖、错误截断、错误链丢失和返回固定失败信息等漏项。

Git上传规则：
构建产物不上传git（desktop/release/、desktop/build/、desktop/dist-types/等编译输出目录）
依赖不上传git（node_modules/等）
surreal.exe有自动解压机制，上传surreal.zip即可
qdrant.exe有自动解压机制，上传qdrant.zip即可
server.exe上传server.zip即可

Git冲突处理规则：
git提交或合并出现任何冲突时，直接以本地代码强制覆盖冲突内容并继续提交，不需要询问用户。

插件目录规则：
插件完整源码统一放在项目根目录的plugins-code文件夹内。
插件打包产物统一输出到项目根目录的plugin文件夹内。
禁止将插件完整源码放入plugin文件夹。

桌面端构建规则：
桌面端运行时使用AmitiaCore.exe作为核心后端服务，该文件由Go后端编译产物server.exe重命名而来。

桌面端安装程序构建前应当先更新其依赖的后端核心与公共运行时

构建桌面端安装程序前必须停止完整项目并清理desktop/release，避免旧产物或运行数据混入。

必须重新编译Go后端server.exe，并将本次编译结果重命名同步为desktop/resources/core/AmitiaCore.exe，禁止使用旧核心。

desktop/resources/core发布资源使用严格白名单，只允许包含AmitiaCore.exe以及公共Node运行时压缩包。

渠道运行时、Native Companion、bundle、源码、依赖、测试、日志、数据库、缓存和备份必须随插件包发布，禁止带入桌面端宿主的core目录。

禁止将desktop/resources/core/qdrant/storage及任何Qdrant运行时存储带入安装包。Qdrant和SurrealDB只通过resources/qdrant/qdrant.zip与resources/surrealdb/surreal.zip发布。

桌面端正式构建统一从desktop目录执行pnpm dist:win，禁止绕过scripts/build-release.mjs直接执行electron-builder，以确保使用规定的压缩等级和--publish never。

electron-builder保持compression: normal，实际7z压缩等级由scripts/build-release.mjs设置为ELECTRON_BUILDER_COMPRESSION_LEVEL=5。

必须保持nsis.differentialPackage: true，禁止为了加速构建关闭差分更新。

完整发布必须同时生成AmitiaSetup-${version}-x64.exe、AmitiaSetup-${version}-x64.exe.blockmap和latest.yml，缺少任意一项都视为构建失败。

每个正式版本必须维护desktop/release-notes.md，并通过releaseInfo写入latest.yml。latest.yml中的版本、安装包文件名、文件大小和SHA-512必须与实际安装包一致。

构建完成后必须验证blockmap可解压解析、app-update.yml指向正确GitHub仓库、安装内容不含运行数据，并检查AmitiaCore.exe与本次Go构建产物哈希一致。

构建完成后只执行本次源码修改直接涉及的桌面端、后端和插件测试，禁止默认运行全量测试；未改动模块不得执行测试。并按项目规则重启完整服务，确认前端、核心、Qdrant、SurrealDB及已安装渠道插件端口和健康接口正常。

未收到用户上传Git的明确指令时，桌面构建和发布过程不得上传GitHub Release或推送Git。

桌面端版本发布规则：
桌面端版本更新托管在自有服务器（amitia.untrammelled.top），使用electron-updater的generic provider，通过FTP上传构建产物到服务器静态目录。

发布基线与更新日志规则：

当用户明确要求构建桌面端 NSIS 安装包或 Android Release APK 时，必须先创建本次发布的源码基线提交，且不得包含构建产物、依赖、缓存、日志或运行数据。

构建版本号必须保持项目当前版本；只有用户明确指定新的版本号时才允许修改，禁止自行递增、降级或调整版本号格式。

构建前必须读取本文件中的最近一次对应平台源码基线提交，与本次源码基线提交进行差异对比。对比范围仅限发布相关源码、配置、迁移和文档；忽略 AGENTS.md 发布记录、构建产物、依赖、缓存、日志与运行数据。

构建前的测试范围必须基于本次源码差异确定，只运行修改文件所属模块的定向测试；禁止默认运行 `go test ./...`、前端全量测试或其他无关模块测试。

构建门禁（release:gate）执行规则：

- 后端门禁只做编译级校验加改动包定向测试：`go mod verify`、`go vet ./...`、`go build ./cmd/server`，以及由源码差异推导出的改动包 `go test`；构建流程中禁止默认执行 `go test ./...`。
- 差异推导默认以 HEAD 为基准并包含工作区改动；工作区干净时用 `AMITIA_RELEASE_BASE_REF` 指定基线提交，推导为空则不执行后端测试。
- 前端门禁只执行 `typecheck` 与 `build`，默认不执行前端测试；需要时用 `AMITIA_RELEASE_FRONT_TEST_FILES` 显式指定要跑的测试文件，禁止默认执行前端全量测试。
- 编译级校验通过即可执行 `pnpm dist:win` 产出发布产物；不得为了"保险"执行未改动模块的测试。
- 门禁环境变量：`AMITIA_RELEASE_BASE_REF`（差异基准提交）、`AMITIA_RELEASE_GO_TEST_PACKAGES`（显式指定后端测试包）、`AMITIA_RELEASE_FRONT_TEST_FILES`（显式指定前端测试文件）、`AMITIA_RELEASE_FULL_GO_TESTS=1`、`AMITIA_RELEASE_FULL_FRONT_TESTS=1`、`AMITIA_RELEASE_RACE_CHECKS=1`、`AMITIA_RELEASE_SKIP_FRONT_TESTS=1`、`AMITIA_RELEASE_PLAN_ONLY=1`（只打印本次门禁范围，不执行任何命令）。
- 发布输出目录默认为 `desktop/release`，可用 `AMITIA_RELEASE_DIR` 指定相对 desktop 的目录名；当 `release` 被系统进程锁住（例如杀软仍持有 `win-unpacked/resources/app.asar`）导致 `EPERM` 时，用它输出到新目录后重试，且该目录仍需落在 `desktop/release-*` 忽略规则内。
- GitHub Actions 的 Backend Full Test Gate 仍按工作流执行全量后端测试，与本地发布门禁相互独立。
- 桌面端构建前先跑一次范围预检确认门禁内容（不执行任何命令）：在 desktop 目录用项目内 node 执行 `scripts/verify-release-prerequisites.mjs`，并设置 `AMITIA_RELEASE_PLAN_ONLY=1`；随后按预检结果执行 `pnpm dist:win`。
- 已知与本轮改动无关的陈旧用例：`internal/search` 的超时断言停留在 `timeoutpolicy` 之前、`internal/system` 仍在断言已删除的 web-chat 路由、`internal/psyche_testdata` 未注入统一入口执行器。它们不在定向门禁范围内，重建对应模块时一并修复。

必须根据差异生成面向用户的更新日志。桌面端更新 desktop/release-notes.md；Android Release 更新对应发布说明。生成内容作为本次发布版本的更新说明。

更新日志截图的样式参考：

- 使用纯白背景，不添加卡片、渐变、纹理、插画或装饰性背景。
- 页面顶部左侧显示简洁的 Amitia 品牌字样，右侧显示 BETA 标识。
- 主标题使用粗体“更新日志”，下方紧接版本号，版本号下放置一条短蓝色横线。
- 正文使用左对齐的黑色或深灰色中文小字号，二级标题用于“更新内容”“适用范围”等分组，三级标题用于“更新通道”等具体分类。
- 更新项目使用圆点列表，行距紧凑，不使用编号徽标、彩色卡片或大面积强调块。
- 截图采用纵向长页面排版，内容连续排列，底部不保留大面积空白。
- 只参考并复用上述排版样式，不在文档或项目中嵌入参考图片。

构建、安装和验证全部通过后，必须更新本文件中的发布基线记录，包含平台、版本、源码基线提交编号、提交日志和记录时间。该记录变更需单独提交，且不得作为下一次发布差异对比的目标提交。

首次执行对应平台构建且不存在已记录源码基线时，以本次源码基线提交的父提交作为差异对比起点。

若工作区存在无关变更、提交冲突或无法判断变更归属，必须先询问用户，禁止混入发布基线提交。

桌面端发布文件必须先上传至服务器临时目录，验证完整性后向用户提供明确的服务器替换命令。用户确认替换成功后，必须将可复用的上传、验证与替换流程补充至本文件。

最近发布源码基线：

- Desktop NSIS（本地构建、未安装、已通过管理端发布）：版本 26.2.0-beta.21，源码基线 1a6e5cdc704ecbbdd068ad4c8c3db35977eb2a8f，提交日志 feat(mobile): apply Amitia app identity；上一已发布基线 7fb423866b1ec1132f7119b92130c0148f1f0030。安装包 artifacts/release-26.2.0-beta.21/交付文件/AmitiaSetup-26.2.0-beta.21-x64.exe，378665982 字节，SHA-256 694021e921c98323d0ed3a7a5103a395c983d53f6d28c3e31b0e055de0464c7a；blockmap SHA-256 ef950a20d3c263a77a1b5be42de457835e7fe2d45e027e0f21abfd94693ee893；latest.yml SHA-256 d3de942b90d0989439f0d490e4df12e59451cffd33a1a177d9801f0509cd7e07；AmitiaCore.exe SHA-256 17014f35886e71a38208e62e8119d0e0819626229f1da9c3427f4889578cebbb，Source Gate bdf30ca81210b4f42e3e0913eb9998e33d463bd57017d26f7f0d79898fc033c8。发布门禁执行 go mod verify、go vet、19 个源码差异包定向测试、前端 typecheck/build、生产渲染挂载、桌宠单轨与真实 Electron 冒烟，Windows x64 Core、包内 693 个 Runtime 资源及 Release Gate 全部通过。已通过管理端发布到 stable 通道（release id 53），线上 latest.yml、EXE、blockmap、SHA-512、更新标题、8 条亮点、完整公告和 EXE 首中尾 Range 内容校验通过。当前环境为 macOS，未执行 Windows 安装和覆盖验证。未推送 Git。记录时间 2026-11-02T12:08:50+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Desktop macOS（本地构建、已启动、已通过管理端发布）：版本 26.2.0-beta.21，源码基线 1a6e5cdc704ecbbdd068ad4c8c3db35977eb2a8f，上一已发布基线 7fb423866b1ec1132f7119b92130c0148f1f0030。ZIP 更新包 artifacts/release-26.2.0-beta.21/交付文件/Amitia-26.2.0-beta.21-arm64-mac.zip，307901773 字节，SHA-256 43d207f78fe8c48e39a1dd59cfc4d6bb5c179bc7a70bfb96a7c4fc31e630d1b8；blockmap SHA-256 b909f45ab2535c1fd67482dd9bf7a5f3b5ecdcc3cae73cc71cb4b2b457c10d4c；latest-mac.yml SHA-256 0ba6762abd806c6414511760c9b54ad25344491400d9080a830fa2753b7b99b4；DMG artifacts/release-26.2.0-beta.21/交付文件/Amitia-26.2.0-beta.21-arm64.dmg，309076040 字节，SHA-256 f91e8cd990ba04bcc85eb33eb1b2dd40b51674c549b3149eab3eb6f4beb95bb9；AmitiaCore SHA-256 d58c42bb42f8089312b5d1ad3908e15a172aed1638655b32a91c92f3971527c6，Source Gate 与桌面端一致。macOS ZIP、DMG、blockmap、Darwin arm64 Core、包内 41 个 Runtime 资源及 Release Gate 全部通过；应用 Bundle 名称和版本为 Amitia / 26.2.0-beta.21，包内签名状态为 ad-hoc，未使用 Developer ID 签名或公证。已通过管理端发布到 stable 通道（release id 54），线上 latest-mac.yml、ZIP、blockmap、SHA-512、更新标题、8 条亮点、完整公告和 ZIP 首中尾 Range 内容校验通过。已直接启动本次构建的 Amitia.app，Electron、Core、Qdrant 与 SurrealDB 正常运行；/api/health HTTP 200、ready=true、version=26.2.0-beta.21；/readyz HTTP 200、degraded（6 ready、1 degraded、0 blocking、0 failed），模型未配置。未推送 Git。记录时间 2026-11-02T12:08:50+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、未安装、已通过管理端发布）：版本 26.2.0-beta.21，versionCode 27，源码基线 1a6e5cdc704ecbbdd068ad4c8c3db35977eb2a8f，上一已发布基线 7fb423866b1ec1132f7119b92130c0148f1f0030。APK artifacts/release-26.2.0-beta.21/交付文件/amitia-26.2.0-beta.21-release-arm64-v8a.apk，154875439 字节，SHA-256 3a0f8891b901b66e65813137179bc76ec81f49b13f27ca48559823aa7d6ed884；Runtime Package artifacts/release-26.2.0-beta.21/交付文件/amitia-runtime-1.0.0-android-arm64.zip，131732649 字节，SHA-256 5592196c413812f496569cb56e08d2427ce82be8bc4be8d0ceb3361f6ce5ec1a，sourceCommit/sourceRevision 与 1a6e5cdc 一致；内嵌 Linux ARM64 后端 SHA-256 2de429e5cedbecefbf0fb8a1f78ec3052383b3cb8999e82386e071aeb1e86438，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 与 Runtime Package 均由同一份源码重新构建，未复用旧 Runtime 或旧后端。Release Vital lint、无障碍 lint、assembleRelease、正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b、apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过；APK 系统显示名称为 Amitia，adaptive icon、roundIcon、前景层和品牌背景资源均确认存在。已通过管理端发布到 stable 通道（release id 55），线上 android/stable.json、stable.json.sig、APK 大小、SHA-256、RSA SHA-256 签名、更新标题、8 条亮点、完整公告与 APK 首中尾 Range 内容校验通过。当前 adb 无连接设备，未执行本次安装、冷启动和实际模型对话验证。未推送 Git。记录时间 2026-11-02T12:08:50+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Desktop NSIS（本地构建、未安装、已通过管理端发布）：版本 26.2.0-beta.20，源码基线 7fb423866b1ec1132f7119b92130c0148f1f0030，提交日志 chore(release): prepare 26.2.0-beta.20 sources；上一已发布基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。安装包 artifacts/release-26.2.0-beta.20/交付文件/AmitiaSetup-26.2.0-beta.20-x64.exe，378555507 字节，SHA-256 9fd13c14fcf0bb04e9764a02c2a74952ae163831fe5f25bd37a581f193a5ba03；blockmap SHA-256 2dbe6d050d992fba2c1f0fe6e5e870aa898760547b8b82f566b883e5327e7168；latest.yml SHA-256 881115b4a747d0c6ec3408219f49b31b8987075a462dc475810d2f1f57d1697c；AmitiaCore.exe SHA-256 e17160f5ac9c2eda68d5eb9b665a21e3c84866ac6531f189948ce273215b5718，Source Gate ce1ea449fbf3f8ea1128bb4ebe7ebd03c914f65373bc92e6cdf7f4253515b75b。发布门禁执行 go mod verify、go vet、183 个源码差异包定向测试、4 个桌宠 race 包、Windows Core 构建、前端 typecheck/build、生产渲染挂载、桌宠单轨与真实 Electron 冒烟，全部通过；freeze 清单 11151 个文件与 693 个包内 Runtime 资源校验通过。修复设备数据提交按 Outbox 扫描与后台投递并发导致已成功写入却返回 503 的竞态，改为按 requestId 精确读取 pending 或已提交确认，并增加幂等回执测试；三核心记忆归属测试连续 20 次通过。已通过管理端发布到 stable 通道（release id 51），线上 latest.yml、EXE、blockmap、SHA-512 与 EXE 首中尾 Range 内容校验通过。当前环境为 macOS，未执行 Windows 安装和覆盖验证。完整项目已用当前源码重建 macOS Core 并重启，Core health.ready=true、version=26.2.0-beta.20，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed），Qdrant、SurrealDB、Electron 与已安装渠道插件正常运行。未推送 Git。记录时间 2026-11-01T02:04:14+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、未安装、已通过管理端发布）：版本 26.2.0-beta.20，versionCode 26，源码基线 7fb423866b1ec1132f7119b92130c0148f1f0030，提交日志 chore(release): prepare 26.2.0-beta.20 sources；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/release-26.2.0-beta.20/交付文件/amitia-26.2.0-beta.20-release-arm64-v8a.apk，154569115 字节，SHA-256 017199ae5ce9efd8eb62a239be86110bd1332d988d3221e518f5ff074b725d5f；Runtime Package artifacts/release-26.2.0-beta.20/交付文件/amitia-runtime-1.0.0-android-arm64.zip，131685345 字节，SHA-256 581aee9771d58942161f81c3c4115086c846e219eb0ac1db2cd025f7c727fc80，sourceCommit/sourceRevision 与 7fb42386 一致；内嵌 Linux ARM64 后端 SHA-256 9b4676c8a61ebf558ef684053aaa09ed2a0c19a40c989959b31f2fac926cf74b，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 与 Runtime Package 均由同一份源码重新构建，未复用旧 Runtime。Release Vital lint、无障碍 lint、assembleRelease、正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b、apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已通过管理端发布到 stable 通道（release id 52），线上 android/stable.json、stable.json.sig、APK 大小、SHA-256、RSA SHA-256 签名与 APK 首中尾 Range 内容校验通过。当前 adb 无连接设备，未执行本次安装、冷启动和实际模型对话验证。更新日志完整逐行审计 34 个历史提交、1375 个文件、6376 个差异片段和 45602 条逐行索引，图片与文本位于 artifacts/release-26.2.0-beta.20/交付文件。完整项目已重启并通过 health/readyz 检查。未推送 Git。记录时间 2026-11-01T02:04:14+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 7c1d429ab0108e86cf7777125f2f8c430d2ccd1b，提交日志 style(mobile): nudge extension icon size；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-extension-icon-final3-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154565715 字节，SHA-256 a1a2f274eaf0099e9b9ba4791dca0a0cec5ebc56d7cbc2dec19ee528f95c0c1e；Runtime Package artifacts/android-extension-icon-final3-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682101 字节，SHA-256 0aeacbd2901fb25d19f4c7eb0535e97cc963320d974237c5008479ed943dfe3a，sourceCommit/sourceRevision 与 7c1d429a 一致；内嵌 Linux ARM64 后端 SHA-256 12485034513ee1cb7f401b4fd663b01cf65a9b44bad10323cec2fcb5cc835913，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 10 项底栏导航与扩展入口测试、图标静态分析、冻结清单校验和 assembleRelease 通过；扩展图标按确认要求在 5.0–19.0 基础上再次轻微放大到 4.6–19.4，圆角调整为 1.55，保持四块拼合造型、1.8 线宽和 21px 底栏尺寸。正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1833，Runtime state=READY，MainActivity 正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T23:56:53+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 54d85c1e7e184a48fdb1e59fb41d114a573a9584，提交日志 style(mobile): slightly enlarge extension icon；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-extension-icon-final2-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154565439 字节，SHA-256 99545c7be756b60826200506f888e118f8e9867682f1772f88ba030dd51b6fdb；Runtime Package artifacts/android-extension-icon-final2-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682045 字节，SHA-256 178a79e8bd606162d810da69f55275533874a6c3650713682c8fb8111be0ccec，sourceCommit/sourceRevision 与 54d85c1e 一致；内嵌 Linux ARM64 后端 SHA-256 55ee2b8c03bab385bf5660b32ba292d3cd21fa58bf06609d15ba3c73f1567d04，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 10 项底栏导航与扩展入口测试、图标静态分析、冻结清单校验和 assembleRelease 通过；扩展图标在四块拼合方案基础上将视觉边界从 5.5–18.5 轻微放大到 5.0–19.0，圆角调整为 1.45，保持 1.8 线宽和 21px 底栏尺寸。正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1711，Runtime state=READY，MainActivity 正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T23:47:14+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 2c700e1d20ecbba85457a95d3b015c40b1a5371b，提交日志 style(mobile): balance extension icon optical size；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-extension-scale-final-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154565527 字节，SHA-256 ceefd35db5262897f1121162105a4aeb6fd9d5e74894506fc48721d1d6c78207；Runtime Package artifacts/android-extension-scale-final-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682045 字节，SHA-256 03d83fce7eade8ea6478483efde16f94fffd1aac95988f082545e0bd6ecefa83，sourceCommit/sourceRevision 与 2c700e1d 一致；内嵌 Linux ARM64 后端 SHA-256 606d9a5abd83437efe9cf513e2d4d5350b0cd9425e1c5544903a4c6bced988d0，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 10 项底栏导航与扩展入口测试、图标静态分析、冻结清单校验和 assembleRelease 通过；扩展图标维持四块拼合方案，将视觉边界从 3.8–20.2 调整为 5.5–18.5、圆角调整为 1.35，保持 1.8 线宽和 21px 底栏尺寸，与其他图标的光学大小统一。正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1418，Runtime state=READY，MainActivity 正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T23:40:23+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 f0ea57ebb9ad736645084403c65bb772f4d088e5，提交日志 feat(mobile): apply modular extension icon；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-archive-icon-final-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154565239 字节，SHA-256 cea595d566b54a5a4deb7965ab4e90ec96e0d24be0842cea98f0dbc43cbb5fa8；Runtime Package artifacts/android-archive-icon-final-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682049 字节，SHA-256 97e9509f80bd7b8285833eba759ac22f8d3140b7a9c73a0284d10ab7a1e4f567，sourceCommit/sourceRevision 与 f0ea57eb 一致；内嵌 Linux ARM64 后端 SHA-256 c504104bf9cadf06e896921e90dd99a16c20f4c71f61b538e2e450da2c483528，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 68 项删除、归档、角色作用域、项目模式、扩展入口和返回导航测试、冻结清单校验和 assembleRelease 通过；修复单条归档、批量归档和归档页单条恢复未携带会话角色导致 409，并将“扩展”图标替换为确认的四块拼合模块方案。正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 2002，Runtime state=READY，MainActivity 与 amitia-server 进程正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T23:33:27+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 ad2e8cd76a42ffa3f5304c08a7a6cd3a9c6c95d1，提交日志 feat(mobile): rename plugin tab to extensions；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-ui-refresh-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154565163 字节，SHA-256 ad583d6e237a193f2a39f08b4ee377715ef9d52e8546de2c2215402346f0e6e2；Runtime Package artifacts/android-ui-refresh-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682081 字节，SHA-256 2f375859f0c051b2be65334f5f3dbedd13c7449c0bfd3856df57f3b802efca7e，sourceCommit/sourceRevision 与 ad2e8cd7 一致；内嵌 Linux ARM64 后端 SHA-256 635229738fa85b95ce49de7ad9df10eb69dcc254cf899f050859927bc047e470，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 15 项项目模式、展开层级、扩展入口和返回导航测试、冻结清单校验和 assembleRelease 通过；项目模式空状态改为与消息模式一致的无容器布局，移除页面内卡片式空状态代码；底部“插件”改为“扩展”，删除旧插头图标并新增模块化扩展图标。正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 2645，Runtime state=READY，MainActivity 与 amitia-server 进程正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T23:21:56+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 5ee1d1ee1d03aeca15a9a699d49c2c563ca8ce6b，提交日志 fix(mobile): make conversation deletion resilient；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-delete-fix-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154574139 字节，SHA-256 0cfaf270388e19e6039c3f8d28d0fb3b123a69c71b7c9062fab6b3e4d8a84ab4；Runtime Package artifacts/android-delete-fix-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682057 字节，SHA-256 78ffd3a0b25f05263920d6f9498053e88e37eb9a4b081f44f3caf79cb99f06f4，sourceCommit/sourceRevision 与 5ee1d1ee 一致；内嵌 Linux ARM64 后端 SHA-256 0f5d2e2a37dd3bdc01bfe78a1f6363c6c8308b8d52233d7edc5be107914adb75，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 55 项删除、并发上下文、返回导航和归档批量测试、冻结清单校验和 assembleRelease 通过；正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1503，Runtime state=READY，MainActivity 与 amitia-server 进程正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。本轮修复会话删除误用历史查询导致并发请求互相覆盖、后端确认成功后仍因本地轮询版本变化误报失败、已删除会话重复删除，以及主列表删除未携带会话角色的问题。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T23:07:49+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 e43c4ee1718a6f59397ea9b6e6325e870a693aaf，提交日志 fix(mobile): handle Android back before exit；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-back-navigation-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154573899 字节，SHA-256 28374ba7f3536ee3323ae79f4b96ff4ed62d6044b338d7ce13dc75e015773ccf；Runtime Package artifacts/android-back-navigation-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682277 字节，SHA-256 8d36c53af4047c41f9630461f08014eb1970f1117e41f4efc84aed022f299ef3，sourceCommit/sourceRevision 与 e43c4ee1 一致；内嵌 Linux ARM64 后端 SHA-256 fa7b8ddf785821e142efac997b00b5ebc144487895a766896e21a113e487d179，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 15 项返回导航定向测试、冻结清单校验和 assembleRelease 通过；正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1541，Runtime state=READY，MainActivity 与 amitia-server 进程正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T22:57:06+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 df022961a3c708fce28d46bfea263d28f4c20ab8，提交日志 feat(chat): add context menus and archived batch actions；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-context-menu-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154559787 字节，SHA-256 94e22910b0db6118a04215f71031cc65c9956fef2cb2677cbc70e6ff1fe42469；Runtime Package artifacts/android-context-menu-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682045 字节，SHA-256 a565fb517e04bd4e017da69cd20a5de493f02bcef4d808e5bffd43be5f55144b，sourceCommit/sourceRevision 与 df022961 一致；内嵌 Linux ARM64 后端 SHA-256 bd05787848649dedc668f12ab869f9f7432f2fcbdd57e314891ccb0bd09ac605，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp b12018 与 SurrealDB 输入哈希校验通过。手机端 6 项定向测试、后端 devicemesh business/coordination 定向测试、冻结清单校验和 assembleRelease 通过；正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1315，Runtime state=READY，MainActivity 与 amitia-server 进程正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。手机端定向静态分析仅报告 14 条既有诊断，本轮改动未新增警告。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T22:42:34+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Desktop NSIS（本地构建、未安装、已通过管理端发布）：版本 26.2.0-beta.19，源码基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38，提交日志 chore(release): refresh beta.19 freeze manifest；上一已发布基线 d60926e188c53cf2b497d2c35bab0bf242b92522。安装包 artifacts/release-26.2.0-beta.19/交付文件/AmitiaSetup-26.2.0-beta.19-x64.exe，362522314 字节，SHA-256 1860ab0bfe20e1a7e8f335c850acd1823051ffbe321a1f666cd8780e1efeba7f；blockmap SHA-256 468340145f39027c339bcd11664b075d64f3c2f322ec885d77395a0c2a0e6d72；latest.yml SHA-256 bc5123b837a6da94a12a38f2534dbd445f9fcb910ec7be8b993404dc74991ab0；AmitiaCore.exe SHA-256 4335473abf6f57bd57985419a7956a3b2c182912d0221c56e031929e3cd8f50a，Source Gate 296ae280737cc876cb786d1c3f74f15a7b34bb006d079dd5a9b8e3ee223018b1，Release Gate PASS，freeze 清单 11110 个文件、662 个包内 Runtime 资源校验通过。构建门禁执行后端定向测试、前端 90 项测试、生产渲染挂载、桌宠单轨与真实 Electron 冒烟，全部通过。已通过管理端发布到 stable 通道（release id 49），线上 latest.yml、EXE、blockmap、SHA-512、公告及首中尾 Range 内容校验通过。当前环境为 macOS，未执行 Windows 安装和覆盖验证。完整项目已用同一份源码重建并启动，Core health.ready=true、version=26.2.0-beta.19，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed），Qdrant 与 SurrealDB 健康接口正常。未推送 Git。记录时间 2026-10-31T01:26:49+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、最终包未安装、已通过管理端发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38，提交日志 chore(release): refresh beta.19 freeze manifest。APK artifacts/release-26.2.0-beta.19/交付文件/amitia-26.2.0-beta.19-release-arm64-v8a.apk，139053735 字节，SHA-256 8d802e89e42e51ff9e66f43d2f6b68a2ceaab2b843f14f6c2d45941711bba208；Runtime Package artifacts/release-26.2.0-beta.19/交付文件/amitia-runtime-1.0.0-android-arm64.zip，116288029 字节，SHA-256 ef6c7bce46745c9a57f656ee64ef3fdafc0935bbc413bbb969f127caf52d7e90，sourceCommit/sourceRevision 与 b4828ae4 一致；内嵌 Linux ARM64 后端 SHA-256 a1c48c4efa3c60dc2cf16667083076b1f8f16b4dce38fee862da2ea6fa9ef108，plugin-host SHA-256 8c2b4fbf07f7b39d098611e42f3c33ca94ec3f36738c67c1bbca08549bc6eaa6，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建，新增锁定 libatomic1，Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs 与 SurrealDB 输入哈希校验通过。Release Vital lint、无障碍 lint、正式证书、apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 和 sourceCommit 校验通过，证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b。已通过管理端发布到 stable 通道（release id 50），线上 stable.json、stable.json.sig 与 APK 可访问，versionCode、versionName、大小、SHA-256、RSA SHA-256 签名及首中尾 Range 内容一致。当前未连接 Android 真机，未执行最终包安装和冷启动。未推送 Git。记录时间 2026-10-31T01:26:49+08:00。本记录提交不是下一次发布差异的源码目标提交。

- 正式插件包（beta.19 全部重新打包）：QQ Bot 1.0.20，1640129 字节，SHA-256 c29d260f0ab19e3a7ec114c848a2974cec3dba2c920eb68fb55af921eadc3880；QQ 个人号 0.2.177，1890175 字节，SHA-256 d089c36787e7ee73dc325e114dffcb5e396e0a61c824fe71723492e1c501ca9b；微信 1.0.26，6466202 字节，SHA-256 0f62da14583d70a5d570b0bb3935bd5307ed1f1a8b17c9835ead34268a21efdd；Emote 1.1.10，23003 字节，SHA-256 c7eb6b1c923c2009cdcaa91cb3f47f6cc4a6926f1986a851b2c37901fc4c2524；Lifestyle 1.0.12，15413 字节，SHA-256 53a2716db7d27e5226b3b33c8b890c753e6e7b86fd59fad77027fc1f136d44f9。全部包位于 artifacts/release-26.2.0-beta.19/交付文件/全部插件，并通过 ZIP 完整性与各自构建校验；汇总包 Amitia-26.2.0-beta.19-all-plugins.zip SHA-256 dc5f619f305d81305d4a7441bc16bb331a9606abbaa4aad3f445f411efe0f8e9。记录时间 2026-10-31T01:28:48+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Desktop NSIS（本地构建、未安装、已通过管理端发布）：版本 26.2.0-beta.18，源码基线 d60926e188c53cf2b497d2c35bab0bf242b92522，功能提交 818f21f8733ff217752ddb22176d9b073020e959，提交日志 merge(release): integrate compatible beta.18 runtime updates；上一已发布基线 0c38b70fd8d32fdf539b4eb1b8b08179671e782c。安装包 artifacts/release-26.2.0-beta.18/交付文件/AmitiaSetup-26.2.0-beta.18-x64.exe，362493435 字节，SHA-256 fa9723ac7cbbe42cc377c76d2caf4384ebf74ab1b05a9597adac8dc93cfaf59f；blockmap SHA-256 5fda3aa5551318aa36069900f17e08d6a9f982a4176547697ae90d8302e411c6；latest.yml SHA-256 623162f8e9c8d8d2f0711cb673309195a277e729368909372e8692e8f0e9a019；AmitiaCore.exe SHA-256 b6d65752a51349de4ed9e2bfe32734a40c45064f523e912a2896d5e874a657a1，Source Gate 8b11d5bbcffe84815a37907dacf1f4b3442ab5df375b5d3c28dae54d00dd629b，Release Gate PASS，freeze 清单 11100 个文件校验通过。构建门禁执行 25 个后端包定向测试、4 个桌宠竞态包、前端 typecheck 与 build、生产渲染挂载、桌宠单轨与真实 Electron 冒烟、包内 Core、运行时白名单和包内运行时校验，全部通过。Windows 内置 PowerShell 7，Node 26.12.1，桌面数据迁移到安装目录旁的 AmitiaData。已通过管理端发布到 stable 通道（release id 47），线上 latest.yml、安装包、blockmap、SHA-512、公告和 EXE 首中尾 Range 内容校验通过；当前环境为 macOS，未执行 Windows 安装和覆盖验证。完整项目已用合并源码重建并启动，Core health.ready=true、version=26.2.0-beta.18，readyz 为 degraded（5 ready、1 degraded、0 blocking、0 failed），degraded 为既有 data_lifecycle 退役检查。已推送 master。记录时间 2026-10-28T01:56:25+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、最终包未安装、已通过管理端发布）：版本 26.2.0-beta.18，versionCode 24，源码基线 d60926e188c53cf2b497d2c35bab0bf242b92522，功能提交 818f21f8733ff217752ddb22176d9b073020e959。APK artifacts/release-26.2.0-beta.18/交付文件/amitia-26.2.0-beta.18-release-arm64-v8a.apk，162724747 字节，SHA-256 127ce30b8c796e85899a76ac3886dcfbc2dbc9e1d36dd6d0012a1521c84752e0；Runtime Package artifacts/release-26.2.0-beta.18/交付文件/amitia-runtime-1.0.0-android-arm64.zip，139988003 字节，SHA-256 2cb9d132efca322d7aea9f2c66d06f0d79a9fb23b2fd4ad5a13964e6f8841a34，sourceCommit/sourceRevision 与 d60926e1 一致；内嵌后端 SHA-256 72916de90f2ddf9cdab4ef24087a6e349316986ebb5143e2b4213a73edd5d2f7，plugin-host SHA-256 8c2b4fbf07f7b39d098611e42f3c33ca94ec3f36738c67c1bbca08549bc6eaa6，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份合并源码重新构建，Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs 与 SurrealDB 输入哈希校验通过。Release Vital lint、无障碍 lint、assembleRelease、正式证书、apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 和 sourceCommit 校验通过，证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b。发布编号 48，stable 通道；线上 stable.json、签名和 APK 可访问，versionCode、versionName、大小、SHA-256、内置公钥 RSA SHA-256 验证及 APK 首中尾 Range 内容一致。当前没有连接 Android 真机，未执行最终包安装、冷启动和真机模型对话。已推送 master。记录时间 2026-10-28T01:56:25+08:00。本记录提交不是下一次发布差异的源码目标提交。

- 正式插件包（beta.18 全部重新打包）：QQ Clawbot 1.0.19，1640146 字节，SHA-256 9ad6a8e5ba6a0f87a8e32c96ef18480ee5bd96eda4e8707259b897e8993c6923；QQ 个人号 0.2.29，1891236 字节，SHA-256 fa9a443a141895bf27557f1db2da1328e7e5ca58cea66f6be23b457b9f64558e；微信 1.0.23，6459549 字节，SHA-256 c04103146b68fdb9f2a4eaa7afb03f9ee99875b107b4e6a7aed78d17b70aafd9；Emote 1.1.10，23003 字节，SHA-256 c7eb6b1c923c2009cdcaa91cb3f47f6cc4a6926f1986a851b2c37901fc4c2524；Lifestyle 1.0.12，15413 字节，SHA-256 53a2716db7d27e5226b3b33c8b890c753e6e7b86fd59fad77027fc1f136d44f9。全部包位于 artifacts/release-26.2.0-beta.18/交付文件/全部插件，并通过 ZIP 完整性及各自构建校验；汇总包 Amitia-26.2.0-beta.18-all-plugins.zip SHA-256 b6b0caf752c1413c6cf7b5ca7fe1eb8684303dfe7a102a67bd4f4cad2ff9c412。记录时间 2026-10-28T01:56:25+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Desktop NSIS / QQ 个人号插件（本地构建、未安装、未发布）：版本 26.2.0-beta.17，QQ 个人号插件 0.2.28，源码基线提交 d40230c9d8f7d26fecc3bf881d51f12117f5bb1a，提交日志 fix(desktop-pet): publish initial hit mask after frame throttle；本次差异对比基线为上一已发布源码 0c38b70fd8d32fdf539b4eb1b8b08179671e782c。安装包 artifacts/release-26.2.0-beta.17-20261026/AmitiaSetup-26.2.0-beta.17-x64.exe，277511354 字节，SHA-256 4bb310a7a24fc1a3c8f5440f5bf9051eb4f7fe8317bf9a93a3673eca3e54fd0e；blockmap SHA-256 a9f00b6eddce6bcbe473570dc6c9cd7f18bd9f07fcfaf15a623c21e0081a39dc；latest.yml SHA-256 b81ce681e67bad2e991c3e06ea2e6904dc02a46e8aa51a055d3026f2cf7f14c0；AmitiaCore.exe SHA-256 59be0b42d18d8cec58cc804bf1cfdc64dced9dd1a1887bded2ba11c4e4b7865c，Source Gate ac90c618ef8a42cbc6bf3592b4fddd8292e7821b51347c421db7dc6027a1be85，Release Gate PASS，freeze 清单 11076 个文件校验通过。QQ 个人号插件 com.amitia.channel-qq-personal-0.2.28.amitiax，1891243 字节，SHA-256 bac847e5b1f67cd5a7621d9a9951baeaf3ab3c8d92edf2cbdc6abef80669e2fc。构建门禁执行 17 个后端包定向测试、前端 typecheck 与 build、生产渲染挂载、桌面宠物 Electron 冒烟、打包 Core、运行时白名单和包内运行时校验，全部通过。当前环境为 macOS，未执行 Windows 安装和覆盖验证；未上传服务器、未发布 GitHub Release、未推送 Git。完整项目已重启，Core health.ready=true，readyz 为 degraded（5 ready、1 degraded、0 blocking、0 failed），degraded 为既有 data_lifecycle 退役检查。记录时间 2026-10-26T23:55:00+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Desktop NSIS（本地构建、未安装、已通过管理端发布）：版本 26.2.0-beta.17，源码基线 0c38b70fd8d32fdf539b4eb1b8b08179671e782c，提交日志 chore(release): refresh beta.17 renderer smoke source manifest；功能修复提交 fef49ff608b17fe0e776991fb42c2a61c3c73835，上一已发布基线 7f110257bb0acb9d5065f89cc534c03060b64a90。安装包 artifacts/release-26.2.0-beta.17/AmitiaSetup-26.2.0-beta.17-x64.exe，274772827 字节，SHA-256 72bb4a7736a60ac560ac2145b63360d00ebfed8e8984e0742407757dfe8b8507，AmitiaCore.exe SHA-256 31b37ead8d232c1fb4319e37865866165184bf599a6d4265805075b333235b7e。发布编号 45，stable 通道；线上 latest.yml 的版本、大小、SHA-512、公告和 blockmap 校验通过，EXE 首中尾 Range 内容一致。构建门禁与生产渲染挂载冒烟通过；Windows 安装未实测。未推送 Git。记录时间 2026-10-26T19:07:16+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、最终包未安装、已通过管理端发布）：版本 26.2.0-beta.17，versionCode 23，源码基线 0c38b70fd8d32fdf539b4eb1b8b08179671e782c，提交日志 chore(release): refresh beta.17 renderer smoke source manifest。APK artifacts/release-26.2.0-beta.17/amitia-26.2.0-beta.17-release-arm64-v8a.apk，136379183 字节，SHA-256 03716a22ec76b73a2ca9efc4edd310f9c384a49707f10fa802706ece941ad611；Runtime SHA-256 eb50a1ea624c28d341bc3aff22f3373ba5f67e415fe36504efaf20c63d8c3bd8，sourceCommit/sourceRevision 与基线一致，同源后端、plugin-host、task-host 均重新构建。正式签名、16 KB 对齐、单 ABI、Release lint、内嵌 Runtime 校验通过；最终包因设备断开未安装，不能将初轮构建的安装结果视为最终包真机验证。发布编号 46，stable 通道；线上 stable.json、签名和 APK 可访问，版本、大小、SHA-256、内置公钥 RSA SHA-256 验证及 APK 首中尾 Range 内容一致。未推送 Git。记录时间 2026-10-26T19:07:16+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Desktop NSIS（本地构建，未安装，已通过管理端发布）：版本 26.2.0-beta.16，源码基线提交 7f110257bb0acb9d5065f89cc534c03060b64a90，提交日志 chore(release): refresh 26.2.0-beta.16 chat top bar sources；本次差异对比基线为上一已发布版本 affb5ac4b6ecfce79778e59edc3fea2bdabd7a17（beta.15）。安装包 artifacts/release-26.2.0-beta.16/AmitiaSetup-26.2.0-beta.16-x64.exe，大小 274661281 字节，SHA-256 44a858953f5eb80e06f6348dee2cb97208398f060e4a0375a77c34c3ac2531be；blockmap SHA-256 6ad12f222c06cbad4c308c212a4d9bb3716b88c5930c93499cfff5f781af6b93；latest.yml SHA-256 fa45f7aaab904a6303de42a23dd098d342184baa0e10c032cdc84bbbd36a08a2；AmitiaCore.exe SHA-256 401a40d7af9348b41ea94729ac983460c67131bcf61e7dcb9e8cc2e534479058，Source Gate 78884671f90cdf59b3096f8b0b143d691a58a8b97990e1ab4fca2e5dddc0423b，Release Gate PASS，freeze 清单 11175 个文件校验通过。EXE、blockmap、latest.yml 完整，SHA-512、blockmap 解压解析、Windows amd64 核心、包内运行时、core 白名单和 generic 更新地址验证通过；包内 core 目录仅含 AmitiaCore.exe 与 node，打包核心哈希与本次 Go 构建产物一致。构建门禁按差异范围执行 73 个后端包定向测试、desktoppet 竞态检查、前端 typecheck 与 build 全部通过，本次修复 3 处后端契约缺陷（设备执行日志被响应覆盖工作节点绑定、owned_delivery 对 RevisionCurrent 的确认版本比较、agent_skill 租户隔离后的作用域查找），其余为 Windows-only 用例与过期断言对齐。已通过管理端接口发布到 stable 通道（release id 43），线上 latest.yml、安装包与 blockmap 可访问，线上版本、文件大小、SHA-512 及首中尾 Range 内容校验通过；当前环境为 macOS，未执行 Windows 覆盖安装。完整项目已重启并从同一份后端源码重建本地 Core，Core 版本 26.2.0-beta.16、health.ready=true，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed），Qdrant 与 SurrealDB 健康接口正常，已安装渠道插件与插件宿主正常。未推送 Git。记录时间 2026-10-26 00:38:32 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建，已安装，已通过管理端发布）：版本 26.2.0-beta.16，versionCode 22，源码基线提交 7f110257bb0acb9d5065f89cc534c03060b64a90，提交日志 chore(release): refresh 26.2.0-beta.16 chat top bar sources；Runtime sourceCommit 与 sourceRevision 与源码基线一致。安装包 artifacts/release-26.2.0-beta.16/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-002957.apk，大小 160535715 字节，SHA-256 c164e1224750e151b620d944e6454a30a36ed36ac95f454c9191c286342aa119；Runtime Package SHA-256 8ef971dfb36f00f9f365b60440ba73be0282b6ec06313d50f0f7cfd19c1db94b，后端 SHA-256 cde694e5561c8dfc71273ed43c893eb3336235a132a0f3df908c3587445b8676，SurrealDB SHA-256 2a363311330f4ac3595a4c348764c700a0c826abb4230c02b63e29822998df1f。已从同一份源码重新编译 Linux ARM64 后端、plugin-host、task-host 并新建 Runtime Package；APK 内嵌 Runtime 哈希与构建输入一致。正式证书匹配、apksigner v2、16 KB zipalign、单 arm64-v8a ABI、Release Vital lint、无障碍 lint、assembleRelease、内嵌 Runtime 哈希和源码提交验证通过，正式证书 SHA-256 为 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），复核 versionCode 22 / versionName 26.2.0-beta.16，com.amitia.amitia_app.debug 不存在；冷启动 am start -W 返回 Status ok、LaunchState COLD、TotalTime 2000，进程在前台，日志无 FATAL EXCEPTION。运行时升级后重新解包内置 Runtime 期间设备 USB 断开，未能完成设备端 /api/health 与 /readyz 的复核，未执行实际模型对话验证。已通过管理端接口发布到 stable 通道（release id 44），线上 stable.json、stable.json.sig 与 APK 可访问，versionCode、versionName、APK 大小、SHA-256、RSA SHA-256 签名和 APK 首中尾 Range 内容校验通过。完整项目已重启，Core 版本 26.2.0-beta.16、health.ready=true，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed）。未推送 Git。记录时间 2026-10-26 00:39:18 +0800。本记录提交不是下一次发布差异的目标源码提交。

本地修复构建验证记录（不替换下方已发布平台基线）：

- Android Release（最终包已构建、未安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 cbea002e25addfd4cedbe5aeb023590f6d66d554，提交日志 fix(mobile): unblock runtime replacement；启动路由功能提交 1a13baebba2de6efd22d0dae92bd55b8287a7213，上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-startup-routing-fix-20261101/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154566263 字节，SHA-256 380659ab1ecd8c933dbfe8ae62cc6cca7fade5bb2b8c5e347ba48a2e70ac82d0；Runtime Package artifacts/android-startup-routing-fix-20261101/amitia-runtime-1.0.0-android-arm64.zip，131682109 字节，SHA-256 aed576a0ddca6828f99fe9f8257de492bc4092e18b3004104b28886a40b3ab8f，sourceCommit/sourceRevision 与 cbea002e 一致；内嵌 Linux ARM64 后端 SHA-256 a0e7bfac24d2bebca01d98207cdc0a8e454c98d14adcb6cd5a1052e38cf18344，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建，锁定输入与 Runtime 校验通过。修复内容：首次创建 GoRouter 前等待 UI Provider 快照完整加载，合并启动期的失效通知，避免 `/conversations` 显示后因路由表刷新再次重解析同一根页面；进程退出 Future 改为在派发退出观察者前完成，避免 Runtime 替换时被阻塞的观察者拖入停止超时。手机端启动、路由和会话导航定向测试 12 项通过，静态分析仅 4 条既有 avoid_print 提示；Runtime Release Kotlin 编译、Release Vital lint 和 assembleRelease 通过。新增进程退出顺序回归用例，但 amitia-runtime 的 JUnit 测试源码集中存在既有接口签名过期，`testDebugUnitTest` 在编译旧用例时失败，未把该模块记录为完整测试通过。正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。启动路由修复中间包曾覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 c65ad257954702da01c55281c7dfa13a367f0c3fc3f838db223adcebc2af814f，冷启动 Status ok、LaunchState COLD、TotalTime 1234；Runtime state=READY，设备端 `/readyz` 与会话接口返回 HTTP 200，最终稳定停在“消息”根页，加载页到根页的短屏录制未观察到第二次根页切换。随后设备 USB 断开，最终 cbea002e 包未能在本轮重新安装；最终包安装、首次 Runtime 替换复测和真机点击验证待设备重新连接后执行。验证记录 artifacts/android-startup-routing-fix-20261101/android-verification.json、device-messages-root.png 与 device-cold-start.mp4。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-11-01T00:27:00+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 bcd0e43240a18f29e63c637beecd1ab924d1405f，提交日志 fix(chat): restore conversation previews and batch actions；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/android-chat-batch-20261031/amitia-26.2.0-beta.19-release-arm64-v8a.apk，154564367 字节，SHA-256 d21c2985d844ee0d6d53e9a9537041e603dc0e612b5cf28448fe1443aa34916e；Runtime Package artifacts/android-chat-batch-20261031/amitia-runtime-1.0.0-android-arm64.zip，131682357 字节，SHA-256 89f4ce329f4d96804c3cd0c604fe8157357916fa514fe11b90922e991ab4c73d，sourceCommit/sourceRevision 与源码基线一致；内嵌 Linux ARM64 后端 SHA-256 167d3e3dae18ce9aab5c4f2c6138ad629fd8fa8835e323c5db2c06e74fc66614，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。后端、两个宿主与 Runtime Package 均由同一份源码重新构建，锁定输入和 Runtime 校验通过。手机端定向测试 49 项、Release Vital lint、无障碍 lint、正式证书、apksigner v2、16 KB zipalign、单 arm64-v8a ABI 和内嵌 Runtime 哈希校验通过。已覆盖安装至 2407FRK8EC（JV9LT8JB8TVKNJ65），设备 base.apk 与本地产物哈希一致；冷启动 Status ok、LaunchState COLD、TotalTime 1368，Runtime state=READY，设备端 /readyz 请求返回 HTTP 200。未发布、未推送 Git。记录时间 2026-10-31T22:01:00+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 a32a3f05ba78c7d938bb3e47d9d53cc3057030c5，功能提交 aa867630，提交日志 fix(mobile): preserve chat navigation after first send；上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/apk/amitia-26.2.0-beta.19-release-arm64-v8a-20261031-210958.apk，154550167 字节，SHA-256 259a71aa4634d056771d54f0f9a65388343ede9f6c1d91da8cfb3d8b525811d9；Runtime Package artifacts/android-chat-route-20261031/amitia-runtime-1.0.0-android-arm64.zip，131680137 字节，SHA-256 9c4a1a4e34c9409c07749e15ef710585d30e134ece7ba78f5ed43377fae43a92，sourceCommit/sourceRevision 与 a32a3f05 一致；内嵌 Linux ARM64 后端 SHA-256 bb65b2fac5c2fbbf714f99646e9d6742f9655de1f2051986bd5474c49ed333a6，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。修复内容：首次发送成功后使用 replace 同步草稿会话路由，保留消息列表返回栈；返回处理按 URI path 识别聊天页，避免 conversationId 查询参数导致判定失效。手机端路由回归测试 16 项、Release Vital lint、无障碍 lint 和 assembleRelease 通过；正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 与组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk 哈希与本地制品一致，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1101，Runtime state=READY。真机完成“新建对话→首次发送→系统返回”验证：返回后 MainActivity 保持前台并回到消息列表，不再直接退出桌面，日志无 FATAL EXCEPTION。截图和验证记录 artifacts/android-chat-route-20261031/device-returned-to-messages.png 与 android-verification.json。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T21:13:37+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.19，versionCode 25，源码基线 58815cc15b6c61588d4c63e4feaa258802074bbc，提交日志 fix(backend): preserve original errors without nil dereferences；发布说明提交 1e6be959bb484604de80b3829d5a05c764f652c7，上一已发布 Android 基线 b4828ae4249f4716c5f75003a69b6b5cbb881a38。APK artifacts/apk/amitia-26.2.0-beta.19-release-arm64-v8a-20261031-204005.apk，154549623 字节，SHA-256 418aaa8244c34535f7648e6325c03b8bddc08ac7c742fbfb16117909f0f2d90d；Runtime Package artifacts/android-beta19-rebuild-20261031/amitia-runtime-1.0.0-android-arm64.zip，131680065 字节，SHA-256 a660e57944261e57d6c846b60f0b8341d98a58bd0e87c9c0d69069eead984aba，sourceCommit/sourceRevision 与 58815cc1 一致；内嵌 Linux ARM64 后端 SHA-256 bbb6781feaed0114d8fa3481b9dc37366a1ec19e9a7e72c9dc3f18cf68f627f6，plugin-host SHA-256 54e33766cb0b9fc7807c2e194a223f93b0a7d6c254dab9615f4a04d3f4012ac2，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。Linux ARM64 后端、plugin-host、task-host 和 Runtime Package 均由同一份源码重新构建；Node 26.12.1、Qdrant 1.19.0、Ubuntu rootfs、libatomic1、libgomp1、llama.cpp 与 SurrealDB 输入哈希校验通过。构建门禁：go mod verify、go vet ./...、213 个宿主可运行差异包测试、6 个 Linux ARM64 专用包测试二进制编译校验、手机端 123 项定向测试、task-host 20 项测试、错误处理审计、Release Vital lint、无障碍 lint 和 assembleRelease 全部通过。正式证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime/sourceCommit 和组件哈希校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk SHA-256 与本地制品一致，versionCode 25 / versionName 26.2.0-beta.19，com.amitia.amitia_app.debug 不存在；冷启动 Status ok、LaunchState COLD、TotalTime 1203，运行时完成内置 1.0.0 修复后 state=READY，MainActivity 与 amitia-server 进程正常运行，日志无 FATAL EXCEPTION。手机 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.19；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。完整项目已刷新 freeze 清单并以最终源码重建 Core，Core 提交 101fd146、SHA-256 a2aeeaf2756ce1cacde9c962ad394f51a111db8831eda617c536fa0eab6d8cd0、Source Gate c9329beaeedfd9efcce77038f2866435cc263e173007a4902126d1eb451e99ff，完整重启后前端 HTTP 200、Core /api/health.ready=true、/readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed）；Qdrant 与 SurrealDB 健康接口 HTTP 200，QQ、QQ 个人号、微信插件健康接口均 HTTP 200，动态端口为前端 57931、Core 57936/18899、Qdrant 57933/57934、SurrealDB 57935、QQ 57964、QQ 个人号 6199、微信 57966，未使用 3000。截图和验证记录 artifacts/android-beta19-rebuild-20261031/device-after-runtime.png 与 android-verification.json。未上传服务器、未发布更新通道、未推送 Git。记录时间 2026-10-31T20:43:27+08:00。本记录提交不是下一次发布差异的源码目标提交。

- Android Release / QQ 个人号插件（本地构建、已安装、未发布）：版本 26.2.0-beta.18，versionCode 24；APK 与 Runtime 源码基线 d4bbc438d9635320dc2be4b74965337e626de539，插件源码基线 937414ad，提交日志 fix(qq-personal): restore Android desktop runtime。APK artifacts/android-channel-final-20261029/amitia-26.2.0-beta.18-release-arm64-v8a.apk，139053619 字节，SHA-256 1882131ce4dfa29cfb01e6f90ec376589dd0cc6983226988eb5e2a96eb9accc1；Runtime amitia-runtime-1.0.0-android-arm64.zip，116287809 字节，SHA-256 93479e2f9518526c055e174269c37d565743c6d9cc2a00ec109ec808018e0ada，sourceCommit 与 d4bbc438 一致；Linux ARM64 后端 SHA-256 1340ff411ec581e14e62dc0570a8c81d7c65d20381e09b388462b3fe21471136。QQ 个人号插件 0.2.57，1942362 字节，SHA-256 0bbffb25edfe9171ab410f3be5c32dc0509adbdf7e9bc6adc7fc4adcc9b9e8b7，treeHash 685f393f478d71f9deeb77e825308466e45d2e86e41c5eae4ce167ee79056cfe，运行 generation-6f4eeaa6-003d-4902-a817-3dffb5c05649。正式构建、apksigner v2、16 KB zipalign、单 arm64-v8a ABI、APK 内嵌 Runtime、设备包哈希与 Debug 包缺席检查通过；模拟器为 emulator-5554。插件测试 70 项、手机端相关测试 84 项通过；页面状态刷新实测 38 个样本、间隔 1468–1524 ms。已解决 PRoot 不实现同身份 setgid/setuid 导致 Xvfb 退出的问题，新增 Linux ARM64 声明式 native companion 兼容层；x11vnc 使用 -noshm 和读取后删除的 0600 密码文件；noVNC 地址修正为 /vnc.html。Xvfb、fluxbox、x11vnc、websockify、Linux QQ、SnowLuma 均已运行，插件健康接口、SnowLuma WebUI、noVNC 返回 HTTP 200，VNC 密码认证和 QQ 扫码登录画面读取通过。SnowLuma 当前仍要求用户本人接受 EULA/隐私说明；同意并重新检测 QQ 进程前，qqOnline 为 false，未执行真实 OneBot 收发或模型对话。截图和 final-verification.json 位于 artifacts/android-channel-final-20261029/。未上传服务器、未发布、未推送 Git。记录时间 2026-10-29T14:04:39+08:00。本记录提交不是下一次发布差异的目标源码提交。

- Desktop NSIS / Android Release（本地构建、未发布）：版本 26.2.0-beta.17，Android versionCode 23，源码基线 0c38b70fd8d32fdf539b4eb1b8b08179671e782c，提交日志 chore(release): refresh beta.17 renderer smoke source manifest，记录时间 2026-10-26T18:11:52+08:00。两端核心与手机 Runtime 均由最终基线重建，正式构建、定向测试和包校验通过；Windows 未安装，最终 APK 因设备断开未安装。完整项目已重启且 readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed）。5 个正式插件已重打包。详见 docs/releases/beta17-local-build-20261026.md 与 artifacts/release-26.2.0-beta.17/release-build-record.json。用户要求先不发布，未上传服务器、未发布 GitHub Release、未推送 Git。本记录不替换已发布平台基线，记录提交不是下一次差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.16，versionCode 22，源码基线 66eeb275959acfbe6c626614dc9e8f6b86eaf6c6，提交日志 feat(navigation): regroup profiles and add character world book hub；父基线 66e6962e9483144dbea35a4fe9d6b85cae175fb4。APK artifacts/android-character-hub-20261026/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-150722.apk，136379727 字节，SHA-256 97a18aecc869800863ccf6b03a4d0bbe1aff01637c31670990abf8e678dd3579；Runtime Package artifacts/android-character-hub-20261026/amitia-runtime-1.0.0-android-arm64.zip，SHA-256 8be7f315aea7e22b863ec77ec07b0bfd381b7aced80616aa424e7c521e5b55c0，sourceCommit 与基线一致，包内 Linux ARM64 后端 SHA-256 0df8375dbfc9931602750cb5b6ba823d92030dd83e6918e010f43917e8bdcc90、vcs.revision 与基线一致且 vcs.modified=false，plugin-host SHA-256 8c2b4fbf07f7b39d098611e42f3c33ca94ec3f36738c67c1bbca08549bc6eaa6、task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601、Qdrant SHA-256 d78155928882a6aa39cca6b79872e32d3902f7f0ae40999812f3481754d0ad09、SurrealDB SHA-256 2a363311330f4ac3595a4c348764c700a0c826abb4230c02b63e29822998df1f。改动内容：手机端删除记忆工具六卡片页，将记忆总览改为桌面式筛选页并清理错误路由层级；桌面端将用户画像从角色导航组迁到记忆导航组；手机端“我的”角色入口新增路径层级 /me/characters 的“角色管理/世界书”二级页，点击后分别进入现有角色与世界书页面。构建门禁：后端 go mod verify、go vet ./...、go build ./cmd/server 及变更包定向测试通过（cmd/server 使用 AMITIA_TEST_NODE 项目 Node）；QQ 渠道 40 项、微信渠道 22 项、task-host 20 项、手机端记忆与路由 70 项、前端 typecheck/build 通过，最终导航改动手机端 16 项、桌面端 7 项及手机端静态分析通过。Release Vital lint 与 assembleRelease 串行通过；正式签名证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b、apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 哈希和 sourceCommit 校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），设备内 base.apk 哈希与本地制品一致，versionCode 22 / versionName 26.2.0-beta.16，com.amitia.amitia_app.debug 不存在，冷启动 LaunchState COLD、TotalTime 1326，AmitiaRuntime state=READY。真机点击“我的 → 角色”先进入新二级页，页面显示“角色管理”和“世界书”，两入口分别进入角色列表与世界书页面，日志无 FATAL EXCEPTION；手机 /api/health ready=true、model=configured、version=26.2.0-beta.16，/readyz 为 degraded（3 ready、1 degraded、0 blocking、0 failed）。未发布、未推送 Git。记录时间 2026-10-26T15:10:00+08:00。本记录提交不是下一次差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.16，versionCode 22，源码基线 4cb3e6e287e2dd630d921327448a2a15fe8df7f6，提交日志 fix(mobile): disable predictive back and use 320ms transitions。APK artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-1025.apk，136353599 字节，SHA-256 03c4975af83bad8cdceb540912e94e3d4b77a121853dd870c4cebb69a8e94e42；Runtime SHA-256 2f0e9731d8e7390f2ba9d74c68460e066863c6f12309def3d4db220fbcecc92a，sourceCommit 与基线一致，包内后端 SHA-256 d321224810a8064522b24aa3d2785045d602987b53c014889f63a224cb0280a0、vcs.revision 与基线一致。本次仅改 Dart 与前端测试，Go 源码与基线无差异，Runtime Package 与手机端内嵌包逐字节一致。改动内容为记忆单轨收敛：物理删除 memory_service.dart 及四条旧 Riverpod 通道，listMemories/pageMemories/listMemoryCandidates 迁入 DeviceOwnedMemoryService，记忆首页、对话记忆上下文、数据备份计数全部切到设备归属通道；修复 memoryView=all 时拒绝同 owner 其他角色记忆的校验缺陷（手机端与桌面端 useOwnedMemoryManagement.ts 同步）；角色详情「记忆」页签改为 embedded 的 OwnedMemoryPage 并按角色作用域取数，根记忆页面 UI 未改。Release Vital lint 与 assembleRelease 串行通过，正式签名与原证书匹配、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 哈希通过；安装后 base.apk 哈希与本地制品一致，com.amitia.amitia_app.debug 不存在，冷启动 LaunchState COLD、TotalTime 1454，AmitiaRuntime state=READY。真机复核：角色「记忆」页签显示「查看范围」而非「全部记忆」，内置后端收到 memoryView=role%3Ab0f9fcbc… 且 HTTP 200；根记忆页导航与空态与改动前一致；手机 /api/health ready=true、model=configured、version=26.2.0-beta.16，/readyz 为 degraded（3 ready、1 degraded、0 blocking、0 failed）。测试：记忆与导航定向 39 项通过、5 项失败，失败集合已用 git stash 在 HEAD lib 上复跑确认完全一致（既有失败），新增单轨守卫用例与内嵌视图用例通过，桌面端 vitest 14 项与 vue-tsc 通过。详情 docs/releases/android-memory-single-track-20261026.md。未发布、未推送 Git。记录时间 2026-10-26T10:30:00+08:00。本记录提交不是下一次差异的目标源码提交。

- Android Release（本地构建、未安装、未发布）：版本 26.2.0-beta.16，versionCode 22，源码基线 7f110257bb0acb9d5065f89cc534c03060b64a90，提交日志 chore(release): refresh 26.2.0-beta.16 chat top bar sources。APK artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-005016.apk，136353071 字节，SHA-256 8ed6c04ab73323973e5adfa981ecf176c48a18ea9fcb0d722e86dffe01ac3642；Runtime SHA-256 7816dd951b1cfb629434d5ef78c1f4dc59257b5918179abd0414e0dcac81c5bb。本次仅改 Dart，Go 源码与基线无差异，Runtime Package 的 sourceCommit 与 HEAD 一致，复用上一轮同源码构建产物。改动内容为全站页面转场改为 QQ / iOS 式共享轴：新增 AmitiaSlidePage/AmitiaSlideRoute，被覆盖页按 secondaryAnimation 联动左移 30% 并叠加 12% 黑色 scrim，二级页由右侧全屏滑入并带左侧投影，左边缘 24dp 命中阈值的手势以顶层路由 controller 驱动交互式返回，根层级（shell 与五个底部 Tab）转场时长归零、仍参与被覆盖时的联动；route_transitions 只保留 amitiaSlidePage，全站调用点统一迁移，shell 由 builder 改为 pageBuilder，五个根 Tab 由 NoTransitionPage 改为 rootLevel 页面，两处 fullscreenDialog 的 MaterialPageRoute（编辑器、实时通话）保留。关键实现约束：route.settings 必须回传 Page 自身以通过 page-based Navigator 的 `!pageBased || route.settings is Page` 断言，buildPage 与 isRootLevel 从最新 settings 读取，否则 Tab 切换会因 route 内容不更新而丢失分支索引。Release Vital lint 与 assembleRelease 串行单 worker 通过，:app:compileFlutterBuildRelease 实际执行；正式签名与原证书匹配、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 哈希与 sourceCommit 均通过。定向测试 test/app 23 项、test/core/ui_runtime 36 项全部通过，test/features/chat 71 项通过、2 项既有失败（chat_message_render_pipeline_test 空扩展槽用例、chat_top_bar_source_test 紧凑尺寸断言），dart analyze 变更文件无问题。当前 macOS 主机未连接任何 Android 设备（adb devices 为空、USB 未枚举、mDNS 无线调试无服务、无可用 AVD），本轮未执行安装、冷启动与真机转场录制，不记录为真机验证通过；已用离屏渲染保存 push/pop 中间帧于 artifacts/android-pagetransition-20261026/。详情 docs/releases/android-page-transition-20261026.md。未发布、未推送 Git。记录时间 2026-10-26T01:00:00+08:00。本记录提交不是下一次差异的目标源码提交。

- Desktop NSIS（本地构建，未安装，已通过管理端发布）：版本 26.2.0-beta.15，源码基线提交 affb5ac4b6ecfce79778e59edc3fea2bdabd7a17，提交日志 chore(release): freeze 26.2.0-beta.15 sources。安装包 artifacts/release-26.2.0-beta.15/AmitiaSetup-26.2.0-beta.15-x64.exe，大小 276080686 字节，SHA-256 5d9fe50d734ef23891cb54a2016e1ad6682461d1ad8d5d1ffea0180993e3731b；blockmap SHA-256 ec2ce2c8c336deb0c30c4ca62102e39067d027b7489d83f17d9ae5b398e2db87；latest.yml SHA-256 f6b1f836a9de79758728f598ed3d4ef3241bf1ce083ee7617e9b81235cd6965b；AmitiaCore.exe SHA-256 e34e40f4112ea8d651166172e7195c1880f5f7c8a80c5b2e8102f005501dd41d，Source Gate 64f0d5f42ac0d3607e7b2d8367a9e3ebafa2fe4d58706be331afd760508fe6c7，Release Gate PASS。EXE、blockmap、latest.yml 完整，SHA-512、blockmap 解压解析、Windows amd64 核心、包内运行时、core 白名单和 generic 更新地址验证通过。已通过管理端接口发布到 stable 通道（release id 41），线上 latest.yml、安装包与 blockmap 可访问，线上版本、文件大小、SHA-512 及首中尾 Range 内容校验通过；当前环境为 macOS，未执行 Windows 覆盖安装。完整项目已重启，前端、Core、Qdrant、SurrealDB 和插件宿主正常，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed）。未推送 Git。记录时间 2026-10-23 23:31:05 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建，未安装，已通过管理端发布）：版本 26.2.0-beta.15，versionCode 21，源码基线提交 affb5ac4b6ecfce79778e59edc3fea2bdabd7a17，提交日志 chore(release): freeze 26.2.0-beta.15 sources；Runtime sourceCommit 与源码基线一致。安装包 artifacts/release-26.2.0-beta.15/amitia-26.2.0-beta.15-release-arm64-v8a-20261023-232429.apk，大小 136737203 字节，SHA-256 bde56d85b3231f31bbd39ae6f6da698d2ad0a858d17f48fef669b9ff6902f02c；Runtime Package SHA-256 155d5c02986b9fa4cd1b781d318f91f34cf7afcc3edf8ffba0f338f13e53800e，后端 SHA-256 cdf617962f725051599f01fe1e65346c9f9d067f9150dc9a4098a3593a637cb2，plugin-host SHA-256 58e7c7f226dac9b18ed57e44cd0a14184db546bc1177d1ca005a233a7991b6b1，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。已从同一份源码重新编译 Linux ARM64 后端、plugin-host、task-host 并新建 Runtime Package。手机端受影响模块 96 项定向测试、Release Vital lint、无障碍 lint、assembleRelease、正式证书匹配、apksigner、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 哈希和源码提交验证通过。已通过管理端接口发布到 stable 通道（release id 42），线上 stable.json、stable.json.sig 与 APK 可访问，versionCode、versionName、APK 大小、SHA-256、RSA SHA-256 签名和 APK 首中尾 Range 内容校验通过。当前没有连接 Android 真机且没有可用 AVD，未执行本次安装、冷启动和实际模型对话验证。完整项目已重启，前端、Core、Qdrant、SurrealDB 和插件宿主正常，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed）。未推送 Git。记录时间 2026-10-23 23:31:05 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.16，versionCode 22，源码基线 7f110257bb0acb9d5065f89cc534c03060b64a90，提交日志 chore(release): refresh 26.2.0-beta.16 chat top bar sources，本次对话页顶部栏改动即包含在该提交中。APK artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-001919.apk，136358699 字节，SHA-256 2544d64b83321153638caed38504e03f9d8c13555c882453351287d74f820cea；Runtime SHA-256 7816dd951b1cfb629434d5ef78c1f4dc59257b5918179abd0414e0dcac81c5bb，后端 6e90a4c50fb3a4639b22e1c6b6e743bdcf672ccd0ccbf525f93933b92f282b10，后端 vcs.revision 与基线一致（vcs.modified=true 仅因工作区存在未提交的 extension-kernel 报表文档）。已按同一份源码重新编译 Linux ARM64 后端、plugin-host、task-host 并重建 Runtime Package，sourceCommit 与基线一致；正式签名、原证书匹配、16 KB zipalign、单 ABI、内嵌 Runtime/sourceCommit、安装后 base.apk 哈希、Release Vital lint 与 assembleRelease 通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），复核 versionCode 22 / versionName 26.2.0-beta.16，com.amitia.amitia_app.debug 不存在；改动内容为对话详情页顶部返回/标题/更多行与系统状态栏改用与根页面一致的 surfacePrimary 背景，工具栏高度由 68dp 对齐为 50dp，并补充 AnnotatedRegion 状态栏样式、侧边栏浮层起点加上状态栏高度。真机像素采样：对话页与消息页顶部 y=0..299 均为 #FFFFFF、y≥300 为 #F7F8FA，状态栏深色图标，冷启动无 FATAL EXCEPTION。手机 health.ready=true、model=configured、readyz=degraded（3 ready、1 degraded、0 blocking、0 failed）。手机端对话模块与导航定向测试 78 项通过、2 项既有失败（chat_message_render_pipeline_test 空扩展槽用例、chat_top_bar_source_test 的 menuWidth 190 断言，已用未改动的 HEAD 文件复跑确认为既有失败，未做范围外改造），不记录为完整测试门禁通过；dart analyze 仅 8 条既有 info。本次为纯手机端改动，未改动 Go 源码逻辑，未重复运行后端包测试。详情 docs/releases/android-chat-header-20261026.md。未发布、未推送 Git。记录时间 2026-10-26T00:25:00+08:00。本记录提交不是下一次差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.16，versionCode 22，源码基线 4925e1653ce65bdeeea08c0e8b7bf9aa89e38d89，提交日志 chore(release): refresh extension kernel readiness report；本次功能改动已包含在父提交 a471b30251d09bf5209ba903e4916717061b4cd2（chore(release): freeze 26.2.0-beta.16 sources）。APK artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261025-235645.apk，160535851 字节，SHA-256 edf462629cee0a4db1e81d1bce4b0fd6afc9745f973634741b711831d0b81e72；Runtime SHA-256 69de59cbf7c2ea5d9a6e5aaf1163746b0f7e124b3d7b12275b3788447c8a514b。已重新编译后端、plugin-host、task-host 并生成 Runtime Package，后端 vcs.revision 与基线一致；正式签名、原证书匹配、16 KB zipalign、单 ABI、内嵌 Runtime/sourceCommit、安装后哈希、Release Vital lint 和 assembleRelease 通过。安装前已卸载 com.amitia.amitia_app.debug，覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），版本复核 versionCode 22 / versionName 26.2.0-beta.16，debug 包不存在；冷启动进入消息页，验证日志无 FATAL EXCEPTION，真机点击右上角加号弹出自定义下拉菜单，含「新对话」「新群聊」，与对话页三个点菜单同款；按用户反馈把消息页加号菜单宽度由 220dp 收窄为 140dp（两项短标签在 375dp 宽屏上占 58.7% 过宽，内容实占约 101.5dp，取 140dp 与 Material 3 最小 112dp 及移动端紧凑菜单 140–160dp 通行值一致），对话页三个点菜单因含 5 字标签保持 220dp 不变，真机实测菜单宽度 140dp。手机 health.ready=true，model=configured，readyz=degraded（3 ready、1 degraded、0 blocking、0 failed），未进行实际模型对话。手机端定向测试 7 项通过；仓库级守卫用例 built-in popup menu buttons are fully replaced 仍失败（余 devices_page.dart、chat_logs_page.dart、knowledge_page.dart、workbench_page.dart、amitia_rich_blocks.dart 共 5 处历史 PopupMenuButton，属既有失败、非本次引入，未做范围外改造），不记录为完整测试门禁通过。本次为纯手机端改动，未改动 Go 源码，未重复运行后端包测试。详情 docs/releases/android-popup-20261025.md。未发布、未推送 Git。记录时间 2026-10-26T00:00:00+08:00。本记录提交不是下一次差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.15，versionCode 21，源码基线 b15ba9f2189790a0da9670587f2ac3dbaafa6bc2，提交日志 chore(release): freeze beta.15 Android role avatar sources。APK artifacts/apk/amitia-26.2.0-beta.15-release-arm64-v8a-20261025-222815.apk，160535843 字节，SHA-256 f151042ea8e7ec3fd31d1734855decbe522ec10801bb12b26fcef47754b7fba8；Runtime SHA-256 04703f5582ba46255d139d38235099468524d75a22c4141c4297d5fbc4e9732a。已重新编译后端、plugin-host、task-host 并生成 Runtime Package，后端 vcs.revision 与基线一致；正式签名、原证书匹配、16 KB zipalign、单 ABI、内嵌 Runtime/sourceCommit、安装后哈希、Release Vital lint、无障碍 lint 和 assembleRelease 通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），二次冷启动进入消息页，标题首字头像已移除，当前对话显示默认角色图标；图片头像、角色关联、停用角色、切换及更新由定向测试验证。手机 health.ready=true，readyz=degraded（3 ready、1 degraded、0 blocking、0 failed）。手机端 57 项、表情接口 61 项通过；go mod verify、go vet、编译及网页 typecheck/build 通过。相对最近本地安装基线无 Go 源码新增变化，不重复跑后端包测试；此前未改动模块失败用例未重测，不记录为全项目测试通过。按纯前端修改规则未重启本地完整服务，现有服务 readyz=ready（6 ready）。详情 docs/releases/android-avatar-20261025.md。未发布、未推送 Git。记录时间 2026-10-25T22:31:33+08:00。本记录提交不是下一次差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.15，versionCode 21，源码基线 be597440a35f980ac442d4164a8985144d49ca0d，提交日志 chore(release): freeze beta.15 Android sources with borderless search。APK artifacts/apk/amitia-26.2.0-beta.15-release-arm64-v8a-20261025-214805.apk，160538831 字节，SHA-256 2a7bd0e313c60e3ec7d8dc70b58e85a8c3885ddcc471de409b3eed41a0689fa5；Runtime SHA-256 b3490d2cc1aebeff99c31de025e87d6243aa0258b8acd33c47fc73af05ef39af。已重建后端、plugin-host、task-host 和 Runtime Package；正式签名、证书匹配、16 KB zipalign、单 ABI、内嵌 Runtime/sourceCommit、安装后哈希、Release Vital lint、无障碍 lint 和 assembleRelease 通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），二次冷启动进入消息页，搜索框无边框、无按压高亮，搜索跳转验证通过。health.ready=true；手机 readyz=degraded（3 ready、1 degraded、0 blocking、0 failed），模型已配置，未进行实际对话。手机定向测试 263 项通过、10 项失败；后端 56 包中 45 包通过、3 包无测试、8 包失败（35 个顶层用例失败）；任务宿主 16 项、QQ 渠道 25 项、表情接口 51 项通过，不记录为完整测试门禁通过。详情 docs/releases/android-search-20261025.md。完整项目重建核心并重启后 readyz=ready（6 ready）。未发布、未推送 Git。记录时间 2026-10-25T21:53:56+08:00。本记录提交不是下一次差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.15，versionCode 21，源码基线 3413979de7b6ab65edba12782a0ed7e76a1ab539，冻结提交 80435f1。APK artifacts/apk/amitia-26.2.0-beta.15-release-arm64-v8a-20261025-133835.apk，159818555 字节，SHA-256 4a7a4e626ccb7a5249bb2169445ae171e31f0034db032571cf4396c109db4984；Runtime SHA-256 4b72afd4625c7c0d743a7b3c60e3400c30d901d89057585fa7604ff2d4075a1f。重新构建后端、plugin-host、task-host 和 Runtime Package。正式签名、16 KB zipalign、单 ABI、内嵌 Runtime/sourceCommit、安装后哈希、Release lint 和 assembleRelease 通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），二次冷启动进入聊天主页，health.ready=true，手机 readyz=degraded（3 ready、1 degraded、0 blocking、0 failed），模型未配置，未测试实际对话。153 项手机端与 16 项任务宿主测试通过；后端 7 个包共 26 个顶层用例失败，详见 docs/releases/android-local-20261025.md，不记录为完整测试门禁通过。完整项目重启后 readyz=ready（6 ready）。未发布、未推送 Git。记录时间 2026-10-25T13:40:54+08:00。记录提交不是下一次差异的目标源码提交。

- Desktop NSIS（本地构建，未安装，已通过管理端发布）：版本 26.2.0-beta.14，源码基线提交 8a87e069b24a2f38e9aa122c830804cdd9a2165b，提交日志 chore(release): freeze 26.2.0-beta.14 sources，发布记录提交 2e46c14e50428ff3f45c292aad2d50c2e223fc1d。安装包 artifacts/release-26.2.0-beta.14/AmitiaSetup-26.2.0-beta.14-x64.exe，大小 275919914 字节，SHA-256 5ace97b1cb10ee558507fcca76648a5e7c83ed0d1cbfa13793b6071aa51006dd；blockmap SHA-256 2fabb6a47fc934f4b094d76710067c313082df07bf9a59f4219ac239cd45a3d2；AmitiaCore.exe SHA-256 8c440cc514f2132ddb824176364a14e2efc579fec2c0ad162d92d9783abcede3，Source Gate 98386ed6a460e5c56e03d4e451ab8f611ae0a0832163bebd5dac804ba9557e9f，Release Gate PASS。EXE、blockmap、latest.yml 完整，SHA-512、blockmap、Windows amd64 核心、包内运行时、core 白名单和 generic 更新地址验证通过。已通过管理端接口发布到 stable 通道（release id 39），线上 latest.yml、安装包与 blockmap 可访问，线上版本、大小和 SHA-512 与本地一致；当前环境为 macOS，未执行 Windows 覆盖安装。未推送 Git。记录时间 2026-10-23 11:39:10 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建，已安装，已通过管理端发布）：版本 26.2.0-beta.14，versionCode 20，源码基线提交 8a87e069b24a2f38e9aa122c830804cdd9a2165b，提交日志 chore(release): freeze 26.2.0-beta.14 sources；发布记录提交与 Runtime sourceCommit 为 2e46c14e50428ff3f45c292aad2d50c2e223fc1d。安装包 artifacts/release-26.2.0-beta.14/amitia-26.2.0-beta.14-release-arm64-v8a-20261023-113143.apk，大小 136611031 字节，SHA-256 3ff59d84fc6fb7d8984a902a3e7744e9a60d9ac88145e670f1e887cfc0b3c90c；Runtime Package SHA-256 497cf4adfeb9378234cec84bdf9f7e36d04f5cb909f9d39b746a2305a350581f，后端 SHA-256 d8f3760b37d769d544689a95315f6b117071b71aa39ec1eb4c4e9f3b411c5812，plugin-host SHA-256 58e7c7f226dac9b18ed57e44cd0a14184db546bc1177d1ca005a233a7991b6b1，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601。已从同一份源码重新编译 Linux ARM64 后端、plugin-host、task-host 和 Runtime Package。APK 已通过正式签名、apksigner、16 KB zipalign、arm64 单 ABI、Release Vital lint、无障碍 lint 和内嵌 Runtime 哈希验证，并覆盖安装到 22081212C（8864cd52）；安装后 APK 哈希与本地制品一致，内置后端 /api/health HTTP 200、ready=true、version=26.2.0-beta.14，/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed），实际模型对话正常。已通过管理端接口发布到 stable 通道（release id 40），线上 stable.json、stable.json.sig 与 APK 可访问，版本、大小、SHA-256 和 RSA 签名验证通过。仅保留正式包。未推送 Git。记录时间 2026-10-23 11:39:10 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Desktop macOS（本地构建、已安装、未发布）：版本 26.2.0-beta.13，源码基线提交 f22f664，提交日志 fix(runtime): cancel gin SSE streams during shutdown。应用安装于 /Applications/Amitia.app，macOS 核心 SHA-256 为 7ac5ada45693dcfa6f288c6e15598357f9f6be4d30e778a67d86a117920bb000，Source Gate 为 579e30d33f68699c7b49dfb6dd3c43410516c7341fb05f687ebff65383a3c0b2，完整 Release Gate、包内核心和运行时资源校验 PASS。安装后启动 /api/health、/livez、/readyz 均返回 200，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed），Qdrant 与 SurrealDB 正常监听。在存在活跃 proactive SSE 和 Desktop Pet WebSocket 时执行退出，应用约 1 秒结束，核心、Qdrant、SurrealDB 全部退出，SSE 请求正常结束，日志无 context deadline exceeded 或 app-update.yml ENOENT；随后重新启动并保持 ready。未执行 Windows NSIS 覆盖安装；未发布、未推送 Git。记录时间 2026-10-23 10:54:00 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.13，versionCode 19，源码基线提交 9bef890，提交日志 fix(android): guard accessibility actions by API level。安装包 artifacts/android-production-20261023-shutdown-fix/amitia-26.2.0-beta.13-release-arm64-v8a.apk，大小 136611207 字节，SHA-256 380df3f39d2b73eebf451d44f24e49ef4af4b56ed1617b1059d394fcb3d321dd；Runtime Package SHA-256 f8a8d7a5da149bda7568893b9660cbdc2ca430cae80018837f692f290cd3c2ec，后端 SHA-256 676a6ed4235f812d025746ef469d2215175f864f9e822e4eb2974dde885c6fb6，sourceCommit 与源码基线一致。已从同一份后端源码重新编译 Linux ARM64 后端、plugin-host 和 task-host 并重新生成 Runtime Package。Release APK 已通过正式签名、apksigner、16 KB zipalign、单 ABI、Release Vital lint、无障碍模块 lint 和内嵌 Runtime 哈希验证，并成功覆盖安装到 22081212C（8864cd52），安装后 APK 哈希与本地制品一致。首次升级启动完成运行时替换后，第二次冷启动进入聊天主页；内置 /api/health HTTP 200、ready=true、model=configured、version=26.2.0-beta.13，/readyz 为 degraded（3 ready、1 degraded、0 blocking、0 failed）。真机实际发送消息后模型正常返回，日志无崩溃。仅保留正式包，Debug/测试辅助包不存在。未发布、未推送 Git。记录时间 2026-10-23 10:54:00 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Desktop NSIS（本地构建，未安装，已通过管理端发布）：版本 26.2.0-beta.13，源码基线提交 dab31cbc46c7c7f93b86bbcb4a796fd6ddcccad9，提交日志 fix(package): refresh recovery state after lease acquisition；发布冻结提交 4c7a224，构建门禁修复提交 eb22892 与 dab31cb。安装包 artifacts/release-26.2.0-beta.13/AmitiaSetup-26.2.0-beta.13-x64.exe，大小 275914182 字节，SHA-256 2b6db2a72b1707a3c8c112f031907fd557f54f1eb6a9165e0216a71a1b917242；AmitiaCore.exe SHA-256 a4bb35eed87d90ef73d2b587c848507873adb9f280fe57191c2dc45ead9364b0，Source Gate 501c7c636187fd76c4c26cccee4b7f0e8d53ebbe37f8c117dbad69591c4952dd，Release Gate PASS。EXE、blockmap、latest.yml 完整，SHA-512、blockmap 解压解析、Windows amd64 核心、包内运行时哈希、core 白名单和 generic 更新地址验证通过。当前环境为 macOS，未执行 Windows 覆盖安装。已通过管理端发布到 stable 通道，线上 latest.yml、安装包与 blockmap 均通过 HTTPS 返回 200，线上版本、大小和 SHA-512 与本地一致；未推送 Git。记录时间 2026-10-23 08:39:45 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建，未安装，已通过管理端发布）：版本 26.2.0-beta.13，versionCode 19，源码基线提交 dab31cbc46c7c7f93b86bbcb4a796fd6ddcccad9，提交日志 fix(package): refresh recovery state after lease acquisition；发布冻结提交 4c7a224，构建门禁修复提交 eb22892 与 dab31cb。安装包 artifacts/release-26.2.0-beta.13/amitia-26.2.0-beta.13-release-arm64-v8a-20261023-082245.apk，大小 136603431 字节，SHA-256 8666a2c0d4d5309f38ff460a5f598ecead7fb2e4048fbaae0c092fbf135691ea；Runtime Package SHA-256 3e0fb7e3153bdd1ff2af603be7b30acc600650d2a1728d6306da632f7b537fec，后端 SHA-256 69ab257e671a9daea931aee3b836e7349e0c93c373533e77a15695e0223ecf3c，plugin-host SHA-256 58e7c7f226dac9b18ed57e44cd0a14184db546bc1177d1ca005a233a7991b6b1，task-host SHA-256 0db8f296b1d8f9df0d7ab6b66c30fc111a8c78ac215f864225fad9e05984c601，sourceCommit 与源码基线一致。已从同一份源码重新编译 Linux ARM64 后端、plugin-host 和 task-host 并重新生成 Runtime Package。手机端 5 个定向测试文件共 19 项、Release lint、assembleRelease、正式签名匹配、apksigner、zipalign 16 KB、单 ABI 与内嵌 Runtime/后端/宿主哈希验证通过。当前未连接 Android 真机，未执行本次安装与冷启动验证。已通过管理端发布到 stable 通道，线上 stable.json、stable.json.sig 与 APK 均通过 HTTPS 返回 200，versionCode、versionName、APK 大小和 SHA-256 与本地一致，stable.json RSA SHA-256 签名验证通过；未推送 Git。记录时间 2026-10-23 08:39:45 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建，已安装、未发布）：版本 26.2.0-beta.12，versionCode 18，源码基线提交 e7e13c65899c438c31d5095c822d7d88ddba4673，提交日志 chore(release): freeze beta.12 Android sources；安装包 artifacts/apk/amitia-26.2.0-beta.12-release-arm64-v8a-20261022-215405.apk，大小 161093447 字节，SHA-256 d80dd12fdf0f17771f9843f32bac9541c6b62e0b3ee0cbff44cfe1fb4237cc3c；Runtime Package SHA-256 a0697961aac8d931522b6f0e5b8dbd266325729e1f47a158141976b091f5e7ae，后端 SHA-256 eaf7a6b3e4029e9b2948fd25535431524eb21900e59021fa4c12638ca559cd2b，sourceCommit 与源码基线一致。已从同一份源码重新编译 Linux ARM64 后端、plugin-host 和 task-host 并重新生成 Runtime Package。手机配置定向测试 3 项、Release lint、assembleRelease、正式签名匹配、apksigner、zipalign、单 ABI 和内嵌 Runtime/后端/宿主哈希验证通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），安装后 APK 哈希与本次制品一致，复核版本为 .12（18），Debug 包不存在。升级自动更新内嵌运行时，第二次冷启动后进入聊天主页且 MainActivity 在前台；内置后端 /api/health HTTP 200、ready=true、version=26.2.0-beta.12；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed），不记录为全环境完全健康。模型尚未配置，未进行实际模型聊天验证。本地完整项目保持运行且 /readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed）。归档 artifacts/release-26.2.0-beta.12/；暂不发布，未推送 Git。记录时间 2026-10-22 21:59:14 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Desktop NSIS（本地构建，未安装、未发布）：版本 26.2.0-beta.12，源码基线提交 7306794cd3729d57cf63000c940c06f9515a5b20，提交日志 test(release): align cleanup gate with supported stores；安装包 desktop/release/AmitiaSetup-26.2.0-beta.12-x64.exe，大小 320356200 字节，SHA-256 0baf4772b0ac9162d3b238cee7b43735a53fa806a5371e0f70938e3c0c9dc49a；AmitiaCore.exe SHA-256 390d31ddb5963473a3ad6a11220456680fdf01dced36369f635ede022b370e6b，Source Gate 55b45780e9232e76985a7c5424e2cf9bd05b000c32a55eadcae0a5e742a93c41，Release Gate PASS。EXE、blockmap、latest.yml 完整，SHA-512、blockmap 解压解析、Windows amd64 核心及包内哈希、core 白名单与 generic 更新地址验证通过。5 个改动包定向测试、编译门禁与桌面窗口 4 项测试通过；完整本地项目已重启，前端、核心、Qdrant、SurrealDB 均 HTTP 200，readyz 为 ready（6 ready、0 degraded、0 blocking、0 failed），当前无已安装渠道包。当前环境为 macOS，未执行 Windows 覆盖安装；已在 Finder 展示 EXE。按用户最新指令暂不发布，未推送 Git；手机端尚未构建，源码版本已同步为 26.2.0-beta.12+18。归档 artifacts/release-26.2.0-beta.12/。记录时间 2026-10-22 21:41:56 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Android Release（本地构建，未安装、未发布）：版本 26.2.0-beta.11，versionCode 17，源码基线提交 f9622bcb8b621a26e66968e9748a1b4883fc4641，提交日志 fix(model): connect scenario routing across clients and core；安装包 artifacts/apk/amitia-26.2.0-beta.11-release-arm64-v8a-20261021-180011.apk，大小 161091839 字节，SHA-256 743af721f2213e1d6bafc0d6a81ffeaee8a07a2dc794bb7204bc02c22b390120；Runtime Package SHA-256 a7011fd2f8ea7dd2976cd38385d996d6fac5133fe3cc83b4e21a975053419529，sourceCommit 与源码基线一致。Release lint、assembleRelease、正式证书匹配、apksigner、zipalign、单 ABI 与内嵌 Runtime/后端/宿主哈希验证通过。未连接真机，未安装或冷启动验证；完整项目已重启，health.ready=true，readyz HTTP 200 但整体为 degraded（5 ready、1 degraded、0 blocking、0 failed），不记录为全环境健康。未发布、未推送 Git。记录时间 2026-10-21 18:01:49 +0800。本记录提交不是下一次发布差异的目标源码提交。

- Desktop NSIS：版本 26.2.0-beta.11，源码基线提交 5a81739f504aeeb4656f2227f3c0352e706ebe4b，提交日志 chore(release): refresh beta.11 freeze manifest，安装包 AmitiaSetup-26.2.0-beta.11-x64.exe，大小 320358989 字节，SHA-256 a94b346d46461b191f2282cdb32c0795291c72b1d1ff971c3ce5ebfffa929c14，记录时间 2026-10-21 16:41:46 +08:00，构建输出目录 desktop/release，通过管理端接口发布到 stable 通道（release id 33），latest.yml、安装包与 blockmap 已通过 HTTPS 返回 200；latest.yml 中的 SHA-512 与实际安装包一致，AmitiaCore.exe SHA-256 为 f99e7af8ad7d790e86cd42805100c6b9971b21bf39309abc42ab374aaf58b9ea，Source Gate 为 c8c805f981afdfe7f2d594af87d3e74471102a87e33c95f85f41a1b753d84802，Release Gate 为 PASS；已在 macOS 安装 Rosetta 2 以运行 NSIS，当前环境未执行 Windows 覆盖安装。

- Android Release：版本 26.2.0-beta.11，versionCode 17，源码基线提交 5a81739f504aeeb4656f2227f3c0352e706ebe4b，提交日志 chore(release): refresh beta.11 freeze manifest，安装包 amitia-26.2.0-beta.11-release-arm64-v8a-20261021-163818.apk，大小 136595551 字节，SHA-256 fbcd8c5b0b6719ce1e19bb58e06f134edfbd82e257eb7e151660f89ce2f41d71，记录时间 2026-10-21 16:43:05 +08:00，通过管理端接口发布到 stable 通道（release id 34），stable.json、stable.json.sig 与 APK 已通过 HTTPS 返回 200，线上 stable.json 的 versionCode、versionName、APK 大小和 SHA-256 均与本地制品一致，stable.json RSA SHA-256 签名验证通过；Runtime Package SHA-256 为 7029fec340e7c7f00c62cc3df129e967251fc130e5a5bc6ecd6cb5c13a43a3f7，sourceCommit 与源码基线一致；已通过 Release lint、Gradle assembleRelease、正式证书匹配、apksigner、zipalign 与 arm64 单 ABI 检查，APK 内嵌 Runtime Package 哈希与构建输入一致；当前未连接 Android 真机，未执行安装与冷启动验证。

- Desktop NSIS：版本 26.2.0-beta.10，源码基线提交 0127a8a369b198b5a4c6fa4d22f4ae34f6c1463b，提交日志 chore(release): freeze repaired Android beta.10 sources，安装包 AmitiaSetup-26.2.0-beta.10-x64.exe，大小 276294879 字节，SHA-256 0ee19eb6d2ac5dcccd9b245c62625298272d3e5b44c927d723160b1355679c7c，记录时间 2026-10-20 22:11:51 +08:00，构建输出目录 desktop/release，通过管理端接口发布到 stable 通道（release id 30），latest.yml、安装包与 blockmap 已通过 HTTPS 返回 200；发布清单中的 SHA-512 与实际安装包一致，blockmap 解析通过，AmitiaCore.exe SHA-256 为 6ae1aef642563a01a45be7fdc0982c7169dea6c94b00410a6a5d5cbe78a514a3，Release Gate 为 PASS；当前运行环境为 macOS，未执行 Windows NSIS 覆盖安装。

- Android Release：版本 26.2.0-beta.10，versionCode 16，源码基线提交 0127a8a369b198b5a4c6fa4d22f4ae34f6c1463b，提交日志 chore(release): freeze repaired Android beta.10 sources，安装包 amitia-26.2.0-beta.10-release-arm64-v8a-20261020-215926.apk，大小 161449335 字节，SHA-256 ed5f766330325524683029ec28d8c718f72554c908d3fb86b049bedc2a993802，记录时间 2026-10-20 22:11:51 +08:00，通过管理端接口发布到 stable 通道（release id 31），stable.json、stable.json.sig 与 APK 已通过 HTTPS 返回 200，线上 stable.json、签名文件与本地制品哈希一致；Runtime Package SHA-256 为 7b2479a34d5011a0ebae02f1a663c28bad1bdf48f6481979a782e8313037bc4a，sourceCommit 与源码基线一致；已通过 Release lint、Gradle assembleRelease、正式证书匹配、apksigner、zipalign 与 arm64 单 ABI 检查；已通过 root 包管理器成功安装到 22081212C 真机，复核 versionCode 16、versionName 26.2.0-beta.10、Debug 包不存在，应用冷启动后 MainActivity 位于前台且进入主聊天页，内置运行时接口正常返回。

- Android Release（本地构建，未发布）：版本 26.2.0-beta.9，versionCode 15，源码基线提交 f77f07d342e8b711baf3e195db9872c618f19141，提交日志 fix(chat): publish owner-confirmed channel append status，安装包 artifacts/apk/amitia-26.2.0-beta.9-release-arm64-v8a-20261020-151752.apk，大小 135266990 字节，SHA-256 39dd8c8a952d3dffc4935c7d507ce4992e5ff7e42c12e4222f5983c4a255d4c8，记录时间 2026-10-20 15:18 +08:00；已通过 Release lint、Gradle assembleRelease、正式证书匹配、apksigner、zipalign 与 arm64 单 ABI 检查；Runtime Package SHA-256 为 9267a2be445e5ffe13df302fc18cc110f213144f4d3bfbe6f3a0c70084ba7916，sourceCommit 与源码基线一致。未检测到已连接真机，未安装、未发布、未推送 Git。此发布记录提交不是下一轮源码差异的目标提交。

- Desktop NSIS：版本 26.2.0-beta.9，源码基线提交 52ae7fb791818b79c78d4dca26f6c75d1ccc204e（版本号提交 ecc2f1cfc，冻结清单提交 52ae7fb79），提交日志 chore(release): refresh 26.2.0-beta.9 freeze manifest，安装包 AmitiaSetup-26.2.0-beta.9-x64.exe，大小 275793862 字节，SHA-256 8aa1327ab5f052488d897ba9e15d30334b2e795de24e1d55efa71a4022ed70d9，记录时间 2026-10-19 23:12:25 +08:00，构建输出目录 desktop/release，通过管理端接口发布到 stable 通道（release id 27），latest.yml、安装包与 blockmap 已通过 HTTPS 返回 200；已在 E:\新455454文件夹\Amitia 覆盖安装，安装后核心 SHA-256 为 b9e429ec9b9129e51c49ade3366e7bfe1bd91d83973163d99d2bd05488a1957c，/readyz 为 ready，/api/health 返回 26.2.0-beta.9，Qdrant 与 SurrealDB 健康接口均返回 200；本次同时生成微信渠道插件 1.0.19、QQ 渠道插件 1.0.17 到根目录 plugin
- Android Release：版本 26.2.0-beta.9，versionCode 15，源码基线提交 52ae7fb791818b79c78d4dca26f6c75d1ccc204e，提交日志 chore(release): refresh 26.2.0-beta.9 freeze manifest，安装包 amitia-26.2.0-beta.9-release-arm64-v8a-20261019-231050.apk，大小 135347554 字节，SHA-256 09c4dc334875a29bf012ee74940d2fb8c744d940490f8142d834baa57148db2c，记录时间 2026-10-19 23:13:53 +08:00，stable（release id 28）与 beta（release id 29）两个通道均已发布，stable.json、beta.json、签名文件与 APK 已通过 HTTPS 返回 200；Runtime Package SHA-256 为 e7608bc47049398a829e636ceffddabc2446074c43881ad19ba07548eecb85b3，sourceCommit 与本次源码基线一致；本次构建时无已连接真机，未执行覆盖安装验证

- Android Release（本地构建、已安装、未发布）：版本 26.2.0-beta.16，versionCode 22，源码基线 4cb3e6e287e2dd630d921327448a2a15fe8df7f6，提交日志 fix(mobile): disable predictive back and use 320ms transitions；本轮工作区仅改动后端 Edit 归属校验、手机端统一通道作用域归一化与记忆页 UI。APK artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-1418.apk，126368975 字节，SHA-256 729d416290b360c97de37eea1cd4dee432f377e1c03ff1b56dfa64f6e83e392f；Runtime Package SHA-256 f2156b38f616a9888e6726de6857fa50b471ebe76ae6cec6d89659f159d2a7f8，sourceCommit 与基线一致，包内 Linux ARM64 后端 SHA-256 0069054737dcc5ab2f13cd6b8bcd8888d99273ba43fa3e48dea122614106b1eb。已从同一份源码重新编译后端并重建 Runtime Package，validate-runtime-package.py 通过；正式签名（证书 SHA-256 119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b）、apksigner v2、zipalign、单 arm64-v8a ABI、内嵌 Runtime 哈希与版本 22/26.2.0-beta.16 校验通过。已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），com.amitia.amitia_app.debug 不存在，冷启动 LaunchState COLD、TotalTime 1645。本轮修复画像删除失败：客户端 data()/edit() 统一按 memoryRecordScope 归一化记录作用域、归属戳比较忽略仅由视图派生的 memoryViewAll、缓存与调用方视图不一致时按同一视图重读，后端 Edit 校验 expectedExecutionScope 时同样忽略 MemoryViewAll；真机新增画像保存后删除，内置后端 POST /api/device-mesh/v1/business/resources/edit 返回 200，画像列表回到「暂无画像」。定向测试：后端 devicemesh business 编辑/删除用例通过，手机端 7 个记忆相关文件 36 项全部通过，新增 device_owned_memory_edit_scope_test.dart 覆盖归属归一化。完整项目已重建本地 Core（Source Gate 9029a93f66bf56e9541f97dd13255fc735f2513617f31ed43361415bffff4f95）并重启，/api/health ready=true、version=26.2.0-beta.16、model=configured；/readyz 需要本地凭据，主机侧未复核。发现但未修复：画像与派生投影删除不成对（删画像留源记忆、删源记忆不停用画像），需后端按 source_id 双向级联，已作为下一项方案上报。真机遗留两条测试记忆（Hello、one）尚未清除。未发布、未推送 Git。记录时间 2026-10-26T14:25:00+08:00。本记录提交不是下一次差异的目标源码提交。

本次发布归档：artifacts/release-26.2.0-beta.16/，包含桌面 EXE、blockmap、Android APK、Runtime Package、Linux ARM64 后端、SurrealDB、changelog.md、changelog.html、changelog.png、latest.yml、release-gate.json、admin-publish-result.json、release-build-record.json 与 SHA256SUMS.txt。

assisted 安装器的更新行为（已实现并验证）：

- 首次安装保持向导（选择用户与安装目录）；后续更新静默覆盖安装到原目录并自动重启，由 `desktop/nsis/installer.nsh` 提供：
  - `customCheckAppRunning`：只精确匹配 `$INSTDIR\` 下的进程并静默结束。electron-builder 默认用 `Path.StartsWith($INSTDIR)`，会把数据目录 `%LOCALAPPDATA%\Programs\AmitiaData` 也判为运行中，导致升级永远提示"Amitia 无法关闭"。
  - `customUnInstallCheck` / `customUnInstallCheckCurrentUser`：旧卸载器返回非 0 时不再中止安装，由新 payload 原地覆盖。
- 验证方式：`AmitiaSetup-<version>-x64.exe /S --updated --force-run`（electron-updater 的原始参数）覆盖已安装版本，安装器约 60 秒完成、`app.asar` 更新、应用自动重启且 `/readyz` 为 ready。

更新通道说明：

- 手机端默认通道是 `stable`（`AMITIA_ANDROID_UPDATE_CHANNEL` 默认值），发布必须同时写 `stable`，否则正式通道客户端会显示"更新服务暂未配置可用版本"。
- 本机同时存在 `C:\WINDOWS\system32\adb.exe`（1.0.40）与 SDK `platform-tools\adb.exe`（1.0.41），两者会互相 kill server 并导致设备掉线、`input tap` 失效；所有 adb 操作必须使用 SDK 的完整路径。

管理端发布接口（当前实际使用的发布通道）：

- 管理端账号：untrammelled
- 管理端密码：925011Mc
- `desktop/scripts/.publish-config.json` 中的 FTP 账号仍是模板占位符 `REPLACE_WITH_FTP_USER`，FTP 上传会返回 `530 Access denied`；发布前必须先把真实 FTP 账号写入该文件，否则改用管理端接口发布。
- 管理端接口发布流程（同源 HTTPS，登录 cookie + `X-CSRF-Token`）：
  1. `POST /api/v1/admin/auth/login`，body 为 `{username,password}`，从响应 `data.csrfToken` 取 CSRF
  2. `POST /api/v1/admin/releases` 创建草稿，桌面端 `product=desktop channel=stable`，手机端 `product=android channel=<stable|beta|alpha> versionCode=<n> abi=arm64-v8a`
  3. `POST /api/v1/admin/releases/<id>/artifacts` 上传制品，multipart 字段 `kind`（桌面端 `installer`/`blockmap`，手机端 `apk`）与 `file`
  4. `POST /api/v1/admin/releases/<id>/validate` 校验，再 `POST /api/v1/admin/releases/<id>/publish` 发布
- 发布结果：桌面端写入 `latest.yml` 与安装包；手机端写入 `releases/<versionCode>/<apk>` 并刷新 `stable.json`/`beta.json`/`alpha.json` 与 `.sig`，通道为累进式（stable 只服务 stable，beta 服务 stable+beta，alpha 服务全部）

发布配置：

- 更新服务器：https://amitia.untrammelled.top/amitia
- FTP配置文件：desktop/scripts/.publish-config.json（含密码，已gitignore，禁止上传git）
- 配置模板：desktop/scripts/.publish-config.example.json
- electron-builder.yml中publish.provider为generic，url为https://amitia.untrammelled.top/amitia
- update-manager.ts中RELEASES_URL指向同一地址

发布命令：

- 构建完成后，在desktop目录执行 pnpm upload 自动上传发布
- 脚本会自动上传 latest.yml、AmitiaSetup-${version}-x64.exe、AmitiaSetup-${version}-x64.exe.blockmap 三个文件
- 上传完成后自动验证 https://amitia.untrammelled.top/amitia/latest.yml 是否可访问

SSH发布流程：

- 服务器地址：8.156.72.48
- 服务器最高控制权 SSH 别名：`amitia-root`，对应用户 `root`，拥有服务器完整控制权限，不是受限部署账号
- 服务器最高控制权私钥：`/Users/untrammelled/.ssh/codex_ashareking_ed25519_20261023`，权限必须保持 `600`
- 服务器最高控制权公钥：`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILg1LKnxpOH7ZfuXZuHQRHRTC7CIXh+7eDzbygK5F8sJ codexdeploy@amitia-20261023`
- 服务器最高控制权公钥指纹：`SHA256:ckQvF9h7VeUtfXDFLI5Q0brkpD5msthHLuJfHdimM9g`
- 私钥、公钥和 `~/.ssh/config` 禁止上传项目或 Git；服务器首次信任时只能将上述公钥写入 `/root/.ssh/authorized_keys`，并使用目录 `700`、文件 `600`
- 部署用户：codexdeploy
- 专用私钥：C:\Users\Untrammelled\.ssh\codex_ashareking_ed25519_20260912
- Windows OpenSSH 可能存在兼容问题，上传使用 Paramiko，连接时禁用 agent 和自动查找密钥
- 生产目录：/www/wwwroot/amitia.untrammelled.top/amitia
- codexdeploy 无生产目录写权限，先上传到 /tmp/amitia-<version>-<timestamp>
- 上传后使用 sha256sum 核对远端文件与本地文件哈希，再提供宝塔 root 终端替换命令
- 桌面端替换 latest.yml、AmitiaSetup-<version>-x64.exe 和 blockmap
- Android 端替换 android/beta.json、android/beta.json.sig，并将 APK 放入 android/releases/<versionCode>/
- 替换完成后删除旧安装包、旧 Android APK 和失效频道清单
- 发布验收必须通过 HTTPS 检查 latest.yml、beta.json、beta.json.sig、EXE 和 APK 均返回 200

发布前检查项：

- 构建产物必须完整（exe + blockmap + latest.yml 三件套）
- 服务器Nginx配置中 /amitia/ 路径必须配置正确的MIME类型和charset utf-8
- 宝塔安全组必须放行FTP 21端口和被动模式端口范围（39000-40000）

管理端（Amitia 更新管理）构建与部署流程：

上线位置：

- 站点根：/www/wwwroot/amitia.untrammelled.top（www:www 755），访问入口 https://amitia.untrammelled.top/
- 更新通道仍在同目录 amitia/ 子目录，桌面端 latest.yml、Android beta.json 由管理服务写入该目录
- 后端安装目录：/www/server/amitia-admin（admin-server 与 amitia-admin.env，env 必须 chmod 600）
- 服务：systemd 单元 amitia-admin.service，User=www，Restart=always，enabled
- 数据库：服务器 MySQL 5.7，库名与用户名均为 amitia_admin，初始化使用 deploy/mysql/amitia-admin-seed.sql
- 管理服务默认监听 127.0.0.1:18880（AMITIA_ADMIN_ADDR），开发代理 target 在 admin-system/vite.config.ts
- 管理前端请求走同源 /api/v1/admin，服务器上必须由 nginx 将 /api/ 反代到 127.0.0.1:18880，否则页面能开但无法登录

本地构建（全部使用项目内 Node 与固定 Go 路径）：

1. 前端：在 admin-system 目录用项目内 node 执行 vite build，产物为 admin-system/dist
2. 后端：GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o admin-server-linux-amd64 ./cmd/admin-server
3. 打包内容：dist/、admin-server、deploy/mysql/amitia-admin-seed.sql，并用 PowerShell Get-FileHash 生成 SHA256SUMS

上传与替换：

1. 用 codexdeploy 账号与专用私钥执行 ssh，在 /tmp 下创建 amitia-admin-<yyyyMMdd-HHmmss> 暂存目录（生产目录 codexdeploy 无写权限）
2. 用 scp -r 把 dist、admin-server、SQL、SHA256SUMS 上传到该暂存目录
3. 上传后必须在该目录执行 sha256sum -c SHA256SUMS，逐文件确认 OK 后再交付替换命令
4. 由用户在宝塔 root 终端执行替换：静态文件落到站点根、后端 install 到 /www/server/amitia-admin、写入 env 与 systemd 单元并 enable --now
5. 站点根目录放静态文件时禁止使用 cp -a <dir>/. ./（会把源目录权限 700 覆盖到站点根，导致 codexdeploy 无法读取且面板操作异常），用 cp -a <dir>/* ./ 或在复制后 chmod 755
6. 站点根存在宝塔受保护的 .user.ini（chattr +i），chown -R 会报 Operation not permitted，授权时只对本次新增文件与目录执行

nginx 要求：

- server 块内需要 location ^~ /api/ 反代到 127.0.0.1:18880（含 proxy_read_timeout 与 client_max_body_size 4096m，桌面安装包为 GB 级）
- 需要 location / 配 try_files $uri $uri/ /index.html，否则 /login、/settings 等前端直链返回 404
- 宝塔「伪静态」文件 /www/server/panel/vhost/rewrite/<域名>.conf 由 vhost 以 server 级 include 引入，其中的 rewrite 会先于 location 匹配执行，必须保持为空
- 修改 vhost 或伪静态后必须 nginx -t 校验并 nginx -s reload，再从服务器本机复测

发布验收：

- 在服务器本机执行：curl -sk -o /dev/null -w '%{http_code}' -H 'Host: <域名>' https://127.0.0.1<path>
- 必须覆盖 /、/login、/settings、/assets/ 下的入口 JS、/api/v1/admin/auth/me（未登录应为 401）、/amitia/latest.yml
- 必须实际提交一次 POST /api/v1/admin/auth/login，确认返回 200「登录成功」且 Set-Cookie 带 HttpOnly、Secure、SameSite=Strict
- 服务必须 systemctl is-active 与 is-enabled 同时为 active/enabled，并确认监听 127.0.0.1:18880

可复用部署流程（2026-10-16 管理端上线时已完整执行并验证）：

本地构建与打包：

1. 在 admin-system 目录用项目内 Node 执行 vite build，产物为 admin-system/dist
2. 在 backend 目录执行：GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o admin-server-linux-amd64 ./cmd/admin-server
3. 在 artifacts/admin-deploy-<yyyyMMdd-HHmmss> 下组装 dist/、admin-server（由 admin-server-linux-amd64 改名）、amitia-admin-seed.sql
4. 用 PowerShell Get-FileHash -Algorithm SHA256 生成 SHA256SUMS，每行格式为「小写哈希 + 两个空格 + 相对路径」；哈希必须转小写，大写哈希会导致远端 sha256sum -c 校验失败

上传（Paramiko，禁用 agent 与自动查找密钥）：

1. sftp 在 /tmp/admin-deploy-<yyyyMMdd-HHmmss> 建目录并递归上传全部文件，admin-server 上传后 chmod 755
2. 远端执行 cd <暂存目录> && sha256sum -c SHA256SUMS，逐文件 OK 后才能向用户交付替换命令
3. Windows 下禁止把含中文的本地路径写入 ascii 编码的临时文件再读取（会被替换成 ? 导致上传空跑），改用环境变量传参

宝塔 root 终端替换命令：

1. cp -a dist/index.html 覆盖站点根 index.html；rm -rf 站点根/assets 后 cp -a dist/assets 落到站点根
2. chown -R www:www 站点根/index.html 与 assets；chmod 755 站点根与 assets；find assets -type f -exec chmod 644 {} +
3. systemctl stop amitia-admin；install -m 0755 -o www -g www admin-server /www/server/amitia-admin/admin-server；chown www:www /www/server/amitia-admin；systemctl start amitia-admin
4. 数据库无需手工操作：新二进制启动时会自动执行 adminMigrations，创建 user_feedbacks 等新表
5. 禁止重新执行 deploy/mysql/amitia-admin-seed.sql，它会重置管理员密码并删除全部管理会话，仅用于首次初始化

替换后必须验收：

1. ls 站点根/assets | grep <本次新增前端分块名>，确认新页面分块已就位
2. 服务器本机 curl 必须覆盖 /、/login、/settings、/feedback、/api/v1/admin/auth/me（未登录 401）
3. 读取 /www/server/amitia-admin/amitia-admin.env 后执行 SHOW TABLES LIKE '<新表名>'，确认增量迁移已生效
4. curl -F 'payload={...}' https://127.0.0.1/api/v1/feedbacks 必须返回 {"code":200,"msg":"反馈已接收"}；返回 404「接口不存在」说明新二进制没有生效
5. 公网复测必须加 --noproxy '*'，本机 7890 代理会让请求超时并误判为服务异常

管理端用户反馈处理规则：

- 管理端用户反馈中的问题在完成实际修复并通过对应模块验证后，必须立即在管理端标记为“已解决”（即已修复）；未完成实际修复前禁止提前标记，并必须回读确认解决状态、解决时间和解决人正确。

启动项目前必须先杀一遍项目占用（环境除外）

Electron 桌面端启动时会自动通过 CoreManager 拉起后端服务（AmitiaCore.exe），无需手动单独启动后端。

数据库版本规则（单版本基线）：

- backend/internal/migration/baseline.sql 是唯一数据库基线，只描述当前版本 schema。
- 禁止维护历史版本迁移链、schema_migrations 版本兼容、旧 checksum、旧结构修复和旧数据升级。
- 新数据库直接应用当前基线；已有数据库只允许结构与当前基线一致，禁止自动转换旧 schema。
- 数据库结构变更直接更新当前基线和当前版本包，不新增面向旧版本的增量迁移。
- 禁止 AutoMigrate 和启动时隐式修复旧表结构。
- 扩展内核当前版本的包更新迁移保留，但禁止迁移或读取旧应用版本、旧插件包和旧 manifest。

Android 构建与真机安装规则：

- 手机端 versionName 必须与桌面端保持一致，versionCode 只能单调递增；两端版本号不一致时禁止发布。
- 仅允许构建、安装和验证 Release 版本 APK，禁止执行、安装或保留任何 Debug 版本 APK。
- 真机安装前必须确认包名为 com.amitia.amitia_app；发现 com.amitia.amitia_app.debug 时必须先卸载，并在安装后复核该 Debug 包不存在。
- 每次修复完成后，只要修改涉及手机端或手机内置 Runtime，必须自动构建 Release APK 并安装到已连接真机；只有用户明确说明仅修改源码或暂不构建时才可以跳过。

Android Release 构建方法：

- 项目构建不设置固定内存预算，不再限制为 1 GiB。默认由 JVM 和构建工具自动配置内存；需要时可根据机器可用内存调整，不再限制堆、Metaspace 和 CodeCache 的合计上限。
- APK 构建使用单个 Gradle worker，关闭 Gradle 并行构建；Kotlin 在 Gradle 同一进程中编译。
- 构建前检查并停止遗留的 Gradle/Kotlin 构建进程，避免重复构建；核验新进程使用的实际内存参数。
- 禁止为 APK 构建启用自动 JVM 堆转储，避免内存不足时生成大体积转储文件。
1. 禁止使用 scripts/build-apk.ps1 的临时源码复制流程。正式构建必须使用项目根目录中的同一份源码。
2. 项目位于中文路径时，统一从 R: 盘符映射构建。R: 必须映射到 D:\桌面\跟进项目\U-Ai，先执行 subst.exe 检查；缺少映射时执行 subst.exe R: "D:\桌面\跟进项目\U-Ai"。R: 只是同一源码目录的盘符映射，不是源码副本，禁止将源码复制到其他目录构建。
3. 当前进程必须配置正式签名环境变量 AMITIA_KEYSTORE_PATH、AMITIA_KEYSTORE_PASSWORD、AMITIA_KEY_ALIAS、AMITIA_KEY_PASSWORD。密钥和密码只能通过环境变量注入，禁止写入 AGENTS.md、源码、配置、日志或构建产物。
4. 构建前设置：
   - $env:PUB_HOSTED_URL = "https://pub.dev"
   - 若 $env:JAVA_TOOL_OPTIONS 不含 jdk.net.unixdomain.tmpdir=，追加 -Djdk.net.unixdomain.tmpdir=C:\Temp，并确保 C:\Temp 存在。
5. 在 R:\mobile_app 执行：
   - flutter clean
   - flutter pub get
   - flutter build apk --release --target-platform android-arm64 --no-tree-shake-icons --config-only
6. 在 R:\mobile_app\android 执行：
   - 先执行 `.\gradlew.bat :app:lintVitalRelease --no-daemon --no-parallel --max-workers=1 '-Pkotlin.compiler.execution.strategy=in-process' '-Dorg.gradle.jvmargs=-Dfile.encoding=UTF-8 -XX:-HeapDumpOnOutOfMemoryError'`
   - 上一步成功退出后，再执行 `.\gradlew.bat assembleRelease --no-daemon --no-parallel --max-workers=1 '-Pkotlin.compiler.execution.strategy=in-process' '-Dorg.gradle.jvmargs=-Dfile.encoding=UTF-8 -XX:-HeapDumpOnOutOfMemoryError'`
   - 两个阶段必须串行执行，使用同一份源码与新构建的 Runtime Package；禁止跳过 Release lint。内存由 JVM 自动配置，遇到内存不足时可根据机器资源调整。
7. 必须使用 Gradle 返回码判断构建结果。由于中文路径下 Flutter 包装命令可能在产物查找阶段误报失败，禁止仅凭 flutter build apk 的产物查找结果判定构建失败或成功。
8. APK 输出路径固定为 R:\mobile_app\android\app\build\outputs\flutter-apk\app-release.apk，等价实际路径为 D:\桌面\跟进项目\U-Ai\mobile_app\android\app\build\outputs\flutter-apk\app-release.apk。
9. 安装前必须验证：
   - applicationId 为 com.amitia.amitia_app
   - versionName、versionCode 与项目当前版本一致
   - apksigner 验证为正式 Release 签名
   - zipalign 验证通过
   - APK 仅包含 arm64-v8a
   - APK 内 assets/runtime-package/amitia-runtime-1.0.0.zip 的 SHA-256 与构建输入一致
10. 安装流程：
    - adb devices -l
    - adb shell pm list packages 检查 com.amitia.amitia_app.debug；存在时先执行 adb uninstall com.amitia.amitia_app.debug
    - adb install -r <APK路径>
    - adb shell dumpsys package com.amitia.amitia_app 复核版本，并再次确认 debug 包不存在
    - adb shell am start -W -n com.amitia.amitia_app/.MainActivity 启动应用
    - adb shell ps -A 与 adb shell dumpsys activity activities 复核应用进程和前台页面
11. 正式 APK 必须复制到 artifacts/apk/，文件名使用 amitia-<version>-release-arm64-v8a-<yyyyMMdd-HHmmss>.apk，并记录文件大小和 SHA-256。

源码副本规则：

- 禁止使用临时复制的源码目录构建、打包、安装或验证正式版本。
- 如因排查临时复制源码到项目根目录以外的位置，使用完成后必须立即删除该复制目录，并在删除后确认项目根目录仍为唯一构建来源。



在制作插件、修复插件功能、开发插件的时候，如果需要给插件系统新增功能，必须将新增的扩展能力抽象为公共扩展能力，让后续其他插件在制作的时候也可以使用。
