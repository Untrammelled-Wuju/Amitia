# 角色运行状态设备归属统一修复

日期：2026-10-21。

## 结论

心理、关系、需求状态已从旧全局表运行链迁移到 `kernel_device_owned_resources`。Owned 聊天、语音级联、电脑端心理页、手机端心理页、桌面宠物情绪读取和备份均切换到同一 Owner 账本。

旧 `psyche_states`、`relationship_states`、`need_states` 仅保留为一次性迁移来源，不再作为正式读写入口。旧 SQLite ContextLoader、旧状态 Store、旧备份贡献器和旧状态读取工具源码已删除。

## 统一结构

- 资源类型：`psyche`、`relationship`、`needs`
- 心理与需求资源：角色级 `role`
- 关系资源：角色级 `user_character`
- 每个资源包含 Owner、角色、revision、来源请求、来源会话和状态更新时间
- 对话状态变更与助手回复在同一次 Owner 提交中完成
- 重复请求使用既有 inbox/outbox 与 request id 幂等机制

## 运行入口

- 读取：`GET /api/device-mesh/v1/business/runtime-state`
- 聊天写入：Owned 回复提交事务
- 语音写入：`OwnedBusiness.ApplyRuntimeState`
- 旧 `/api/psyche/snapshot`、`/api/psyche/state` 与 `/api/psyche/messages/:id` 已退役

## 数据迁移

启动时同时迁移物理设备 Owner 与 Core Owner。迁移按角色、资源类型和 Owner 幂等执行，每个 Owner 写入独立迁移 checkpoint。

迁移不覆盖已存在的账本资源，新系统数据始终优先。旧表在迁移后不再参与聊天、页面、桌面宠物或备份运行链。

## 两端接入

- 电脑端角色详情新增“心理状态”标签
- 电脑端角色详情和仪表盘使用 Owner 运行状态接口
- 手机端 `CharacterDetailService` 使用同一接口并保持现有心理页 DTO
- 电脑端与手机端均校验返回 Owner、角色和正式响应结构

## 备份与恢复

新增 `runtime_state.records` 数据可移植组件，备份完整 Owner、类型、角色、revision、更新时间和正文。旧心理与关系备份组件及其恢复端口已删除。

## 定向验证

- 后端聊天空包完整测试通过
- 设备归属业务包完整测试通过
- 桌面宠物行为装配测试通过
- 治理迁移、运行时加载器与启动装配定向测试通过
- 前端类型检查通过
- 前端运行状态投影与角色路由测试通过
- 手机端相关文件静态分析无问题
- `go build ./cmd/server` 通过
- 前端生产构建通过

## 真实运行验证

- 从当前源码重新编译并部署桌面核心
- 完整启动 Electron、前端、Core、Qdrant、SurrealDB 与运行时宿主
- `/readyz` 返回 `ready`，阻断、失败和降级项目均为 0
- 真实聊天触发 `psyche`、`relationship`、`needs` 三个资源写入，revision 均为 1
- 心理页从 50% 效价、0% 熟悉度更新为 51%、1%
- 完整重启后账本 revision、数值和更新时间保持不变
- 旧 `/api/psyche/snapshot` 返回 404
- 当前运行核心与本次源码构建产物 SHA-256 均为 `604d7f1660343c13e2999d360062dbadbb17f4713a29a590151f491f2714bb01`

真实测试创建的临时角色、会话、消息、状态资源和请求账本已按精确编号清理。测试后有效角色数为 0，临时 Owner 资源数为 0。

## 验证边界

当前环境未构建 Android Release APK，也未执行 Android 真机覆盖安装。手机端完成源码接入和静态分析，仍需在 Android Release 构建后执行真机 API 与页面验收。

本轮未提交或推送 Git。
