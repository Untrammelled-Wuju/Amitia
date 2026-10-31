# 数据迁移单轨化与敏感配置迁移

## 结论

项目原先同时存在旧 `amitia-config-v1` 配置迁移和新 `amitia-backup` 数据可携带系统。旧系统只处理 `app_settings`，生成明文 JSON，缺少归档校验、冲突预检、恢复前备份和敏感数据策略；电脑端与手机端又同时展示两套入口。新系统虽然覆盖角色、聊天、记忆、模型、插件、工作区等资源，但模型 API Key 没有进入迁移包，预检也没有说明导入后必须重新授权。

本次将用户数据迁移收敛到 `/api/data-portability/*` 单一路径。

## 功能迁移

| 旧能力 | 当前能力 |
| --- | --- |
| 导出全部应用设置 | `settings.records.v1` |
| 导入应用设置 | 带预检、冲突策略、事务和恢复前备份的迁移导入 |
| 旧配置导入覆盖同名设置 | `duplicate`、`skip`、`replace` 统一冲突策略 |
| 无归档完整性校验 | ZIP 清单、SHA-256、组件数量和大小校验 |
| 无恢复前备份 | 导入和恢复执行前自动创建完整内部备份 |
| 无敏感配置策略 | `includeSecrets`、敏感组件标记和强制加密 |

模型配置迁移保留模型名称、协议、地址、模型名、上下文参数、能力、供应商配置和测试状态。选择携带秘密时同时迁移模型 API Key；不携带秘密时清除顶层 API Key，并递归删除 `provider_config_json` 中的令牌、密钥、口令和凭证字段。

## 敏感数据规则

1. 迁移包选择 `includeSecrets=true` 时必须设置至少 8 位口令，否则返回 `BACKUP_ENCRYPTION_REQUIRED`。
2. 含秘密归档使用 `AMITIAE1` 加密格式，必须以口令解密后才能预检和导入。
3. 完整灾难恢复备份始终包含秘密，电脑端和手机端均要求设置至少 8 位口令。
4. 普通迁移默认不包含秘密，预检明确提示 API Key 和应用敏感设置需要在目标设备补填。
5. 模型供应商配置采用递归敏感键过滤，避免只遗漏顶层 `api_key`。
6. 旧版迁移包没有秘密策略元数据时，只有已加密旧包允许恢复敏感内容；未加密旧包包含秘密会被拒绝。

## 客户端

电脑端设置页只保留新版用户数据导出、导入和备份恢复入口，并新增“包含 API Key、令牌等敏感配置”。选择完整格式或勾选敏感配置时自动强制加密。

手机端“数据与备份”只保留迁移导出、迁移导入和完整备份。导出预检新增敏感配置选项，导入确认展示后端返回的组件警告。

迁移路径继续按现有模式路由：

- 本地模式调用本地 Core。
- 统筹 Core 模式调用当前业务数据权威。
- 远端 Owner 模式调用当前授权 Core。
- 物理灾难恢复仍路由到实际设备 Runtime。

## 删除的旧代码

- `/api/config/export`
- `/api/config/import/preview`
- `/api/config/import/confirm`
- 旧配置导出、导入预览、导入确认 Handler 与 Service 方法
- 电脑端旧配置迁移卡片和调用
- 手机端旧配置迁移操作和调用

旧路由由回归测试固定为不再注册。

## 验证

- 后端：`internal/chat`、`internal/system`、`internal/system/dataportability`、`internal/configwrite` 定向测试通过。
- 电脑端：`vue-tsc --noEmit`、生产构建和数据迁移路由测试通过。
- 手机端：修改页 `dart analyze` 通过；Flutter 测试因本机缺少 `rustup` 无法构建 `syntax_highlighter_plus` 原生资产，不是本次 Dart 源码错误。
- 真实 Core：旧三条路由均返回 HTTP 404。
- 真实 Core：无秘密导出成功，模型组件标记为敏感，模型 NDJSON 不含 `api_key`。
- 真实 Core：无秘密导入预检返回“导入后需要重新授权或补填 API Key”警告。
- 真实 Core：含秘密但不加密的预检返回 `BACKUP_ENCRYPTION_REQUIRED`。
- 真实 Core：含秘密加密导出成功，文件头为 `AMITIAE1`，预检识别 `includesSecrets=true`。
- 完整项目已从当前源码重建并启动，Core、Qdrant、SurrealDB 健康检查通过。
