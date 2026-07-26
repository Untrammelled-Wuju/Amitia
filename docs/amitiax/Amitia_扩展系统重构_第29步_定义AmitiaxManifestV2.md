# Amitia 扩展系统重构第 29 步实施文档

## 第 29 步：定义 `.amitiax` Manifest v2

---

## 一、步骤目标

在第 21—28 步完成 Extension Kernel 核心领域、生命周期、依赖、注册、运行、宿主 API、存储和事件系统后，正式定义 `.amitiax` v2 的分发声明格式。

本步骤目标是：

> 将 `.amitiax` 建立为 Amitia 唯一扩展包格式，并以 Manifest v2 描述 Extension、Module、Contribution、Runtime、Dependency、Permission、Scope、Resource、Lifecycle、Compatibility、Integrity 与 Developer Metadata。

Manifest v2 只是领域模型的输入格式，不是运行时真值。

固定链路：

```text
manifest.json
→ Manifest DTO
→ Schema Validation
→ Semantic Validation
→ ExtensionDefinition Builder
→ Canonical Domain Model
```

禁止：

```text
manifest.json
→ 直接启动 Runtime
manifest.json
→ 直接注册 Tool
manifest.json
→ 直接授予权限
```

---

## 二、文件名与编码

唯一入口：

```text
manifest.json
```

要求：

- UTF-8；
-禁止 BOM；
-JSON；
-文件大小限制；
-不支持注释；
-不支持动态表达式；
-不支持环境变量插值；
-不支持远程 `$ref`；
-不支持执行代码生成 Manifest。

Manifest Schema Version：

```json
{
  "manifestVersion": 2
}
```

---

## 三、顶层结构

建议：

```json
{
  "manifestVersion": 2,
  "extension": {},
  "publisher": {},
  "compatibility": {},
  "modules": [],
  "dependencies": [],
  "permissions": [],
  "resources": [],
  "lifecycle": {},
  "integrity": {},
  "development": {}
}
```

必须严格限制未知字段。

发布模式下未知字段默认拒绝；开发模式可报告但仍不得传入领域层。

---

## 四、Extension 元数据

示例：

```json
{
  "extension": {
    "id": "com.example/weather",
    "name": {
      "zh-CN": "天气扩展",
      "en-US": "Weather Extension"
    },
    "description": {
      "zh-CN": "提供天气工具、工作流和界面扩展"
    },
    "version": "1.2.0",
    "license": "MIT",
    "homepage": "https://example.com",
    "repository": "https://example.com/repository",
    "categories": ["tools", "mcp"],
    "keywords": ["weather"],
    "icon": "assets/icon.png"
  }
}
```

要求：

- ID 符合 ExtensionID；
-版本符合 SemVer；
-名称和描述长度限制；
-URL 使用允许协议；
-Icon 必须位于包内；
-分类仅用于展示；
-不得把分类当作权限或 Runtime 类型。

---

## 五、Publisher

示例：

```json
{
  "publisher": {
    "id": "com.example",
    "name": "Example",
    "keyId": "ed25519:example-main",
    "contact": "security@example.com"
  }
}
```

Publisher 声明必须与签名验证结果绑定。

Manifest 自报 Publisher 不构成信任。

---

## 六、Compatibility

示例：

```json
{
  "compatibility": {
    "host": {
      "minVersion": "1.0.0",
      "maxVersion": "2.x"
    },
    "platforms": ["windows", "macos", "linux"],
    "architectures": ["x64", "arm64"],
    "requiredFeatures": [
      "extension-kernel-v1",
      "host-api-v1"
    ]
  }
}
```

要求：

-平台枚举固定；
-架构枚举固定；
-Host 版本范围可解析；
-Feature ID 稳定；
-不兼容包不得安装为可运行状态。

---

## 七、Modules

每个包至少一个 Module。

示例：

```json
{
  "modules": [
    {
      "id": "main",
      "type": "runtime",
      "name": {
        "zh-CN": "主要功能"
      },
      "enabledByDefault": true,
      "runtime": {
        "type": "javascript",
        "entry": "modules/main/dist/index.js"
      },
      "contributions": []
    }
  ]
}
```

Module ID 在 Extension 内唯一。

---

## 八、Module 字段

建议：

```json
{
  "id": "main",
  "type": "composite",
  "version": "1",
  "name": {},
  "description": {},
  "enabledByDefault": true,
  "platforms": [],
  "runtime": {},
  "dependencies": [],
  "permissions": [],
  "resources": [],
  "contributions": [],
  "policies": {}
}
```

Module 的局部声明不得绕过 Extension 顶层安全约束。

---

## 九、Runtime 声明

示例：

