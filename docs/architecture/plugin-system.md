# Amitia 插件体系

## 目标

Amitia 宿主只提供通用插件协议、贡献适配器、动态扩展点、权限与安装生命周期。官方市场、第三方市场、工具、页面、主题、渠道和自动化都是普通插件，不拥有宿主专属安装入口。

插件市场插件通过以下通用原语自行组合业务：

- `host.network.request` 获取自身或其他来源的插件目录。
- `host.plugin.preview` 固定包地址与 SHA-256 后生成安装预览。
- `host.plugin.install` 携带预览会话和用户逐项确认执行安装。
- `ui_provider`、`ui_slot`、`ui` 贡献渲染自己的市场页面。
- `tool`、`hook`、`event`、`schedule` 等贡献实现插件自己的业务逻辑。

宿主不认识“市场条目”“市场分类”或“市场安装”这类业务概念。市场语义完全属于插件。

Amitia 官方市场本身也是预装普通插件 `com.amitia.builtin.plugin-market`。它通过 `tool`、`ui`、`ui_provider` 和 `route.registry` 贡献提供页面与行为，不拥有专用数据库、固定前端路由或市场安装接口。禁用该插件后，市场页面和导航随插件生命周期一起消失；其他第三方市场插件仍可独立提供相同能力。

## 插件包

插件包以统一的 `manifest.json` 为权威清单，包含插件身份、版本、发布者、模块、运行 Adapter、服务提供与依赖、动态贡献能力、静态贡献、权限、兼容性、资源、数据和完整性信息。包归档另行携带内容树、文件哈希和发布签名。

`.amitiax`、`.gamex` 和 `.petx` 只是包文件扩展名别名。三者共用同一 Manifest、签名、安装事务、Adapter Registry、Service Graph、权限和生命周期，不维护独立插件格式或第二套运行时。

安装流程不接受清单自报结果作为安全证明。宿主会重新计算归档哈希和内容树，验证签名与发布者信任，扫描包安全风险，检查兼容性和权限，再生成短期预览会话与确认令牌。

## 贡献模型

贡献由两层组成：

- `kind` 是插件语义类型，可使用命名空间字符串，例如 `com.example.market/page`。
- `adapter` 是必填的宿主执行协议，例如 `ui`、`ui_slot`、`tool`、`event`、`hook`、`schedule`。

所有贡献必须显式声明已注册 Adapter 和正数 `contractVersion`。宿主按照 Adapter 声明的 Schema 校验贡献的 `spec`，不再根据 `kind` 隐式推断执行协议。

新增执行语义时，由受信任的 Adapter Provider 向 Adapter Broker 注册新协议；普通插件扩展业务类型只需要复用已安装 Adapter。

示例：

```json
{
  "id": "com.example.market/page",
  "kind": "com.example.market/page",
  "adapter": "ui",
  "contractVersion": 1,
  "schema": {
    "type": "object",
    "required": ["kind", "slot"],
    "properties": {
      "kind": { "type": "string" },
      "slot": { "type": "object" }
    }
  },
  "name": { "default": "Plugin Market" },
  "spec": {}
}
```

## 动态 UI

插件可声明新的 `ui_slot`，再通过 `ui` 贡献向该槽位注册内容。槽位所有者、父槽位、支持类型、作用域、排序、失败隔离和合同版本由槽位定义决定。

SDK 的强类型槽位表可继续为内置宿主槽位提供精确类型；插件自定义槽位通过 `slots.registerDynamic()` 注册，不需要宿主新增槽位枚举。Web 与 Flutter 运行时都消费同一套动态槽位快照。

`ui` 贡献的渲染 `kind` 也接受命名空间类型，例如 `com.example.market/page`。渲染面由显式 `sandbox` 决定，Schema UI、受控 Web 模块和宿主原生适配继续共享同一权限与生命周期边界。

## 标准目录

`amitia.plugin.catalog/v1` 是不依赖任何具体市场的插件索引格式。目录可托管在官方、第三方或私有 HTTPS 服务中，条目必须包含扩展 ID、版本、包 URI 和 64 位 SHA-256。

目录中的 `trustLevel`、`publisher` 和 `signature` 只用于发现和展示，不能替代宿主对 `.amitiax` 包的最终验证。宿主的远程目录来源会保留目录 ID、来源类型、来源 URL、发布者和条目元数据，用于审计与来源追踪。

来源信息会写入安装预览和包构件的验证报告，因此在短期预览会话过期后仍可追溯。多个第三方或私有目录通过 `AMITIA_EXTENSION_CATALOG_URLS` 同时注册，各目录使用独立来源 ID。

JSON Schema 位于 `docs/schemas/plugin-catalog.v1.schema.json`，TypeScript 类型和校验函数位于 `sdk/plugin-sdk/src/catalog.ts`。

## 安装协议

1. 调用方取得目录条目，并确认 `packageUri` 与 `sha256`。
2. 调用 `host.plugin.preview`。
3. 宿主下载包、校验哈希、密钥、内容树、清单、兼容性、安全策略和权限。
4. 宿主返回预览会话、必需确认项、权限、作用域、风险标记和问题列表。
5. 调用方展示审查结果，并收集用户对每个必需确认项的明确同意。
6. 调用 `host.plugin.install`，携带预览会话和确认映射。
7. 宿主重新绑定预览会话、签发确认令牌并进入安装 Saga。
8. 安装完成后发布扩展、贡献和 UI 缓存变更事件。

预览会话绑定空间、作用域、包哈希、清单哈希、内容树、权限策略和过期时间。跨空间使用、篡改确认项、重复消费或超时都会失败。

## 发布边界

用户端只读取发布目录和安装插件，不提供插件发布入口。管理端通过“发布插件包”一次上传 `.amitiax`、SBOM、分类和版本说明；扩展 ID、版本、名称、描述、发布者、信任级别、关键词、兼容性和包内签名均从插件包解析，不依赖手工创建插件条目。

## 当前开放度

- 自定义贡献 `kind`：支持，复用已注册适配器。
- 自定义贡献 Schema：支持，按 JSON Schema 2020-12 在安装前校验，禁止远程 `$ref` 加载。
- 运行时贡献注册：支持。
- 服务提供、服务依赖和拓扑激活：支持。
- Adapter 信任分级和协议版本：支持。
- 外部 Adapter Provider 的 build、activate、deactivate、discard 和 health RPC 契约：支持。
- 外部 Adapter Provider 的 stdio JSON-RPC 传输、并发调用、取消和关闭回收：支持。
- Manifest `adapterProviders` 声明、可信服务 Supervisor 挂载和安装/卸载生命周期同步：支持。
- 插件服务目录和运行时绑定：支持。
- 插件自定义 UI 槽位：支持。
- 动态槽位注册：支持。
- 通用网络发现：支持。
- 通用插件预览和安装 Host API：支持。
- 标准第三方目录：支持。
- 来源与发布者追踪：支持。
- 逐项权限确认：支持。
- 市场专属宿主能力：不提供。
- 官方市场作为预装普通插件：支持。
