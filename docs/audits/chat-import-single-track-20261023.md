# 聊天导入单轨修复与生产模拟

日期：2026-10-23。

## 原问题

聊天导入同时存在旧 `/api/import` 直接写历史 `conversations`、`messages` 表的链路，以及 `/api/imports/*` 导入链路。电脑端在确认后还会执行 Device Mesh 的 `from-import` 将数据提升到 Owner 账本，手机端没有执行该步骤，因此手机提示导入完成时只写入了历史表。旧 `ImportView.vue` 和 `/api/imports/upload` 也仍留在源码中，其中上传接口返回固定成功但没有保存任何数据。

## 单轨设计

- `chat_import_batches` 保存导入批次、角色、状态、幂等键和 Owner 晋升结果。
- `chat_import_items` 保存规范化消息。
- `/api/imports/parse-text` 只解析，不写聊天。
- `/api/imports/confirm` 只写新的批次暂存表。
- `chat-management/from-import` 通过 Device Mesh 将批次内容幂等写入 `kernel_device_owned_resources`。
- 摘要从批次暂存消息生成，记忆候选使用既有 Owner 记忆候选接口。
- 历史 `source=import` 会话由版本化迁移复制到新批次表，旧表只作为迁移来源保留。

## 客户端接入

- 电脑端解析、确认、Owner 晋升、摘要和记忆提取全部使用真实接口。
- 手机端增加与电脑端相同的 Owner 晋升，晋升成功后才显示“已加入设备聊天”，并提供进入正式聊天的入口。
- 两端导入历史直接读取新批次状态，可对迁移出的待晋升批次执行“加入聊天”。
- 删除旧 `/api/import`、旧 `ImportView.vue`、虚假上传接口和旧导入专用记忆接口。

## 云端与其他模式

- `/api/imports` 被标记为 Device Agent 本地路径，避免 Cloud Core 保存设备导入源数据。
- Device Agent 路由注册同一套导入接口。
- Owner 晋升继续走 Device Mesh 业务接口，本地、Device Agent、Cloud Core 和统筹模式使用相同会话契约。
- 云设备执行门禁新增检查，防止 `/api/imports` 回退到 Cloud Core。

## 验证

- 后端导入批次仓储、系统接口、Device Mesh 导入、迁移和编译级测试通过。
- 前端类型检查和导入晋升契约测试通过。
- Flutter 静态分析、设备本地路由测试通过。
- 云设备执行契约、桌宠单轨和生命周期门禁通过。

## 真实生产模拟

从当前后端源码重新编译临时核心，使用独立临时数据目录和动态端口启动完整 Core、Qdrant、SurrealDB：

1. 真实解析 3 条标准格式消息。
2. 真实确认生成 1 个批次和 3 个暂存消息。
3. 使用相同内容重复确认，返回同一个批次 ID。
4. 历史 `conversations`、`messages` 表始终为 0。
5. 通过真实 Device Mesh `from-import` 晋升，生成 1 个 Owner 会话和 3 条 Owner 消息。
6. 使用不同请求 ID 重复晋升，仍返回同一个会话，Owner 资源没有增加。
7. 云端统筹链路通过 `import-promoted` 回调设备本地，批次状态更新为 `promoted`，并记录正式会话 ID、Owner 和 Core。
8. 旧 `/api/import`、`/api/imports/upload` 和旧导入专用记忆入口均返回 404。
9. 正式历史查询实际返回 1 个导入会话和 3 条消息，消息均带原批次、原 Owner 和原消息 ID 的导入来源。

模拟数据与临时核心已移动到可恢复目录：

`/tmp/u-ai-chat-import-final-cleanup-20261023`

## 验证边界

临时模拟环境没有模型配置，因此没有调用真实模型生成摘要或记忆候选；摘要生成使用系统接口的确定性测试替身验证，记忆候选复用既有 Owner 记忆测试和客户端契约。未修改管理端，未修改编译后发布产物，未使用 3000 端口，未推送 Git。
