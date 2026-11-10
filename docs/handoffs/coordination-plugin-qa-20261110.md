# 统筹模式、多设备与插件平台深度测试交接

## 续接核验补充（2026-11-10）

- 前文将二维码中的 `54796` 判作旧 Core 动态端口并不准确：它是局域网 TLS 配对监听端口，与 Core 的回环动态端口不同。旧二维码包含的局域网 IP 仍可能因网络变化而失效。
- 已按用户确认的方案修改桌面端 Core 端点通知和配对页面；二维码会在 Core 实例变化、局域网端点变化、监测流断开或超时、有效期结束后清除。真实桌面 Core 中断后旧码清除已验证；隔离测试中真实替换 TLS 监听器，验证端点事件、旧端口关闭和新端口指纹连接。未实际切换宿主 Wi-Fi。
- 已按当前源码重建 macOS Core 并完整重启。局域网事件流、桌面和前端定向测试及类型检查通过；冻结清单已通过项目脚本刷新和验证。只有可选浏览器组件因缺少 Chromium 降级。
- QA 桌宠插件 1.0.8 已真实卸载、同版重装、启用，并完成 20 轮连续禁用和启用。卸载后插件列表不再包含该插件，但关联桌宠安装接口仍报告启用和可见；重装后恢复原测试状态，不能认定卸载零残留。
- 桌宠安装投影的心跳时间停留在旧状态快照，健康值为 syncing；实时 Runtime 会话仍有心跳，权威实际状态为 healthy。针对健康投影和插件卸载清理的方案已向用户提出，尚未获确认，不得擅自修改。
- 五条早先桌宠升级操作仍等待 Runtime 确认并周期性报告幂等冲突。对应旧修订命令已完成且 outbox 保留当时快照；恢复代码使用当前修订的状态重建旧修订命令。按原命令和不可变快照核验后再推进的方案尚未获确认，不得手工改库或清除现场。
- 本地插件市场页面加载时，外部市场目录接口单独请求返回 HTTP 500，原始错误为 `读取插件市场分类: dial tcp 127.0.0.1:3306: connect: connection refused`。但本地市场实际调用首先被权限层以 `trusted_only service.network.request` 拒绝，不能断言页面收到了外部 HTTP 500：已安装的内置市场插件权威信任级别为 bundled、网络权限记录为 granted，而信任检查器未接受 bundled。宿主审计显示内层拒绝约 1 毫秒后返回，外层调用三分钟后超时；页面超时后错误被隐藏并显示空市场。单独启动的插件宿主能立即处理模拟 RPC 拒绝响应，尚不能据此定位完整 Go-to-Node 链路的等待点。信任来源校验、错误传播与展示修复尚未获确认；管理端未授权修改。
- 本机中心的 TLS 入口绑定私有地址，但未发现来源网络过滤。无法仅凭私有地址绑定证明端口映射或跨网段路由下的访问范围；外网实际可达性未测试。
- 隔离测试通过：手机端配对与审批流 23 项；四种设备平台方向的重复、反向和自扫码拒绝；中心数据只读及管理员授权撤销；跨设备能力授权撤销；游戏租约与动态行为目录；插件市场运行时及下载哈希校验。macOS 不支持已验证任务强隔离，实际 TLS 设备任务授权重连测试被跳过。以上不替代真实多设备验收。
- 截至本次核验，模拟器 `emulator-5556` 在线但桌面自动化不能操作其窗口；真实手机未连接，中心可信设备数仍为 1（中心自身），模拟器尚未完成配对。用户需在设备界面完成配对并审批；网络与权限条件具备后再继续。
- 工作区仍有大量未提交改动；`待修复.md` 的删除已由用户确认保留。未拉取、提交或推送 Git，未发布构建。

以下为续接前的原始记录，运行状态与结论以本节补充为准。

记录日期：2026-11-10，时区 Asia/Shanghai。
工作目录：`/Users/untrammelled/Desktop/Project/U-AI`
当前分支：`develop`

## 一句话结论

桌面本地统筹中心的主链路、插件安装更新、失败回滚和完整重启恢复已经实际跑通；多设备配对、权限授权、远程中心、游戏模式跨设备租约以及最终发布构建仍未完成。当前存在一个明确的待排查问题：桌面核心重启并更换动态端口后，配对页面仍展示旧端口二维码，未提示二维码已失效或自动重建。

