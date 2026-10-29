# Amitia 手机渠道工作区与 QQ 运行环境交接

记录日期：2026-10-29，时区 Asia/Shanghai。
工作目录：`/Users/untrammelled/Desktop/Project/U-AI`。

## 当前结论

Xvfb、x11vnc、noVNC、SnowLuma 和 Linux QQ 的启动链路已经解决并完成模拟器验证；QQ 已扫码登录。最终插件为 0.2.57。SnowLuma 仍要求用户本人接受 EULA 与隐私说明，协议端在用户确认前不会载入 QQ 进程。

用户最后要求输出交接文档，所以本轮转为交接，没有将未完成的运行环境验收标记为完成。

## 用户目标和已确认方案

用户指出手机渠道详情中的“消息统计”不能仅重复渲染渠道会话，要求以桌面端实际行为为准；QQ 个人号插件页面需符合移动端布局规范，减少大面积空白；安装进度必须实时显示。

已对照桌面源码 `front/src/views/channels/ChannelHostMessagesPanel.vue`：

- 保留四项统计。
- `singleConversation` 渠道直接展示消息内容。
- 多会话渠道先显示当前渠道的会话列表，选中后展示消息。
- 消息使用通用 `conversation.message_renderer` 和现有回退气泡。
- 微信 manifest 明确声明 `singleConversation: true`；不能仅凭用户口述将其他插件强行改成单会话，应继续以桌面实际元数据为准。

用户已明确说“一次性做完”“做完之前禁止停止任务，全部都做”。随后又明确授权：**允许当前工作区全部改动纳入正式构建基线**。不要再次询问已经授权的构建和验证事项。

## 项目约束

- 中文回复。用户未要求 skill，禁止使用 skill。用户未要求并行代理，不要自行派发代理。
- 只改源码，不直接改编译产物；产物通过构建脚本生成。项目代码不添加注释。
- 禁止 Git pull、push、上传服务器或发布；本轮只有本地基线提交。
- 不修改管理端。不使用 3000，不随意修改项目端口。
- 项目 Node 必须使用 `/Users/untrammelled/Desktop/Project/U-AI/AmitiaData/node/bin/node`，版本 26.12.1。
- 使用 pnpm/tsx 时将该 Node 目录放在 PATH 首位。不要使用系统 Node。
- 手机 APK 构建必须重新构建同源后端、plugin-host、task-host 和 Runtime Package。
- 应用版本保持 `26.2.0-beta.18`，versionCode 24。用户未授权改应用版本。
- 插件发生内容更新需要正常提升插件版本；安装器拒绝相同版本但内容不同的包。本轮 QQ 插件最终为 0.2.57。
- 纯前端改动不重启完整项目；后端改动需要完整项目重启。本轮已执行过完整项目重建和重启。
- 不 reset、checkout 或回滚用户/其他任务代码。基线中包含用户明确授权纳入的原有工作区改动。

## Git 和源码基线

当前分支为原有 `master`，未推送。主要本地提交：

- `75a8bca3`：纳入已授权工作区源码、渠道消息和页面自动刷新修改。
- `80efd1b6`：QQ 插件更新到 0.2.45。
- `11d88195`：消息正文测试、角色作用域参数、XKB 路径配置。
- `1afc0123`：更新发布源码冻结清单。
- **当前 HEAD：`d4bbc438d9635320dc2be4b74965337e626de539`**，补齐后端 Schema 刷新间隔契约、下载字节进度和缓存校验等。

最终已安装 APK 和 Runtime 对应 `d4bbc438`。插件源码和制品最终为 0.2.57。

当前未提交文件：

- `plugins-code/channels/qq-personal/amitia-extension.json`
- `plugins-code/channels/qq-personal/runtime/package.json`
- `plugins-code/channels/qq-personal/runtime/src/index.ts`
- `plugins-code/channels/qq-personal/runtime/src/mobile-desktop.ts`
- `plugins-code/channels/qq-personal/runtime/src/mobile-environment.test.ts`
- `plugins-code/channels/qq-personal/runtime/src/mobile-environment.ts`
- `plugins-code/channels/qq-personal/runtime/src/mobile-runtime.ts`
- `plugins-code/channels/qq-personal/ui/mobile/schema.json`
- 新文件 `plugins-code/channels/qq-personal/runtime/src/mobile-desktop.test.ts`
- 本交接文档。

