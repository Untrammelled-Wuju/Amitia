# B2 OpenMinis跨平台基线冻结报告

## 1. 执行结果

**状态**: PASS

**结论**: OpenMinis跨平台复合基线已完整冻结。三层基线（源码、Android、iOS）均验证通过，无阻断项，无未确认项。

**执行时间**: 2026-08-07T02:09:46+08:00  
**执行机器**: [用户名已脱敏] Windows 10

## 2. 为什么采用复合基线

OpenMinis 不是单一平台项目，其跨平台能力分布在源码仓库、Android App 和 iOS App 三个独立产品中。仅锁定一个 Tag 无法完整代表：

- Linux Sandbox 能力依赖 commit 中的源码变化，必须锁定源码时间点
- Android Release 包含 APK 资产，必须锁定 Tag 和 Release 元数据
- iOS App Store 版本可能落后于源码，必须单独锁定产品版本

三层基线互相不可替代，任何一个漂移都会导致 B5 扫描结果不可靠。

## 3. 冻结截止时间

| 字段 | 值 |
|------|------|
| 冻结截止时间（北京时间） | 2026-08-07T02:01:00+08:00 |
| 冻结截止时间（UTC） | 2026-08-06T18:01:00Z |
| 实际执行时间（北京时间） | 2026-08-07T02:09:46+08:00 |

执行时间在冻结截止时间之后 8 分 46 秒，符合要求（执行时间不必不晚于截止时间）。

## 4. 源码基线

| 字段 | 值 |
|------|------|
| 仓库 | https://github.com/OpenMinis/OpenMinis.git |
| 默认分支 | main |
| 冻结截止时间前最后提交 | 9cf3a855fecd27bb5735b84cacbd56852a3ab8dd |
| 短提交 | 9cf3a85 |
| 提交时间（UTC） | 2026-07-25T15:21:22Z |
| 提交时间 <= 截止时间 | TRUE |
| 冻结后是否有新提交 | FALSE（当前 main HEAD 即为此提交） |
| Tree SHA | 370a8ae93681b82bc8d7a3cb50b28f1879504bfd |
| 归档 SHA-256 | 6273c9a63ead2409618527d29745eaf478b803fb6cd03bd4f43e3bfd148de783 |
| Git 追踪文件数 | 1604 |
| 已生成文件哈希数 | 1604 |
| 提交信息 | Merge pull request #91 from OpenMinis/v1.10 |
| 提交作者 | Ethan <wsvn53@gmail.com> |

**特殊发现**: 冻结截止时间前最后提交同时也是当前 main HEAD，说明在冻结时间之后 main 分支无新提交。Android Tag 0.22-preview 也指向同一个 commit（三层基线重合）。

## 5. Android Release 基线

| 字段 | 值 |
|------|------|
| Release 名称 | Android 0.22-preview |
| Git Tag | 0.22-preview |
| Tag 类型 | Annotated Tag |
| Tag Object SHA | 28bfbe1886fb6dbb73d35ca30b72e23941bfd41d |
| Peeled Commit SHA | 9cf3a855fecd27bb5735b84cacbd56852a3ab8dd |
| 短提交 | 9cf3a85（与预期一致） |
| 是否为 Prerelease | TRUE |
| 发布时间 | 2026-08-01T12:28:44+08:00 |
| Tree SHA | 370a8ae93681b82bc8d7a3cb50b28f1879504bfd |
| 归档 SHA-256 | 6273c9a63ead2409618527d29745eaf478b803fb6cd03bd4f43e3bfd148de783 |
| 资产数量 | 1 |
| 已计算资产哈希数量 | 1 |
| 资产名称 | MinisApp-0.22-preview-arm64-v8a.apk |
| 资产大小 | 14,325,362 字节（约 13.7 MB） |
| 资产 SHA-256 | 980C123CCB0670FED750877BA77E716F9B926B50F64A9D7561442B1F22FEE47D |

**验证结果**: Android Tag 短提交与预期（9cf3a85）完全匹配，状态确认。

## 6. iOS App Store 基线

| 字段 | 值 |
|------|------|
| App Store 应用 ID | 6759188481 |
| 冻结预期版本 | 1.11 |
| Apple 当前返回版本 | 1.11 |
| 版本验证状态 | VERSION_MATCH |
| Bundle ID | com.openminis.app |
| 应用名称 | Open Minis |
| 最低系统版本 | 17.0 |
| 当前版本发布日期 | 2026-07-31 |
| App Store 链接 | https://apps.apple.com/us/app/open-minis/id6759188481 |

**验证结果**: App Store 返回版本与冻结预期版本完全一致，状态确认。

