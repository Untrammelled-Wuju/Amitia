# 深度审计：日程提醒业务未完整收敛

时间：2026-10-14，Asia/Shanghai。源码基线：`22ed4860756c45fa9f0f1d9618bc8ae77cd7e0b4`。

## 结论

确认一项 P1 问题：日程提醒同时存在旧工具写入、当前用户提醒业务和 Kernel 调度底座，用户功能没有完整贯通。旧创建工具返回成功，但写入的记录不进入当前手机／电脑提醒页面和提醒执行器；手机提供的每月重复提醒在执行一次后会关闭；提醒状态和清除背压入口还存在固定返回或无实际操作的情况。

按用户要求，确认这项重大问题后停止扩展审计。不能据此推断项目只剩这一项问题，也没有估算其余问题总数。本轮未修改业务源码，未构建手机或桌面产物，未启动正式项目，未写入真实数据库，未发送消息，未提交或推送 Git。

## 三条并存的路径

| 路径 | 实际存储和操作 | 手机／电脑的对应接入 | 结果 |
|---|---|---|---|
| 旧 `create_schedule` 工具 | 直接 INSERT `schedules`，返回 `SUCCESS` 和已确认副作用 | 提醒管理页面没有读取这张表 | 旧工具调用成功不等于当前提醒已建立 |
| 当前用户提醒业务 | `/api/reminders` → `reminder.Service`／`Repository` → `reminders`；30 秒轮询到期项，调用聊天服务发送提醒 | 两端提醒页面均使用 `/api/reminders` | 页面与这条业务真实接通，但没有收编旧工具的写入路径，重复规则也不完整 |
| Kernel Schedule | `ScheduleService`、`extension_schedule_definitions`／states／triggers／runs 等 | 手机开发者调度页面使用扩展调度接口 | 已有更完整的调度底座，但不能把它的存在当作旧 `schedules` 已进入执行器的证据 |

不同用途的调度器可以并存。这里的错误是同一“用户创建日程／提醒”的功能承诺没有映射到实际可管理、可执行的业务记录。

## 1. 创建工具报告成功，当前提醒看不到也不会消费

`backend/internal/agent/tool/schedule.go:25` 注册 `create_schedule`，描述包含提醒、闹钟、单次和重复日程。`main.go:147` 仍给这套工具绑定实际应用 SQL 连接。

`schedule.go:92` 直接向 `schedules` 写入标题、描述、截止时间、重复模式、渠道和 pending 状态。`schedule.go:98` 随后返回“OK 已创建日程”，并设置 `schedule_create` 副作用 `Confirmed=true`。

当前手机服务 `mobile_app/lib/core/services/reminder_service.dart:10` 和电脑页面 `front/src/views/reminders/Reminders.vue:445` 都请求 `/api/reminders`。后端模型 `backend/internal/reminder/model.go:21` 明确对应 `reminders` 表。实际到期扫描在 `repository.go:56` 和 `service.go:320`，只读取 `reminders`。

旧 `schedules` 表不是历史基线中的孤立定义：旧工具和扩展资源登记／清理代码仍操作它。但本轮运行源码搜索未发现负责把这些记录导入当前提醒业务或实际消费这张表的提醒执行器。

旧工具还没有把执行上下文中的角色、对话和 Space 保存到日程业务行，仅在工具审计信息中记录部分上下文；不能用审计日志代替正式提醒的归属和执行参数。

隔离复现结果：

- 初始 `GET /api/reminders` 返回业务码 200、空列表。
- 调用现有工具执行入口后，返回 `SUCCESS`、已确认副作用和“已创建日程”。
- `schedules` 有 1 条记录，`reminders` 为 0 条。
- 再次调用实际提醒 Handler 的 GET，仍返回业务码 200、空列表。
- 正式 `Reminder.Repository.Due(50)` 返回 0 条；测试中旧日程时间已到期。
- 正向对照：通过实际提醒 Handler 的 POST 创建提醒后，GET 能列出该记录，证明不是所有列表请求都被模拟成空结果。

