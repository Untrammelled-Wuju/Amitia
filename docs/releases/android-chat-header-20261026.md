# 对话详情页顶部栏与状态栏对齐根页面（26.2.0-beta.16 / 22）

## 需求

对话详情页顶部的返回按钮、标题、更多按钮所在行，以及其上的系统状态栏，背景色与高度要和根页面（消息页）顶部标题栏、状态栏保持一致。改完后构建 Release APK 并安装到真机。

## 现状与问题

- 根页面用 `AmitiaAppBar`，`backgroundColor: context.surfacePrimary`（浅色为 `#FFFFFF`），`toolbarHeight: AmitiaAppBar.rootPageToolbarHeight = 50`，状态栏透明 + 深色图标。真机测量：`y=0..299` 全白，即 42.3dp 状态栏 + 50dp 工具栏。
- 对话页顶部栏 `_ChatTopBar` 高度写死 68dp、内容区 `Padding(fromLTRB(12,8,12,8))`、不加任何背景，透出的是页面底色 `backgroundPrimary`（浅色 `#F7F8FA`）。真机测量：`y=0` 起一直 `#F7F8FA`，与根页面明显不一致，且没有向系统声明状态栏样式。

## 改动（`mobile_app/lib/features/chat/presentation/pages/chat_page.dart`）

1. `_chatTopBarHeight` 由写死 `68` 改为引用共享令牌 `AmitiaAppBar.rootPageToolbarHeight`（50），保证与根页面工具栏同高、后续只改一处。
2. `_ChatTopBar.build` 外层加 `AnnotatedRegion<SystemUiOverlayStyle>`：`statusBarColor: transparent`、`systemStatusBarContrastEnforced: false`，图标明暗按 `ThemeData.estimateBrightnessForColor(surfacePrimary)` 推导，与 `AmitiaAppBar`、`AmitiaSystemBars` 的做法一致。
3. 内部包 `ColoredBox(color: context.surfacePrimary)`，让状态栏区域一起被白色填充；`SafeArea(bottom: false)` → `SizedBox(height: _chatTopBarHeight)` → `Padding(EdgeInsets.symmetric(horizontal: 12))` 承载原有 `Row`，去掉原来的上下 8dp 内边距（68dp 高度里靠 padding 撑出的部分不再需要）。
4. 侧边栏浮层 `Positioned(top: ...)` 由 `_chatTopBarHeight` 改为 `MediaQuery.paddingOf(context).top + _chatTopBarHeight`，因为顶栏收缩 18dp 后浮层起点需要按状态栏高度重新对齐。

改动文件仅此一个，未触碰 Go 源码、Web 端与管理端。

## 验证

### 构建

- 源码提交：`7f110257bb0acb9d5065f89cc534c03060b64a90`（`chore(release): refresh 26.2.0-beta.16 chat top bar sources`）。
- 后端按同一份源码重新编译 Linux ARM64：`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" ./cmd/server`，`vcs.revision` 与提交一致，SHA-256 `6e90a4c50fb3a4639b22e1c6b6e743bdcf672ccd0ccbf525f93933b92f282b10`。
- Runtime Package 由 `runtime/scripts/build-fresh-runtime-package-android-arm64.py` 重新生成，SHA-256 `7816dd951b1cfb629434d5ef78c1f4dc59257b5918179abd0414e0dcac81c5bb`，`sourceCommit` 与提交一致，`validate-runtime-package.py` 严格校验通过。
- `:app:lintVitalRelease` 与 `assembleRelease` 串行执行，均 BUILD SUCCESSFUL。

### APK

- `artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-001919.apk`，136358699 字节，SHA-256 `2544d64b83321153638caed38504e03f9d8c13555c882453351287d74f820cea`。
- applicationId `com.amitia.amitia_app`，versionCode 22，versionName 26.2.0-beta.16。
- `apksigner` 正式签名校验通过，且证书 SHA-256 与手机原安装包一致；`zipalign -c -P 16` 16 KB 对齐通过；仅 `arm64-v8a`；内嵌 Runtime Package 哈希与构建输入一致，内嵌后端为 ARM64 ELF。
- 安装到 2407FRK8EC（JV9LT8JB8TVKNJ65）后设备内 `base.apk` 哈希与本地制品一致；`com.amitia.amitia_app.debug` 不存在。

### 真机

- 冷启动进入对话详情页，日志无 FATAL EXCEPTION。
- 像素采样（x=610 竖直方向）：对话页 `y=0..299` 全为 `#FFFFFF`，`y=300` 起为 `#F7F8FA`；根页面消息页同样 `y=0..299` 为 `#FFFFFF`，`y=301` 起为 `#F7F8FA`。两页顶部区域颜色与高度（299px ≈ 92.3dp = 42.3dp 状态栏 + 50dp 工具栏）完全一致。
- 状态栏时间与图标为深色，白底可读。
- 手机内置后端 `/api/health` 返回 `ready=true`、`model=configured`；`/readyz` 为 `degraded`（3 ready、1 degraded、0 blocking、0 failed），与改动前一致。

### 测试

- `flutter test test/features/chat/ test/app/chat_navigation_test.dart`：78 项通过，2 项失败。
- 两项失败均为既有失败，与本次改动无关：`chat_message_render_pipeline_test.dart` 的 `message content cannot be replaced by a blank extension slot`（用未改动的 HEAD 版本文件复跑同样失败），`chat_top_bar_source_test.dart` 的 `chat call control uses the compact size`（断言 `menuWidth: 190`，现有实现为 220，属既有产品取值差异）。
- `dart analyze lib/features/chat/presentation/pages/chat_page.dart` 仅 8 条既有 info 级提示，本次改动区间无新增问题。

## 状态

未发布、未推送 Git。构建过程记录与截图位于 `artifacts/android-chatheader-20261026/`。
