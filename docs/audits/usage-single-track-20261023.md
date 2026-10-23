# 使用统计单轨修复与验收

记录时间：2026-10-23。范围：本轮审计确认的一项重大问题——模型运行链路、Owner 业务链路与旧表使用统计混用。发现该项后停止扩展审计。

## 原问题与实证

旧 `system/usage_service.go` 从 `messages`、`conversations` 等历史表读取调用与 Token，按模型接口读取的却是 `model_configs` 配置列表；费用固定为零。当前正式聊天写入 Owner 的 `kernel_device_owned_resources`，旧统计无法反映当前聊天。旧 `usage-clear` 直接把聊天消息 Token 更新为零，会破坏原始业务数据。

修复前用隔离数据库复现：Owner 当前消息含 120 Token 时旧统计仍为零；向旧消息表写入 7 Token 后旧统计显示 7；清空操作修改旧消息 Token，而 Owner 消息不受影响。这不是前端显示缓存问题。

## 采用的行业通用方案

复用现有 mindruntime 计量模块、模型协议适配器、设备身份、Owner 数据端口与统筹授权链，扩展持久计量能力，不建立第二套聊天或设备通信系统。

1. 将语言模型调用计量与消息、会话、记忆、反馈等业务记录统计分开。一次逻辑模型调用记录一次，不以聊天消息条数推算调用次数；工具循环、摘要等实际触发模型的调用同样计量。
2. 持久记录调用 ID、Core/设备/Owner/角色/请求、模型、供应商、来源、状态、输入/输出/缓存 Token 和时间。记录不包含提示词、消息正文、API Key。
3. Begin 先落库，完成时幂等更新；模型失败、取消和进程中断分别保留状态。重启将未完成调用标记为 interrupted。数据库不可用不悄悄漏记调用。
4. 业务数据始终在真实 Owner 查询，通过原设备通信通道取得聚合结果；远端 Owner 离线时返回不可用，不改读 Core 的旧聊天表。
5. 普通云端设备只能读取本设备在当前 Core 的模型用量；授权 Core 管理身份才能读取 Core 汇总。重置必须匹配加载时的 Core/设备范围并验证当前权限。
6. 重置仅推进持久序号基线，保留原消息、Token、调用记录。调用归属按开始序号确定，重置前已开始的调用属于旧区间。概览的总量与今日量在同一数据库快照中聚合。
7. 缺失 Usage 与供应商明确返回零区分。存在缺失调用时总 Token 为 null，同时返回已知 Token 与缺失调用数。没有可靠价格时费用为 null，不虚构零费用。
8. 日期统一为 UTC。趋势按连续日历日期排列，最多最近 30 日；手机展示最近 7 日。重置后的日期范围从重置日开始；启用计量前的日期标为 unmetered。

参考：OpenTelemetry 官方 [GenAI Token metrics](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-token-metrics.md) 与 [Client inference](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/client-inference.md)。本次采用其调用操作与 Token 分开计量、使用供应商返回值的原则，不宣称已部署完整 OpenTelemetry 导出平台。

## 复用迁入与旧代码清理

- 复用现有 OpenAI Chat、Responses、Anthropic、Gemini、Ollama 解析及输入/输出/缓存 Token 采集，补充 Usage 是否真实返回的标记。
- 复用 MNN 本地模型结果与流式事件，缺失 Usage 不再被当成真实零。原模型请求、工具、JSON 与流式功能保留。
- 扩展 mindruntime，新表为 `runtime_model_usage`、`runtime_usage_baselines`。已有内存缓存命中指标继续工作，未创建第二套供应商客户端。
- 保留现有 overview、daily/periodic、models、sources、clear API 路径，全部改接真实计量与 Owner 聚合；按天的会话、记忆、反馈功能迁入 Owner 路径。
- 删除旧 `GetUsageOverview/GetUsageDaily/GetUsageModels/GetUsageSources/ClearUsage` 实现及服务接口，删除管理员动作中的 `usage-clear` 与消息 Token 清零分支，无旧统计回退路径。
- 电脑端趋势与高级页、手机端仪表盘与高级页均调整。未知、离线和请求失败明确显示，模型名称列表来自真实调用，重置传原统计范围。
- 原历史聊天表及其他模块仍需使用的业务数据保留；本轮删除的是旧统计代码，不是用户历史数据。

## 模式覆盖

| 模式 | 模型计量 | 业务统计 |
| --- | --- | --- |
| 本地桌面/手机内置 Core | 当前 Core 持久记录实际语言模型调用 | 当前真实 Owner |
| 私有云、设备非统筹 | 当前 Core 按调用设备隔离；普通设备不读其他设备用量 | 远程设备 Owner，经原 TLS/设备通信端口聚合 |
| 统筹 | 同一计量入口、同一权限过滤 | Core Owner，切换模式后不会混入原设备 Owner 数据 |
| Owner 离线/授权失效 | 可独立读取仍有权限的模型计量 | unavailable 或拒绝返回，不伪装零 |

