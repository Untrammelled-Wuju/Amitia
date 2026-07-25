# 后端 Package & .amitiax v1 分类

> 范围：`backend/internal/extension/package_archive.go`, `package_parser.go`, `package_service.go`, `package_installer.go`, `package_lifecycle.go`, `package_recovery.go`, `package_repository.go`, `package_handler.go`, `package_protocol.go`, `package_test_runner.go`, `schema/manifest.schema.json`

---

## 一、保留并抽取

### PKG-001: 归档安全
- **文件**: `package_archive.go`
- **类型/函数**: `readPackageZIP`, `validatePackagePath`, `validatePackageFile`, `stablePackageZIP`
- **当前职责**: ZIP 安全读写
- **目标分类**: 保留并抽取
- **判定依据**: 归档安全是通用能力
- **目标组件**: Package Manager
- **抽取目标**: 独立安全归档包

### PKG-002: Checksum
- **文件**: `package_archive.go`
- **类型/函数**: `buildChecksums`, `validateChecksums`, `packageCanonicalDigest`, `packageHash`
- **当前职责**: 文件校验和
- **目标分类**: 保留并抽取
- **判定依据**: 通用数据完整性校验
- **目标组件**: Package Manager
- **抽取目标**: 独立 Checksum 工具

### PKG-003: 签名验证
- **文件**: `package_archive.go`
- **类型/函数**: `verifyPackageSignature`, `packageSignatureDocument`
- **当前职责**: 扩展包签名验证
- **目标分类**: 保留并抽取
- **判定依据**: 通用数字签名
- **目标组件**: Package Manager
- **抽取目标**: 独立签名验证器

### PKG-004: 版本比较
- **文件**: `package_protocol.go` (semver 逻辑)
- **当前职责**: 版本号比较
- **目标分类**: 保留并抽取
- **判定依据**: 通用版本比较
- **目标组件**: Package Manager
- **抽取目标**: 独立 semver 包

### PKG-005: 安装事务
- **文件**: `package_installer.go`
- **类型/函数**: `Install`, 事务性安装逻辑
- **当前职责**: 原子安装
- **目标分类**: 保留并抽取
- **判定依据**: 通用事务模式
- **目标组件**: Package Manager
- **抽取目标**: 通用安装事务管理器

### PKG-006: 补偿/回滚
- **文件**: `package_installer.go`, `package_recovery.go`
- **类型/函数**: 安装失败补偿、回滚
- **当前职责**: 安装回滚
- **目标分类**: 保留并抽取
- **判定依据**: 通用回滚能力
- **目标组件**: Package Manager
- **抽取目标**: 独立回滚管理器

### PKG-007: 恢复
- **文件**: `package_recovery.go`
- **类型/函数**: 启动恢复逻辑
- **当前职责**: 包恢复
- **目标分类**: 保留并抽取
- **判定依据**: 通用恢复能力
- **目标组件**: Package Manager
- **抽取目标**: 独立恢复管理器

### PKG-008: 卸载预览
- **文件**: `package_lifecycle.go`
- **类型/函数**: 卸载影响分析
- **当前职责**: 预览卸载后果
- **目标分类**: 保留并抽取
- **判定依据**: 通用依赖分析
- **目标组件**: Dependency Resolver

### PKG-009: 文件类型限制
- **文件**: `package_archive.go`
- **类型/函数**: `packageFileKind`
- **当前职责**: 文件类型识别
- **目标分类**: 保留并抽取
- **判定依据**: 通用文件类型校验
- **目标组件**: Package Manager

---

## 二、改造后复用

### PKG-101: PackageService
- **文件**: `package_service.go`
- **类型/函数**: `PackageService`, 安装/升级/导出/卸载
- **当前职责**: 包管理服务
- **目标分类**: 改造后复用
- **判定依据**: 服务能力正确，但绑定旧 Manifest
- **目标组件**: Package Manager
- **目标新模型**: v2 Manifest 支持

### PKG-102: Artifact Store
- **文件**: `package_repository.go`
- **类型/函数**: `packageArtifactRecord`, `packageVersionRecord`
- **当前职责**: Artifact 存储
- **目标分类**: 改造后复用
- **判定依据**: 存储逻辑正确
- **目标组件**: Package Store

