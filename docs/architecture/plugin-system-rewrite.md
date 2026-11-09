# Amitia 插件系统原地重构

## 目标

Amitia 只保留一套插件契约和运行时。内置插件、第三方 `.amitiax`、工具、页面、渠道、自动化、主题和服务提供者全部通过同一套安装、依赖、权限、激活、停用和卸载流程运行。

旧插件格式不保留运行时兼容。所有源码插件在本次重构中迁移到新契约，旧包由安装器明确拒绝。

`.gamex` 和 `.petx` 只保留为包文件扩展名别名。三种后缀使用同一个 Manifest、签名、完整性、安装事务、Adapter Registry、Service Graph、权限和生命周期实现，不新增独立包格式或第二套插件运行时。

## 不可插件化边界

- 启动、恢复和最小管理界面。
- 签名、发布者信任和安装事务。
- 权限执行、安全审计和密钥根。
- 数据库事务原语。
- 文件系统和进程监管。
- 插件协议、Adapter Broker 和 Service Contract。
- Agent 执行、模型协议、会话和记忆等基础运行能力。

## 单一插件模型

每个插件包包含一个 `manifest.json`。Manifest 中的 Adapter 必须显式声明，宿主不得根据贡献类型推断 Adapter。

Manifest 至少包含：

- 插件身份、版本、发布者和兼容性。
- 模块及其运行 Adapter、入口和宿主访问范围。
- 模块提供的服务和依赖的服务。
- 允许动态注册的 Adapter 与数量上限。
- 静态贡献、权限、资源、数据版本和完整性。

`plugincontract.Manifest` 是后端唯一的 Manifest 模型。包解析、安装预览、签名校验、恢复、版本比较、更新事务、权限同步和 Service Graph 都直接使用该模型。运行时需要的 `domain.ExtensionDefinition` 只作为单向领域投影，不再保存为第二份 Manifest，也不存在旧格式转换器。

源码目录按发布来源明确分离：

- `plugins-builtin/` 只存放随 Amitia 分发的固定业务插件，必须声明 `system.defaultInstall` 和 `system.defaultEnabled`。
- `plugins-code/` 只存放普通、官方市场、示例和第三方开发插件，不允许存放固定业务插件。

两个目录仍使用完全相同的 Manifest、权限、Adapter、生命周期和构建流程；目录分离只表达发布归属，不形成第二套运行时。

`system.managed`、`system.required`、`system.builtin` 等控制键只允许由宿主内置定义写入。普通 Manifest 使用这些键会产生校验错误，不能通过元数据冒充系统模块。

模块的服务依赖在安装前形成依赖图。循环依赖、版本冲突和缺失的必选服务阻止安装。

## Adapter Broker

Adapter Broker 是贡献执行协议的唯一入口。

Adapter 描述包含：

- Adapter ID 和契约版本。
- 支持的贡献类型及 JSON Schema。
- 权限模板和宿主能力。
- 安装、激活、停用和卸载协议。
- 调用、流式、取消和健康检查协议。
- 支持的平台和发布者信任要求。

宿主级 Adapter 只能来自系统、官方或已验证发布者。社区插件只能使用已安装 Adapter，不得注册新的宿主级执行协议。

Adapter Conformance Test Kit 通过 `RunAdapterConformance` 统一验证 Adapter 注册、贡献路由、Build、Activate、Deactivate、Discard 和失败回滚。

外部 Adapter 通过 `adapter.invoke` 和 `adapter.cancel` 承载新的执行语义。插件只能调用 Manifest `dynamic.allowedAdapters` 明确允许的 Adapter，并必须声明 Adapter 描述中的全部权限。SDK 对应入口为 `context.adapters.invoke/cancel`。

## Service Graph

服务契约包含服务 ID、版本范围、契约哈希、作用域、部署位置和提供者策略。

激活顺序固定为：

1. 解析并锁定完整依赖图。
2. 准备全部提供者。
3. 按拓扑顺序激活无依赖服务及其依赖者。
4. 执行健康检查。
5. 发布运行时快照。
6. 提交安装事务。

停用和卸载按反向依赖顺序执行。热替换使用新旧提供者并行准备、健康检查、排空旧实例、原子切换和失败回滚。

核心服务使用保留命名空间，插件不得覆盖。

## 动态贡献

插件只能在 Manifest 声明的能力范围内动态注册和注销贡献。

每次注册必须重新校验插件身份、Adapter、Schema、权限、作用域、冲突和数量上限。注册成功后发布统一快照，使后端工具目录和前端 UI Registry 同步刷新。

卸载时必须反向撤销全部贡献、监听器、服务、路由、定时器和缓存。

UI Provider 必须按扩展身份清理，不能只依赖单个贡献 ID。卸载、包替换或回滚后，旧 Provider 不得继续出现在运行时快照或提供页面能力。

