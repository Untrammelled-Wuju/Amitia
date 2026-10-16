# Workbench 任务接入审计

审计日期：2026-10-15。按“确认一项重大问题即可停止”的要求，本轮确认一项 P1 接入缺陷后停止扩大范围。未修改业务源码，未构建、安装或上传发布产物。

后续处理：用户已确认方案并授权修复，实施与验证见 `docs/audits/workbench-owned-contract-fix-20261015.md`。下文保留修复前审计证据。

## P1：任务与检查点接口未接入当前 Owned 会话及云端身份契约

后台已经提供多 Agent、Coding Worker 任务状态和持久化检查点投影，但新工作台的手机端、电脑端仍直接向接口提交 UI 使用的会话引用；后台按真实会话资源 ID 精确查找。接口还仅允许本机 UI 身份，拒绝云端设备凭据产生的合法设备身份。

这是同一项跨端接入缺陷的两个断点。已确认范围是工作台任务列表、任务摘要及检查点历史；不能据此断言任务执行本身失败，也未确认其他工作台功能存在同样问题。本项属于新功能未完整接入现用系统，未发现或声称存在两套任务数据库并行运行。

### 用户实际影响

- 在 Owned 聊天中已产生后台任务时，两端工作台可能返回成功响应但显示没有任务，任务数量也显示为零。
- 使用同一 Owned 会话引用读取已有任务的检查点时返回 404。
- 使用云端设备身份访问任务列表、检查点接口时返回 403；即使改用真实会话 ID，身份限制仍会阻断访问。
- 因而用户无法通过这些页面可靠查看后台任务进度、Coding Worker 审核提示及检查点历史。现有界面和接口存在，但调用契约没有打通。

## 源码证据

1. `front/src/runtime/device-owned-conversation-reference.ts:5` 将界面会话标识生成为 `meshconv1:<ownerId>:<id>`。`front/src/composables/useDeviceOwnedConversation.ts:430`、`:439` 将聊天事件和响应的会话 ID 包装为该引用，并另外保留真实资源 ID。
2. `front/src/views/web-chat/BuiltinWebChatView.vue:340` 将当前会话交给工作台；`front/src/stores/workbench.ts` 只去除首尾空格，没有解析归属引用。`front/src/components/workbench/WorkbenchResourceView.vue:3` 原样传给任务组件。
3. `front/src/components/workbench/WorkbenchTasksView.vue:129`、`front/src/components/workbench/workbench-task-checkpoints.ts:57` 原样将此标识作为 `conversationId` 请求后端。
4. 手机端 `mobile_app/lib/core/services/device_owned_chat_service.dart:1312` 生成同类引用，`:1319`、`:1324` 分别保留原 ID 并包装 UI ID；`mobile_app/lib/features/chat/runtime/conversation_runtime_controller.dart:615` 保存 UI 会话 ID。`mobile_app/lib/features/chat/presentation/pages/chat_page.dart:3112` 将当前会话传入工作台，`mobile_app/lib/features/workbench/presentation/pages/workbench_page.dart:363` 再传入任务页。任务页 `workbench_tasks_view.dart:76`、检查点组件 `workbench_task_checkpoints.dart:52` 原样请求该 ID。
5. `backend/internal/workbenchtasks/handler.go:42` 直接读取查询参数，`backend/internal/workbenchtasks/projector.go:28` 直接用它构造查询作用域；`:57`、`:71` 对记录及父目标会话 ID 作精确比较，没有 Owned 资源解析或归属路由。任务创建侧 `backend/cmd/server/tool_multi_agent.go:218` 继承实际执行作用域的会话 ID。
6. `backend/internal/workbenchtasks/handler.go:38`、`checkpoint_history_handler.go:14` 明确只接受 `PrincipalLocalUI`。`backend/internal/middleware/security/auth.go:234` 将合法设备凭据构造为 `PrincipalTrustedDevice`，因此有设备权限也无法通过这两个入口。
7. `front/src/runtime/runtime-adapter.ts` 和 `mobile_app/lib/core/backend_transport/routed_backend_service_api.dart` 的本机接口前缀包含工作台浏览器、终端，但没有任务接口。任务请求随业务 Core 访问云端时，会遇到上述身份拒绝。

## 接口复现

使用真实 `workbenchtasks.NewHandler` 注册 HTTP 路由并调用真实投影逻辑，注入同一条合法任务记录：空间 `space-a`、真实会话 `conv-a`、任务 `task-a`、一个运行中的 Coding Worker。只替换记录读取器及已认证主体上下文，不模拟接口返回值，不写入项目实际业务数据库。会话引用采用客户端现有格式。

| 已认证身份 | 会话参数 | 接口 | 实际结果 |
| --- | --- | --- | --- |
| LocalUI | `conv-a` | 任务列表 | 200，1 条任务 |
| LocalUI | `meshconv1:space-a:conv-a` | 任务列表 | 200，0 条任务 |
| TrustedDevice | `conv-a` | 任务列表 | 403 |
| LocalUI | `conv-a` | 检查点历史 | 200，1 条记录 |
| LocalUI | `meshconv1:space-a:conv-a` | 检查点历史 | 404 |
| TrustedDevice | `conv-a` | 检查点历史 | 403 |

诊断源文件存档：`artifacts/audits/workbench-owned-scope-20261015/repro-main.go.txt`。

完整 HTTP 响应：`artifacts/audits/workbench-owned-scope-20261015/http-repro.json`。

另外对运行中本机服务做了只读请求，但实际查询没有任务数据，因此不使用该空库结果证明“有任务却查不到”。上表来自隔离记录的真实 Handler 复现；云端结论结合身份生成、客户端路由和 Handler 授权分支确认，没有冒称生产云端真机端到端验收。临时 Go 诊断目录已清理，证据存档保留。

## 建议修复边界

1. 将任务列表和检查点查询接入项目统一 Owned 资源上下文，明确区分 UI 会话引用、资源所有者、真实会话 ID 和执行 Core。
2. 由受信路由解析并校验归属，再到拥有该执行记录的 Core 查询；手机端、电脑端使用同一契约。
3. 服务端按主体权限与资源归属授权设备访问，保留空间隔离。不能仅放开所有 TrustedDevice，也不能只在客户端剥掉前缀后无条件查询当前 Core。
4. 补充具有真实任务记录的跨层验证：本机 Owned、云端设备、不同 owner/space 的拒绝访问、列表与检查点一致性、Core 切换后不展示旧会话数据。

此前针对裸会话 ID 和本机 UI 的组件、接口测试无法证明上述 Owned 与云端组合已接通。本轮已确认一项重大缺陷；其余模块没有继续穷举，不能据此给出项目剩余问题总数。