## 定向验证

新增 10 项后端测试函数、3 项电脑端测试和 2 项手机端测试均通过；同时通过改动模块相关现有 Usage/cache 定向用例。未默认执行后端或前端全量测试。

- Store：同一请求中的多次逻辑调用分别计量、幂等结束、分设备隔离、重置保留记录、未知与真实零区分、重启持久化及 running 恢复。
- 真实 HTTP 测试供应商：OpenAI 兼容、Anthropic、Gemini、Ollama；流式重复最终 Usage 不重复累加，缺失 Usage 保持未知，失败与取消记录状态。
- 本地 MNN 解析：缺失、空对象与真实零 Usage 区分。
- System：旧表 Token 不干扰新统计、模型聚合、重置不修改消息、设备隔离、过期范围拒绝、连续日历和历史未计量状态。
- 真实 TLS 配对联测：Android 身份设备与 Core 完成非统筹/统筹模式切换，验证 Owner 的 2 条消息/1 个会话聚合、设备模型用量隔离、日统计、重置保留原消息与调用记录。
- 电脑端：未知值与真实零、Owner 不可用时不伪造汇总、云端设备/Core 范围显示。Vue typecheck 与生产 build 通过。
- 手机端：调用真实统计路径的服务测试、设备范围、未知 Token、空响应与离线错误。Flutter 定向测试通过；分析无 error/warning，有一处高级设置页原有单行 if 风格 info。
- 改动 Go 包 vet 通过；源码差异空白检查通过。

## 本地生产配置验收

使用项目正式启动脚本、真实数据库与已配置的 deepseek-flash，创建独立临时角色，通过正式 Owner 消息接口执行对话。

| 场景 | 实测结果 |
| --- | --- |
| 首轮真实对话 | 2 条业务消息，3 次模型调用；输入 18,057、输出 354，总计 18,411 Token；接口与持久记录逐条一致 |
| 重发同一 requestId | 未新增模型调用 |
| 重置统计起点 | 基线推进至 3，展示调用数归零；原消息正文、旧消息 Token、调用记录均逐条保持一致 |
| 第二轮真实对话 | 新区间 2 次调用；输入 20,364、输出 359，总计 20,723 Token；缓存输入 11,392 Token |
| 完整重启 | 调用数 2、Token 20,723、缓存 11,392、基线 3 和计量开始时间保持不变 |
| 精确清理 | 仅删除临时角色关联的 5 条调用、18 项 Owner 资源、2 项业务请求及相关测试记录，恢复验收前的统计基线；原角色集合和原业务数量一致 |
| 最终日历接口 | 连续 30 个 UTC 日期；今日已计量，过去未计量日期返回 null/unmetered |

最终源码又完成核心重建与完整项目重启，Core `/api/health` ready=true，`/readyz` 为 ready（6 ready、0 degraded、0 blocking、0 failed）；前端、Qdrant、SurrealDB 健康接口均为 HTTP 200。端口由项目原启动配置分配，未改端口配置，未使用 3000。

最终核心 SHA-256：`3c85f198fa8da3a41510f1863c5a7f6b2738496690817f148402655b4bb7dd9b`

最终 Source Gate：`ff01e961bf8616726cd8e1959ef73dec55db9786799d30f6c0c449f9528d56c5`

源码冻结清单由项目脚本重新生成。运行中的核心来自上述源码构建，未手改编译产物。

## 验证边界

本次覆盖现有语言模型调用链，不将独立生图、Embedding、ASR 等计入语言模型数字；接口明确返回 `meteringScope=language-model`。计量口径为模型逻辑调用，不等同供应商账单中的 HTTP 重试次数。

历史调用缺少实际调用级证据，不能从旧聊天消息反推；返回计量开始时间，历史标记未计量。费用尚无可靠价格来源，返回未计价，而非零。

真实模型验收在本地生产配置运行；云端及手机身份通过真实 TLS 配对、临时独立数据库模拟完成，未在用户远程服务器部署。手机端本轮是源码、服务定向测试与分析验证，未重建或安装 APK；桌面端未构建/发布安装包。桌面主窗口可启动，但首次隐私同意界面未代用户确认，未声称完成其后的桌面 UI 人工点选验收。

未修改管理端、版本号或发布配置；未拉取、提交或推送 Git。上一轮关系时间的既有工作区变更保留。本轮只完成这一项重大问题的修复，不代表项目其余模块均已审计完毕。
