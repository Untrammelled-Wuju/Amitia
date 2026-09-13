# Amitia Desktop Pet Package v1 契约

## 概述

本目录定义 Amitia 桌宠系统的唯一 Package Schema 1 契约。

Go 后端和 Electron 前端必须共同遵守本契约。

## 文件清单

| 文件 | 说明 |
|------|------|
| `manifest.schema.json` | Manifest JSON Schema |
| `action.schema.json` | Action JSON Schema |
| `integrity.md` | Integrity 算法说明 |
| `runtime-compatibility.md` | Runtime 兼容性规则 |
| `path-rules.md` | 路径规则 |
| `golden/` | Golden Fixture 目录 |

## 职责分工

| 组件 | 职责 |
|------|------|
| Go CanonicalWriter | 写入 Manifest 和 Action JSON |
| Go Validator | 验证 Manifest、Action JSON、Integrity |
| TypeScript CanonicalPackageReader | 读取 Manifest 和 Action JSON |
| PackageIntegrityVerifier | 验证 Integrity（Electron 端） |

## 关键规则

- SchemaVersion 只能精确匹配 1
- Schema 1 不再静默补字段
- `sha256` 字段在 Go 和 TS 完全一致
- Manifest 使用 `playbackMode`，不再使用 `loopType`
- ReturnTo 使用 `default`，不再使用 `default_idle`
- Quality Verdict 使用 `accepted`/`accepted_with_warning`/`needs_review`/`rejected`