不能将当前状态描述为“统筹模式全部完成”或“项目零缺陷”。

## 用户已确认的产品和架构决策

- 用户先选择“统筹模式”，再选择“以本机作为统筹中心”或“使用远程统筹中心”。
- 不提供独立“局域网模式”选项。
- 二维码只是手动地址配对的快捷方式，二维码与输入地址必须走同一配对流程。
- 本机中心仅允许局域网设备访问，外网设备不能直接进入。
- 远程模式切换连接时不迁移、不覆盖本机数据；用户可以手动操作数据文件。
- 聊天、消息、项目、文件和提醒归设备自己所有。
- 角色、记忆、团队、世界书、用户画像和模型配置由统筹中心管理。
- 主动任务、MCP、Skill、工作流和扩展包由设备自行管理。
- 每台设备自主决定向中心开放哪些能力；中心不需要枚举这些开放项。
- 非中心设备默认只能调用自身能力并使用中心数据。
- “A 是否有权调用 B 的能力”由统筹中心判断。
- 中心可以授权非中心设备调用中心设备能力。
- 非管理员子设备对中心管理的数据只读，相关操作按钮必须禁用。
- 手机可以作为统筹中心。
- 控制信令优先 WebSocket，适合流式场景使用 SSE，避免轮询。
- 桌宠和游戏模式服务在本机运行，最终模型能力由中心提供。
- 游戏模式同一时间只允许一台设备持有主机；其他设备可以查看状态并通过对话控制。
- 桌宠和游戏行为必须从插件动态读取，不能写死。
- 内置插件不要求每次发布都执行严格更新校验。
- 不兼容旧版数据；允许清理 AmitiaData 测试数据。

## 当前运行状态

- AmitiaCore：运行中。
- 当前 Space：`space_7loa3serga3xcsmves23nswipq`。
- 当前 Core 动态端口：`63447`。
- 当前 Qdrant HTTP/gRPC：`63444` / `63445`。
- 当前 SurrealDB：`63446`。
- QA 桌宠插件：`1.0.8`，已安装、已启用。
- QA 桌宠安装记录：`1.0.8`，`current_release_id=qa-pet-v1-release-1.0.8`，运行同步已确认。
- 当前桌面中心“已绑定设备”：`0`。
- 真机：`JV9LT8JB8TVKNJ65`，型号 `2407FRK8EC`，测试期间曾断开，交接前重新识别为在线。
- 模拟器：`emulator-5556`，运行中。
- 旧模拟器：`emulator-5554`，仍为 offline，不应作为测试目标。

## 当前最需要接手排查的问题

### 旧二维码在 Core 重启后仍被显示

桌面对应页面已经生成过包含端口 `54796` 的配对二维码。后续完整重启后，实际 Core 端点已经变为 `63447`，但页面仍展示旧二维码和旧原始配对信息，没有“已失效/重新生成”状态，也没有自动重建。

这会导致：

- 子设备扫描后连接旧端口。
- 桌面端收不到新的审批请求。
- 用户无法仅从界面判断二维码已经无效。

建议先在桌面端 DOM/状态层核对：

- 重启后是否重新读取了 `AmitiaData/run/endpoint.json`。
- 配对 offer 是否持久化了旧 endpoint。
- 页面是否只在用户点击“生成二维码”时刷新 offer。
- 是否应将 endpoint 和 offer 的 Core 实例代次绑定，并在端点多代次变化时主动失效。

### 当前模拟器仍停留在旧二维码确认弹窗

模拟器当前识别到旧地址 `https://10.138.182.247:54796`。该地址不是当前桌面中心地址。

不要再点击“配对”，应先修复或刷新二维码，再重新执行：

1. 桌面生成当前端口的二维码。
2. 模拟器从图片选择二维码。
3. 桌面审批。
4. 重启桌面 Core 和移动端。
5. 验证无需再次审批。

## 已完成并实际验证的修复

### 插件平台统一 `.petx` 契约

QA 插件已切换到统一 `amitia-extension.json`、`petx` 动态贡献、统一 Manifest 和真实 NOTICE。

相关文件：