```json
{
  "runtime": {
    "id": "main",
    "type": "javascript",
    "entry": "modules/main/dist/index.js",
    "protocol": "amitia-runtime-rpc/1",
    "instancePolicy": "singleton_per_module",
    "limits": {
      "maxMemoryMB": 256,
      "maxConcurrentCalls": 8,
      "maxQueueDepth": 64
    },
    "recovery": {
      "restartOnCrash": true,
      "maxRestarts": 3
    }
  }
}
```

Manifest 只能请求资源上限，宿主可收紧。

---

## 十、Contribution 声明

统一格式：

```json
{
  "id": "get_forecast",
  "type": "tool",
  "name": {},
  "description": {},
  "runtime": {
    "runtimeId": "main",
    "entryType": "tool",
    "entryName": "get_forecast"
  },
  "spec": {},
  "dependencies": [],
  "permissions": [],
  "scope": {},
  "exposure": {},
  "policies": {}
}
```

Contribution ID 最终由 ExtensionDefinition Builder 转为稳定全局 ID。

---

## 十一、Tool Contribution

示例：

```json
{
  "id": "get_forecast",
  "type": "tool",
  "spec": {
    "inputSchema": {
      "type": "object",
      "properties": {
        "city": {"type": "string"}
      },
      "required": ["city"],
      "additionalProperties": false
    },
    "outputSchema": {
      "type": "object"
    },
    "riskLevel": "low",
    "sideEffectLevel": "none",
    "modelExposure": {
      "enabled": true,
      "name": "get_weather_forecast"
    }
  }
}
```

JSON Schema 必须使用宿主支持的固定版本和安全子集。

---

## 十二、Agent Skill Contribution

示例：

```json
{
  "id": "weather-assistant",
  "type": "agent_skill",
  "spec": {
    "skillFile": "modules/skills/weather/SKILL.md"
  }
}
```

Agent Skill 本体不包含 Runtime Handler。

---

## 十三、Workflow Contribution

示例：

```json
{
  "id": "daily-weather-brief",
  "type": "workflow",
  "spec": {
    "definition": "modules/workflows/daily-weather.json",
    "toolExposure": {
      "enabled": true,
      "name": "run_daily_weather_brief"
    }
  }
}
```

---

## 十四、MCP Server Contribution

示例：

```json
{
  "id": "weather-mcp",
  "type": "mcp_server",
  "spec": {
    "transport": "stdio",
    "command": "bin/weather-mcp",
    "args": [],
    "toolAllowlist": ["get_forecast"],
    "autoConnect": false
  }
}
```

要求：

-Command 必须指向包内受完整性保护文件，或声明外部依赖；
-禁止 Shell 字符串；
-Args 数组化；
-环境变量使用 Secret Reference；
-默认不得自动连接未知 Server。

---

## 十五、Provider Contribution

用于声明：

-模型 Provider；
-语音 Provider；
-向量 Provider；
-图片 Provider；
-其他宿主定义接口。

Provider 必须声明接口版本和能力，不得直接替换全局默认 Provider。

---

## 十六、Hook 与 Event

Hook 示例：

```json
{
  "id": "message-filter",
  "type": "hook",
  "spec": {
    "hookPoint": "message.before_send",
    "phase": "filter",
    "priority": 10,
    "entryName": "filter_message"
  }
}
```

Event Subscription 示例：

```json
{
  "id": "conversation-updated",
  "type": "event_subscription",
  "spec": {
    "eventType": "conversation.message.created",
    "entryName": "on_message_created",
    "delivery": {
      "maxAttempts": 3,
      "idempotent": true
    }
  }
}
```

---

## 十七、Schedule 与 Background Task

Schedule 必须声明：

-固定 Entry；
-Recurrence；
-Timezone；
-重叠策略；
-默认 Enabled；
-输入模板；
-Scope 要求。

动态创建能力另受 Host API 权限限制。

---

## 十八、UI 与 Desktop Contribution

UI 声明：

```json
{
  "id": "settings-page",
  "type": "ui",
  "spec": {
    "slot": "extension.settings.page",
    "entry": "modules/ui/settings/index.html",
    "sandbox": "webview_restricted"
  }
}
```

Desktop 声明可以包括：

-菜单；
-命令；
-快捷键；
-托盘项；
-窗口入口。

不得声明任意 Electron 主进程代码。

---

## 十九、Dependencies

顶层和 Module/Contribution 均可声明：

```json
{
  "type": "tool",
  "target": "mcp/com.example.weather/get_forecast",
  "version": "^1.0.0",
  "required": true,
  "scope": "execute",
  "resolution": "installed_preferred"
}
```

必须转换为 DependencyDefinition。

---

## 二十、Permissions

Manifest 只声明需求：

```json
{
  "id": "network.http.request",
  "reason": {
    "zh-CN": "访问天气服务"
  },
  "constraints": {
    "domains": ["api.example.com"],
    "methods": ["GET"]
  }
}
```

