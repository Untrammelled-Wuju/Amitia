# 手机端 QQ 式共享轴页面转场（26.2.0-beta.16）

## 需求

手机端页面与页面之间的切换需要有联动位移：进入二级页面时，二级页面从右向左推入，同时根页面也向左移动；从二级页面返回时，二级页面从左向右退出，根页面同步从左向右回到原位。目标是与 QQ / iOS 的共享轴（shared-axis / parallax）导航一致，并且支持左边缘滑动返回。

用户已确认三点：范围覆盖全部页面层级；位移幅度与暗化程度按建议值实施；需要左边缘手势返回。

## 现状与问题

原实现有两套转场：`slideFadePage`（全屏滑入 + 整页淡入）与 `backTargetTransition`（被覆盖页 -8% 位移 + 淡出）。问题有三点：根页面的页面对象由 `StatefulShellRoute.indexedStack` 的 `builder:` 生成，go_router 会退化为默认 `MaterialPage`，转场完全由平台决定；五个底部根 Tab 使用 `NoTransitionPage`，完全不响应 `secondaryAnimation`；被覆盖页只有 -8% 位移且快速淡出，视觉上等于静止。

## 方案（行业标准）

对齐 Material 3 shared-axis X 转场与 iOS/QQ 的 push 语义：被覆盖页读取自身 `secondaryAnimation`（由上层路由驱动，0 → 1），按比例位移并叠加一层黑色 scrim；新推入页只由自身 `animation` 驱动，从 100% 宽度滑到 0，并带一层左侧投影把它与底页区分开。手势返回时只驱动顶层路由的 `AnimationController`，底页通过 `secondaryAnimation` 自动跟随，因此手指拖动过程与最终动画完全一致。

参数取值：底页位移 30%（`amitiaSlideRatio = 0.3`），scrim 峰值 12%，进入 380ms / 退出 300ms，曲线 `easeOutCubic` / `easeInCubic`（沿用项目 `AppMotion`）。手势判定：左边缘 24dp 起手，位移超过 50% 或速度大于 700px/s 时完成返回，否则回弹；回弹与完成都使用 260ms 内的按比例插值，避免一次滑动直接跳到终点。

根层级（shell 与五个底部 Tab）标记 `rootLevel`，转场时长归零，因此在 Tab 之间切换时没有多余动画，但它们作为被覆盖页时仍然参与 30% 联动位移。

## 实现

新增 `mobile_app/lib/app/amitia_page_route.dart`：

`AmitiaSlidePage<T>` 是 `Page<T>` 子类，`createRoute` 生成 `AmitiaSlideRoute<T>`，并把自身作为 `settings` 传入，保证 page-based Navigator 的 `route.settings is Page` 断言成立。`buildPage` 与 `isRootLevel` 都从最新的 `settings` 读取，而不是构造时捕获的值，这样 go_router 在 Tab 切换时更新同一个 Page（key 不变）就能刷新内容，不会重建路由、也不会丢掉分支 Navigator 的选中索引。

`AmitiaSlideRoute<T>` 继承 `PageRoute<T>`，实现 `buildTransitions`：外层 `SlideTransition(-0.3, 0)` 绑定 `secondaryAnimation`，内部 `Stack` 依次叠放入场页与 scrim `FadeTransition(0 → 0.12)`；入场页仅在非根层级加 `SlideTransition(1 → 0)` 与 `BoxShadow(-4, 0, blur 18)`。

手势由 `_AmitiaBackGestureDetector` 提供：`RawGestureDetector` 搭载自定义的 `_AmitiaBackGestureRecognizer`（`HorizontalDragGestureRecognizer` 子类），重写 `addAllowedPointer`，落点不在左边缘就直接返回、不进入手势竞技场，因此不会与页面内的横向滑动组件抢事件。拖动时调用 `route.updateBackGesture` 修改 `controller.value`，手势开始与结束包在 `navigator.didStartUserGesture` / `didStopUserGesture` 之间，保证系统返回键与手势状态一致。

