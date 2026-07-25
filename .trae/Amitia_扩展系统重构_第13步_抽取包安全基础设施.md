# Amitia 扩展系统重构第 13 步实施文档

## 第 13 步：抽取包安全基础设施

---

## 一、步骤目标

在第 12 步已经建立统一资源所有权模型的基础上，从当前 `.amitiax` v1 Package、Agent Skill ZIP 导入、Workshop Artifact、MCP 依赖安装和其他本地扩展导入链路中，抽取一套独立于旧 Package 领域模型的 Package Security Infrastructure。

本步骤的目标是：

> 将所有扩展包、插件包、Agent Skill 包、Workflow 包、资源包和未来 `.amitiax` v2 包共用的归档安全、内容完整性、签名验证、发布者信任、临时目录、原子落盘、安装补偿、版本快照和回滚安全能力统一抽取出来。

当前系统中相关能力可能分散在：

- `package_archive.go`；
- `package_parser.go`；
- `package_installer.go`；
- `package_lifecycle.go`；
- `package_recovery.go`；
- Agent Skill ZIP 导入；
- Workshop 安装；
- MCP 依赖包；
-前端本地文件导入；
- Electron 文件选择与数据目录处理；
-配置加密与签名；
-各自独立的临时目录和清理逻辑。

如果继续由各业务模块分别处理，将长期存在：

- 路径穿越防护不一致；
- ZIP 炸弹检查不一致；
- 符号链接处理不一致；
- Checksum 计算规则不同；
- 签名验证只覆盖部分文件；
- Manifest 与资源内容未绑定；
- 发布者信任与包安装耦合；
- 安装失败后临时文件残留；
- 原子替换边界不一致；
- 回滚快照无法保证完整；
- Agent Skill 导入绕过 Package 安全；
- Workshop 产物绕过签名和内容校验；
- Windows、macOS、Linux 路径规则不一致；
- 包解压后才发现超限，造成磁盘和内存风险；
- 安装器直接信任文件扩展名；
- 同一包在预览和安装阶段解析结果可能不同。

本步骤完成后，系统必须形成统一安全处理链：

```text
Untrusted Input
→ Source Acquisition
→ Size & Type Gate
→ Archive Inspection
→ Path Normalization
→ Entry Policy Validation
→ Content Hashing
→ Manifest Binding
→ Signature Verification
→ Publisher Trust Evaluation
→ Secure Extraction
→ Immutable Staging
→ Atomic Commit
→ Rollback Snapshot
→ Cleanup & Audit
```

---

## 二、职责边界

Package Security Infrastructure 负责：

- 输入来源识别；
- 文件大小限制；
- 文件类型识别；
- MIME 与 Magic Number 检查；
- 归档结构检查；
- 压缩比检查；
- 文件数量限制；
- 单文件大小限制；
- 总解压大小限制；
- 路径规范化；
- 路径穿越防护；
- 绝对路径防护；
- 符号链接与硬链接防护；
- 特殊文件防护；
- 文件名冲突防护；
- Unicode 规范化；
- 大小写冲突检测；
- Checksum；
- 内容树 Hash；
- Manifest 与内容绑定；
- 签名验证；
- 发布者身份与信任；
- 临时目录；
- Secure Extraction；
- 只读 Staging；
- 原子 Commit；
- 失败补偿；
- 版本快照；
- 回滚基础能力；
- 临时资源清理；
- 安全审计；
- 安全报告。

Package Security Infrastructure 不负责：

- 解释具体 `.amitiax` Manifest 业务字段；
- 决定 Tool、Agent Skill、Workflow 或 Plugin 如何注册；
- 管理 Extension 生命周期；
- 管理 Permission Grant；
- 管理 Scope Binding；
- 管理 Runtime；
- 决定用户是否安装；
- 处理前端页面；
- 管理 MCP 连接；
- 执行插件代码；
- 处理扩展业务数据迁移。

---

## 三、目标组件

建议拆分为：

```text
PackageSecurity
├── PackageSourceReader
├── FileTypeDetector
├── ArchiveInspector
├── ArchivePolicy
├── SafePathResolver
├── EntryValidator
├── ContentHasher
├── ManifestBindingVerifier
├── SignatureVerifier
├── PublisherTrustService
├── SecureExtractor
├── StagingManager
├── AtomicCommitter
├── SnapshotManager
├── RollbackCoordinator
├── CleanupManager
├── SecurityReportBuilder
└── SecurityAuditWriter
```

每个组件必须：

- 可独立测试；
- 不依赖旧 `PackageService`；
- 不依赖 `SkillDefinition`；
- 不依赖 Plugin Runtime；
- 不直接写扩展业务表；
- 不直接调用前端；
- 不直接启用扩展。