源码冻结清单与后续插件修改尚未同步。下一轮真正需要新建发布基线/构建时，通过 `node scripts/build-freeze-scope.mjs --write` 正常刷新；不要手工改哈希。

尚未更新 AGENTS.md 的本次最终构建验收记录，因为完整安装运行验收还没有通过。最终通过后再单独提交记录，记录提交不能作为下一次源码差异目标。

## 已完成的源码改动

### 渠道消息

文件：`mobile_app/lib/features/channels/presentation/pages/channel_messages_page.dart`。

- 单会话渠道在统计页内嵌 `ChannelMessageDetailPage(embedded: true)`。
- 多会话渠道保留当前渠道列表，点击进入只读正文页。
- 按当前渠道过滤，不混入其他渠道。
- 正文读取传递会话的 characterId，避免错误角色作用域。
- 消息读取错误有明确展示，不再仅落入空态。
- 复用通用消息渲染 Provider。

新增测试：`mobile_app/test/features/channels/channel_host_messages_test.dart`，覆盖单会话内嵌正文、多会话点击正文、排除其他渠道。

### 通用自动刷新

- Dart：`SchemaUILifecycle.refreshIntervalMs`。
- Renderer：定时执行 lifecycle.onRefresh，限制 1000–60000 ms，防止重叠；后台和非当前路由暂停；dispose 取消 timer；切换 document 取消旧 timer。
- 后端：`backend/internal/extension/kernel/schema_ui/schema.go` 增加 JSON 字段并校验范围。
- 后端编译/序列化往返测试已经覆盖刷新间隔保留。
- 前端共享 Schema 类型同步增加可选字段。
- Schema progress 支持 indeterminate。

重要实机发现：原先只改 Dart 会失败，因为后端 SchemaUILifecycle 结构体丢掉 refreshIntervalMs。已在 d4bbc438 修好，不能回退该字段。

实际模拟器日志已经证明，无手动刷新时 `/api/status` 请求约每 1500 ms 出现一次：
`artifacts/android-channel-final-20261029/automatic-refresh.json`。

### QQ 手机页面

文件：`plugins-code/channels/qq-personal/ui/mobile/schema.json`。

- 去掉多层 page/section/card 嵌套与原始诊断字段面板。
- 连接和安装状态使用紧凑文本层级。
- 最终 0.2.57 使用 key_value 两列展示 QQ 号、客户端、接收、回复，替代有多余高度的 Grid。
- `index.ts` 新增有序数组 `displayMetrics`，防止后端 JSON map 排序打乱指标顺序。
- lifecycle 每 1500 ms 刷新 status。
- 安装中禁用重复安装按钮。
- 设置与访问入口继续复用现有 Schema 宿主能力。

### 安装进度和重试

- `mobile-qq-package.ts` 通过流量计数报告下载 MB、总大小、百分比，约 500 ms 更新一次。
- 已下载 QQ 包必须校验锁定 SHA-256，校验通过可复用。
- `mobileEnvironmentReady()` 校验 prepared.json schemaVersion 和必要文件，包括 xkbcomp 和 XKB 数据。
- 已准备环境重试时跳过 299 个依赖的重复解包，仍会执行 linkMobileEnvironment 和桌面启动。
- 新增缓存失效测试，删除 xkbcomp 时不得误判环境可复用。

## 最终 APK 和 Runtime

目录：`artifacts/android-channel-final-20261029/`。

APK：
`amitia-26.2.0-beta.18-release-arm64-v8a.apk`

SHA-256：
`1882131ce4dfa29cfb01e6f90ec376589dd0cc6983226988eb5e2a96eb9accc1`

Runtime：
`amitia-runtime-1.0.0-android-arm64.zip`

SHA-256：
`93479e2f9518526c055e174269c37d565743c6d9cc2a00ec109ec808018e0ada`

sourceCommit/sourceRevision：
`d4bbc438d9635320dc2be4b74965337e626de539`

新编译后端：`backend/amitia-server`（相对上述制品目录）。
后端 SHA-256：
`1340ff411ec581e14e62dc0570a8c81d7c65d20381e09b388462b3fe21471136`

已验证：正式 apksigner v2 签名、16 KB zipalign、单 arm64 ABI、APK 内嵌 Runtime 与输入哈希一致、Runtime sourceCommit 正确、包内后端与本次编译后端一致。

