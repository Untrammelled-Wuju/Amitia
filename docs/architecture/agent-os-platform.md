# Agent OS 平台

## 目标

Amitia Agent OS 是本地优先、跨设备协同的个人 Agent 操作系统。核心提供 Agent、团队、审批、凭据、网络、运行时、预算、评估、活动、市场和 A2A 通用契约；邮箱、日历、手机号、浏览器 Profile 和钱包 Provider 由独立插件发布。

## 后端边界

| 模块 | 责任 |
| --- | --- |
| `backend/internal/agentplatform` | AgentProfile、版本发布、团队、成员、审批、对账、运行时、策略、预算、评估、市场、A2A |
| `backend/internal/adminrelease` | 市场分类、插件包、版本上传、发布、撤回、下载和签名/SBOM 元数据 |
| `backend/cmd/server/marketplace_installer.go` | 从市场下载并验证插件，再进入 Extension Kernel 预览、确认和安装 Saga |
| `backend/cmd/server/a2a_executor.go` | 将公开 A2A 任务送入现有 UnifiedEntry |
| `plugins-code/agent-identities` | 身份 Provider 插件源码 |
| `plugin/agent-identities` | 身份插件构建产物 |

## 统一 API

业务端统一使用 `/api/agent-platform`：

- `/agents`：生命周期、测试、发布、回滚、导出和导入
- `/agents/:id/export-archive`：导出含版本、团队、授权声明和电脑快照的可携带包
- `/agents/import-archive`：校验并恢复 Agent 配置、版本历史、团队、凭据声明和电脑快照
- `/teams`：团队、团队模板、负责人、人类/Agent 成员、消息路由、共享上下文和任务交接
- `/users`：人类成员、邀请、会话和角色权限
- `/approvals`：持久审批和恢复
- `/reconciliations`：不可确认副作用对账
- `/credentials`：每 Agent 凭据和短期能力令牌
- `/network-policies`：域名、端口、上传和外传策略，支持带 revision 的原子更新
- `/resource-leases`：资源锁和幂等操作
- `/budgets`：Token、费用、工具调用、并发和预算占用
- `/runtimes`：本机、OCI、远程和 Android 统一生命周期；远程实例通过校验后的 HTTP(S) endpoint 注册；`:id/exec` 提供有超时、输出上限和工作目录边界的终端执行
- `/host-control`：桌面屏幕、鼠标、键盘和窗口接管
- `/evaluations`：评估套件、结果、成本和发布门禁；自动评估回填本次真实 `runtime_model_usage` 的 Token、费用和延迟
- `/activity`：面向用户的 Agent 活动、等待原因和成本
- `/market`：分类、搜索、安装和更新
- `/a2a`：Agent Card 和带 Ed25519 签名、时间窗与防重放校验的远程任务

公开 A2A 入口位于 `/api/public/a2a`。

## 市场

扩展中心新增 `/extensions/market`。插件市场按分类、信任级别、平台和关键词筛选，已安装插件会显示为“更新”。

管理端新增 `/marketplace`：

- 创建和管理分类
- 创建和编辑插件信息
- 上传 `.amitiax` 版本
- 发布、替代和撤回版本
- 查看 SHA-256、SHA-512、SBOM、兼容性和下载次数
- 下载已发布或历史版本
- 上传时校验 `.amitiax` ZIP、路径、扩展 ID 和版本
- 发布时复核文件 SHA-256、manifest、SBOM、兼容性声明和官方签名

市场文件保存在管理端发布根目录的 `marketplace/` 下。核心通过 `AMITIA_PLUGIN_MARKET_URL` 同步远程目录；本地开发也会索引 `plugin/agent-identities`。索引官方包前会验证包完整性、Publisher、Key ID 和 Ed25519 签名，验证失败时拒绝写入官方目录。

## 身份插件

身份插件使用 JavaScript 运行时和受控 Host Access：

- `com.amitia/agent-identity-email`
- `com.amitia/agent-identity-calendar`
- `com.amitia/agent-identity-phone`
- `com.amitia/agent-identity-browser-profile`
- `com.amitia/agent-identity-wallet`

插件不保存 Provider 凭据。`CredentialGrant.secretRef` 指向加密 Secret Store，插件通过 `host.secret.get` 临时读取。

## 角色模型

每个角色可独立保存模型、思考开关和思考强度覆盖。角色未配置时继续使用当前会话兼容值或全局默认模型。

- 解析优先级为“本轮显式设置、角色覆盖、旧会话兼容设置、全局默认模型”。
- 角色只保存模型编号和思考参数，不复制或修改 `model_configs` 中的模型本体配置。
- 多角色对话的输入框模型面板会显示角色选择；切换角色后，模型名称、思考开关和强度滑轨显示该角色自己的设置。
- 群聊每个子发言按对应角色的覆盖设置进入 UnifiedEntry，不会把某个角色的调整扩散给其他成员。

## 信任边界

- 本机 Runtime 直接使用宿主进程和独立工作目录，不提供操作系统沙箱；需要隔离时必须选择 OCI/isolated。
- OCI Runtime 只挂载自己的 workspace，默认关闭网络，CPU、内存、PID 和磁盘配额随实例配置。
- 桌面接管必须创建并批准短期会话。
- 桌面预览持续刷新，点击坐标映射到宿主屏幕并支持键盘输入，关闭会话立即撤销。
- 外部副作用必须审批；结果不确定时进入待对账。
- 已发布 Agent 的模型、权限、工具/技能白名单、团队预算和工作区会进入真实对话执行链。
- 模型完成回调按稳定 Usage ID 幂等写入空间、Agent 和团队预算，并生成带操作者归属的活动事件。
- 自动评估按本次执行时间窗汇总真实模型用量，避免 UI 评估结果停留在 0 Token。
- Agent 电脑终端对本地和 OCI Runtime 使用统一执行契约，OCI 工作目录被限制在 `/workspace`，输出上限为 16 MiB。
- 自动任务受自治等级、每日动作数、团队预算和外部动作审批约束。
- 钱包转账默认关闭，真实操作要求独立审批令牌。
- Cloud Core 不接受未签名的本地插件包。
- 官方插件发布者公钥作为内核信任根内置，官方包必须匹配 `com.amitia` Publisher 和固定 Key ID。
- 所有失败保留原始错误并返回上游调用方。

## 源码基线

- Git HEAD：`c7b64c2e`
- 分支：`develop`
- 基线时间：`2026-11-03`
- 构建产物、依赖、运行数据和插件产物不上传 Git。
