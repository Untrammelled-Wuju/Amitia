# B13 ResourceURI与Workspace统一资源合同补强报告

## 1. 执行结果

状态：PASS_NO_CODE_CHANGE

执行结果：经过对B9P8冻结清单和Reuse Matrix B13条目的逐项审查，并对backend/pkg/resourceuri/所有源码（5个实现文件、2个测试文件）及生产调用方（7个生产文件共19次引用）完成审计后，发现现有ResourceURI合同已完全覆盖B13要求的所有18项语义，未出现任何PARTIALLY_SUPPORTED或MISSING项。按照B13规则第85条，允许PASS_NO_CODE_CHANGE，不强行抽象。

## 2. B9P8输入

- B9P8状态：PASS（b9p8_status.json）
- Resolved manifest ID：AMITIA-POST-B9-RESOLVED-V1
- Frozen at：2026-08-07T19:30:00+08:00
- Release gate：POST-B9-B10-RELEASE-GATE（b10Allowed: true）
- 已读取：final_capability_manifest.json / final_permission_manifest.json / final_provider_manifest.json / final_runtime_manifest.json / final_canonical_system_manifest.json / final_execution_guard.json / resolved_post_b9_manifest.json / final_step_reuse_matrix.json

## 3. Construction Mode

定义（来自final_step_reuse_matrix.json B13条目）：EXTEND

实际B13执行：REUSE（因为ALLOWED_NEW_COMPONENTS为空，且审计未见真实Gap，因此无需任何EXTEND）。

## 4. Canonical ResourceURI体系

- Canonical Package：backend/pkg/resourceuri/
- Official Scheme：amitia://（唯一正式资源Scheme）
- 核心文件：resource_uri.go / resource_root.go / resolver.go / physical_resolver.go / errors.go
- 包内类型：ResourceURI / ResourceRoot / ResourceKind / ResolvedResource / ResourceResolver / PhysicalRoots / PhysicalResolver

## 5. 当前URI Scheme

- 唯一正式Scheme：amitia://
- 验证：Parse严格匹配大小写不敏感前缀"am://" + url.Parse验证Scheme=="amitia"
- 不支持第二正式Scheme：file:// / content:// / sftp:// / ssh:// / minis:// / workspace:// 等均视为外部/Provider内部表示
- 禁止query/fragment/userinfo/port

## 6. 当前Resource Roots

| Root | Filesystem | Virtual | 说明 |
|------|-----------|---------|------|
| workspace | yes | no | 用户/Agent可操作主工作区 |
| attachments | yes | no | 消息附件资源 |
| data | yes | no | 应用持久化内部数据 |
| cache | yes | no | 缓存数据 |
| runtime | yes | no | 运行时核心目录 |
| config | yes | no | 应用配置 |
| extensions | yes | no | 扩展/技能宿主 |
| logs | yes | no | 应用日志 |
| temp | yes | no | 临时文件 |
| native | no | yes | 虚拟Root（剪贴板等非文件资源），PhysicalResolver返回ResourceKindVirtual + ErrNonFilesystemResource |

总计10个Root，每项均已在resource_root.go以ResourceRoot string类型常量声明，并通过allow-list校验。

## 7. 当前Resolver

- 接口：ResourceResolver（resolver.go） — Resolve(ResourceURI) (ResolvedResource, error) + Reverse(localPath string) (ResourceURI, error)
- 实现：PhysicalResolver（physical_resolver.go）
  - 构造：NewPhysicalResolver(PhysicalRoots) (*PhysicalResolver, error)
  - 工厂：PhysicalRootsFromRuntimePaths(util.RuntimePaths) PhysicalRoots
  - Resolve：查找root映射 → filepath.Join(rootPath, segments) → filepath.Clean → assertWithinRoot
  - Reverse：对绝对路径，按最长匹配+稳定优先级选择对应Root，返回amitia:// URI
- 已覆盖生产调用方：config.go / sidecar/resolver.go / nodeenv/resolver.go / qdrantlayout/resolver.go / qdrantenv/resolver.go / qdrantenv/candidates.go / script_host/resolver.go

## 8. 当前Reverse Resolver

- 由PhysicalResolver.Reverse实现
- 算法：对每个已知root计算filepath.Rel → 选择rel不以".."开头且root路径最长（或稳定优先级最高）的root
- 路径转换：filepath.Separator → "/"；清理"./
- 拒绝：不落入任何已知root的路径返回ErrResourceOutsideRoots

## 9. 当前安全机制

- Scheme校验：必须为amitia
- Root校验：必须在allow-list中
- 路径归一化：
  - 拒绝反斜杠（\）
  - 拒绝null字节
  - 拒绝控制字符
  - URL-decode后检测..段
  - path.Clean + 剩余".."前缀检查作为纵深防御
