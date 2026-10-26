# 记忆模块单轨收敛与角色内嵌记忆视图（26.2.0-beta.16 / 22）

## 需求

审计发现记忆模块存在"新旧两套系统并存且混用"：旧 `MemoryService`（云端 `/api/memories`、`/api/memory-candidates`）与设备归属体系 `DeviceOwnedMemoryService`（`/api/device-mesh/v1/business/memories`）同时存在，页面按调用点分别走两条通道。要求物理删除 `MemoryService`，把它的能力迁到新服务，页面请求全部改到新服务；根记忆页面 UI 不改。

## 现状与问题

- `mobile_app/lib/core/services/memory_service.dart` 提供 `list/page/listCandidates`，`providers.dart` 暴露 `memoryServiceProvider`、`memoryListProvider`、`memoryListByCharacterProvider`、`memoryCandidateListProvider` 等四条 Riverpod 通道，与 `DeviceOwnedMemoryService` 的设备归属、版本化、执行范围校验完全无关。
- 记忆首页、对话记忆上下文、数据备份计数分别走旧通道，写入却走新通道，读到的数据范围与写入的归属不一致。
- 角色详情的"记忆"页签只显示 `characterId` 过滤后的 3 条预览，缺少新系统的范围切换、候选、图谱、编辑入口。

## 改动

### 1. 能力迁移与旧服务删除

- `device_owned_memory_service.dart` 增加 `listMemories`、`pageMemories`、`listMemoryCandidates`，全部建立在既有 `query()` 之上：`listMemories` 按 `page/pageSize` 翻页直到满足目标条数，`pageMemories` 用 `_pageScopes` 缓存把后端返回的 `nextCursor` 与它的原始执行范围配对（后端游标内嵌 Scope/Filter/Catalog 哈希，翻页必须回传原始范围）。
- 删除 `memory_service.dart`，删除 `providers.dart` 中四条旧通道与相关 provider 定义，`MemoryPage` 移入 `core/models/memory.dart`。
- 调用点切换：记忆首页 `memory_hub_page.dart` 改用 `pageMemories(cursor, limit: 10)`；对话记忆上下文 `chat_page.dart` 改用 `listMemories(characterId, pageSize: 8, sort: 'recently_used')`；数据备份 `backup_page.dart` 改用新增的 `ownedMemoryCountProvider`。

### 2. 全部记忆视图校验缺陷（双端同源）

`query()` 的结果校验按"行 `roleId == StorageRole(scope,kind)`"判断，`memoryView=all` 时同一 owner 下其他角色的记忆会被判为非法并抛错。界面切到新通道后，多角色用户的全部记忆页会直接报错。改为与后端 `ValidateSnapshot` 一致的三分支判定，并按 `memoryRecordScope(scope, partition)` 归一化每行范围。桌面端 `front/src/composables/useOwnedMemoryManagement.ts` 的同款校验同步修正。

### 3. 角色详情"记忆"页签改为角色作用域的完整记忆视图

- `character_detail_page.dart`：`case 2` 改为 `OwnedMemoryPage(characterId: character.id, embedded: true)`，概览页的"最近记忆"改由 `characterMemoryPreviewProvider`（`listMemories(characterId, pageSize: 3)`）驱动。
- `owned_memory_page.dart`：新增 `embedded` 参数。`embedded == true` 时不渲染 `AmitiaAppBar`，改为在内容区首行渲染"记忆"标题与搜索记忆/筛选与管理/记忆图谱/新增记忆四个入口，其余布局、筛选、图谱、编辑、候选流程完全复用；`embedded == false` 时仍走原 `AmitiaScaffold`，根记忆页面外观与行为不变。

### 4. 内嵌视图的范围退化（真机发现）

`initState` 里 `selectRole(widget.characterId)` 在首帧角色列表尚未加载时会抛错，`_role` 落空；`_load` 在刷新后按 `_role` 校验角色，校验失败即把 `_memoryView` 从 `role:<id>` 退回 `all`，于是角色的"记忆"页签在真机上请求的是全部记忆。修复：`_load` 在校验前先从 `_memoryView` 恢复 `_role`。根记忆页（`characterId == null`，`_memoryView` 为 `all`）行为不变。

## 验证

### 构建