### PKG-103: 依赖解析
- **文件**: `package_installer.go`
- **类型/函数**: 依赖检查
- **当前职责**: 包依赖解析
- **目标分类**: 改造后复用
- **判定依据**: 依赖解析逻辑正确
- **目标组件**: Dependency Resolver

### PKG-104: Operation Audit
- **文件**: `package_repository.go`
- **类型/函数**: `packageOperationRecord`
- **当前职责**: 安装操作记录
- **目标分类**: 改造后复用
- **判定依据**: 审计模式正确
- **目标组件**: Audit Store

### PKG-105: Config Migration
- **文件**: `package_installer.go`
- **类型/函数**: `packageConfigMigration`
- **当前职责**: 配置迁移
- **目标分类**: 改造后复用
- **判定依据**: 配置迁移逻辑
- **目标组件**: Migration Manager

---

## 三、仅用于迁移

### PKG-201: Manifest v1 Parser
- **文件**: `package_parser.go`
- **类型/函数**: v1 Manifest 解析
- **当前职责**: .amitiax v1 格式解析
- **目标分类**: 仅用于迁移
- **迁移来源**: v1 扩展包
- **迁移目标**: v2 Manifest Parser
- **删除条件**: 旧包全部转换

### PKG-202: 旧 Package API
- **文件**: `package_handler.go`
- **类型/函数**: 所有 HTTP handler
- **当前职责**: v1 包 API
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧 API
- **迁移目标**: 新 Extension Kernel API
- **删除条件**: 新 API 就绪

### PKG-203: 旧安装记录
- **文件**: `package_repository.go`
- **类型/函数**: `extension_package_installations` CRUD
- **当前职责**: 旧安装历史
- **目标分类**: 仅用于迁移
- **迁移来源**: `extension_package_installations`
- **迁移目标**: 新 Package Store
- **删除条件**: 安装记录迁移完成

### PKG-204: 旧 Artifact/Export
- **文件**: `package_repository.go`
- **类型/函数**: `extension_package_exports`, `extension_artifacts`
- **当前职责**: 旧导出和 Artifact
- **目标分类**: 仅用于迁移
- **迁移来源**: 旧 Artifact
- **迁移目标**: 新 Artifact Store
- **删除条件**: Artifact 迁移完成

### PKG-205: Manifest Schema v1
- **文件**: `schema/manifest.schema.json`
- **当前职责**: v1 Schema 校验
- **目标分类**: 仅用于迁移
- **迁移来源**: v1 格式校验
- **迁移目标**: v2 Schema
- **删除条件**: v1 包全迁移

---

## 四、最终删除

### PKG-301: 旧 Parser 主路径
- **文件**: `package_parser.go`
- **类型/函数**: `parseExtensionPackage` 主流程
- **当前职责**: v1 解析
- **目标分类**: 最终删除
- **替代组件**: v2 Manifest Parser

### PKG-302: 二选一包模型
- **文件**: `package_parser.go`, `package_installer.go`
- **类型/函数**: Workflow/Instructions 二选一
- **当前职责**: 只支持两种包类型
- **目标分类**: 最终删除
- **替代组件**: 多类型 Capability/Contribution 包

### PKG-303: Manifest 中未接通的 Plugin 分支
- **文件**: `package_parser.go`
- **当前职责**: 未实现的 Plugin 解析分支
- **目标分类**: 最终删除
- **替代组件**: 新 Plugin 体系

### PKG-304: 硬编码分支
- **文件**: `package_installer.go`
- **类型/函数**: `installWorkflowPackage`, `installInstructionsPackage`
- **当前职责**: 硬编码分支
- **目标分类**: 最终删除
- **替代组件**: 统一安装器 + 类型分发

### PKG-305: PackageHandler
- **文件**: `package_handler.go`
- **类型/函数**: `PackageHandler`
- **当前职责**: Package HTTP API
- **目标分类**: 最终删除
- **替代组件**: Extension Kernel HTTP API