APK 已安装到模拟器，设备内 base.apk 哈希与上述 APK 一致。不要再把上一轮 9f72f84 或本轮中间包 279aa291 当成最终包。

## 当前 QQ 插件制品

文件：`plugin/com.amitia.channel-qq-personal-0.2.57.amitiax`。
SHA-256：`0bbffb25edfe9171ab410f3be5c32dc0509adbdf7e9bc6adc7fc4adcc9b9e8b7`。
treeHash：`685f393f478d71f9deeb77e825308466e45d2e86e41c5eae4ce167ee79056cfe`。

设备已安装、启用 0.2.57，运行 generation：
`generation-6f4eeaa6-003d-4902-a817-3dffb5c05649`。

0.2.49/0.2.50/0.2.51 是中间产物，不要作为最终交付。若再次修改已安装的 0.2.57 内容，需要正常升插件版本，不能直接改 .amitiax 或绕过同版本内容校验。

## 测试与日志

已通过：

- Flutter 相关定向 84 项。
- 新增渠道正文行为 2 项。
- QQ runtime 最新 68/68。
- 后端改动包：internal/chat、internal/devicemesh/business、internal/devicemesh/coordination、internal/extension/kernel、internal/extension/kernel/trusted_service。
- cmd/server 首次有一个跨设备投递用例失败，单用例重试通过，随后整个 cmd/server 包完整复跑通过（78.787s）。不要隐去首次失败记录。
- schema_ui 包通过；修复了迁移测试仍引用已删除 channels/qq 路径的问题，改为 qq-clawbot 并补 QQ personal Schema 验证。
- go mod verify、go vet ./... 在前一轮基线通过；新增 schema_ui 修改后该包 vet/test 通过。
- Release Vital lint 通过（1m48s）。最终 d4bbc438 只新增后端 Schema 字段与插件变化，Dart/Android 源码与该 lint 轮相同。
- 消息页与模型的定向静态分析通过。通用 renderer 有既有 lint info/未用声明警告，不要称全项目静态分析零告警。

本机临时日志：

- `/tmp/amitia-final-apk-build.log`
- `/tmp/amitia-final-runtime-build.log`
- `/tmp/amitia-final-runtime-validation.json`
- `/tmp/amitia-final-backend-build.log`
- `/tmp/amitia-final-signature.log`
- `/tmp/amitia-final-lint.log`
- `/tmp/amitia-final-flutter-tests.log`
- `/tmp/amitia-channel-content-tests.log`
- `/tmp/amitia-qq-final-tests.log`
- `/tmp/amitia-final-schema-tests.log`
- `/tmp/amitia-final-go-tests.log`（首次失败记录）
- `/tmp/amitia-final-server-recheck.log`（完整重试通过）

不要默认跑全量测试。

## 当前模拟器与服务

- `emulator-5554`，AVD Pixel_10_Pro，只有这一个模拟器。
- 包名 `com.amitia.amitia_app`，versionCode 24。
- Android 新 Core 已 READY，`/api/health` ready=true，model=not_configured。
- QQ 已通过 noVNC 扫码登录并保持登录态，尚未执行实际模型聊天。
- Xvfb、fluxbox、x11vnc、websockify、Linux QQ 和 SnowLuma 均已实际启动。
- 直接插件 `/api/status` 返回 401 是正常的，需要宿主服务凭据。手机页面能通过宿主正常访问；不要绕过认证。

端口保持：Core 18899、QQ 插件 6199、SnowLuma 5099、noVNC 6081、x11vnc 5900。
ADB 转发可使用 28899、26199、25099、26081、25900，禁止 3000。

覆盖安装新 Runtime 时发生过一次 ANR（WorkflowAutomationHealthJobService onStartJob），等待修复解包后恢复。最后一次 d4bbc438 安装后 Core 已恢复 READY。不要粗暴清数据或重复启动第二台模拟器。

## Xvfb XKB 初始化（已解决）

历史失败文本：

```
Xvfb 已退出（code 1）
_XSERVTransmkdir: Owner of /tmp/.X11-unix should be set to root
XKB: Failed to compile keymap
Keyboard initialization failed. This could be a missing or incorrect setup of xkeyboard-config.
Failed to activate virtual core keyboard: 2
```

排查与修复过程：

