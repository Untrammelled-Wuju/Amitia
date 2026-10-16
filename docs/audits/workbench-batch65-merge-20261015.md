# Workbench Batch65 合并与检查

日期：2026-10-15，Asia/Shanghai。范围为源码合并、必要的集成修复和定向验证，不包含安装包构建、真机安装、Git 提交或推送。

## 输入与备份

- 输入包：`E:/下载/U-Ai_Workbench_Batch65_%E7%B4%AF%E8%AE%A1%E5%AE%8C%E6%95%B4%E6%9B%BF%E6%8D%A2%E6%96%87%E4%BB%B6%E5%8C%85.zip`。
- SHA-256：`4affbf32bd5abee9328f4e1775281cb2e70f38e1a22c196f42b335425f175bcf`。
- 包含 355 个文件：325 个新增、30 个已有文件的替换版本。
- 合并基线：`22ed4860756c45fa9f0f1d9618bc8ae77cd7e0b4`，未拉取远端。
- 合并前已有文件逐字节备份：`artifacts/merges/batch65-20261015-171657/local-before.zip`。
- 路径、输入／原文件／合并文件哈希和操作清单：同目录 `manifest.json`。

逐项读取验证 ZIP CRC，检查路径越界、重复路径、符号链接、编译产物及明显私钥／令牌特征，未发现这些问题。17 个工作台 JSON 契约均可解析。包内实施记录作为历史材料保留，其完成率声明不作为本地验收证据。

## 合并处理

325 个新增文件导入对应源码、测试、契约、脚本与文档目录。26 个无本地改动的已有文件采用包内版本。以下 4 个文件基于 HEAD、本地工作区、包内版本做三方合并，未产生文本冲突：

- `backend/cmd/server/router.go`
- `backend/cmd/server/services.go`
- `front/src/composables/useConversationRuntime.ts`
- `front/src/views/web-chat/BuiltinWebChatView.vue`

保留本地统一主动任务服务、备份装配和新 HTTP 契约；没有恢复旧提醒 Repository、旧服务装配或已退役的主动消息监听。保留通知跳转时角色、Owner、Core 的归属检查。此前两端主动任务更名及电脑端界面重构保持原有成果。

工作台新增内容覆盖聊天中的资源面板、任务投影、终端、浏览器观察、Coding Worker 工作树分配／检查点／审核／合并流程及两端相关视图，不额外创建另一套主动任务系统。

## 实际发现并修复

| 位置 | 问题与处理 |
|---|---|
| 前端工作台事件 | 可选事件 payload 被直接访问，导致真实工程类型错误；改用安全可选访问。 |
| 前端终端测试 | 包内测试仍假设旧的二参数请求；增强为验证真实 origin、超时和 AbortSignal 契约。 |
| 手机工作台 | 两处相对 import 层级错误、Wrap 不存在的 mainAxisSize、Axis.both 不存在、可空 API 表达式类型推断错误；修复并去除无用导入。 |
| 手机终端输出 | ANSI CSI 起始方括号被当作结束符，导致控制序列解析错误；修复并增加跨分片 CSI／OSC 回归。 |
| 浏览器授权 | 独立浏览器操作仅校验 Space，没有统一要求控制权限；补齐 TrustedDevice 或可信 LocalUI、有效设备身份、专用控制权限，保留原有范围及控制租约检查。纯能力查询保留。 |
| Windows 终端凭据 | 使用 POSIX mode 判断导致 Windows 默认权限值被误判，任务运行凭据无法启用；采用平台权限语义，Unix 严格检查保留。 |
| Windows Git 工作树锁 | 三个锁目录沿用 POSIX 权限判断，导致合法目录被拒绝；复用已有平台权限判定。 |
| Windows 短路径 | 系统 TEMP 的 8.3 路径与规范化长路径比较不等，导致合法数据根目录被拒绝；规范化可信数据根目录，保留受管子目录的反符号链接检查。 |
| Windows Go 验证环境 | 安全环境白名单遗漏 LOCALAPPDATA，Go 子进程无法定位构建缓存，不能产生真实验证证据；保留该必要系统位置，继续隔离宿主密钥及 Go／Git 注入配置。 |
| Windows 工作树删除 | 从待删除的 linked worktree 自身作为 Git 工作目录会占用该目录；从 Git 列出的同仓库其他存活工作树执行非 force 删除，脏文件和绑定校验保留。 |
| Git 测试夹具 | 测试 helper 继承宿主 core.autocrlf，与产品隔离全局 Git 配置的行为不一致，导致夹具制造脏状态；让夹具使用相同安全环境，不修改产品的脏状态拒绝或用户全局 Git 配置。 |
| 终端关闭 | 只终止 shell 可能遗留子进程；Windows 使用有超时、隐藏窗口的系统进程树终止，Unix 补进程组终止。 |