---

## 四、输入来源模型

建议定义：

```go
type PackageSource struct {
    SourceType   PackageSourceType
    LocalPath    string
    DisplayName  string
    Origin       string
    ExpectedSize int64
    Metadata     map[string]any
}
```

`PackageSourceType`：

```text
local_file
uploaded_file
workshop_artifact
generated_artifact
migration_artifact
remote_download
system_bundle
```

本步骤至少实现：

- local_file；
- uploaded_file；
- workshop_artifact；
- migration_artifact；
- system_bundle。

远程下载可预留接口，但不在本步骤实现完整商店下载。

---

## 五、不可信输入原则

所有外部包都必须视为不可信，包括：

- 用户本地选择的 `.amitiax`；
- ZIP；
- Agent Skill 包；
- Workshop 生成包；
- 从旧系统迁移的 Artifact；
- 官方包；
- 本地开发者包；
- 远程下载包；
- 已签名包。

签名通过只说明：

- 内容与签名匹配；
-签名身份可验证。

签名通过不代表：

- 包一定安全；
-权限一定合理；
-运行时代码可信；
-可以绕过归档检查；
-可以绕过资源限制；
-可以自动安装。

---

## 六、文件类型识别

禁止只根据扩展名判断包类型。

必须同时检查：

```text
extension
MIME
magic number
archive structure
manifest presence
```

示例：

```text
文件名：example.amitiax
扩展名：.amitiax
Magic Number：ZIP
Archive Root：存在 manifest.json
```

任何不一致必须：

- 拒绝；
-或标记高风险并禁止继续安装。

不允许：

- `.amitiax` 实际为可执行文件；
- ZIP 中嵌套原生可执行文件却未声明；
-伪装图片；
-伪装文本；
-错误 MIME 被静默接受。

---

## 七、归档策略

建议定义：

```go
type ArchivePolicy struct {
    MaxArchiveBytes          int64
    MaxEntryCount            int
    MaxSingleEntryBytes      int64
    MaxTotalUncompressedBytes int64
    MaxCompressionRatio      float64
    MaxPathLength            int
    MaxDirectoryDepth        int
    AllowedFileTypes         []string
    ForbiddenFileTypes       []string
    AllowSymlink             bool
    AllowHardlink            bool
    AllowNestedArchive       bool
    AllowExecutable          bool
}
```

不同包类型可以有不同 Policy，但必须由统一系统执行。

---

## 八、必须阻止的归档风险

至少覆盖：

### 1. 路径穿越

```text
../
..\ 
%2e%2e/
Unicode 变体
重复分隔符
```

### 2. 绝对路径

```text
C:\...
\\server\share
/var/...
~/...
```

### 3. 符号链接

防止解压后指向：

- 包外目录；
-用户目录；
-系统目录；
-其他扩展目录；
-Secret 目录。

第一阶段默认禁止。

### 4. 硬链接

默认禁止。

### 5. 特殊文件

禁止：

- 设备文件；
-管道；
-Socket；
-Block Device；
-Character Device；
-NTFS Alternate Data Stream；
-macOS 特殊资源路径滥用。

### 6. ZIP 炸弹

检查：

- 压缩比；
-总解压大小；
-嵌套归档；
-重复 Entry；
-异常 Header；
-Data Descriptor；
-递归压缩。

### 7. 文件覆盖

禁止不同 Entry 在规范化后指向同一路径。

### 8. Unicode 冲突

必须处理：

- NFC/NFD；
-全角字符；
-不可见字符；
-大小写折叠；
-尾随空格；
-尾随点；
-Windows 保留名称。

---

## 九、跨平台路径规范

必须统一处理 Windows、macOS、Linux。

### Windows

禁止或处理：

```text
CON
PRN
AUX
NUL
COM1-COM9
LPT1-LPT9
尾随点
尾随空格
ADS 路径
盘符
UNC
超长路径
```

### macOS

处理：

- NFD；
-大小写不敏感文件系统；
-Resource Fork；
-`.DS_Store`；
-扩展属性。

### Linux

处理：

- 大小写敏感；
-符号链接；
-设备文件；
-权限位；
-可执行位。

包在任一受支持桌面平台存在冲突时，应在安装预览中明确提示。

---

## 十、SafePathResolver

建议接口：

```go
type SafePathResolver interface {
    NormalizeArchivePath(path string) (NormalizedPath, error)

    ResolveWithinRoot(
        root string,
        normalized NormalizedPath,
    ) (string, error)

    DetectCollision(
        paths []NormalizedPath,
        platform Platform,
    ) []PathCollision
}
```

要求：