1. 必须使用真实 Ubuntu 包 `x11-xkb-utils`，其中有 xkbcomp。不要改成不存在的 `xkbcomp` 包名。
2. 299 个图形依赖和 QQ 已完整解包，prepared.json 存在。
3. `MobileDesktop.environment()` 已设置私有库目录、PATH、XKB_CONFIG_ROOT、字体和 Python 目录。
4. Xvfb 启动已显式传入 `-xkbdir <mobile/root/usr/share/X11/xkb>`。
5. 已修复 displayfd 返回 0 被 `display || 1` 错转为 1 的错误，现为 `display ?? 1`，新增用例通过。
6. 0.2.50 独立 xkbcomp 从文件编译 keymap 成功，但 Xvfb 仍失败。
7. 0.2.51 进一步改为 `/bin/sh -c` + `/usr/bin/xkbcomp` + `-R<私有XKB目录>` + 标准输入管道编译；这一步也通过，随后仍在 Xvfb 内部失败。
8. 所以目前证据不支持继续简单归因于缺包、目录缺失或普通 stdin 管道不可用。
9. `mobile-desktop.ts` 已累积保留 stderr，`verifyKeyboard()` 是现有独立编译检查。重试准备环境现在可走缓存，不要再强制删除 prepared.json 和 299 个包。
10. 最终验证确认 PRoot 对 Xvfb 内部的 `setgid(getgid())` 和 `setuid(getuid())` 返回 `ENOSYS`，XKB 报错只是后续表现。
11. 插件 0.2.57 增加仅面向 Linux ARM64 的兼容层，仅允许同身份 `setgid/setuid` 的 `ENOSYS` 无操作成功；Xvfb 已正常启动，未修改 Xvfb 二进制，也未将整个后端切换为 fake root。
12. x11vnc 使用 `-noshm` 启动；VNC 密码采用 0600 临时密码文件和 `-passwdfile rm:`，认证与画面读取通过。

历史验证假设：

Xserver 的 `Popen()` 在 fork 子进程执行 `/bin/sh` 之前会调用 `setgid(getgid())` 和 `setuid(getuid())`，失败直接 `_exit(127)`，因此可能没有额外 stderr。
源代码参考：https://github.com/mirror/xserver/blob/master/os/utils.c ，函数 Popen。

Android 当前 Proot 启动参数没有 `-0`。项目已有通用 fakeRoot 能力，但环境组装器明确设为 false：

- `mobile_app/android/amitia-runtime/src/main/kotlin/com/amitia/amitia_app/runtime/proot/internal/ProotEnvironmentAssembler.kt`
- `DefaultProotCommandBuilder.kt` 在 fakeRoot=true 时添加 -0。
- `ProotEnvironmentAssemblerTest.kt` 现有测试明确断言 false。

应先用受控子进程验证相同 UID/GID 的 setgid/setuid 是否失败，或继续确认 Xvfb 内部 fork/Pclose 行为；不要没有证据就全局翻转 fakeRoot。若确实要调整原生运行环境，需分析通用兼容方案并做相应原生定向测试，重新同源构建 Runtime/APK。不能通过修改 Xvfb 二进制绕过。

## 最终状态

1. 0.2.57 插件已安装，Xvfb、fluxbox、x11vnc、websockify、Linux QQ 和 SnowLuma 均实际运行。
2. 插件健康接口、SnowLuma WebUI、noVNC 和 VNC 密码认证通过；VNC 画面确认 QQ 已完成扫码登录。
3. 最终截图已保存到 `artifacts/android-channel-final-20261029/`。
4. SnowLuma 因 `consentRequired=true` 等待用户本人接受 EULA 与隐私说明；接受并重新检测 QQ 进程前，插件 `qqOnline` 仍为 false，不能声称真实 OneBot 收发已通过。
5. 当前模拟器没有渠道真实聊天数据；消息正文行为由有数据的 widget 测试验证。
6. 源码、制品和测试记录已补齐；AGENTS.md 发布记录单独提交。

## 常用命令

QQ 测试和打包：

```sh
PATH="/Users/untrammelled/Desktop/Project/U-AI/AmitiaData/node/bin:$PATH" ./AmitiaData/node/bin/node plugins-code/channels/qq-personal/runtime/node_modules/tsx/dist/cli.mjs --test plugins-code/channels/qq-personal/runtime/src/*.test.ts
AMITIA_ZIG=/path/to/zig ./AmitiaData/node/bin/node plugins-code/channels/qq-personal/scripts/build-package.mjs
```