- 源码基线：`4cb3e6e287e2dd630d921327448a2a15fe8df7f6`（`fix(mobile): disable predictive back and use 320ms transitions`），本轮改动未提交，`backend/` 无改动。
- Runtime Package：`artifacts/android-memory-single-track-20261026/amitia-runtime-1.0.0-android-arm64.zip`，SHA-256 `2f0e9731d8e7390f2ba9d74c68460e066863c6f12309def3d4db220fbcecc92a`，`sourceCommit` 为上述基线，内嵌后端 `payload/runtime.tar.xz:backend/amitia-server` SHA-256 `d321224810a8064522b24aa3d2785045d602987b53c014889f63a224cb0280a0`，`go version -m` 的 `vcs.revision` 与基线一致。本轮只改 Dart，Go 源码零改动，手机端内嵌包与该包逐字节一致（`mobile_app/android/app/src/main/assets/runtime-package/amitia-runtime-1.0.0.zip` 哈希相同）。
- `:app:lintVitalRelease` 与 `assembleRelease` 串行执行，均 BUILD SUCCESSFUL。

### APK

- `artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-1025.apk`，136353599 字节，SHA-256 `03c4975af83bad8cdceb540912e94e3d4b77a121853dd870c4cebb69a8e94e42`。
- applicationId `com.amitia.amitia_app`，versionCode 22，versionName 26.2.0-beta.16，仅 `arm64-v8a`。
- `apksigner` v2 通过，证书 SHA-256 `119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b`；`zipalign -c -P 16` 通过；内嵌 Runtime Package 哈希 `2f0e9731…` 与构建输入一致。
- 安装到 22081212C（8864cd52）后设备内 `base.apk` 哈希与本地制品一致；`com.amitia.amitia_app.debug` 不存在。

### 真机

- 冷启动 `LaunchState: COLD`、`TotalTime 1454`，`AmitiaRuntime snapshot: state=READY`。
- 我的 → 角色 → 角色详情 → 更多 → 记忆：内嵌视图渲染"记忆"标题、四个入口、类型/查看范围筛选与空态文案，无 `AppBar`，无异常。
- 修复前该页筛选条显示"全部记忆"，修复后显示"查看范围"；内置后端日志对应请求为
  `GET /api/device-mesh/v1/business/memories?characterId=b0f9fcbc-…&limit=128&memoryView=role%3Ab0f9fcbc-…`，HTTP 200。
- 根记忆页（底部"记忆"标签）导航项、最近记忆空态与改动前一致。
- 手机内置后端 `/api/health` 返回 `ready=true`、`model=configured`、`version=26.2.0-beta.16`；`/readyz` 为 `degraded`（3 ready、1 degraded、0 blocking、0 failed），与改动前一致。

### 测试

- `test/features/memory`、`test/app/chat_navigation_test.dart`、`test/core/services/owned_memory_management_test.dart`、`test/core/services/memory_single_track_test.dart`：39 通过、5 失败。
- 5 项失败全部位于 `memory_feature_parity_test.dart`（记忆页忽略旧版本数据、用户画像默认角色范围、memory/timeline 历史来源范围、搜索筛选横屏），已用 `git stash push -- mobile_app/lib` 在 HEAD 的 lib 上复跑，失败集合与计数完全一致（+9 -5），为改动前既有失败。
- 新增守卫用例 `test/core/services/memory_single_track_test.dart`：`memory_service.dart` 不存在、除 `device_owned_memory_service.dart` 外无文件引用 `business/memories`、`business/memory-candidates`、旧 provider 名称。
- 新增 `test/features/memory/memory_embedded_view_test.dart`：覆盖内嵌视图按 `role:<id>` 取数、"角色晚于首帧加载"时的范围退化回归、四个入口存在且无 `AppBar`。
- 桌面端 `vitest run src/__tests__/ownedMemoryManagement.test.ts` 14 项通过，`vue-tsc --noEmit` 无输出（通过）。
- `flutter analyze` 变更文件无 error/warning 级问题；`chat_page.dart:1787` 的 `unawaited_return_in_try_block` 位于 artifact 上传辅助函数，非本次改动行，属既有告警。

## 归属视图归一化修复（14:18 构建）

### 问题

真机删除画像时报 `Bad state: 记录原始归属已变化，请重新加载后再修改`，删除不落库。