- 结构限制：拒绝fragment (#), query (?), userinfo (@), port
- 物理Root逃逸防护：assertWithinRoot
- 反向Root逃逸防护：Reverse仅匹配已知root
- Virtual Root隔离：native root不触及文件系统即返回错误

## 10. 生产调用方

通过grep `"resourceuri\."` 扫描backend/，共识别7个生产文件、19处引用：

1. backend/config/config.go — Parse（配置解析）
2. backend/internal/scriptruntime/sidecar/resolver.go — PhysicalRootsFromRuntimePaths + NewPhysicalResolver + Parse（sidecar资源）
3. backend/internal/scriptruntime/nodeenv/resolver.go — PhysicalRootsFromRuntimePaths + NewPhysicalResolver + Parse（Node候选）
4. backend/internal/vectorstore/qdrantlayout/resolver.go — PhysicalRootsFromRuntimePaths + NewPhysicalResolver + MustParse（Qdrant配置/数据）
5. backend/internal/vectorstore/qdrantenv/resolver.go — Parse + PhysicalRootsFromRuntimePaths + NewPhysicalResolver（Qdrant环境）
6. backend/internal/vectorstore/qdrantenv/candidates.go — Parse（Qdrant候选）
7. backend/internal/extension/kernel/script_host/resolver.go — PhysicalRootsFromRuntimePaths + NewPhysicalResolver + Parse（脚本宿主）

关键结论：PhysicalRootsFromRuntimePaths + NewPhysicalResolver已在5个独立域复用，是事实上的标准初始化模式。

## 11. Post-B9 Resource需求

从B9P8 final_capability_manifest.json（builtin/resource/*类capability）、final_provider_manifest.json（涉及附件/数据/缓存/运行时/配置的Provider）、final_runtime_manifest.json及final_permission_manifest.json（RESOURCE_ACCESS语义）中抽取18项ResourceURI合同需求。详见required_resource_semantics.json。

## 12. 已支持合同

全部18项Required Semantics已支持：

1. uri.parse（Parse/MustParse）
2. uri.normalize（normalizeAndValidatePath）
3. uri.serialize（String(), round-trip验证通过）
4. uri.root-validation（parseResourceRoot + allow-list）
5. root.workspace（ResourceRootWorkspace）
6. root.attachments（ResourceRootAttachments）
7. resolver.resolve（PhysicalResolver.Resolve）
8. resolver.reverse（PhysicalResolver.Reverse）
9. resolver.traversal-protection（多层级检测）
10. resolver.root-escape-protection（assertWithinRoot）
11. resource.kind.filesystem-vs-virtual（ResourceKind枚举）
12. resource.handle-non-filesystem（ErrNonFilesystemResource）
13. uri.no-secret（拒绝userinfo/port/fragment/query）
14. uri.unicode-path（UTF-8 / url.PathEscape）
15. cross-platform-path（逻辑"/" + 物理filepath.Join）
16. uri.query-fragment-rejection（Parse严格拒绝）
17. provider-virtual-root（native Virtual root留作Provider扩展点）
18. error-classification（8种细粒度错误，errors.go）

详见resource_contract_gap_matrix.json。

## 13. Partial Support

无。

## 14. Missing Contract

无。

## 15. 实际代码修改

无代码修改。

按照B13第85条规则，所有Required Semantics均已满足，因此PASS_NO_CODE_CHANGE；不允许为了"完成B13"强行抽象。

## 16. URI语义

- Logical URI，非Physical OS Path
- 永远使用 / 作为路径分隔符
- Path segment通过url.PathEscape编码
- canonical equality：Parse(Normalize(s)).String() 稳定；round-trip测试通过
- 中文路径支持：UTF-8 → Percent-encoding序列化
- 不依赖当前工作目录
- 物理路径仅由Resolver生产，不进入Logical URI

## 17. Root语义

- Workspace：用户/Agent操作的工作区，独立生命周期
- Attachments：文件附件
- Data / Cache / Config / Extensions / Logs / Temp：各有明确职责边界
- Runtime：运行时核心，不得与workspace混淆
- Native：虚拟Root，用于非文件资源

## 18. Workspace语义

- ResourceRootWorkspace（"workspace"）是日常用户/Agent操作的默认逻辑根
- 现有B9P4已冻结的workspace语义无破坏
- 不把runtime/logs/cache/config混入workspace

## 19. Provider Extension合同

现有合同已足够：ResourceResolver接口 + ResourceKind枚举 + Root allow-list + Virtual Root（native）

建议未来B14/B90-B92按照最小扩展原则接入：
- 新Root：生命周期/安全边界/存储职责与现有Root明显不同时再新增
- Provider-backed虚拟资源可纳入native root体系，或新增Root，由B90/B14决定
- 保持amitia://唯一正式Scheme

详见resource_provider_contract.json。

## 20. Platform Resource合同

- 现有native Root + ResourceKindVirtual 提供最小平台资源合同
- file:// / content:// / sftp:// 属于外部表示，由Provider吸收或转换，不进入amitia://核心
- 不把Android content://直接取代amitia://

## 21. Local / Remote资源边界

- Local：PhysicalResolver + PhysicalRoots（基于util.RuntimePaths）
- Remote（SAF/SFTP/SSH/Git）：由B90-B92实现Provider；ResourceURI只提供合同
- 合同就绪：ResourceResolver接口类型与ResourceKind足够承载Provider扩展

## 22. Permission边界

- ResourceURI不拥有Permission Authority
- resourceuri包内无PermissionRegistry/PermissionStore/AccessGrant DB
- 真实权限由PermissionDefinitionRegistry + PermissionBroker处理
- ResourceURI只负责寻址，不负责授权决策

## 23. Runtime边界

- amitia://runtime/ 保持为RuntimeHost根目录
- 未被workspace或其他Root覆盖
- B13未修改Runtime相关语义

## 24. Path Traversal安全

覆盖矩阵：
- .. segment拒绝（raw + 编码后）
- %2e%2e 拒绝（URL-decode后再次扫描）
- 反斜杠拒绝（逻辑URI层拒绝；物理层由filepath处理）
- control char / null byte拒绝
- path.Clean + 残余".." 检查作为纵深防御
- assertWithinRoot 物理路径检查
- Root escape测试通过：workspace/../data 被拒
- Symlink：物理路径通过Rel+路径前缀判断，合理覆盖

## 25. Root Escape安全

- assertWithinRoot 用于Resolve
- Reverse仅匹配已知root，outside path返回ErrResourceOutsideRoots
- 未被生产代码绕开

## 26. 向后兼容

- 0代码修改 → 向后兼容由定义保证
- 现有amitia://workspace/等URI语义未改变
- 全部10个Root保持
- LocalResolver/ReverseResolver行为未变
- 7个生产调用方API形态未变

## 27. B14 Browser输入

B13为B14提供的统一资源合同输入：

- 下载目标：建议amitia://workspace/downloads/<file>或amitia://attachments/browser/<uuid>/<file>
- 上传源：resourceuri.Parse(raw) + PhysicalResolver.Resolve(uri)
- 临时文件：amitia://temp/...
- 持久化结果：amitia://workspace/...

详见B14_input_manifest.json。

## 28. B90～B92 Provider输入

B13为后续Provider提供的合同输入：

- 现有ResourceResolver + ResourceKind + ResourceRoot合同已就绪
- Android SAF：content://为Provider内部Endpoint，不替代amitia://；推荐新增Root，如ResourceRootProviderSAF；未来Step：B90
- SFTP：sftp://为Provider内部Endpoint；ResourceDescriptor + ProviderBinding满足需求；B90
- SSH：可复用ResourceResolver合同；B91
- Git：可复用ResourceResolver合同；B92

详见B90_B92_input_manifest.json。

## 29. Deferred Gap

- SAF Provider实现：B90
- SFTP Provider实现：B90
- SSH Provider实现：B91
- Git Workspace Provider实现：B92
- iOS File Provider：later
- 细粒度Permission Definition（14个缺失）：B11
- Provider-backed Stat元数据（size/mtime/kind）：由B14/B90-B92决定是否需要引入ResourceInfo合同；B13不提前加入

## 30. Duplicate System Validation

- ResourceURI2 / Workspace2 / ResourceResolver2 / WorkspaceURI2 / ResourceScheme2：均未创建
- GlobalResourceRegistry2 / PermissionSystem2 / RuntimeSystem2：均为0
- ProductionFakeProvider：0
- SecondOfficialScheme：amitia://保持唯一

详见duplicate_system_validation.json。

## 31. 测试

- go test -count=1 ./pkg/resourceuri/... — PASS（25个测试）
- go test -race ./pkg/resourceuri/... — PASS
- go vet — 0 issues
- 覆盖：10个Root parsing / 路径归一化 / 遍历拒绝 / 反序列化 MustParse / round-trip / Resolve / Reverse / longest-root优先级 / 稳定优先级 / outside path / native virtual rejection / unconfigured root / 不依赖环境

详见test_results.json。

## 32. 修改文件

无代码修改。

## 33. 阻断项

无。

## 34. 最终结论

1. B13仅审计并确认现有backend/pkg/resourceuri/合同有效，未扩展任何代码（因为allowedNewComponents为空且审计未发现任何真实Gap）。
2. amitia://继续作为Amitia唯一正式资源Scheme。
3. workspace继续使用现有ResourceRootWorkspace，没有建立Workspace2。
4. Local/Platform/Remote资源均已拥有统一逻辑资源合同：ResourceURI + ResourceResolver + ResourceKind + ResourceRoot allow-list + native Virtual Root。
5. ResourceURI保持与Permission和Tool Execution解耦；包内无Permission权威。
6. Path Traversal保护完整（raw/encoded/backslash/control/null/dot-clean/Resolv物理路径检查）；Root Escape保护完整（assertWithinRoot + Reverse match）。
7. SAF/SFTP/SSH/Git正确留给B90-B92，B13只设置了ResourceProviderContract参考。
8. Browser已通过现有amitia://workspace/和amitia://attachments/合同获得B14需要的统一资源合同（详见B14_input_manifest.json）。
9. 允许进入B14。