不得声明：

```text
granted=true
```

---

## 二十一、Scope

Manifest 可声明允许或推荐 Scope：

```json
{
  "allowed": ["global", "character"],
  "default": "character",
  "requiresExplicitBinding": true
}
```

Manifest 不能创建具体角色绑定。

---

## 二十二、Resources

资源声明：

```json
{
  "id": "weather-reference",
  "type": "reference",
  "path": "resources/weather.md",
  "mime": "text/markdown",
  "retention": "extension_private"
}
```

资源路径必须经过 Package Security。

---

## 二十三、Lifecycle

示例：

```json
{
  "lifecycle": {
    "dataMigrations": [
      {
        "id": "settings-v1-to-v2",
        "from": "<2.0.0",
        "to": ">=2.0.0",
        "runtime": "task",
        "entry": "migrations/settings-v2.js",
        "reversible": true
      }
    ]
  }
}
```

不允许安装阶段任意执行脚本。

Lifecycle Entry 只能由 Lifecycle Manager 在安全阶段调用。

---

## 二十四、Integrity

Manifest 可以包含文件清单引用，但最终完整性以包安全层生成的内容树和签名为准。

示例：

```json
{
  "integrity": {
    "contentTree": "sha256:...",
    "files": "integrity/files.json"
  }
}
```

---

## 二十五、Development

仅开发包允许：

```json
{
  "development": {
    "revision": "dev-20260726-01",
    "sourceMaps": true,
    "hotReload": true
  }
}
```

生产安装时忽略或拒绝危险开发字段。

---

## 二十六、禁止字段

Manifest 禁止：

-任意代码字符串；
-宿主数据库配置；
-用户目录绝对路径；
-Secret 明文；
-Permission Grant；
-角色 ID；
-会话 ID；
-Runtime Actual State；
-Tool Handler 地址；
-Go 函数名反射；
-Electron 主进程脚本；
-关闭安全检查字段；
-远程自动下载执行项。

---

## 二十七、Canonicalization

Manifest 解析后转换为 Canonical DTO：

-字段默认值固定；
-数组顺序规则固定；
-ID 规范化；
-SemVer 规范化；
-路径规范化；
-未知字段处理固定；
-计算 Manifest Hash。

原始 JSON 不直接进入领域层。

---

## 二十八、版本升级规则

Manifest v2 的兼容扩展：

-新增可选字段；
-新增 Contribution Type 需 Host Feature；
-破坏性变更进入 Manifest v3；
-不得用隐式行为改变已有字段语义。

---

## 二十九、Schema 产物

必须生成：

```text
schemas/amitiax-manifest-v2.schema.json
schemas/contributions/*.schema.json
schemas/runtime/*.schema.json
schemas/permissions/*.schema.json
```

---

## 三十、测试要求

覆盖：

-最小 Manifest；
-多 Module；
-全部 Contribution；
-未知字段；
-非法 ID；
-非法 SemVer；
-路径越界；
-Secret 明文；
-Permission Grant；
-跨平台；
-开发字段；
-同版本不同 Hash；
-Canonicalization；
-大型 Manifest；
-恶意深度；
-错误 Schema。

---

## 三十一、实施任务

1. 定义 Manifest v2 DTO。
2. 定义顶层 JSON Schema。
3. 定义 Module Schema。
4. 定义 Runtime Schema。
5. 定义各 Contribution Schema。
6. 定义 Dependency/Permission/Scope/Resource Schema。
7. 定义 Lifecycle/Integrity/Development Schema。
8. 实现 Canonicalization。
9. 实现 Manifest → Domain Builder 输入映射。
10. 生成示例包。
11. 建立 Schema 版本兼容测试。
12. 冻结旧 Manifest 字段新增。
13. 输出旧 v1 → v2 映射表。
14. 完成安全测试。

---

## 三十二、验收标准

1. `.amitiax` 只有一个 Manifest v2 入口。
2. Manifest 与领域模型分离。
3. 支持多 Module。
4. 覆盖全部 Contribution 类型。
5. Runtime、Dependency、Permission、Scope 边界明确。
6. 不允许 Secret、Grant、角色绑定和运行状态。
7. Schema 严格且可版本化。
8. Canonicalization 稳定。
9. Manifest 可构建 ExtensionDefinition。
10. 旧 Parser 不再新增字段。
11. 可进入第 30 步多模块包结构。

---

## 三十三、执行约束

> Manifest v2 只描述扩展声明，不执行扩展，不授予权限，不创建具体 Scope，不决定 Runtime Actual State。

禁止：

-Manifest 直连 Registry；
-Manifest 直启 Runtime；
-Manifest 明文 Secret；
-Manifest 自报 Trusted；
-Manifest 自动 Grant；
-Manifest 引用包外路径；
-开发模式绕过关键安全。