## 7. Git 与 Tree 校验

| 验证项 | 结果 |
|--------|------|
| 源码提交 = worktree HEAD | TRUE (9cf3a85) |
| 源码提交时间 <= 截止时间 | TRUE |
| Android Tag = 0.22-preview | TRUE |
| Android 短提交 = 9cf3a85 | TRUE |
| Android worktree HEAD = Tag Commit | TRUE |
| 源码 Tree SHA = Android Tree SHA | TRUE |
| 源码归档 SHA-256 | 6273c9a63ead2409618527d29745eaf478b803fb6cd03bd4f43e3bfd148de783 |
| Android 归档 SHA-256 | 6273c9a63ead2409618527d29745eaf478b803fb6cd03bd4f43e3bfd148de783 |

**重要发现**: 源码基线、Android Tag、当前 main HEAD 指向同一个 commit（9cf3a85），三层源码完全一致。

## 8. Android 发布资产

| 资产名 | 类型 | 大小 | SHA-256 |
|--------|------|------|---------|
| MinisApp-0.22-preview-arm64-v8a.apk | application/vnd.android.package-archive | 14,325,362 | 980C123CCB0670FED750877BA77E716F9B926B50F64A9D7561442B1F22FEE47D |

- 资产存储位置: `../_parity_sources/OpenMinis/release-assets/`
- 未进入 Amitia 仓库
- 未安装 APK
- 未运行能力扫描

## 9. Submodule 校验

**父仓库 Submodule 数量**: 4（直接）  
**递归跟踪 Submodule 总数**: 5（包含嵌套）

### 9.1 Submodule 清单

| Submodule 路径 | URL | Source 提交 | Android 提交 | 匹配 |
|----------------|-----|-------------|--------------|------|
| deps/ish | https://github.com/OpenMinis/ish-arm64.git | de124dd | de124dd | TRUE |
| deps/ish/deps/libapps | https://github.com/ish-app/libapps | b8cacae | b8cacae | TRUE |
| deps/ish/deps/libarchive | https://github.com/libarchive/libarchive | fc6563f | fc6563f | TRUE |
| deps/ish/deps/linux | https://github.com/ish-app/linux | 8ec9bf1 | 8ec9bf1 | TRUE（跳过，禁用的 submodule） |
| deps/proot | https://github.com/OpenMinis/proot.git | 8cf13e9 | 8cf13e9 | TRUE |

### 9.2 嵌套 Submodule

`deps/ish` 内部包含嵌套 submodule（libapps、libarchive、linux），全部已递归追踪。

`deps/ish/deps/linux` 是禁用 submodule（skip），在父仓库的 `.gitmodules` 中配置。

### 9.3 Submodule 验证结论

- Source 基线与 Android Tag 的 submodule 提交 **完全一致**（TRUE）
- 4 个有效 submodule 全部初始化
- 1 个禁用 submodule（按设计跳过）
- 所有 submodule 提交均已记录到 submodule_manifest.json

## 10. Git LFS 校验

| 项目 | Source 基线 | Android Tag |
|------|-------------|-------------|
| LFS 文件数 | 0 | 0 |
| .gitattributes 含 filter=lfs | 否 | 否 |
| 缺失 LFS 对象 | 0 | 0 |

**结论**: OpenMinis 不使用 Git LFS，无需额外 LFS 处理。

## 11. License 与第三方许可证

| 文件 | SHA-256 | 大小 | Source 中存在 | Android 中存在 | 内容一致 |
|------|---------|------|:---:|:---:|:---:|
| LICENSE | 230184f60bae2feaf244f10a8bac053c8ff33a183bcc365b4d8b876d2b7f4809 | 35,823 | YES | YES | YES |
| THIRD_PARTY_LICENSES.md | 12bc425b5476c349d2e7f3336c315e5c94d60925b0e710712daeb7ae275eb5ae | 4,342 | YES | YES | YES |

**License 预期**: GPL-3.0  
**验证**: LICENSE 文件头部确认为 GPL-3.0 license。

## 12. 源码完整性

| 字段 | Source | Android |
|------|--------|---------|
| Git 追踪文件数 | 1604 | 1604 |
| 已生成文件哈希数 | 1604 | 1604 |
| 失败哈希数 | 0 | 0 |
| 归档 SHA-256 | 6273c9a6... | 6273c9a6... |
| 哈希算法 | SHA-256 | SHA-256 |

## 13. 冻结时间后的上游增量