手机测试/构建需要 Rust PATH：

```sh
PATH="/Users/untrammelled/.cargo/bin:/Users/untrammelled/Desktop/Project/U-AI/AmitiaData/node/bin:$PATH" flutter test --no-pub test/features/channels/channel_host_messages_test.dart test/core/ui_runtime/schema_ui_renderer_reload_test.dart
PATH="/Users/untrammelled/.cargo/bin:/Users/untrammelled/Desktop/Project/U-AI/AmitiaData/node/bin:$PATH" FROZEN_RUNTIME_PACKAGE_PATH=/绝对路径/新Runtime.zip flutter build apk --release --target-platform android-arm64 --no-pub --no-tree-shake-icons
```

不要删除 `--no-tree-shake-icons`。Flutter native-assets 有时打印 `File modified during build. Build must be rerun.`；本轮最终构建仍正常完成，不要仅凭这行认定构建失败并并行启动第二个构建。必须等待原进程结束，检查最终 BUILD/exit 状态。

后端构建：

```sh
ALLOW_DIRTY=1 VERSION=26.2.0-beta.18 OUTPUT_DIR=/绝对路径/backend bash backend/scripts/build-linux-arm64.sh
```

Runtime 输入仍可复用经过锁定哈希校验的第三方下载包（不可复用旧后端产物）：

- `artifacts/release-26.2.0-beta.18/build-inputs/node-v26.12.1-linux-arm64.tar.xz`
- 同目录 `ubuntu-base-24.04.4-base-arm64.tar.gz`
- 同目录 `qdrant-aarch64-unknown-linux-musl.tar.gz`
- 同目录 `surreal-linux-arm64`
- `artifacts/android-channel-workspace-tabs-20261029/build-inputs/libatomic1_14-20240412-0ubuntu1_arm64.deb`

使用 `runtime/scripts/build-fresh-runtime-package-android-arm64.py --help` 获取参数。需先从源码构建 runtime/plugin-host、runtime/task-host。

## UI 自动化注意

本机有 `/tmp/amitia-ui.py`：使用 adb uiautomator 新鲜快照，按完整 content-desc/text 查找并点击，再等待 2 秒读取结果。

```sh
python3 /tmp/amitia-ui.py
python3 /tmp/amitia-ui.py 刷新状态
python3 /tmp/amitia-ui.py '开始安装 QQ 与 SnowLuma'
```

不要盲用旧坐标连续点。导入包后预览可能较晚出现，必须看到“确认更新”再点；否则相同位置可能是其他插件的开关。本轮曾误触 ASR，已恢复启用，最后复核 ASR 为已启用。

更新插件的标准 UI 路径：插件 → 扩展包 → 安装本地包 → Downloads 中选新包 → 等待预览兼容性 ok → 确认更新。adb push 的位置为 `/sdcard/Download/`。

返回 QQ 的路径：我的 → 渠道消息 → QQ 个人号。

## 完整项目当前状态

本机完整项目已按源码 d4bbc438 重建 Core 并真正重启，最后 `/readyz` 为 ready，6 ready / 0 degraded / 0 failed / 0 blocking。

最后观测内部动态端口：前端 64894、Qdrant HTTP 64898、SurrealDB 64900，QQ 插件 6199。分别验证 HTTP 200/healthz。端口由项目分配，重启后需重新检查，不能硬改成固定值。

`start-all-services.sh` 仅停 Core/数据库，可能保留旧桌面 pnpm/vite/Electron。直接重复执行曾导致新实例被旧实例单例机制挡住、旧 Core 又被杀，最终没有后端。已通过仅终止本项目旧 pnpm 进程树后再运行脚本修复。下一次重启也要确认旧桌面进程树已正确退出，不能杀其他项目或 Codex 进程。

## 给下个对话的简短提示词

手机渠道与 QQ 插件任务已完成代码和本地运行验收。APK d4bbc438 已安装，QQ 插件 0.2.57 已安装；Xvfb、x11vnc、noVNC、SnowLuma 与 Linux QQ 均启动，QQ 已扫码登录。SnowLuma 要求用户本人接受 EULA/隐私条款，确认前 OneBot 仍不会连接，不得将该步骤代劳。未推送、未发布、未改管理端。
