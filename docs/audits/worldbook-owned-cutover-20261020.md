# 世界书设备归属统一修复

日期：2026-10-20。

## 结论

世界书已从旧 Core 全局表和旧 `/api/world-book` 运行入口迁移到设备归属资源账本。知识库继续作为共享 RAG，世界书继续保留角色配置与关键词匹配语义，两者没有合并。

## 统一结构

- 资源类型：`worldbook`
- 全局规则分区：`@global-worldbook`
- 角色规则分区：当前角色 ID
- 所有者：执行设备或统筹 Core
- 写操作：`requestId`、原执行范围、资源 revision 和所有者确认
- 聊天加载：当前所有者数据库中的全局规则与当前角色规则
- 提示词：世界书作为结构化不可信上下文加载，不再拼接到系统指令正文

## 运行入口

- 读取：`GET /api/device-mesh/v1/business/data?kind=worldbook`
- 保存：`POST /api/device-mesh/v1/business/worldbook/manage`
- 匹配测试：`POST /api/device-mesh/v1/business/worldbook/match`

旧 `/api/world-book` 注册、处理器、仓储、提示词端口和服务初始化已删除。

## 数据迁移

启动时按当前节点执行所有者迁移。设备模式迁到物理设备 Owner，Cloud Core 模式迁到 Core Owner。旧全局规则迁到 `@global-worldbook`，旧角色规则迁到对应角色。迁移记录位于 `kernel_device_owned_resources` 的 `checkpoint/worldbook-migration`，重复启动不会重复写入。

## 两端接入

- 电脑端世界书页使用设备归属对话服务的 `data` 和世界书管理接口。
- 手机端世界书页使用 `DeviceOwnedChatService` 与新的 `WorldBookService`。
- 两端都核验响应所有者、请求编号、checkpoint 版本和执行范围；核验失败不显示保存成功。
- 页面支持全局规则和当前角色规则两个明确作用域。

## 验证

- 真实 TLS、设备签名、配对和双 Core 测试覆盖设备所有者、统筹 Core、写入幂等、CAS 冲突、删除、匹配测试和聊天快照加载。
- 定向迁移测试覆盖全局与角色分区及重复启动幂等。
- 备份测试覆盖 Owner、角色、revision 与规则正文恢复。
- 聊天测试覆盖启停状态、角色隔离、全局与角色规则合并。
- 电脑端类型检查及世界书定向测试通过。
- 手机端世界书写入与所有者确认测试通过，定向静态分析无错误或警告。