改写 `mobile_app/lib/app/route_transitions.dart`：只保留 `amitiaSlidePage` 一个工厂函数，删除 `slideFadePage`、`drawerSlideFadePage`、`chatRootPage`、`backTargetTransition`。

改造路由：`builtin_route_catalog.dart` 中 `StatefulShellRoute.indexedStack` 由 `builder:` 改为 `pageBuilder:` 返回 `rootLevel` 的 `AmitiaSlidePage`，五个底部根 Tab 由 `NoTransitionPage` 改为 `rootLevel` 的 `amitiaSlidePage`，其余全部业务路由统一走 `amitiaSlidePage`；`router.dart` 三个页面同样迁移。

两处 `MaterialPageRoute` 保留：`core/widgets/amitia_editor.dart` 的编辑器与 `features/chat/presentation/pages/chat_page.dart` 的实时通话页，两者都是 `fullscreenDialog: true` 的垂直全屏模态，语义上不属于横向 push。

无障碍：`MediaQuery.disableAnimations` 为真时直接返回 `child`，不播放任何位移与暗化。

## 测试

`test/app/route_transitions_test.dart` 重写为 7 项：push 过程中根页位移落入 (0, -0.3] 且稳定在 -0.3；pop 时根页回到 0；左边缘滑动 520px 触发返回；90px 拖动回弹；屏幕中部拖动不触发返回；根层 `transitionDuration` 为 0 而普通页大于 0；`disableAnimations` 时立即呈现。

定向结果：

- `flutter test test/app`：23 项全部通过
- `flutter test test/core/ui_runtime`：36 项全部通过
- `flutter test test/features/chat`：71 项通过，2 项既有失败（`chat_message_render_pipeline_test.dart` 空扩展槽用例、`chat_top_bar_source_test.dart` 的紧凑尺寸断言），与本轮转场改动无关，未在本次范围内修改
- `dart analyze` 对全部变更文件：No issues found

## 构建

同一份源码（HEAD `7f110257bb0acb9d5065f89cc534c03060b64a90`）构建，本次只改 Dart，Go 侧无差异，Runtime Package 的 `sourceCommit` 与 HEAD 一致，因此复用上一轮同源码产出的 `artifacts/android-chatheader-20261026/amitia-runtime-1.0.0-android-arm64.zip`（SHA-256 `7816dd95…ca514b`）。

`flutter build apk --config-only` 后执行 `:app:lintVitalRelease` 与 `assembleRelease`，两者串行、单 worker、`--no-daemon --no-parallel`，均 BUILD SUCCESSFUL；`:app:compileFlutterBuildRelease` 实际执行，Dart AOT 为本次源码编译结果。

产物：`artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-005016.apk`，136353071 字节，SHA-256 `8ed6c04ab73323973e5adfa981ecf176c48a18ea9fcb0d722e86dffe01ac3642`。

校验：包名 `com.amitia.amitia_app`，versionCode 22 / versionName 26.2.0-beta.16，正式签名且证书与既有正式包一致，`zipalign -c -P 16` 通过，仅 `arm64-v8a`，内嵌 Runtime 哈希与输入一致，Runtime `sourceCommit` 与源码提交一致，后端为 ARM64 ELF。

## 状态

已构建并完成制品校验，未安装、未发布、未推送 Git。

当前 macOS 主机未连接任何 Android 设备（`adb devices` 为空，USB 未枚举到设备，mDNS 无线调试无服务），因此本轮未执行安装、冷启动与真机转场录制。设备恢复连接后需要补做：卸载 `com.amitia.amitia_app.debug`（若存在）、`adb install -r` 覆盖安装、复核版本与安装后 APK 哈希、冷启动进入消息页、进入对话详情与返回各录一次转场、左边缘滑动返回验证，并在真机上确认根页面联动位移与暗化符合预期。