- 规范化与实际落盘使用同一逻辑；
-预览和安装使用同一结果；
-禁止 `filepath.Clean` 后就直接认为安全；
-必须确认最终路径仍在 Root 内；
-必须防止链接跳转；
-必须处理平台大小写规则；
-必须提供可测试的纯函数部分。

---

## 十一、Entry Validation

每个归档 Entry 必须检查：

- 路径；
-类型；
-大小；
-压缩大小；
-权限位；
-可执行位；
-文件后缀；
-MIME；
-Hash；
-是否 Manifest 声明；
-是否重复；
-是否允许；
-是否属于模块；
-是否是隐藏文件；
-是否是临时文件；
-是否包含 Secret。

建议定义：

```go
type ArchiveEntryInfo struct {
    Path             string
    NormalizedPath   string
    Kind             EntryKind
    CompressedSize   int64
    UncompressedSize int64
    Mode             uint32
    MIMEType         string
    Hash             string
}
```

---

## 十二、内容 Hash

必须支持：

### 1. 单文件 Hash

建议：

```text
SHA-256
```

### 2. 内容树 Hash

对所有 Entry 按规范化路径排序后计算：

```text
path
size
file hash
mode policy
```

得到：

```text
content_tree_hash
```

### 3. 包文件 Hash

对原始归档计算：

```text
archive_hash
```

三个 Hash 用途不同：

- archive_hash：确认原始文件未变；
- content_tree_hash：确认解压内容未变；
- entry_hash：确认单文件。

---

## 十三、Manifest 与内容绑定

Manifest 必须明确绑定内容树。

建议未来 Manifest 包含：

```json
{
  "integrity": {
    "algorithm": "sha256",
    "contentTreeHash": "...",
    "files": {
      "modules/main/index.js": "...",
      "skills/weather/SKILL.md": "..."
    }
  }
}
```

当前 `.amitiax` v1 若没有该字段：

- 可由 Security Layer 生成内部内容清单；
- 标记为 legacy_integrity；
-不得伪装成签名完整性；
-迁移后由 v2 重新生成。

---

## 十四、签名模型

建议定义：

```go
type PackageSignature struct {
    Algorithm      string
    KeyID          string
    PublisherID    string
    SignedAt       time.Time
    ManifestHash   string
    ContentTreeHash string
    Signature      []byte
}
```

签名至少必须覆盖：

- Manifest；
-内容树 Hash；
-包 ID；
-版本；
-发布者 ID；
-签名算法；
-签名时间；
-宿主兼容声明。

不得只签 Manifest 文件而不绑定资源内容。

---

## 十五、签名验证

建议接口：

```go
type SignatureVerifier interface {
    Verify(
        ctx context.Context,
        input SignatureVerificationInput,
    ) SignatureVerificationResult
}
```

结果至少包含：

```text
unsigned
valid
invalid
unknown_key
revoked_key
expired_key
publisher_mismatch
content_mismatch
unsupported_algorithm
```

签名无效必须阻止安装。

未签名包是否允许安装，由后续策略和开发者模式决定，但必须明确风险。

---

## 十六、密钥与发布者身份

发布者身份不得只使用显示名称。

建议稳定标识：

```text
publisher_id
key_id
```

必须支持：

- 多密钥；
-密钥轮换；
-密钥撤销；
-过期；
-证书链或可信根；
-本地开发密钥；
-系统官方密钥；
-用户信任密钥。

私钥不应由 Amitia 普通安装流程管理。

Amitia 主要保存：

- 公钥；
-信任状态；
-撤销状态；
-发布者元数据。

---

## 十七、Publisher Trust Service

建议：

```go
type PublisherTrustService interface {
    Evaluate(
        ctx context.Context,
        publisherID string,
        keyID string,
        signatureResult SignatureVerificationResult,
    ) PublisherTrustResult

    Trust(
        ctx context.Context,
        request PublisherTrustRequest,
    ) error

    RevokeTrust(
        ctx context.Context,
        publisherID string,
        keyID string,
    ) error
}
```

信任等级可包括：

```text
official
trusted
user_trusted
unknown
blocked
revoked
development
```

信任不等于权限授权。

---

## 十八、可信发布者规则

即使发布者为 official：

- 仍必须做归档检查；
-仍必须做权限差异；
-仍必须做版本兼容；
-仍必须做资源限制；
-仍必须通过安装事务；
-仍必须写审计。

可信发布者可影响：

- 是否显示额外警告；
-是否允许自动更新；
-是否允许某些受限 Contribution；
-是否允许后台更新检查。

不得影响：

- Path Traversal；
-ZIP Bomb；
-签名内容不匹配；
-不支持的平台；
-宿主版本不兼容。

---

## 十九、Secure Extraction

