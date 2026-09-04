# Runtime 兼容性规则

## 唯一字段名

只允许：

```
minRuntimeVersion
maxRuntimeVersion
renderMode
```

Schema 2 不再兼容 `minimumRuntimeVersion` 写法。

## SemVer 规则

必须使用严格 SemVer：

```
major.minor.patch
可选 prerelease（-alpha.1）
可选 build metadata（+build.123）
```

以下必须失败：

```
1
1.0
1.x.0
abc
空字符串
负数
```

## 兼容判断

要求：

```
currentRuntime >= minRuntimeVersion
AND
(maxRuntimeVersion == null OR currentRuntime <= maxRuntimeVersion)
```

同时校验：

```
minRuntimeVersion <= maxRuntimeVersion
```

## Runtime Version 来源

不得在源码散落硬编码 `CURRENT_RUNTIME_VERSION = "1.0.0"`。

改为从构建配置或 desktop package.json 的独立字段 `desktopPetRuntimeVersion` 获取。

启动时验证它是合法 SemVer。当前桌面端唯一来源为 `desktop/package.json#desktopPetRuntimeVersion`。

Runtime v2 的握手契约版本独立记录在 `desktopPetRuntimeContractVersion`，构建门禁必须校验其与后端 `CurrentSchemaVersion` 一致；服务端在 `Hello` 阶段拒绝不匹配版本，禁止先进入 Ready 再延迟失败。

## Render Mode

Schema 2 只允许：

```
sprite
```

## Binding 规则

| Policy | 要求 |
|--------|------|
| `bound` | `sourceCharacterId` 非空 |
| `unbound` | `sourceCharacterId` 为空 |
| `legacy_inferred` | 只允许 V1Reader 产生，V2Writer 禁止写 |