| 项目 | 状态 |
|------|------|
| main 分支冻结后提交数 | 0 |
| 当前 main HEAD | 与冻结提交相同 |
| 是否存在更高 Android Release | FALSE（0.22-preview 为最新） |
| 是否存在更高 iOS 版本 | FALSE（当前仍为 1.11） |

**结论**: 冻结时间之后无上游增量，当前状态即为冻结基线。

## 14. 输出文件

```
docs/parity/openminis/baseline/2026-08-07/
├── B2_OpenMinis跨平台基线冻结报告.md      （本报告）
├── baseline.json                            （基线汇总元数据）
├── source_git_metadata.json                 （源码 Git 元数据）
├── android_git_metadata.json                （Android Git 元数据）
├── android_release_metadata.json            （GitHub Release API 原始响应）
├── android_release_assets.json              （Android 资产清单）
├── android_release_assets.json.bak          （备份，如存在）
├── ios_appstore_metadata.json               （iOS App Store 元数据）
├── ios_appstore_metadata_us.json            （US 区 App Store 原始数据）
├── ios_appstore_metadata_cn.json            （CN 区 App Store 原始数据）
├── official_website_metadata.json           （官网元数据）
├── upstream_delta.json                      （上游增量记录）
├── source_archive.sha256                    （源码归档 SHA-256）
├── source_files.sha256                      （源码逐文件 SHA-256）
├── android_archive.sha256                   （Android 归档 SHA-256）
├── android_files.sha256                     （Android 逐文件 SHA-256）
├── license.sha256                           （LICENSE 文件 SHA-256）
├── third_party_licenses.sha256              （第三方许可证 SHA-256）
├── source_submodules.txt                    （Source submodule 状态）
├── android_submodules.txt                   （Android submodule 状态）
├── source_submodules_preinit.txt            （初始化前状态）
├── submodule_manifest.json                  （Submodule 完整清单）
├── source_lfs_inventory.txt                 （Source LFS 检查）
├── android_lfs_inventory.txt                （Android LFS 检查）
├── top_level_tree.txt                       （根目录结构）
├── repository_layout.txt                    （仓库布局描述）
├── verification.log                         （验证日志）
└── README.md                                （使用说明）
```

## 15. 阻断项与未确认项

### 15.1 阻断项

**无**

所有关键验证项均已通过：
- 源码提交唯一解析并验证（时间、commit、tree）
- Android Tag 验证通过（tag = 0.22-preview, 短提交 = 9cf3a85）
- iOS App Store 元数据完整（版本 1.11 确认）
- Submodule 全部初始化并一致

### 15.2 未确认项

**无**

所有预期字段均有确定值。

## 16. B5 输入基线

B5 扫描应使用以下输入，避免漂移：

| 输入项 | 位置 or 值 |
|--------|-----------|
| 源码只读目录 | `../_parity_sources/OpenMinis/worktrees/source-baseline` |
| 源码完整提交 | 9cf3a855fecd27bb5735b84cacbd56852a3ab8dd |
| 源码 Tree SHA | 370a8ae93681b82bc8d7a3cb50b28f1879504bfd |
| Android 只读目录 | `../_parity_sources/OpenMinis/worktrees/android-0.22-preview` |
| Android Tag | 0.22-preview |
| iOS 元数据文件 | `ios_appstore_metadata.json` |
| baseline.json 位置 | `docs/parity/openminis/baseline/2026-08-07/baseline.json` |
| submodule_manifest.json | `docs/parity/openminis/baseline/2026-08-07/submodule_manifest.json` |
| Source 文件哈希 | `source_files.sha256` |
| Android 文件哈希 | `android_files.sha256` |
| Android 资产哈希 | `android_release_assets.json` |
| 源码归档 | `source_archive.sha256` |
| Android 归档 | `android_archive.sha256` |

## 17. 最终结论

**PASS: OpenMinis 跨平台复合基线已完整冻结**

本次冻结三层基线全部验证通过：

1. **源码基线**: 锁定 main 分支在冻结时间前的最后提交 `9cf3a855fecd27bb5735b84cacbd56852a3ab8dd`（2026-07-25）
2. **Android Release**: 验证 Tag `0.22-preview` 指向预期提交 `9cf3a85`，获取完整元数据和 APK 资产
3. **iOS App Store**: 验证当前公开版本为 `1.11`，与冻结预期一致

无阻断项，无未确认项，无上游增量。OpenMinis 源码未进入 Amitia 仓库（位于 `../_parity_sources/OpenMinis/`），Amitia 业务代码零修改。

B5 可以安全读取 `docs/parity/openminis/baseline/2026-08-07/` 目录下的全部文件作为不可漂移输入，并参考本报告锁定 commit 和目录。
