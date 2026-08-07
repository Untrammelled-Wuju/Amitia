# OpenMinis 跨平台基线冻结 (2026-08-07)

## 目录用途

本目录存储 OpenMinis 跨平台对标基线的冻结产物，为后续 B5 能力扫描提供不可漂移的输入。

所有产品与元数据均通过独立于 Amitia 的验证通道获取，未修改任何 Amitia 业务代码。

## 为什么不是只锁定一个 Tag

OpenMinis 是跨平台项目，分布在三个独立产品中：

- **源码仓库** (GitHub): 包含 Linux Sandbox、iSH、PRoot、Skills 等核心实现
- **Android App** (GitHub Release): 公开发布的 APK，包含 Android 特有能力（Runtime Service、JNI 等）
- **iOS App** (App Store): 独立发布版本号，可能落后于源码

单 Tag 无法同时锁定这三层。本次冻结按以下三层推进：

| 层 | 冻结对象 | 验证状态 |
|----|---------|---------|
| 源码基线 | main @ 9cf3a85 (2026-07-25) | 已冻结 |
| Android Release | 0.22-preview (2026-08-01) | 已冻结 |
| iOS App Store | 1.11 (2026-07-31) | 已冻结 |

三层基线重合（源码提交与 Android Tag 指向同一 commit），但流程上仍需分别验证。

## 冻结截止时间

```
2026-08-07T02:01:00+08:00（北京时间）
2026-08-06T18:01:00Z（UTC）
```

## 关键标识符

| 标识符 | 值 |
|--------|------|
| 源码基线提交 | `9cf3a855fecd27bb5735b84cacbd56852a3ab8dd` |
| 源码短提交 | `9cf3a85` |
| Android Tag | `0.22-preview` |
| iOS 冻结版本 | `1.11` |
| baseline_id | `OMN-2026-08-07-CROSSPLATFORM` |

## OpenMinis 源码未进入 Amitia 仓库

OpenMinis 所有源码、Submodule、产物均保存在 Amitia 仓库外部：

```
../_parity_sources/OpenMinis/
├── repository/          # 裸仓库（mirror clone）
├── worktrees/           # 只读 worktree
│   ├── source-baseline/
│   └── android-0.22-preview/
├── release-assets/      # APK 等资产
├── appstore/            # iOS 元数据（预留）
└── temporary/           # 临时的归档文件
```

Amitia 仓库内仅存基线文档、元数据和校验文件，全部位于 `docs/parity/openminis/baseline/2026-08-07/`。

## 如何重新验证

### 验证源码提交

```bash
# 在 ../_parity_sources/OpenMinis/repository 目录
git rev-list -1 --before="2026-08-06T18:01:00Z" main
# 应返回 9cf3a855fecd27bb5735b84cacbd56852a3ab8dd
```

### 验证 Android Release

```bash
curl -s "https://api.github.com/repos/OpenMinis/OpenMinis/releases/tags/0.22-preview"
# 应返回 tag_name=0.22-preview, prerelease=true, assets 包含 APK
```

### 验证 App Store 版本

```bash
curl -s "https://itunes.apple.com/lookup?id=6759188481&country=us"
# 应返回 version=1.11, bundleId=com.openminis.app
```

### 初始化 Submodule

```bash
cd ../_parity_sources/OpenMinis/worktrees/source-baseline
git submodule update --init --recursive
```

## B5 开始前必须读取

1. `baseline.json` - 基线汇总，含所有关键 commit、hash 和资产信息
2. `submodule_manifest.json` - Submodule 完整清单，含递归 commit 验证
3. `source_git_metadata.json` - 源码 Git 元数据
4. `android_git_metadata.json` - Android Tag 元数据
5. `ios_appstore_metadata.json` - iOS App Store 元数据
6. `upstream_delta.json` - 冻结后的上游增量（当前无增量）
7. `B2_OpenMinis跨平台基线冻结报告.md` - 完整冻结报告

## 禁止自动跟随 main

B5 执行中不得：

- 切换到 OpenMinis 新 commit
- 使用不断变化的 origin/main HEAD
- 下载或切换到未冻结的 Android Release
- 假设 iOS App Store 发布了新版本（B2 冻结版本为 1.11）

任何上游变化应记录到 `upstream_delta.json`，不得修改本目录冻结基线。

## 目录输出清单

必输出（已全部生成）：
- baseline.json
- source_git_metadata.json
- android_git_metadata.json
- android_release_metadata.json
- android_release_assets.json
- ios_appstore_metadata.json
- official_website_metadata.json
- upstream_delta.json
- source_archive.sha256
- source_files.sha256
- android_archive.sha256
- android_files.sha256
- license.sha256
- third_party_licenses.sha256
- source_submodules.txt
- android_submodules.txt
- submodule_manifest.json
- source_lfs_inventory.txt
- android_lfs_inventory.txt
- top_level_tree.txt
- repository_layout.txt
- verification.log
- README.md
- B2_OpenMinis跨平台基线冻结报告.md

## 联系方式

B2 执行完成后，如对基线有疑问，请查阅冻结报告或联系 Amitia 项目维护者。