- `plugins-code/qa-desktop-pet-plugin/amitia-extension.json`
- `plugins-code/qa-desktop-pet-plugin/build_package.py`
- `plugins-code/qa-desktop-pet-plugin/NOTICE.txt`
- `backend/internal/extension/kernel/amitiax/qa_petx_contract_test.go`

### 插件包构建不确定与 releaseId 冲突

原问题：

- 相同源码重复构建 ZIP 哈希不同。
- 修改插件版本时仍复用旧 pet releaseId，后端正确拒绝为“同一 release 内容变化”。

修复：

- ZIP 内文件时间固定。
- releaseId 默认从包版本推导，也允许显式覆盖。

验证：

- 同一版本连续构建两次，SHA-256 一致。
- 1.0.7 故障包复用 1.0.6 releaseId 后，系统拒绝安装且不污染运行版本。

### 更新失败后的版本表半提交

原问题：

- 运行时激活失败后，`package_versions` 已把目标版本标为 current。
- `extension_installations` 仍保留旧版本。
- 后续更新持续报 `current version label mismatch`。

修复：

- `packageUpdateCompensation` 记录版本是否已写入。
- 失败时恢复旧版本为唯一 current。
- 目标失败版本标记为 removed。
- 同步恢复 installation 的 version/generation 指针。

相关文件：

- `backend/internal/extension/kernel/package_update_saga.go`
- `backend/internal/extension/kernel/package_version_repository.go`
- `backend/internal/extension/kernel/package_r3_success_recovery_test.go`

真实故障注入结果：

- 故意失败更新后，扩展仍为 `1.0.7`。
- 桌宠实际仍为 `1.0.7`。
- 失败目标 `1.0.8` 为 removed。
- 随后正常 1.0.8 更新成功。

### 桌宠资源重复 Detach 不幂等

原问题：

- 更新补偿清理缓存后，再次 Detach 返回 `handle not found`。
- 一次已有完成操作的正常重试因此被误判失败。

修复：

- 对已不存在的关联资源 Detach 视为已经完成。
- 补充重复 Detach 回归测试。

文件：

- `backend/internal/desktoppet/integration/production_capabilities.go`
- `backend/internal/desktoppet/integration/production_capabilities_test.go`

### 桌宠安装幂等键跨包操作冲突

原问题：

- 幂等键只包含扩展 ID 和内容哈希。
- 新包操作重试同一内容时，与旧 `waiting_runtime_ack` 操作冲突。

修复：

- 幂等键加入 `lastOperationId`，缺失时回退 generation/version。
- 同一包操作内仍然复用稳定键，新包操作不会误撞旧操作。

文件：

- `backend/cmd/server/petx_asset_release_bridge.go`
- `backend/cmd/server/petx_asset_release_bridge_test.go`

### 同一幂等键锁长期累积

原问题：

- `sync.Map` 中的幂等锁永不删除，长期运行存在键空间增长。

修复：

- 改为引用计数的键锁。
- 空闲键立即删除。
- 高频唯一键测试和 `-race` 通过。

文件：

- `backend/internal/desktoppet/installation/coordinator/coordinator.go`
- `backend/internal/desktoppet/installation/coordinator/idempotency_test.go`

### Space 身份首次创建竞态

原问题：

- 多个 Core 进程同时首次启动时，可能各自生成不同 Space。
- 一个进程创建业务数据库，另一个进程写入不同 Space 身份，最终触发设备归属冲突。

修复：

- 使用跨进程独占锁文件串行化首次身份创建。
- 进程获得锁后二次读取身份文件。
- 64 并发创建测试重复 20 次通过。

文件：

- `backend/internal/spaceidentity/store.go`
- `backend/internal/spaceidentity/store_test.go`

### 桌宠禁用语义

已修复：

- Disable 发送 `runtime.command.ensure_absent`。
- Recovery 的 ensureAbsent 条件改为 disable + desiredEnabled=false + desiredVisible=false。

文件：

- `backend/internal/desktoppet/installation/coordinator/coordinator.go`
- `backend/internal/desktoppet/installation/recovery/production_ports.go`

### Runtime 拒绝事件状态机与重放

已修复：

- 允许派发前状态接收 `desired_rejected`。
- 同一终态拒绝事件跨 WebSocket 会话幂等重放。
- 已完成或 superseded 命令不会被拒绝事件倒退。
- durable 命令允许旧会话命令跨新会话重放。
- Runtime 协议失败写入日志，不再静默断线。