没有通过放松权限、跳过真实逻辑或弱化断言隐藏失败；没有引入新依赖或数据库结构变更。

## 验证结果

- 前端源码 Vue 类型检查通过；工作台／终端 47 项、关联事件归约／通知／提醒契约 19 项、远程浏览器错误处理 3 项、Owned 任务与原主动任务界面 57 项，共 126 项不重复测试通过。首轮旧终端票据断言失败已修正并通过复测，不将复测重复计入总数。
- 手机相关 30 项测试通过；定向 Dart analyze 退出码 0，无错误和警告，保留 44 条既有格式／弃用 info。
- `go build ./cmd/server` 通过，使用项目指定的 Go 1.26.1。
- `internal/interaction`、`internal/devicemesh/agent`、`internal/browser`、`internal/agent`、`internal/workspace`、`internal/workspace/git` 相关包测试通过。
- Kernel 运行工具外观、浏览器提供者、任务 UI 范围与 SSE UI Host 的定向回归通过。
- 服务器 Coding Worker、远程浏览器错误状态及原主动任务的真实 TLS／断连／Owner 与 Provider 变更测试通过。
- 工作台 browser、terminal、http、tasks 包定向测试通过，包括越权身份、Windows 任务凭据和真实父子进程回收。
- Git 工作树全包先暴露 Windows 平台问题，执行 463.861 秒后有失败；修复后失败组执行 384.012 秒，仅余 4 个受全局 autocrlf 污染的夹具失败。这 4 项在隔离全局 Git 配置后全部定向复测通过（63.368 秒）；其余失败项、删除／脏文件保护／平台路径代表性回归均已通过。最后没有重跑完整 Git 工作树包，因此不表述为最终全包通过。Windows 缺少符号链接权限及 POSIX 专属用例按条件跳过。
- 合并脚本的认证烟测／发布证据门禁单元测试 11 项通过，不代表发布门禁本身已完成。
- 相关源码 `git diff --check` 通过。

首轮局域网 TLS 测试受到当前 HTTP_PROXY／HTTPS_PROXY 环境影响；仅在测试进程向 NO_PROXY 追加私有网段后，相关用例和 DeviceMesh 全包通过，没有修改产品代理或证书校验。首次 interaction 编译遭遇 Windows 临时编译器文件占用，重试通过。尝试运行较大的 Kernel 包级套件在既有 ListOperationsPagination 阶段达到超时；改为本次修改直接涉及的定向验证并通过，不将该套件记录为全量通过。

## 运行验证与边界

最终核心为本次合并及平台修复源码重新编译的 `backend/server.batch65-final.exe`，SHA-256：`6c827271ce99f9150aaa8eb4f1373145782ee8427c0e74ff6a35b0e0ab4c9bc1`。

原 `backend/server.codex2.exe` 在停止项目进程后仍被 Windows 文件占用。遵循用户此前不结束占用进程、继续源码与测试的选择，本次通过原完整启动流程在调用时指定新核心路径；未修改启动脚本、项目端口或原核心文件。恢复常规启动脚本之前仍须解除原文件占用并同步新核心，否则脚本会优先旧核心。

认证 HTTP 检查：工作台浏览器能力、工作台任务列表、终端会话列表、原主动任务状态均返回 HTTP 200；主动任务 schedulerRunning=true。浏览器能力如实返回 available=false，当前本机未启用 Browser Runtime，不能据此声称已完成真实 Chromium 操作验收。读取接口未创建真实任务、未启动用户终端、未发送消息。

最终 18899 监听进程已核实为 PID 43588、`backend/server.batch65-final.exe`。较早一轮启动检查因管理前端 15179 尚未就绪返回 1；最后一次完整启动返回 0，独立复核 18899、15178、15179、18000、19178、18998 六个健康入口全部 HTTP 200。本轮包内非测试 Go 源码没有晚于最终编译结果的改动。

Windows 子进程测试在本机执行通过；Unix 终止逻辑没有在本机 Windows 环境执行真进程验收。生产云、多设备、真实 Chromium、手机真机与安装包发布不属于本轮已通过项。
