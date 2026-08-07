# B1 Operit基线冻结报告

## 1. 执行结果

PASS：Operit v1.12.0基线已完整冻结。

## 2. 固定基线

| 字段 | 值 |
| --- | --- |
| 对标项目 | Operit |
| 上游仓库 | https://github.com/AAswordman/Operit.git |
| Git Tag | v1.12.0 |
| Release名称 | Operit AI V1.12.0 |
| 预期提交短哈希 | fc76cf5 |
| 解析提交短哈希 | fc76cf5 |
| 完整提交 | fc76cf5b5086c9ca85eba54384588dccd729315c |
| Tree哈希 | 75545711ba67d87a2dc863b6685f6ae84626d465 |
| Release发布日期 | 2026-07-01T14:47:17Z |
| 基线冻结日期 | 2026-08-07 |
| 基线编号 | OPR-v1.12.0-fc76cf5 |

## 3. Git验证结果

- Tag名称：v1.12.0
- Tag类型：lightweight（直接指向commit对象）
- Tag Object：fc76cf5b5086c9ca85eba54384588dccd729315c
- 完整提交：fc76cf5b5086c9ca85eba54384588dccd729315c
- 短提交（7位）：fc76cf5
- Tree哈希：75545711ba67d87a2dc863b6685f6ae84626d465
- 工作区状态：clean
- 远程仓库：https://github.com/AAswordman/Operit.git
- 提交主题：1.12.0
- 作者日期：2026-07-01T17:57:33+08:00
- 提交日期：2026-07-01T17:57:33+08:00

## 4. Release验证结果

- Release ID：347505424
- Tag名称：v1.12.0
- Release名称：Operit AI V1.12.0
- Draft：false
- Prerelease：false
- 创建时间：2026-07-01T09:57:33Z
- 发布时间：2026-07-01T14:47:17Z
- Target Commitish：main
- Release说明长度：5262字符（含多语言）
- Reactions总计：56

## 5. 发布资产

| 名称 | 大小 | 内容类型 | SHA-256 |
| --- | --- | --- | --- |
| app-release.apk | 398753547 bytes | application/vnd.android.package-archive | ec0db6374e721ea810e4df0c184d2a59b59c1a5c46a879577b07e9a69680bfdb |

额外资产（GitHub自动生成）：
- tarball_url: https://api.github.com/repos/AAswordman/Operit/tarball/v1.12.0
- zipball_url: https://api.github.com/repos/AAswordman/Operit/zipball/v1.12.0

注意：官方APK未下载到本地，SHA-256由GitHub API提供。

## 6. 源码完整性

- Git追踪文件总数：2656
- 成功计算SHA-256的文件数：2646
- GITLINK_PTR条目（submodule指针）：10
- 失败文件数：0
- 源码归档SHA-256：CEA0156B56F024B6A6DB9394E1B028EBEF3D4E7F19027E836183FDEB44F77AE0
- 归档大小：123392000 bytes
- 归档方式：git archive --format=tar（确定性生成）

## 7. License状态

- License文件存在：是
- 文件路径：LICENSE
- SHA-256：40C41F5F4ED9E2F268A93DF3AE779C8F258F63A779B14922DDCAE6D296D96FF4
- 备注：材质为项目根目录的LICENSE文件

## 8. Submodule状态

- 子模块总数：10
- 初始化状态：uninitialized（未初始化）
- 影响：源码快照中的submodule目录为空，已以GITLINK_PTR标记
- 子模块列表：
  - app/src/main/cpp/thirdparty/ncnn (c4193aadbbb56582aa87b1850dd3d98fb8fd936d)
  - app/src/main/cpp/thirdparty/sherpa-ncnn (c61e50d61e9fbed5972afa4d95bc560e168affe2)
  - fbx/third_party/ufbx (83bc7cf44f76bc8622de63b809a42b5d557cd733)
  - llama/third_party/llama.cpp (b7ad48ebda2287c778fd826606d7b3b3570f60ab)
  - mmd/third_party/bullet3 (63c4d67e337017f9d8b298c900e9aabdb69296e7)
  - mmd/third_party/saba (29b8efa8b31c8e746f9a88020fb0ad9dcdcf3332)
  - mnn/src/main/cpp/MNN (e96280a6ebea86bfab7b6cd17ff1780d92f2a188)
  - quickjs/thirdparty/quickjs (f1139494d18a2053630c5ed3384a42bb70db3c53)
  - terminal (ad7c16ecbe1f13584bccad16ec40155a7a824496)
  - tools/hotbuild/OperitNightlyRelease (350165237e89b370e45213dcd7ab02ee7e7a6361)

## 9. Git LFS状态

- .gitattributes文件：不存在
- Git LFS文件数：0
- 检测方式：检查.gitattributes是否存在

## 10. 输出文件

docs/parity/operit/baseline/v1.12.0/ 目录下共13个文件：

1. B1_Operit基线冻结报告.md - 本报告
2. baseline.json - 基线核心元数据
3. release_metadata.json - 官方Release原始元数据
4. git_metadata.json - Git验证元数据
5. release_assets.json - 发布资产清单
6. source_archive.sha256 - 源码归档SHA-256
7. source_files.sha256 - 逐文件SHA-256清单（2656行）
8. license.sha256 - License文件SHA-256
9. top_level_tree.txt - 顶层目录结构
10. submodules.txt - 子模块清单
11. lfs_inventory.txt - LFS清单（NONE）
12. verification.log - 验证日志
13. README.md - 目录说明

## 11. 阻断项与未确认项

无。

所有验证项已通过：
- 仓库验证通过（AAswordman/Operit）
- Tag验证通过（v1.12.0）
- 提交验证通过（fc76cf5）
- Release元数据已保存
- 源码归档SHA-256已生成
- 逐文件哈希完整（2646文件哈希 + 10 GITLINK_PTR）
- License状态已记录
- Submodule状态已记录
- Git LFS状态已记录

## 12. B4输入基线

B4阶段应从以下只读源码目录开始扫描：

- 只读源码目录：D:\桌面\跟进项目\_parity_sources\Operit-v1.12.0
- Tag：v1.12.0
- 完整提交：fc76cf5b5086c9ca85eba54384588dccd729315c
- baseline.json位置：docs/parity/operit/baseline/v1.12.0/baseline.json

注意：
- Submodule内容未包含在主仓库快照中（需B4决定是否初始化）
- 源码归档位于 D:\桌面\跟进项目\_parity_sources\operit-v1.12.0.tar
- 逐文件哈希位于 D:\桌面\跟进项目\_parity_sources\source_files.sha256
- Git追踪文件清单包含10个GITLINK_PTR条目，B4分析时需注意

## 13. 最终结论

PASS：Operit v1.12.0基线已完整冻结。所有B1验收条件均已满足：
- 上游仓库正确
- Tag精确为v1.12.0
- 提交短哈希精确为fc76cf5
- 完整SHA已解析并记录
- Tree SHA已解析并记录
- Release元数据已保存
- 资产清单已保存
- 源码归档SHA-256已生成
- 所有Git追踪文件均有SHA-256（或GITLINK_PTR标记）
- License路径与SHA-256已记录
- Submodule状态已确认
- Git LFS状态已确认（无LFS）
- Operit源码未进入Amitia仓库
- Amitia业务代码零修改
- B4边界未执行能力扫描
