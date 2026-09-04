# Amitia Desktop Pet Package v2 契约

## 概述

本目录定义 Amitia 桌宠系统的唯一 Package Schema 2 契约。

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
| Go V2Writer | 写入 Manifest 和 Action JSON |
| Go Validator | 验证 Manifest、Action JSON、Integrity |
| TypeScript StrictPackageContractReader | 读取 Manifest 和 Action JSON |
| PackageIntegrityVerifier | 验证 Integrity（Electron 端） |

## 旧包兼容

Schema 1 包只通过 V1Reader（隔离兼容 Reader）读取，转换为 Runtime Schema 2 内存模型，输出 Legacy Warnings。

V1Reader 不得写新 Schema 2 Package，不得修改原旧包，不得将未验证质量伪装为 accepted。

## 关键规则

- SchemaVersion 只能精确匹配 1 或 2
- Schema 2 不再静默补字段
- `sha256` 字段在 Go 和 TS 完全一致
- Manifest 使用 `playbackMode`，不再使用 `loopType`
- ReturnTo 使用 `default`，不再使用 `default_idle`
- Quality Verdict 使用 `accepted`/`accepted_with_warning`/`needs_review`/`rejected`