这验证的是现有旧工具调用路径的错误成功语义，不代表每次当前聊天都会暴露并调用该旧工具。新聊天的工具权限、暴露规则和工具选择还可能影响可达性；即使某入口屏蔽它，当前应用仍保留的写入实现和提醒业务也没有整合完成。

## 2. 手机“每月重复”实际只执行一次

`mobile_app/lib/features/reminders/presentation/pages/reminders_page.dart:312` 提供 `monthly: 每月`，创建／更新时把它作为 `repeatRule` 传给后端。旧工具也明确声明支持 monthly。

提醒服务接受并保存该规则，但 `backend/internal/reminder/service.go:362` 的推进分支仅处理 `daily` 和 `weekly`；其余规则进入 `default`，在 `service.go:377` 设置 `enabled=0`。monthly 因此被按一次性提醒关闭。

隔离验证中通过实际 POST 创建 `repeatRule=monthly` 的提醒，接口确认成功。只在内存数据库内调整到期时间，并让真实 `Service.Start`、到期扫描、`TriggerNow` 和 `advance` 执行一次。聊天发送端采用隔离返回的替身，不调用模型或真实渠道。触发成功后，该提醒的实际 `enabled` 变为 0。

电脑当前选项只包含一次、每天、每周，手机额外提供每月，两端与后端重复规则契约不一致。

## 3. 状态面板存在固定“正常”返回

本节为调用链静态确认，未模拟真实渠道故障或实际队列拥塞：

- `backend/internal/reminder/service.go:258` 的 `QueueSummary` 由提醒列表计算数量，`recentFailures` 固定为 0、`backpressure` 固定为 false，未读取触发历史中的失败或真实背压状态。
- 手机在 `reminders_page.dart:732`、`:736` 直接显示这些字段，所以“失败 0”“队列正常”不能视为真实状态核验结果。
- `backend/internal/reminder/handler.go` 的 `ClearBackpressure` 只返回“背压标记已清除”，没有调用清理逻辑。手机服务和电脑页面均真实请求该入口，因此这是后端成功响应缺少实际操作，前端无法通过响应判断。

这些问题属于同一日程提醒功能的管理、执行和状态反馈未闭环，不另算新的审计项。

## 复现方法与边界

先尝试现有项目端口上的 `/api/reminders`，当时 `127.0.0.1:18899` 没有服务监听，因此没有进行真实服务写入，也没有为了审计启动完整项目。

随后使用 Go HTTP `httptest` 和 SQLite 内存库；建表直接采用当前声明式基线中的相关表，调用实际工具执行入口、Reminder Handler、Repository 和 Service。模拟请求设置固定审计 Space，绕开正式身份认证的复杂装配；本轮复现针对业务连接和执行效果，不声称测试了生产鉴权、设备路由或真机端到端流程。

复现断言同时检查 HTTP 状态和业务响应码，避免把“HTTP 200、业务码 400”误计为创建成功。所有业务数据都在内存库中销毁。

证据和复现程序保存在本地忽略的审计输出目录：

- `artifacts/audits/reminder-integration-20261014/proof.json`
- `artifacts/audits/reminder-integration-20261014/reproduce.go.txt`
- `artifacts/audits/reminder-integration-20261014/reproduction.log`

## 建议整合方向

统一用户提醒的业务实体、归属、创建、编辑、删除、重复规则、执行状态和历史记录；聊天工具、手机、电脑及插件调用共同走这套业务服务。可以复用现有提醒代码和 Kernel 的可靠调度能力，保留插件调度的独立职责与接口。

创建成功应同时代表实际业务记录持久化和进入可执行调度流程，而非只写入旧表。将已有创建工具改为这套入口后，再退役旧 `schedules` 读写和多余执行适配。固定状态值与清除背压的空操作要替换成真实状态源和实际执行结果。

补齐 monthly 的下一次触发计算与两端统一规则，并覆盖日期边界。定向验收至少包含：工具创建后两端可见、到期实际进入发送、重复提醒继续有效、归属正确、失败可查、清除操作真实生效、同一请求重试不重复创建或触发。

本报告没有执行上述整改。
