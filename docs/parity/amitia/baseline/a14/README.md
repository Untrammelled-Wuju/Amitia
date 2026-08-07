# Amitia A14 Baseline Documentation

## 目录用途

本目录包含B3任务生成的Amitia项目在A14完成后的源码基线冻结文档。

**重要**: 这是A/B双线同步推进计划中的B3步骤输出，仅冻结工程状态，不包含能力完成度分析。

## 基线编号

- **Baseline ID**: AMT-A14-3daeaf3
- **Project**: Amitia
- **Baseline Step**: A14
- **Freeze Time**: 2026-08-07T00:00:00+08:00
- **Source Mode**: WORKTREE_SNAPSHOT

## Git信息

- **Repository Root**: D:/桌面/跟进项目/U-Ai
- **Branch**: develop
- **Commit**: 3daeaf3c0a82e33213e0a52d84cfaf8f68f78eab
- **Short Commit**: 3daeaf3
- **Tree SHA**: 1f49ab81759aa83ac53a2deffe2e471dafe2

## A14状态

**UNVERIFIED** - 当前未找到A14完成的直接证据。

A14要求验证:
- Linux ARM64 Go交叉编译
- Linux AMD64 Go交叉编译
- Windows AMD64 Go回归构建
- CGO依赖检查
- 基础启动检查

## 源码包

mitia-a14-source.tar 包含Git追踪文件树（HEAD提交的纯归档），SHA-256已记录在 source_archive.sha256 中。

**注意**: 不包含未追踪文件（4个 .go 源文件 + 3个构建产物）。

## 如何验证

### 重新验证文件哈希

`ash
# Windows PowerShell
Get-Content docs\parity\amitia\baseline\a14\source_files.sha256 | ForEach-Object {
     =  -split '  ', 2
     = [0]
     = [1]
     = (Get-FileHash -Path  -Algorithm SHA256).Hash.ToLower()
    if ( -eq ) { "OK: " } else { "FAIL: " }
}
`

### 重新验证依赖清单

`powershell
Get-Content docs\parity\amitia\baseline\a14\dependency_locks.sha256 | ForEach-Object {
     =  -split '  ', 2
     = [0]
     = [1]
     = (Get-FileHash -Path  -Algorithm SHA256).Hash.ToLower()
    if ( -eq ) { "OK: " } else { "FAIL: " }
}
`

### 重新生成源码归档

`ash
git archive --format=tar --output=amitia-a14-source.tar HEAD
`

## B6开始前必须读取的文件

在开始B6能力扫描前，必须读取以下文件:

1. aseline.json - 基线唯一标识和完整性信息
2. epository_metadata.json - 仓库元数据
3. source_roots.json - 所有源码根位置
4. component_manifest.json - 组件结构和入口文件
5. source_files.sha256 - 文件完整性基线
6. dependency_manifests.json - 依赖约束
7. 14_verification.json - A14验证状态

## B3不包含以下内容

- Agent能力完成度分析
- Operit/OpenMinis能力对比
- 三方能力矩阵
- 功能缺失清单
- 代码修改建议
- 测试结果判定

## 后续源码变化

如果源码发生任何变化（新增文件、修改代码、更新依赖）:

1. **不得覆盖** 当前基线文件
2. 应创建新的基线目录（如 15/）
3. 更新 aseline.json 中的状态为 SUPERSEDED
4. 记录变更原因和时间

## 联系与反馈

本基线由CatPaw B3 Agent自动生成。如有疑问，请参考:
- B3任务定义文档
- erification.log 中的执行记录
- 原始GitHub/GitLab提交历史