解压必须进入独立 Staging Root。

流程：

```text
Create secure temp root
→ Set restrictive permissions
→ Extract one entry at a time
→ Re-check path
→ Enforce size limits
→ Write with exclusive create
→ Verify written hash
→ Normalize permissions
→ Final scan
→ Mark staging immutable
```

禁止：

- 直接解压到最终扩展目录；
-覆盖已有文件；
-信任归档库默认行为；
-先全量解压再检查；
-沿用归档中的高权限位；
-保留 setuid/setgid；
-创建链接；
-执行解压后的代码。

---

## 二十、Staging Manager

建议：

```go
type StagingManager interface {
    Create(ctx context.Context, purpose string) (StagingArea, error)
    Seal(ctx context.Context, stagingID string) error
    Verify(ctx context.Context, stagingID string) error
    Cleanup(ctx context.Context, stagingID string) error
}
```

StagingArea 必须记录：

- ID；
-路径；
-Owner；
-来源；
-创建时间；
-过期时间；
-状态；
-内容 Hash；
-清理责任。

Staging 目录属于 Temporary Resource，纳入第 12 步资源所有权模型。

---

## 二十一、不可变 Staging

完成安全解压后，Staging 内容必须被视为不可变。

后续：

- Parser；
-Preview；
-Signature；
-Install；

必须读取同一份 Sealed Staging。

不得：

- 预览后重新读取原始包；
-安装时再次解压；
-前端确认期间允许文件被替换；
-使用用户原始路径继续安装。

---

## 二十二、预览与安装一致性

必须建立：

```text
import_session_id
archive_hash
content_tree_hash
staging_id
security_report_id
```

用户确认安装时必须重新验证：

- Session 未过期；
-Staging 未被修改；
-Hash 一致；
-Security Report 一致；
-Permission Diff 一致；
-Publisher Trust 未变化；
-宿主版本未变化到不兼容状态。

若任何关键条件变化，必须重新预览。

---

## 二十三、Atomic Commit

建议：

```go
type AtomicCommitter interface {
    Commit(
        ctx context.Context,
        request CommitRequest,
    ) CommitResult
}
```

Commit 流程：

```text
1. Verify sealed staging
2. Verify target ownership
3. Create rollback snapshot
4. Write pending database state
5. Atomically move/copy content
6. Verify final content
7. Commit database transaction
8. Activate resource
9. Mark snapshot retained
10. Cleanup staging
```

跨磁盘或跨文件系统无法原子 rename 时：

- 必须使用 copy + fsync + verify + swap；
-不得假装是原子操作；
-失败时执行补偿。

---

## 二十四、文件系统持久化保证

需要根据平台使用：

- 临时文件；
-exclusive create；
-fsync；
-目录 fsync；
-原子 rename；
-权限设置；
-文件锁；
-Hash 校验。

若平台无法完全保证：

- 必须在文档中明确边界；
-使用恢复扫描；
-不得声称绝对原子。

---

## 二十五、Snapshot Manager

升级、回滚和替换前必须生成 Snapshot。

Snapshot 包含：

- 当前 Artifact；
-内容 Hash；
-Manifest；
-资源所有权；
-引用图；
-Scope；
-Permission Requirement；
-配置快照；
-版本；
-数据库关联；
-运行时绑定。

Snapshot 不应包含：

- 不必要的 Secret 明文；
-运行中连接；
-不可序列化闭包；
-临时 Cache。

---

## 二十六、Rollback Coordinator

建议：

```go
type RollbackCoordinator interface {
    Prepare(
        ctx context.Context,
        request RollbackPrepareRequest,
    ) (RollbackSnapshot, error)

    Restore(
        ctx context.Context,
        snapshotID string,
    ) RollbackResult

    Verify(
        ctx context.Context,
        snapshotID string,
    ) error
}
```

回滚必须：

- 先停止新版本 Runtime；
-恢复文件；
-恢复数据库；
-恢复 Owner 与 Reference；
-恢复 Scope；
-恢复 Tool 定义；
-恢复配置；
-重新验证 Hash；
-重新激活；
-写审计。

---

## 二十七、补偿事务

文件系统与数据库不能使用同一原子事务，因此必须采用补偿设计。

建议记录安装步骤：

```text
operation_step
status
rollback_action
```

示例：

```text
staging_verified
snapshot_created
database_pending
files_committed
database_committed
runtime_activated
```

应用崩溃后 Recovery 可以根据最后步骤处理。

---

## 二十八、Recovery Journal

建议建立恢复日志：

