# B14 Browser Provider共享合同与Kernel接入底座报告

## 1. 执行结果

状态：PASS

B14位干B13→B14→B15资源/浏览器/媒体合同轨道。任务是根据Reuse Matrix B14定义（NEW_PROVIDER+EXTEND模式，correctedResponsibility："在Extension Kernel内新建Browser Provider，通过Registry注册到CapabilityCatalog，复用PermissionBroker和ExecutionPipeline，禁止新建独立Browser Runtime"），在不实现真实浏览器引擎的前提下，建立Amitia唯一的Browser Provider共享合同。

实际完成：创建了纯合同代码包 `backend/internal/browser/`，定义了完整的 Browser Provider、Session、Tab、Navigation、DOM、Interaction、ResourceTransfer 接口与领域类型，并扩展 Kernel RuntimeType 加入 `RuntimeTypeBrowser = "browser"`。

## 2. B9P8输入

- B9P8状态：PASS（AMITIA-POST-B9-RESOLVED-V1）
- final_capability_manifest.json：Parity Manifest声明
- final_runtime_manifest.json：RuntimeBinding定义（Provider执行器绑定范式）
- final_provider_manifest.json：Provider注册方式
- final_permission_manifest.json：53个Permission定义（39重用/14缺失）
- corrected_capability_registry.json：identified external/browser/* capabilities

## 3. B13输入

- B13状态：PASS_NO_CODE_CHANGE
- B14_input_manifest.json：提供下载/上传/temp/workspace/attachment合同
- resource_provider_contract.json：ResourceResolver + ResourceKind合同
- resource_security_contract.json：NO_SECRET_IN_URI / NO_PERMISSION_AUTHORITY等不变量
- provider_gap_inventory.json：SAF/SFTP/SSH/Git Provider延迟到B90-B92

## 4. Construction Mode

定义：NEW_PROVIDER（primary）+ EXTEND（secondary）

- NEW_PROVIDER: Browser 域合同包 backend/internal/browser/ 创建
- EXTEND: RuntimeType 常量添加（RuntimeTypeBrowser） + 复用Kernel合同

## 5. 当前Browser相关代码盘点

扫描结果（backend、mobile_app、desktop、front、runtime）：

- Existing BrowserDomain (Go): 无
- Existing Automation Providers（Go）: 无
- Existing BrowserToolDefinitions: 无
- Existing Runtime Binding (Go): 无
- Existing Permission Mappings: 无（Browser作为未来Provider）

特殊发现：backend/internal/realtime/proxy.go含"browser"字样，但这是WebSocket代理层对客户端连接的称呼，不是Browser Provider。

sidecar bundle.mjs 中发现 playwright-core@1.60.0 依赖，属于JavaScript侧面包，不在Go Kernel内。

结论：Go后端 Extension Kernel 中完全没有Browser Provider相关实现，B14是新领域合同。

## 6. WebView与Browser Runtime边界

当前项目：
- 无可识别的 WebView UI渲染组件
- 移动端项目中无可识别的 Agent-facing WebView Bridge
- 如发现WebView，将被归类为 PRODUCT_WEBVIEW 而不是 BROWSER_AUTOMATION

## 7. HTTP Client与Browser边界

扫描结果：
- 无_net/http_或_resty_实例被识别为Browser能力
- 所有HTTP客户端仅被视为网络I/O工具，不等同于DOM/Tab/Navigation/Browser状态

## 8. Final Browser Capability

从B9P8 corrected_capability_registry.json中抽取与Browser Provider直接相关的能力：

- Domain: browser
- Related Capabilities:
  - external/browser/automation_execute_browser (核心)
  - external/browser/capture_browser_screenshot
  - 其他若干control类能力 (not 直接 B Provider 范畴，属 VirtualDisplay 独立范畴)
- 当前状态：能力声明在 ParityManifest，生产 ToolRegistry 中未注册

## 9. Tool Exposure

B14 不注册任何Tool到生产ToolRegistry。所有Tool注册推迟到B79-B83，由实际Browser Provider实现执行。

browser_tool_exposure_mapping.json: 显式空数组。

## 10. Permission Mapping

Browser 权限全部复用现有Kernel权限体系：
- TOOL_EXECUTE: 导航/交互/截图等
- RESOURCE_ACCESS: 上下文件下载
- NETWORK_ACCESS: 网络请求

Browser没有单独的PermissionCenter。

browser_permission_mapping.json: 详细映射。

## 11. Runtime Binding

- RuntimeTypeBrowser = "browser" 已加入 Kernel constants
- RuntimeBinding 未来用于绑定具体Provider执行器（CDP/Playwright/Chromium）
- B14 仅定义 contract，不实现真实 Adapter (推迟到B79-B83)

## 12. Browser Provider合同

定义文件: backend/internal/browser/provider.go
- 核心接口: BrowserProvider（含 Sessions/Tabs/Navigate/Observe/Interact/Resources 子管理器）
- Provider-neutral: 没有 PlaywrightPage / CDPSession / BrowserTabRef 等泄露的中层类型
- productionProviderImplemented: false (contract-only)

## 13. Session合同

- SessionID: opaque string (自定义类型 BrowserSessionID)
- Lifecycle: created / ready / closing / closed / failed
- Concurrency: 显式要求多Session支持；禁止 global currentSession
- Cleanup: CloseSession 触发 tab cleanup

## 14. Tab合同

- TabID: opaque string (自定义类型 BrowserTabID)
- Ownership: Tab belongs to Session；跨Session操作禁止
- Active tab: 显式 ActivateTab；禁止 implicit global current tab
- States: loading / ready / failed

## 15. Navigation合同

- URL: 仅允许 http/https（默认安全策略）
- 禁止 javascript:/file:/data:/chrome:/about: 等危险scheme
- file:// 必须通过 amitia:// ResourceURI 转换
- Result: BrowserNavigationResult (SessionID/TabID/URL/FinalURL/Title)

## 16. DOM合同

- Snapshot: BrowserDOMSnapshot (SessionID/TabID/URL/Title/Content)
- 输出限制: MaxDepth 限制 + Truncated 标志
- 序列化安全: 禁止 chan/func/unsafe pointer/provider native object
- Element reference: BrowserElementRef (Selector + StableID)
- Stale behavior: ProviderError stale_element code

## 17. Element Reference

- StableID: opaque identifier 稳定引用
- Selector 输入允许 (CSS/XPath/Role)，但不等同同稳定ID
- 动态网页 selector != element identity
- Stale element 错误可由 Provider 返回

## 18. Interaction合同

- 操作: click/input/select/hover/scroll
- Request: BrowserInteractionRequest (SessionID/TabID/Element/action/inputText)
- Result: BrowserInteractionResult (Success/Stale/ErrorHint)
- 所有操作均要求显式 SessionID + TabID

## 19. ResourceURI接入

所有Browser资源交互都围绕B13 ResourceURI:
- Download: 结果返回 ResourceURI (amitia://workspace/downloads/...)
- Upload: 数据源为 ResourceURI (amitia://attachments/...)
- Screenshot: 结果返回 ResourceURI (amitia://temp/screenshots/...)
- 无物理路径暴露

## 20. Upload合同

BrowserUploadRequest 数据:
- SessionID: BrowserSessionID
- TabID: BrowserTabID
- ResourceURI: string (公开唯一输入，来自B13)
- TargetInput: string (可选，目标input元素selector)

禁止: 裸physicalOS path

Provider 内部可调用 resourceuri.Parse + PhysicalResolver.Resolve 获取物理路径

## 21. Download合同

BrowserDownloadRequest 数据:
- SessionID + TabID
- ResourceURI (目标保存位置)
- Filename (可选)

BrowserDownloadResult 数据:
- ResourceURI (正式资源引用)
- Filename/SizeBytes/ContentType (元数据，方便Agent识别内容)

## 22. Browser Artifact

Screenshot结果作为Browser Artifact:
- ResourceURI: 指向 amitia://temp/screenshots/<filename>
- Width/Height: 像素尺寸
- B15 负责统一 Media Capability Adapter; B14 仅使用 ResourceURI 合同

## 23. State Projection

- Browser Domain State: BrowserSessionState/BrowserTabState 域类型
- 所有权: Browser Domain拥有真实状态；Kernel Execution Status保持独立
- Protocol Projection: Domain owns state, Protocol projects state (B9P5模式)
- 0 新增 GlobalStateStore

## 24. Error Projection

- Domain: BrowserError 含 typed code + message + Cause
- Code分类: 11种标准错误码
- 支持 cause 链 (Unwrap)
- 0 新增 GlobalError Registry
- 不泄露 CDP URL/profile path/credentials

## 25. Security Boundary

- 0 新增 BrowserPermissionCenter
- 0 新增 BrowserToolRegistry
- 0 新增 BrowserExecutionPipeline
- 所有Permission决策复用PermissionBroker
- 所有Tool执行复用ExecutionPipeline

## 26. Credential/Cookie边界

B14合同层:
- 不默认提供 Cookie 访问
- 不默认提供 Password/Autofill
- 未来如需要: 必须单独标高风险
- 默认EPHEMERAL 策略 (非持久化保存credential)

## 27. File访问边界

- file:// 仅作为 Provider 内部表示
- 上下传必须通过 amitia:// ResourceURI
- 物理路径仅 ResourceURI Resolver 处理

## 28. 实际代码修改

### backend/internal/browser/types.go (新建)
- Symbol: 所有Browser领域类型定义
- 修改: 新增文件，定义 SessionID/TabID/SessionState/TabState 及所有DTO
- 原因: 建立统一领域类型，防止未来Provider泄露内部实现
- 仅合同: 是，纯类型+常量
- Backward Compatible: 是（纯新增）

### backend/internal/browser/errors.go (新建)
- Symbol: BrowserError, BrowserErrorCode, 错误构造函数
- 修改: 新增文件，11种错误码
- 原因: Browser域独立错误分类
- 仅合同: 是
- Backward Compatible: 是

### backend/internal/browser/provider.go (新建)
- Symbol: BrowserProvider + 6个分域Manager接口
- 修改: 新增文件，定义Provider-neutral接口
- 原因: 防止未来Playwright/CDP直接暴露
- 仅合同: 是
- Backward Compatible: 是

### backend/internal/browser/contract_test.go (新建)
- Symbol: fakeBrowserProvider 及测试
- 修改: 新增文件
- 原因: 验证合同可用且类型正确
- 仅合同: 是（测试文件）
- Backward Compatible: 是

### backend/internal/extension/kernel/capability/runtime_adapter.go (修改)
- Symbol: RuntimeTypeBrowser RuntimeType = "browser"
- 修改: +1行常量
- 原因: 满足 B14 Reuse Matrix requiredRuntimeBindings: ["browser"]
- 仅合同: 是
- Backward Compatible: 是（其他代码不变，仅扩展）

## 29. Backward Compatibility

- 0现有文件重命名
- 0现有接口修改
- 0第三方依赖
- 0 DB schema
- Build通过: `./internal/browser/...` + `./internal/extension/kernel/...`

## 30. Duplicate System Validation

- BrowserToolRegistry2: 0
- BrowserCapabilityCenter2: 0
- BrowserPermissionCenter2: 0
- BrowserExecutionPipeline2: 0
- Workspace2: 0
- ResourceURI2: 0
- ProductionFakeBrowserProvider: 0 (仅测试文件内的fake)

## 31. B15输入

B14为B15 Media适配器轨道提供的输入：
- BrowserScreenshot → amitia://temp/screenshots/<filename>
- BrowserDownloadResult.ContentType 作为媒体类型元数据
- 所有 Browser Artifact 均通过 ResourceURI 指向
- B15 可定义统一 MediaCapabilityAdapter 包装

## 32. B79～B83真实实现输入

B79-B83需要实现的实际Browser能力:
1. BrowserProvider 生产实现 (Playwright/CDP/Chromium)
2. 浏览器进程生命周期管理
3. RuntimeAdapter 真实实现
4. ToolDefinition 注册
5. Permission wiring
6. 真实Navigation/DOM/Interaction执行

## 33. 测试

- browser contract: PASS (7 tests)
- Race: PASS
- Kernel regression: PASS
- ResourceURI regression: PASS
- gofmt: PASS
- goMod / goSum: 无变化

## 34. Source Boundary

- 新建文件: 4（contract-only package）
- 修改文件: 1 (+1 constant)
- 未预期文件: 0
- go.mod: 不变
- go.sum: 不变
- 数据库: 不变

## 35. 阻断项

无。

## 36. 最终结论

1. B14完全建立在现有Extension Kernel和B13 ResourceURI之上
2. Browser拥有唯一Provider-neutral共享合同 (backend/internal/browser/)
3. Browser Session/Tab/Navigation/DOM/Interaction边界明确
4. 上传下载统一使用ResourceURI，无物理路径暴露
5. Browser Tool继续使用现有ToolRegistry（0个注册在此步）
6. Browser Permission继续使用现有PermissionBroker
7. Browser执行继续通过现有ExecutionPipeline/RuntimeBinding
8. 没有建立Browser Tool Registry、Permission Center、Workspace2或ResourceURI2
9. 未提前实现B79～B83的真实Browser Runtime
10. Browser真实Provider实现已经完整移交B79～B83
11. 允许进入B15
