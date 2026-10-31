# 知识检索单轨与双端能力对齐

日期：2026-10-31。范围：知识检索有效路由、电脑端与手机端能力对齐、本地与统筹/远端路由契约、真实 Core 模拟验证。

## 结论

知识检索原先同时存在新旧策略判断和双端接入不一致：

- 聊天检索在 `chat` 包内维护意图识别和 ACTIVE、LEGACY、SHADOW 路由。
- 知识搜索接口接收客户端指定级别，但没有返回可视化的有效路由契约。
- 电脑端显示运行时策略、索引健康、向量审计、孤儿检查、恢复审查、Trace、SLO、Benchmark 和 Canonical Outline。
- 手机端只接入知识源、任务、搜索、原文读取、对账和 GC，无法独立完成同等诊断与治理。

本次按“服务端是有效路由唯一权威、客户端消费同一事实契约、手机端与电脑端能力对等”的行业标准修复。

## 行业标准设计

1. 意图识别、策略开关、Feature Gate 和降级原因全部在知识域计算一次。
2. 聊天链路与客户端检索验证共用同一个路由解析器。
3. 搜索响应返回请求级别、实际级别、路由模式、策略模式、影子级别、降级状态和降级原因。
4. 自动检索明确使用聊天路由；手动指定级别明确标记为诊断路由。
5. 手机端和电脑端使用同一后端接口、同一错误语义、同一权限模型，不建立移动端降级系统。
6. 本地 Core、统筹 Core 和远端 Owner 继续通过现有 `BackendServiceApi` 路由层选择当前业务权威。
7. 旧策略保留为受控 Feature Flag 兼容模式，但不再拥有第二套独立判断代码。

## 后端修改

- 新增统一知识路由解析器，集中处理文档意图、ACTIVE/LEGACY、ACTIVE_SHADOW/LEGACY_SHADOW 和 Feature Gate 降级。
- 删除聊天包内独立的 `knowledgeRoute`、`chooseKnowledgeRoute` 和 locator 正则实现。
- 新增 `routingMode` 请求字段：
  - `chat`：按聊天实际策略自动决定检索级别。
  - `diagnostic`：使用客户端指定级别，但仍由服务端执行 Feature Gate。
- Search 响应新增：
  - `requestedRetrievalLevel`
  - `retrievalLevel`
  - `routingMode`
  - `policyMode`
  - `routeReason`
  - `shadowLevel`
  - `shadowReason`
  - `routeDegraded`
  - `routeDegradationReason`
- 检索 Trace 使用同一有效策略事实记录 ACTIVE、LEGACY、SHADOW 和诊断路由。

## 电脑端修改

- 自动级别请求发送 `routingMode=chat`，手动指定级别发送 `routingMode=diagnostic`。
- 搜索结果展示路由模式、策略模式、请求级别与实际级别、影子策略和路由降级原因。

## 手机端修改

- 自动检索使用聊天路由，手动级别使用诊断路由。
- 新增完整知识诊断页，接入：
  - Runtime 策略
  - 索引 Outbox 健康
  - 向量抽样核验
  - 反向孤儿检查
  - 分页巡检与巡检历史
  - 页回执、二次核验和复核历史
  - 人工恢复审查、计划检查和操作轨迹
  - 检索 Trace
  - SLO
  - Benchmark 历史
- 知识源详情补齐 Canonical Outline 分页和受限原文读取。
- 恢复审查使用 Core 管理员权限版本头，和电脑端保持同一权限边界。
- 手机端高级系统页补齐设备身份校验入口。
- 所有知识接口继续使用现有 `BackendServiceApi`，不在客户端复制本地或云端判断。

## 已确认的其他手机端同类问题

当前源码中以下历史问题已经完成同源修复，本次未重复创建第二套实现：

- 聊天导入已统一走 Owner 提升链路。
- 消息反馈已在聊天气泡提交、查询和删除，并在仪表盘读取统一统计。
- 日程提醒的手机端已接入清理策略、未来计划、队列摘要、执行历史和背压清理。
- 桌宠安装、启停、默认动作、位置重置、切换、升级、降级、修复和卸载均跟踪真实 operation。

## 验证

后端：

- `go vet ./...`
- `go build ./cmd/server`
- `go test ./internal/knowledge ./internal/chat`
- `go test ./cmd/server -run 'Knowledge|Search|Health' -count=1`

电脑端：

- `vue-tsc --noEmit`
- 知识路由单元测试 2 项通过
- Vite 生产构建通过

手机端：

- 知识、知识诊断、高级系统、服务和路由契约定向 `flutter analyze` 无问题。
- `flutter test`/`dart test` 受本地缺少 `rustup` 阻塞，失败发生在 `syntax_highlighter_plus` 的 Rust native asset 构建，未进入测试用例。

真实 Core：

- 使用当前源码重建 `AmitiaCore`，SHA-256 为 `748ee9e689cf2ab925b8643da9a369ea3d3156edfe40a723a7b35b78d0a6342c`。
- Core 版本 `26.2.0-beta.19`，`health.ready=true`，最终 `readyz` 为 `ready`，6 项 ready、0 degraded、0 blocking、0 failed。
- 普通聊天请求实际路由为 `NONE / active / chat_no_document_intent`。
- 文档原文请求实际路由为 `AGENTIC / active / chat_document_intent`。
- 手动 AGENTIC 请求实际路由为 `AGENTIC / diagnostic / explicit_level`。
- Runtime、索引健康、SLO、Trace、Benchmark、来源列表、向量抽样、孤儿检查和身份校验接口均返回 HTTP 200。
- 临时知识源完成上传、`ready_degraded` 入库、10 项 Outline、聊天路由 FAST 检索和 8 条证据命中。
- 临时知识源已删除，来源列表恢复为空，再次读取返回“知识库资源不存在”。
- 当前环境没有已配置的向量嵌入服务，最终空库下向量抽样和反向孤儿检查均明确返回 `empty`，不能视为全库向量一致性通过证明。

## 边界

- 未修改管理端、版本号和端口。
- 未拉取、提交或推送 Git。
- 未构建或安装 Android Release APK。
- 统筹/远端模式通过现有统一传输层和路由契约覆盖；本轮真实运行环境为本地 Core，没有第二台实体设备参与。