### 安装投影与 Runtime 权威状态

已修复：

- 安装操作读取 Runtime applied/rejected 事件并进入最终状态。
- 拒绝会置为 `failed_terminal`，保留错误码和消息。
- 空运行时快照可清空 actual release/visibility，但不会清空 installationId/petId。
- 生命周期成功确认前等待权威状态快照同步。

### 前端插件操作后不刷新安装区

已修复：

- `PetPluginManager` 在安装、更新、启停、卸载后发出 changed 事件。
- `PetInstallationsView` 收到事件后重新读取桌宠安装列表。

### 移动端运行模式入口

源码已增加“我的”页面中的直接“运行模式”入口，并保留“设置”入口。

文件：

- `mobile_app/lib/features/profile/presentation/pages/profile_hub_page.dart`
- `mobile_app/test/features/profile/profile_channel_entry_test.dart`

注意：这次最后的小修改尚未重新构建最终 APK。

## 已完成真实端到端验证

### 桌面插件生命周期

实际路径：

`1.0.5 安装 → 启用 → 1.0.6 更新 → 1.0.7 更新 → 故障更新失败回滚 → 再次正常更新 → 1.0.8 更新 → 完整重启`

最终状态：

- 插件：`1.0.8`，enabled。
- 安装记录：`1.0.8`。
- desiredRevision 与 appliedDesiredRevision 一致。
- `actualVisible=true`。
- `pending=false`。
- 完整重启后仍保持 `1.0.8`，无需重新审批或重新启用。

### 更新失败注入

已验证两次：

- 复用旧 releaseId 的更新失败。
- 运行时激活阶段的更新失败。

两次都验证了：

- 旧扩展版本仍是 current。
- 旧桌宠版本仍是 current。
- 失败目标版本为 removed。
- 后续正常更新可以继续。

### 启动恢复

- 清理数据后，最新桌面 Core 可以正常完成首次 onboarding。
- 手机旧数据会因 active-task Provider 契约变化而崩溃；清空应用数据后最新 APK 可以冷启动。
- 该行为符合“不兼容旧数据”的既定约束，但发布说明必须明确要求清理旧数据。

## 已执行的测试与证据

### 已完成的全量测试

- Backend：`go test -count=1 ./...` 全量通过。
- Backend：`go vet ./...` 全量通过。
- Desktop：53 个测试文件、457 项通过；1 个文件、1 项按既有条件跳过。
- Flutter：1421 项全量通过。
- Frontend：227 个测试文件、1071 项通过。

注意：这些全量结果是在后续若干最后修复之前取得的。最后新增的 Space 锁、更新补偿、资源幂等、前端联动和移动端入口只完成了定向测试，必须重新跑全量。

### 最后修复后的定向测试

- `internal/spaceidentity`：`go test -race -count=20` 通过。
- `internal/extension/kernel` 更新补偿测试通过。
- `internal/desktoppet/integration` 重复 Detach 测试通过。
- `cmd/server` Petx 幂等键作用域测试通过。
- `mobile_app/test/features/profile/profile_channel_entry_test.dart` 通过。

### 当前可信度分级

真实端到端已验证：

- 桌面本地统筹 onboarding。
- 桌面插件安装、更新、启用、禁用。
- 插件更新失败回滚。
- 桌面完整重启后的版本和桌宠状态恢复。
- 移动端清空数据后的最新 APK 冷启动。

真实集成环境已验证：

- Backend Core、Qdrant、SurrealDB、Extension Kernel、桌面 Electron、移动 Runtime 的本地启动。

仅自动化/隔离测试验证：

- 部分并发状态机、更新补偿、Provider 生命周期和故障注入场景。

仅静态检查：

- 跨设备权限模型和部分业务规则。

尚未验证：

- 外网远程统筹中心。
- 真机与模拟器完成当前中心配对。
- 多设备管理员只读和 A→B 能力授权。
- 中心模型能力被多个子设备共享。
- 游戏模式跨设备单主机租约。
- 发布版桌面安装包、Linux ARM64 Runtime 和最终 APK 全量验收。

## 尚未完成的工作

### 多设备与权限

