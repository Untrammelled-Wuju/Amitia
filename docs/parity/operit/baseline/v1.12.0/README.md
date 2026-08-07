# Operit v1.12.0 基线冻结

## 用途

本目录为 Operit / OpenMinis 能力对齐计划B1步骤的输出，包含 Operit v1.12.0 版本的完整基线冻结信息。

## 基线编号

OPR-v1.12.0-fc76cf5

## 不包含Operit完整源码

本目录不包含 Operit 源码文件，仅包含基线元数据和完整性清单。

Operit 源码的临时只读位置：
- `D:\桌面\跟进项目\_parity_sources\Operit-v1.12.0`

## 如何重新验证Tag

```bash
# 克隆仓库
git clone --filter=blob:none --no-checkout https://github.com/AAswordman/Operit.git _parity_sources/Operit-v1.12.0

# 获取并检出 Tag
git -C _parity_sources/Operit-v1.12.0 fetch --force --depth=1 origin tag v1.12.0
git -C _parity_sources/Operit-v1.12.0 checkout --detach v1.12.0

# 验证
git -C _parity_sources/Operit-v1.12.0 rev-parse --short=7 HEAD
# 预期输出: fc76cf5
```

## 如何重新生成哈希

```bash
# 生成源码归档
git -C _parity_sources/Operit-v1.12.0 archive --format=tar --output=operit-v1.12.0.tar v1.12.0

# 计算归档SHA-256
Get-FileHash -Algorithm SHA256 operit-v1.12.0.tar

# 生成逐文件哈希（需先解压归档）
```

## B4开始前必须检查

1. `baseline.json` - 确认基线状态为 FROZEN
2. `git_metadata.json` - 确认 resolved_commit 为 fc76cf5b5086c9ca85eba54384588dccd729315c
3. `source_files.sha256` - 确认 2656 行（2646 哈希 + 10 GITLINK_PTR）
4. `release_metadata.json` - 确认 Release 元数据完整
5. `release_assets.json` - 确认资产清单完整
6. 只读源码目录路径有效

## 禁止自动跟随Operit主分支

本基线锁定在 Tag v1.12.0（commit fc76cf5）。B4扫描必须基于此冻结版本，不得自动拉取或切换到最新主分支。

## 子模块说明

Operit v1.12.0 包含 10 个子模块（submodule），其内容未包含在主仓库快照中。B4如需分析子模块内代码，需单独初始化对应子模块。
