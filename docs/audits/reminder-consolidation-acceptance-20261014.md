# 日程提醒与主动任务整合验收

范围：已确认方案的全量源码整合及定向验证。没有 APK／桌面安装包构建、真机安装、Git 提交或推送。本文件区分源码与自动化验证、实际本地服务检查、尚未进行的发布验收。

## 开发清单

| 项目 | 实现位置 | 验证依据 |
|---|---|---|
| B01、B06、B07 | reminder 服务、操作、历史、设置及 HTTP handler | 创建幂等、跨设备／插件隔离、版本冲突、真实历史与清理测试 |
| B02 | Kernel Schedule、SQLite repository | 到期游标、原子回滚、租约续期、代次失效、并发序列、DST／月末、恢复、次数结束测试 |
| B03、C01、C02 | reminder_runtime、Owned business／coordination | 实际 TLS 配对与签名调用；客户端断开后云 Core 继续执行；私有来源离线拒绝且不复制；Owner／Provider 变更拒绝执行 |
| B04 | create_schedule 与 Kernel 工具注册 | 共享服务调用、任务 ID 与确认回执；无旧表写入 |
| B05、C03 | scheduled business、delivery、notificationruntime | 稳定消息检查点、单连接事务提交与回滚、撤权拒绝、分目标状态、通知回执单调更新与设备隔离 |
| M01、M02、D01 | 现有两端提醒页面与 DTO／契约 | 手机契约 4 项、网页契约 3 项；Dart 定向分析与 Vue 类型检查 |
| M03、D02 | 原生通知运行时、桌面 SSE、网页 scheduled notification | 通知显示／点击／失败 ACK、偏好设置、正确 Owner／角色路由；桌面 3 项与网页通知 3 项测试 |
| P01、A04 | reminder_host_routes、Host API、SDK scheduled-tasks | 真 UI Schema 可信范围注入、实际 Gateway 调用、SDK transport 及权限隔离测试 |
| P02 | 技术 Schedule API 与开发者页 | host_workload 不进入技术列表、不被通用取消／卸载接口越权操作 |
| A01、A02、A03 | plugins/proactive 运行时、manifest、两端 UI Schema 与角色入口 | 无独立 timer；策略、配额、免打扰、冷却与未回复减速接共享账本；插件运行时／UI 契约及打包验证 |
| X01 | reminder archive／backup contributor、owned-resource retirement | 签名当前格式备份、恢复默认暂停、不导出执行授权、不重放旧触发；旧格式／篡改／越权拒绝 |
| 旧数据退役 | migration 20261014001 | 清除旧业务行、不影响新 Kernel 任务及无关数据；缺旧表幂等测试 |

## 关键行为

聊天、手工、插件创建都保存到 Kernel Definition／State／Trigger／Run 账本。用户提醒页面只管理 user_reminder，主动任务通过插件统一管理；开发者技术调度保留，但没有第二个用户业务执行器。

提醒执行绑定创建时确认的 Owned 角色、会话和 Owner。模型调用之后，最终消息提交再次核验当前权限和代次，并与 Kernel 共用数据库事务；取消、撤权和 Provider 切换不能被迟到的模型结果绕过。

消息保存、渠道意图、系统通知分别显示实际结果。预览仅检查配置与授权，不谎称模型安全评估或已经发送；排队、部分成功、未知与失败不会被空响应转换为成功。

桌面／网页系统通知通过设备级 SSE 与持久 outbox 关联，只有实际展示／点击／失败才更新对应回执。通知偏好禁用会同步终端注册并取消未投递通知。云 Core 不依赖手机在线完成消息保存；终端离线不伪装成通知已展示。

保留主动程度、规则、角色级配额、免打扰、最短间隔、未回复冷却及随机抖动。冷却保留小数倍率，极端组合饱和为最大正时长，避免 duration 溢出绕过策略；清理保护相同策略观察窗口。

历史清理真实删除终态明细，保留触发去重、次数预算与未完成投递证据。当前格式恢复的只读历史同样从统一定义中实际删除，维护操作不推进任务代次或改变执行配置。

## 定向测试

后端涉及的 reminder、Kernel Schedule、Kernel SQLite、Owned business／coordination、notificationruntime、SSE、迁移、Host UI 范围与服务器运行装配测试通过。没有执行 go test ./... 或无关模块全量测试。

SDK 类型检查与真实 transport 契约通过；主动任务插件运行时、UI 契约、构建与包验证通过。插件包位于根目录 plugin，源码位于 plugins。旧 legacy UI 源码与空目录已删除。

最终交叉检查删除手机角色调试页旧主动任务／每日重生入口和 CompanionService 六个退役接口方法；保留现行时间线、延迟回复与生活规则功能。手机与网页旧 proactive_message 事件、专属通知及聊天监听也已删除，统一 scheduled_notification 与正常消息同步保留。删除后网页 12 项、手机 4 项定向测试与类型检查通过；Dart 没有错误或警告。

前端源码类型检查通过；定向配置排除既有测试文件缺失 Node typings 的无关问题。手机定向分析没有错误或警告，存在既有格式／弃用 info。自动化通知测试不能替代真实 OS、浏览器权限弹窗或真机后台验收。

## 运行与验证边界

完整项目使用原有端口 18899、15178、15179、18000、19178、18998，不使用 3000。验证核心由本次 backend 源码重新 go build，未复用旧核心或 Runtime Package。

2026-10-14 23:22 起最新源码验证核心实际运行于 `backend/server.reminder-candidate.exe`，PID 82700，SHA-256 为 `6ee9216f45f65435de30f7be17aeb6f8194d5cf3e75f7228bb46966472444f25`。原 `server.codex2.exe` 停止后仍被 Windows 文件占用，未强行修改该文件；完整启动流程本次只在调用时指定新核心路径，未修改启动脚本、端口或数据目录。

Restart Manager 诊断确认占用者为 DeltaForceClient-Win64-Shipping.exe。用户明确选择“暂时不退出，继续源码与测试”，因此原启动文件同步延后；常规启动脚本仍优先旧 server.codex2.exe，后续恢复常规启动之前必须解除占用并同步本次新编译核心，不能将当前验证误记为已替换原文件。

完整启动返回码为 0，后端 ready=true，其余五个服务 HTTP 200。本地凭据认证的 `/api/reminders/status?kind=user_reminder`、`/api/reminders/channels` 和 `/api/reminders/trigger-history` 均 code=200，schedulerRunning=true。上述检查没有创建真实用户任务或发送外部消息。

本地已安装扩展列表为空，新主动任务包已构建和验证，未为验收擅自给真实用户安装插件或授予新权限。云端验证使用真实 TLS 集成环境及断连情景，不声称已发布到生产云服务器。

手机 APK、真实手机／桌面 OS 通知展示、生产云部署与安装包发布仍属于后续发布验收，不计为本轮源码与测试结果。