- 当前二维码使用旧端点，配对未完成。
- 桌面已绑定设备仍为 0。
- 未验证待审批列表、拒绝、重复申请、二维码多次扫码。
- 未验证子设备重启后免审批恢复。
- 未验证管理员权限撤销和只读按钮禁用。
- 未验证设备能力开关、A 调用 B、中心授权判断。
- 未验证手机作为中心、桌面作为子设备。
- 未验证并发配对和审批竞态。

### 远程统筹中心

- 外网远程中心尚未测试。
- 需要用户开放端口后再执行。
- 需要验证 TLS、设备凭证、断线重连、服务重启和重复请求。
- 需要验证切换远程中心时本机数据不变。

### 统筹数据面

- 角色、记忆、团队、世界书、用户画像的跨设备读写和只读限制。
- 中心模型配置向子设备提供文本、识图、语音。
- 本机数据与中心数据混合时的归属和错误提示。
- 删除被其他设备引用的数据。

### 桌宠和游戏

- 桌宠插件卸载、零残留、重新安装。
- 同版本重装和高频快速启停。
- 游戏模式单设备主机租约。
- 其他设备查看游戏状态并通过对话控制。
- 游戏/桌宠行为从插件动态读取的完整链路。
- 桌宠和游戏本地服务使用中心模型。

### 插件平台

- 插件市场 `1.4.4` 的完整 UI 与多设备链路。
- 插件更新下载中断、重试、取消和恢复。
- 同版本不同内容拒绝策略的真实界面回归。
- 内置插件跳过严格更新的边界验证。

### 发布构建

- 最新移动端修改后重新构建 APK。
- 最新 macOS Core 和完整桌面安装包。
- Linux ARM64 Runtime。
- APK 签名、zipalign、ABI、内嵌 Runtime SHA-256。
- 最终 release gate 与冻结清单校验。

## 建议接手顺序

1. 修复桌面二维码在 Core 端点变化后的失效/刷新逻辑。
2. 清理模拟器 Gallery 中旧二维码，只保留当前中心二维码。
3. 用模拟器完成扫码图片配对、桌面审批、重启恢复。
4. 用真机完成相同流程。
5. 验证管理员只读、能力开关、A→B 授权。
6. 完成插件卸载、重装和插件市场生命周期。
7. 完成游戏模式单主机租约和动态插件行为。
8. 用户开放端口后测试远程统筹中心。
9. 全量重跑 Backend、Desktop、Frontend、Flutter。
10. 全量重建 Core、桌面安装包、Linux Runtime 和 APK并校验签名与哈希。

## 工作区与 Git 状态

- 当前存在大量未提交源码和测试改动。
- `待修复.md` 当前处于已删除状态，删除不是本轮交接文档操作产生的；接手时先向用户确认该删除是否应保留。
- `AGENTS.md` 已加入 Git 忽略，不应提交。
- 本轮没有执行 Git pull、commit、push 或发布。
- `DESKTOP_PET_FINALIZATION_SHA256SUMS.txt` 已通过脚本刷新。
- 禁止手工修改冻结清单哈希，应执行：

```bash
node scripts/build-freeze-scope.mjs --write
node scripts/build-freeze-scope.mjs --verify
```

## 关键测试数据与恢复位置

本轮旧数据均采用可恢复移动，未直接删除：

- `artifacts/qa-20261110/AmitiaData-pre-clean-20261110-1503`
- `artifacts/qa-20261110/AmitiaData-update-corruption-20261110-1535`
- `artifacts/qa-20261110/AmitiaData-bootstrap-race-20261110-1538`

当前 `AmitiaData` 是重新初始化的数据。

## 关键命令

```bash
cd backend
go test -count=1 ./...
go vet ./...

cd desktop
pnpm test
pnpm typecheck
node scripts/build-amitiacore.mjs

cd front
pnpm test

cd mobile_app
flutter test
flutter build apk --release --no-tree-shake-icons

cd plugins-code/qa-desktop-pet-plugin
python3 build_package.py
```

完整重启：

```bash
./start-all-services.sh
```

不要使用 3000 端口。

## 安全与凭据

- 测试用模型 API Key 只通过运行界面输入，不应写入源码、文档、日志或 Git。
- 交接后如需重新配置模型，请在 onboarding 或模型设置中重新输入。
- 不要将 `AmitiaData`、本地令牌、设备身份私钥或测试凭据上传到 Git。