根因是同一记录在两个位置保存了不同的作用域快照：`DeviceOwnedChatService.data()` 缓存的行沿用服务端完整 `executionScope`（含请求视图派生字段 `memoryViewAll`），而记忆/画像页面按 `memoryRecordScope(scope, rowRoleId)` 把 `memoryViewAll` 移除并把 `memoryRoleId` 置为行内角色。客户端归属戳包含 `memoryViewAll`，两份快照因此不相等，`edit()` 在发出请求前直接拒绝。

### 修复

- 客户端新增 `_resourceScope(scope, resource)`，`data()`、`edit()` 的资源读取与 `resources` 兜底读取统一按记忆类型走 `memoryRecordScope`，非记忆类型原样保留，确保缓存与页面口径一致。
- 客户端新增 `_recordStamp(scope)`，归属比较不再包含仅由视图派生的 `memoryViewAll`；其余权威字段（Core、设备、角色、版本、epoch、权限修订等）保持严格比较。
- 缓存与调用方视图不一致时，`edit()` 按 `memoryViewForScope(expectedScope)` 重读一次记录再校验，避免历史视图快照导致合法修改被拒。
- 后端 `Edit` 校验 `expectedExecutionScope` 时忽略 `MemoryViewAll`（该字段由请求视图派生，`ResolveMemoryScope` 会按 `MemoryRoleID` 重算），其余字段仍按结构体严格相等比较；新增 `TestEditingAcceptsScopeCarryingViewSelector` 回归用例。
- 客户端新增 `test/core/services/device_owned_memory_edit_scope_test.dart`：覆盖“全部记忆视图下删除不携带 `memoryViewAll`”和“归属比较忽略 `memoryViewAll`”两条路径。

### 构建与验证

- 后端以同一份源码重新编译 Linux ARM64 `amitia-server`（SHA-256 `0069054737dcc5ab2f13cd6b8bcd8888d99273ba43fa3e48dea122614106b1eb`），重打包 Runtime Package `amitia-runtime-1.0.0.zip`（SHA-256 `f2156b38f616a9888e6726de6857fa50b471ebe76ae6cec6d89659f159d2a7f8`，`sourceCommit 4cb3e6e287e2dd630d921327448a2a15fe8df7f6`），`validate-runtime-package.py` 通过。
- Release APK：`artifacts/apk/amitia-26.2.0-beta.16-release-arm64-v8a-20261026-1418.apk`，126368975 字节，SHA-256 `729d416290b360c97de37eea1cd4dee432f377e1c03ff1b56dfa64f6e83e392f`；`versionCode 22`、`versionName 26.2.0-beta.16`、单 `arm64-v8a`、`apksigner` 正式证书（`119d8ebe…`）、`zipalign` 与内嵌 Runtime 哈希校验通过。
- 定向测试：后端 `go test ./internal/devicemesh/business -run 'TestEditing|TestDeletingConversation'` 通过；手机端 7 个记忆相关测试文件共 36 项全部通过。
- 真机 2407FRK8EC（JV9LT8JB8TVKNJ65）覆盖安装后冷启动 `LaunchState: COLD`、`TotalTime 1645`，`com.amitia.amitia_app.debug` 不存在；新增画像“Hello/preference 分类可选”保存成功，点删除后内置后端 `POST /api/device-mesh/v1/business/resources/edit` 返回 200，画像列表立即回到“暂无画像”，不再出现归属失效提示。
- 完整项目已重建本地 Core（`desktop/scripts/build-amitiacore.mjs`，Source Gate `9029a93f…`）并重启，`/api/health` 返回 `ready=true`、`version=26.2.0-beta.16`、`model=configured`。

### 已知待定问题（未在本轮修改）

用户画像与长期记忆是同一批写入产生的两条记录：`POST /business/memories/manage`（`partition=profile`）同时写 `memory` 与派生 `profile`。当前删除画像只删投影，源记忆仍留在记忆总览；反向删除源记忆时派生画像也不会同步停用。修复需要后端按 `source_id` 建立双向级联，并配套端到端用例，属于数据模型级改动，已作为下一项待确认方案上报，本轮未改动。

## 未发布

本轮只做本地 Release 构建与真机安装验证，未发布更新通道，未推送 Git。
