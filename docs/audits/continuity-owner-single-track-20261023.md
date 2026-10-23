# 持续事项 Owner 单轨修复与生产模拟

日期：2026-10-23。

## 原问题

持续事项同时存在旧全局表和新的 Device Mesh Owner 资源两套事实源：

- 旧 `/api/continuity/*` 写 `continuity_threads`、`continuity_thread_bindings`、`continuity_waits`、`continuity_thread_events`。
- 新 `/api/device-mesh/v1/business/continuity` 写 `kernel_device_owned_resources`。
- 电脑端和手机端在新服务未就绪时仍会回退旧接口。
- 聊天自动识别、工作流、任务、审批和设备上线事件仍由旧 Resolver、Observer、WaitCoordinator 和 RuntimeBridge 处理。
- 旧持续事项没有迁移到 Owner 账本，新页面与聊天运行上下文会看到不同数据。

## 修复方案

采用单一事实源、Expand-Migrate-Contract、幂等事件消费和租约隔离：

1. `kernel_device_owned_resources` 中的 `continuity` 资源成为唯一正式事实源。
2. Owner 文档新增持久化 bindings，保留 conversation、workspace、workflow、task、request 和 execution 关联。
3. 自动语义识别、显式会话绑定、工作区绑定和持续事项创建迁入 Owner 链路。
4. 聊天完成后的状态提炼、事件记录和用户等待项迁入 Owner 观察提交。
5. 工作流、任务、审批和设备上线事件迁入 Owner 运行时事件和信号入口。
6. 定时与依赖等待继续由现有 Owner continuity worker 领取，使用 revision、claim lease、heartbeat 和完成确认防止重复执行。
7. 启动时将旧表线程、绑定、等待项和事件幂等复制到当前真实 Owner；旧表只作为迁移来源，不再参与正式运行。
8. 删除旧 HTTP 路由、Handler、Repository、Resolver、Observer、WaitCoordinator、RuntimeBridge 和旧上下文加载器。
9. 电脑端与手机端删除 `/api/continuity` 回退，新服务不可用时明确失败。

## 模式覆盖

- 本地设备：Owner 为当前设备。
- 云端非统筹 Device Agent：Owner 仍为真实设备，经 Device Mesh 数据端口访问。
- 云端统筹 Core：Owner 为 Core。
- Core 切换、权限变化、模式变化：旧 execution scope 被拒绝，持续事项暂停或只读，不自动重放未确认执行。

## 定向验证

- Owner 绑定、聊天观察、用户等待、依赖事件、等待唤醒和无重复模型调用通过。
- 统筹 Core Owner 的事件写入通过。
- 旧表迁移保留 thread revision、binding、wait 和 event，重复启动不重复导入。
- 自动聊天解析在首轮创建事项，在同一会话第二轮复用原事项。
- 电脑端持续事项 12 项测试通过，类型检查和生产构建通过。
- 手机端 8 项持续事项 Owner 分页与写入测试通过；本轮文件无 error 或 warning。
- 后端 `go vet`、定向完整业务测试和服务器编译通过。
- 云设备执行门禁和桌宠生命周期门禁通过。

## 真实生产模拟

使用当前正式配置模型和完整 Electron、Core、Qdrant、SurrealDB 环境：

1. 创建唯一临时角色。
2. 模型实际回复首轮生产部署请求，自动生成 1 条 Owner 持续事项。
3. 事项包含 conversation binding、模型提炼的 summary/currentState/nextAction、用户等待项和事件。
4. 同一会话发送第二轮消息，仍只有 1 条持续事项，未创建第二套记录。
5. 旧四张 continuity 表始终为 0，旧 `/api/continuity/threads` 和 `/api/continuity/signals` 返回 404。
6. 写入唯一旧表线程、binding、wait 和 event，完整重启后复制到 Owner，revision 7、1 个 binding、1 个 wait 和 1 个 event 完整保留。
7. 再次重启后没有重复迁移。
8. 按精确角色和资源 ID 清理两次模型对话、Owner 资源、计量、交互、迁移夹具和旧表数据；复核全部为 0。

最终运行状态：

- Core SHA-256：`58c1b70c89bd6d99e9ef9aa612c1e06fc225792be8a188c5108d3581b219eedd`
- Source Gate：`a35e79ce37b6cd704441f287620fa278058db79afd8a3225bf92a2c0eeed3260`
- 冻结范围：11095 项
- `/api/health`：HTTP 200，ready=true，模型已配置
- `/readyz`：ready，6 ready、0 degraded、0 blocking、0 failed

## 验证边界

工作流、任务和设备信号通过真实 Owner 业务测试验证，未在正式用户任务中故意制造失败的线上工作流。Android 本轮未构建或安装 APK；手机端完成源码、服务测试和定向静态分析。未修改管理端、版本号或发布配置，未拉取或推送 Git。