内置壳页面可以通过 `allowsExtensionChildren` 声明开放挂载点。所有插件使用同一挂载规则提供子页面，宿主不得维护具体官方插件路径白名单；Web、桌面和移动端必须执行相同的开放与保护语义。

页面框使用 `display.chrome` 声明 `default` 或 `frameless`。Schema UI 的横排标签使用通用 `tab-bar` 布局或变体。宿主不得按插件 ID、贡献 ID 或官方节点 ID 注入页框和视觉分支。

UI Host 导航以当前 Router 的实际路由表为权威。动态 Provider 注册路由后即可通过 `host.ui.navigate` 导航，不得维护第三方路径白名单；仅登录、引导、隐私和恢复路径保持不可被插件导航。

Provider 导航项必须声明归属层级：`primary`、`settings`、`profile`、`workspace` 或 `contextual`。桌面主图标栏只消费 `primary`；设置子页、用户菜单、工作区上下文和插件内部入口不得混入主栏。Web 与 Flutter 使用同一归属语义。

通用扩展页面同时支持 `schema_page` 和 `web_page`。`web_page` 客户端模块通过短期页面能力令牌、入口内容哈希和模块目录边界从受控端点加载，并获得宿主提供的 `uiContext`、`uiActions` 与 `uiNavigate` 门面。

运行时调试、BDI 决策可视化和长期运行维护由 `com.amitia/runtime-insights` 提供。插件通过 `host.runtime.snapshot` 和 `host.runtime.maintenance.*` 读取或修改状态，不直接调用管理员 HTTP 接口。

## SDK 与 CLI

JavaScript 宿主把 `@amitia/plugin-sdk` 作为受控虚拟依赖提供。插件包不得通过自带同名包替换宿主 SDK；正式构建会把同一源码基线的 SDK 发行物同步到插件宿主目录，并由 Node 权限边界限制可读范围。

宿主运行时上下文统一暴露贡献、服务、Adapter、事件、存储、密钥引用、资源和类型化 Host API。工具定义中的 `metadata.handlerName` 必须与 Manifest 的 `runtime.handlerName` 一致，调用时由宿主注册表按该名称分发。

`amitia-ext init` 生成的工具项目直接声明静态工具贡献和动态能力上限，`build` 使用 CLI 自带的类型定义完成离线构建。脚手架默认发布到 Windows、macOS 和 Linux，随后可执行严格校验、打包、签名、检查、安装、启用、停用和卸载。

## 生命周期

插件生命周期统一为：

```text
待安装 -> 已安装 -> 待配置 -> 已启用 -> 运行中
                              -> 降级 -> 失败 -> 停用 -> 卸载
```

依赖者存在时不得直接移除必选服务提供者。更新失败必须恢复旧版本。可选插件故障不得阻止核心启动。

随包插件目录按扩展包名称和语义版本选择最新候选，避免历史版本重复安装。单个候选在安全扫描、Manifest 解析、依赖预检、安装、启用或迁移阶段失败时，宿主记录隔离告警并继续处理其他插件，不把插件故障提升为 Core 启动失败。

插件持久状态按扩展 ID 隔离并执行统一的字节数与键数配额。`data.backup` 在更新前生成状态备份；`data.export` 是导出卸载策略的前置能力声明；`data.uninstall` 决定卸载时保留、删除或先导出再删除状态。导出失败必须中止卸载，不得在无导出证据时删除数据。

运行时重启耗尽或隔离后，宿主必须释放该扩展的动态贡献、服务提供与依赖、待处理注册和缓存。正常更新仍保留注册，只有不可恢复故障才进入隔离清理。

## 源码迁移顺序

1. 统一 Manifest、贡献、Adapter 和服务契约。
2. 实现 Adapter Broker 和 Service Graph。
3. 重写 JavaScript 宿主、SDK、CLI 和安装器。
4. 重写前端动态路由、Provider 和 Slot 注册。
5. 迁移插件市场、渠道和页面样板。
6. 批量迁移其余内置非核心业务模块。
7. 删除旧 Manifest、旧 Adapter 映射和全部业务特权分支。
8. 执行安全、故障恢复、跨端和零残留验收。

## 完成条件

- 后端、SDK、CLI 和插件市场只接受一种 Manifest。
- 新增非核心功能不需要修改宿主业务源码。
- 第三方插件可以替换任意可替换的内置模块。
- 插件依赖图能够自动激活和反向卸载。
- 动态贡献可在运行期增删并同步到前后端。
- 可选插件全部故障时核心仍可启动和恢复。
- 卸载后不残留工具、页面、服务、监听器、进程和数据。
- 官方模块不存在独立的安装、路由、服务和生命周期通道。
