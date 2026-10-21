# 角色生活系统单轨修复

日期：2026-10-20。

## 结论

角色生活系统以 `com.amitia/lifestyle` 插件为唯一业务实现。手机端旧 `CompanionService`、旧角色调试页及其路由已退役，不再调用不存在的 `/api/companion/*` 业务接口。

该问题同时命中了三类审计风险：新旧双轨、客户端虚假接入，以及接口返回成功但关键字段未落库。修复采用单轨、契约优先、持久化边界规范化和正式生命周期安装。

## 行业标准方案

1. 单一事实源：角色生活规则、事件和时间线只由 Lifestyle 插件拥有，客户端只通过正式 UI 贡献和 Bridge 访问。
2. 契约优先：角色标识是全部生活状态命令的必填字段，由运行时 Schema 在写入前校验，禁止生成空角色键。
3. 边界规范化：插件状态统一使用驼峰 DTO，读取历史持久化数据时兼容下划线字段，避免接口成功而业务字段丢失。
4. 生命周期受控：插件包经过 preview、confirm、install/uninstall、final gate 和 generation 指针切换，禁止直接写安装目录。
5. 平台资产隔离：Windows 使用 `node.exe.zip`，macOS/Linux 使用 `node.zip`，macOS/Linux 二进制路径为 `node/bin/node`。
6. 权限与归属一致：心理状态等角色数据同时纳入手机端和电脑端的角色归属路由，避免客户端绕过 Core 协调策略。
7. 生产验收：真实启动 Electron、Core、Qdrant、SurrealDB 和插件运行时，通过真实 UI Session 执行跨会话读写、失败路径和清理复核。

## 修复内容

- 手机端删除 `CompanionService`、`CompanionStateDto` 和 `/characters/:id/debug` 调试页。
- 心理状态读取迁入 `CharacterDetailService`，继续使用正式 `/api/psyche/snapshot`。
- `/api/psyche` 纳入角色归属路由，云模式下根据当前 Core 协调策略选择设备 Agent 或业务 Core。
- 电脑端同步采用相同的心理状态归属判断。
- Lifestyle 插件删除没有执行链支撑的 `delayedReplyEnabled` 配置，避免把未实现能力展示为可用功能。
- Lifestyle 插件清单删除不存在的“旧数据迁移”声明。
- Lifestyle 插件包默认输出调整到根目录 `plugin/lifestyle`，与项目插件产物目录规则一致。
- `payload.characterId` 改为运行时必填契约，缺失时立即失败且不写状态。
- 固定事件、特殊事件、调课记录的归一化同时兼容驼峰和数据库下划线字段，并增加关键字段断言。
- Node 运行时选择按宿主平台区分归档和可执行文件路径。

## 功能边界

作息、工作、固定事件、特殊事件、课程调整、时间线、冲突检测和生活上下文由 Lifestyle 插件统一提供。

延迟回复、规则执行日志和旧全量重置接口不属于当前正式能力。旧入口已删除，后续如需恢复，必须分别接入统一 Schedule、Owned Chat、投递回执和审计账本后再开放，不得使用插件定时器或独立队列模拟。

## 生产模拟验证

- 完整项目重启后 Electron、Core、Qdrant、SurrealDB、插件宿主和前端均在线。
- `/livez`、`/readyz`、`/api/health` 均返回 200，健康项 `readyCount=6`，数据库状态为 `ok`。
- Lifestyle 包 SHA-256 为 `c0098cead44c9df5223f4f2da8539802d2914d6b28dc114a71774480d9523672`，treeHash 为 `1c0ae7ed76e28399ac9f5ab8a31f290b31563084576bd75aa0738e031a735edf`。
- 插件经正式卸载、上传、预览、确认和安装流程更新，安装记录 artifact 为 `artifact-sha256-c0098cead44c9df5223f4f2da8539802d2914d6b28dc114a71774480d9523672`，状态为 installed/enabled。
- 真实 UI Session 缺少角色时在 25 ms 内拒绝，错误指向 `/payload required(characterId)`，空角色键数量保持为 0。
- 真实 UI Session 成功写入并跨会话读回睡眠 `22:30/06:30`、固定事件 `19:10-20:05`、特殊事件日期和调课记录。
- UI 快照中存在桌面上下文 `lifestyle-character-tab`、移动上下文 `lifestyle-character-tab-mobile`，以及 `page.provider`、`route.registry` Provider。
- 旧 `/api/companion/state` 返回 404，证明旧业务入口未继续被正常使用。
- 测试角色、`v1/profile/` 空键、测试角色 profile/events 键均已按精确目标清理，并再次确认不存在。

## 验证范围

定向验证覆盖插件运行时、插件包完整性与哈希、手机端路由测试、接口路由测试、Dart 静态分析、前端类型检查、Node 平台解析测试和完整项目启动验收。

当前 macOS 环境没有连接 Android 设备，因此手机端完成了 28 项 Flutter 定向测试和静态分析，未声称执行真机安装。桌面端执行了真实生产链路模拟。