```go
type RecoveryJournalEntry struct {
    OperationID   string
    PackageID     string
    Version       string
    Step          string
    State         string
    StagingID     string
    SnapshotID    string
    TargetPath    string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

启动时扫描：

- pending；
-committing；
-rolling_back；
-cleaning。

不得仅依赖内存 defer 做补偿。

---

## 二十九、临时资源清理

CleanupManager 必须处理：

- 过期 Staging；
-失败解压目录；
-过期 Import Session；
-孤儿 Snapshot；
-中断 Commit；
-下载临时文件；
-Workshop 临时包；
-迁移临时文件；
-锁文件。

清理必须：

- 校验 Owner；
-校验路径；
-禁止跟随链接；
-记录审计；
-有失败重试；
-不能误删最终安装目录。

---

## 三十、安全报告

建议定义：

```go
type PackageSecurityReport struct {
    ReportID           string
    SourceType         string
    ArchiveHash        string
    ContentTreeHash    string
    EntryCount         int
    TotalCompressed    int64
    TotalUncompressed  int64
    CompressionRatio   float64
    PathIssues         []SecurityIssue
    TypeIssues         []SecurityIssue
    SizeIssues         []SecurityIssue
    SignatureResult    SignatureVerificationResult
    PublisherTrust     PublisherTrustResult
    PlatformIssues     []SecurityIssue
    Warnings           []SecurityIssue
    BlockingIssues     []SecurityIssue
    CreatedAt          time.Time
}
```

前端安装预览只展示结构化结果，不自己重新分析包。

---

## 三十一、安全问题等级

建议：

```text
info
warning
high
critical
```

### Critical

直接阻止：

- 路径穿越；
-签名内容不匹配；
-归档炸弹；
-特殊文件；
-目标目录逃逸；
-Hash 不一致；
-已撤销密钥；
-文件被预览后修改。

### High

默认阻止或要求开发者模式：

- 未签名代码包；
-未知可执行文件；
-不支持平台；
-未知发布者高风险权限；
-嵌套归档；
-原生二进制。

### Warning

可继续但明确提示：

- 未签名纯声明包；
-开发者签名；
-过旧 Manifest；
-大小接近限制；
-包含非必要隐藏文件。

---

## 三十二、Agent Skill 导入接入

Agent Skill ZIP 不得继续使用独立安全解压逻辑。

统一流程：

```text
PackageSecurity Inspect
→ Secure Extraction
→ Sealed Staging
→ Agent Skill Parser
→ Agent Skill Import
```

Agent Skill 特定规则由 Parser 处理，但：

- 路径；
-大小；
-文件类型；
-Hash；
-Staging；

必须来自 Package Security。

---

## 三十三、Workshop 接入

Workshop 生成内容也必须经过 Package Security。

即使内容由本地 AI 生成：

- 仍可能生成危险路径；
-仍可能生成超限文件；
-仍可能写入 Secret；
-仍可能生成不支持文件；
-仍可能包含危险脚本。

Workshop 产物必须：

```text
Build Artifact
→ Security Inspect
→ Secure Staging
→ Preview
→ Install
```

不得直接写 Registry 或最终目录。

---

## 三十四、MCP 依赖包接入

未来若 Agent Skill 或 Extension 声明安装本地 MCP Server：

- 安装包本身经过 Package Security；
-命令和运行时权限另由 Permission Broker；
-子进程由 Runtime Supervisor；
-Package Security 不执行命令；
-签名不代表可自动运行。

---

## 三十五、原生二进制规则

第一阶段建议默认：

- `.amitiax` 不允许未声明原生二进制；
-声明后标记高风险；
-仅受支持平台；
-必须签名；
-必须 Hash 绑定；
-不得在安装阶段执行；
-后续由 Trusted Service Runtime 管理。

禁止将原生二进制直接复制后自动启动。

---

## 三十六、脚本文件规则

JavaScript、TypeScript、Shell、PowerShell、Python 等脚本：

- 必须被 Manifest 声明；
-必须在 Integrity 清单；
-必须受 Runtime 类型约束；
-不得在安装阶段执行；
-不得通过安装 Hook 执行；
-不得利用文件扩展名伪装；
-不得默认获得可执行权限。

---

## 三十七、Secret 扫描

Package Security 可做有限 Secret 检测：

- 私钥格式；
-常见 Token 前缀；
-`.env`；
-凭据文件；
-证书私钥；
-硬编码密码风险。

检测结果只作警告或阻止策略，不能保证发现全部 Secret。

不得把扫描到的 Secret 内容写入报告，只记录：

- 文件；
-类型；
-位置摘要；
-风险。

---

## 三十八、与资源所有权模型的关系

Package Security 创建的资源必须纳入第 12 步模型：

### Staging

```text
owner=temporary
resource_type=temporary_directory
```

### Snapshot

```text
owner=extension/package 或 migration
resource_type=artifact
```

### Final Artifact

```text
owner=extension_package
resource_type=artifact
```

### Recovery Journal

属于系统运行资源，关联 Operation。

Package Security 不自行管理长期 Owner 逻辑，而通过 ResourceOwnershipService 注册。

---

## 三十九、与审计模型的关系

以下操作必须写 Audit Event：

- 包检查；
-安全拒绝；
-签名失败；
-未知发布者；
-用户信任发布者；
-Commit；
-回滚；
-恢复；
-临时资源清理失败；
-Hash 不一致；
-开发者模式绕过警告。

不得记录：

- 私钥；
-Secret 内容；
-完整包内容；
-未脱敏用户路径。

---

## 四十、持久化建议

建议目标表：

```text
package_security_reports
package_integrity_entries
package_signatures
publisher_keys
publisher_trust
package_staging_areas
package_recovery_journal
package_snapshots
package_cleanup_jobs
```

是否复用旧表需根据第 4 步分类决定。

原则：

- Security Report 与 Package 业务记录分离；
-签名与信任分离；
-Staging 为临时状态；
-Snapshot 为回滚资产；
-新安全数据不写旧 Package Operation 表作为唯一真值。

---

## 四十一、兼容旧 `.amitiax` v1

旧包允许进入安全检查，但必须标记：

```text
legacy_manifest
legacy_integrity
unsigned_or_legacy_signature
```

兼容策略：

- 允许安全解压；
-生成内部内容 Hash；
-不伪造发布者；
-不自动信任；
-不允许扩展旧 Manifest 能力；
-后续迁移为 v2；
-旧 Parser 只读取 Sealed Staging。

---

## 四十二、API 建议

内部接口：

```text
POST /api/extensions/security/inspect
GET  /api/extensions/security/reports/:id
POST /api/extensions/security/sessions/:id/confirm
DELETE /api/extensions/security/sessions/:id
GET  /api/extensions/publishers
POST /api/extensions/publishers/:id/trust
DELETE /api/extensions/publishers/:id/trust
```

本步骤不要求直接对用户暴露全部接口，但后端应有统一 Service。

---

## 四十三、前端安装预览

前端应展示：

- 包来源；
-原始大小；
-解压大小；
-文件数量；
-签名状态；
-发布者；
-信任状态；
-平台兼容；
-高风险文件；
-完整性；
-阻止原因；
-警告；
-Import Session 过期时间。

前端不得：

- 自行计算签名；
-自行判断 Path；
-重新读取本地文件做安装；
-隐藏 Critical Issue；
-将 Unknown Publisher 显示为 Verified。

---

## 四十四、开发者模式

开发者模式可以允许：

- 未签名包；
-开发签名；
-本地目录包；
-快速重载。

但仍不得绕过：

- 路径穿越；
-ZIP 炸弹；
-特殊文件；
-目标目录逃逸；
-Hash 不一致；
-文件替换；
-临时目录清理。

开发者模式操作必须明显标记并写审计。

---

## 四十五、测试要求

必须新增：

### 1. 文件类型测试

- 正确 `.amitiax`；
-伪装扩展名；
-错误 Magic；
-错误 MIME；
-损坏归档。

### 2. 路径测试

- `../`；
-Windows 路径；
-UNC；
-绝对路径；
-Unicode；
-尾随点；
-尾随空格；
-保留名；
-大小写冲突；
-NFC/NFD。

### 3. 归档测试

- ZIP Bomb；
-大量 Entry；
-单文件超限；
-总大小超限；
-嵌套归档；
-重复路径；
-特殊文件；
-符号链接；
-硬链接。

### 4. Hash 测试

- Archive Hash；
-Entry Hash；
-Content Tree Hash；
-排序稳定性；
-跨平台稳定性；
-修改检测。

### 5. 签名测试

- Valid；
-Invalid；
-Unknown Key；
-Revoked；
-Expired；
-Publisher Mismatch；
-Content Mismatch；
-Unsupported Algorithm。

### 6. Trust 测试

- Official；
-User Trusted；
-Unknown；
-Blocked；
-Revoked；
-Key Rotation。

### 7. Staging 测试

- 创建；
-权限；
-Seal；
-修改拒绝；
-过期；
-清理；
-异常退出恢复。

### 8. Commit 测试

- 成功；
-文件系统失败；
-数据库失败；
-跨文件系统；
-Hash 失败；
-目标冲突；
-并发安装。

### 9. Rollback 测试

- 正常；
-Snapshot 缺失；
-内容损坏；
-数据库失败；
-Runtime 激活失败；
-恢复后验证。

### 10. Recovery Journal 测试

模拟每个步骤崩溃。

### 11. Agent Skill 接入测试

确保不再独立解压。

### 12. Workshop 接入测试

确保产物必须经过安全层。

### 13. 资源所有权测试

Staging、Snapshot、Artifact 正确注册。

### 14. 审计测试

安全拒绝和信任变更有记录且无 Secret。

---

## 四十六、实施任务

### Task 1：提取归档安全代码

从旧 Package 和 Agent Skill 中提取通用检查逻辑。

### Task 2：定义 PackageSource 与 ArchivePolicy

建立统一输入和限制模型。

### Task 3：实现 FileTypeDetector

不再只依赖扩展名。

### Task 4：实现 ArchiveInspector

只检查，不直接解压。

### Task 5：实现 SafePathResolver

统一跨平台路径规则。

### Task 6：实现 EntryValidator

统一文件、大小、类型和权限检查。

### Task 7：实现 ContentHasher

支持 Archive、Entry 和 Content Tree Hash。

### Task 8：实现 ManifestBindingVerifier

建立 Manifest 与内容树绑定能力。

### Task 9：实现 SignatureVerifier

支持密钥、签名和内容匹配。

### Task 10：实现 PublisherTrustService

支持信任、撤销和密钥轮换。

### Task 11：实现 SecureExtractor

逐项安全解压到 Staging。

### Task 12：实现 StagingManager

支持 Seal、Verify、Cleanup。

### Task 13：实现 AtomicCommitter

统一最终落盘。

### Task 14：实现 SnapshotManager

支持升级和回滚快照。

### Task 15：实现 RollbackCoordinator

统一恢复。

### Task 16：实现 Recovery Journal

支持崩溃恢复。

### Task 17：实现 CleanupManager

清理临时资源。

### Task 18：接入 ResourceOwnershipService

登记 Staging、Snapshot、Artifact。

### Task 19：接入统一 Audit

记录安全和信任事件。

### Task 20：迁移 Agent Skill 导入

改为读取 Sealed Staging。

### Task 21：迁移 Workshop 安装

禁止直接安装。

### Task 22：迁移旧 Package Parser

只允许读取 Security Session。

### Task 23：建立统一安全预览 API

提供前端结构化报告。

### Task 24：增加旧安全入口统计

识别仍直接解压或写最终目录的位置。

### Task 25：完成故障注入与跨平台测试

覆盖 Windows、macOS、Linux。

---

## 四十七、建议目录结构

建议：

```text
backend/internal/extension/kernel/package_security/
├── source.go
├── policy.go
├── file_type.go
├── archive_inspector.go
├── archive_entry.go
├── safe_path.go
├── entry_validator.go
├── hash.go
├── integrity.go
├── signature.go
├── publisher.go
├── trust.go
├── extractor.go
├── staging.go
├── commit.go
├── snapshot.go
├── rollback.go
├── recovery_journal.go
├── cleanup.go
├── report.go
├── storage.go
└── audit.go
```

测试夹具：

```text
backend/testdata/package-security/
├── valid/
├── path-traversal/
├── zip-bomb/
├── symlink/
├── hardlink/
├── unicode-collision/
├── case-collision/
├── invalid-signature/
├── revoked-key/
├── modified-after-preview/
└── recovery-states/
```

目录仅为建议。

---

## 四十八、性能要求

安全检查必须可控，但不能为性能牺牲安全。

建议：

- 流式 Hash；
-流式 Entry 检查；
-不一次性读取整个归档；
-限制内存；
-大文件分块；
-预览结果复用 Sealed Staging；
-避免安装阶段重复解压；
-签名和 Hash 可缓存，但必须绑定 Archive Hash；
-并发检查数量有上限；
-临时目录空间提前检查；
-磁盘不足时提前拒绝。

---

## 四十九、风险控制

### P0：路径与内容逃逸

- Path Traversal；
-Symlink；
-Absolute Path；
-特殊文件；
-目标覆盖；
-预览后替换。

### P1：完整性与信任错误

- Manifest 未绑定内容；
-签名只覆盖部分文件；
-已撤销密钥仍通过；
-未知发布者显示可信；
-Hash 规则跨平台不稳定。

### P2：事务与恢复错误

- Commit 部分成功；
-Snapshot 不完整；
-回滚无法恢复；
-Recovery Journal 缺失；
-临时文件泄漏。

### P3：兼容和性能问题

- 旧包无法导入；
-大包检查过慢；
-前端报告不清；
-开发者模式过度受限。

---

## 五十、本步骤不做的事情

本步骤明确不做：

- 不定义 `.amitiax` v2 完整 Manifest；
-不实现依赖解析；
-不实现 Extension Kernel 生命周期；
-不实现插件 Runtime；
-不执行任何包内代码；
-不实现扩展市场下载；
-不实现自动更新；
-不实现完整沙箱；
-不删除旧 Package Parser；
-不删除旧 Package 表；
-不迁移生产包；
-不改变现有扩展业务功能；
-不允许通过开发者模式绕过关键归档安全。

---

## 五十一、验收产物

完成后必须提交：

### 1. 包安全主文档

```text
docs/extension-kernel/13-package-security-infrastructure.md
```

### 2. Package Security 核心代码

至少包含：

- PackageSource；
-ArchivePolicy；
-FileTypeDetector；
-ArchiveInspector；
-SafePathResolver；
-EntryValidator；
-ContentHasher；
-SignatureVerifier；
-PublisherTrustService；
-SecureExtractor；
-StagingManager；
-AtomicCommitter；
-SnapshotManager；
-RollbackCoordinator；
-Recovery Journal；
-CleanupManager。

### 3. 安全报告模型

能够输出 Blocking、Warning 和 Info。

### 4. 签名与发布者模型

支持：

- 签名验证；
-密钥；
-撤销；
-信任；
-开发者身份；
-官方身份。

### 5. Sealed Staging

预览和安装必须读取同一份不可变 Staging。

### 6. 原子提交与补偿

支持失败恢复和跨文件系统降级。

### 7. Agent Skill 接入报告

确认导入不再独立解压。

### 8. Workshop 接入报告

确认产物不再直接安装。

### 9. 旧 Package 接入报告

确认 Parser 只读取安全 Session。

### 10. 资源所有权接入

Staging、Snapshot 和 Artifact 已登记。

### 11. 迁移统计报告

列出：

- 仍直接读取原始包的入口；
-仍直接解压的入口；
-仍直接写最终目录的入口；
-仍绕过签名/Hash 的入口；
-仍未登记临时资源的入口。

### 12. 测试报告

覆盖：

- 文件类型；
-路径；
-归档；
-Hash；
-签名；
-信任；
-Staging；
-Commit；
-Rollback；
-Recovery；
-清理；
-跨平台；
-故障注入。

---

## 五十二、验收标准

本步骤通过必须满足：

1. 所有扩展包输入被视为不可信。
2. 文件类型不再只依赖扩展名。
3. 路径穿越、绝对路径、链接和特殊文件可被阻止。
4. ZIP Bomb 和大小限制可被阻止。
5. 跨平台路径冲突可被识别。
6. Archive、Entry 和 Content Tree Hash 已统一。
7. Manifest 可与内容树绑定。
8. 签名验证覆盖 Manifest 与内容。
9. 发布者信任与权限授权分离。
10. 预览和安装读取同一 Sealed Staging。
11. 安装不再直接解压到最终目录。
12. Commit 具有补偿与恢复日志。
13. 升级前可生成完整 Snapshot。
14. 回滚可验证恢复结果。
15. Agent Skill 和 Workshop 已接入统一安全层。
16. Staging、Snapshot 和 Artifact 已纳入资源所有权。
17. 高风险安全事件进入统一审计。
18. 新安全数据不双写旧系统。
19. 跨平台与故障注入测试通过。
20. 后续第 14 步可以安全抽取 MCP 协议基础设施。

---

## 五十三、退出条件

只有满足以下条件后，才能进入第 14 步“抽取 MCP 协议基础设施”：

- Package Security 核心组件已落地；
-Sealed Staging 已落地；
-Hash 与签名模型已落地；
-发布者信任已落地；
-Secure Extraction 已落地；
-Atomic Commit 已落地；
-Snapshot 与 Rollback 已落地；
-Recovery Journal 已落地；
-Agent Skill 和 Workshop 已接入；
-旧 Package Parser 已停止直接读取原始包；
-关键安全测试通过；
-没有新增绕过安全层的包入口。

---

## 五十四、执行约束

执行本步骤时必须遵守：

> 包安全基础设施必须独立于具体扩展类型，任何包都必须先通过安全层，业务 Parser 才能读取内容。

禁止出现：

- Agent Skill 继续自行解压；
-Workshop 继续直接写最终目录；
-旧 PackageService 在安装阶段重新解压；
-签名通过后跳过路径检查；
-官方包跳过安全检查；
-开发者模式绕过路径和大小限制；
-预览使用原始包、安装重新读取另一个包；
-直接信任文件扩展名；
-Manifest 只签自身不绑定资源；
-Commit 失败后无 Recovery Journal；
-Staging 不纳入临时资源管理；
-新旧安全链长期并行。

本步骤完成后，Amitia 必须具备一套跨平台、可验证、可恢复、可审计、可复用的扩展包安全基础。
