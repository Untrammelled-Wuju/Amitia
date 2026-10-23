# 隐私扫描与脱敏统一归属入口修复

记录日期：2026-10-23（Asia/Shanghai）。本次范围为隐私扫描、手动脱敏、扫描历史及两端对应接入，不改变管理端发布系统。

## 问题与方案

原聊天与导入已经写入 kernel_device_owned_resources，隐私服务仍查询和修改旧 messages/conversations，导致当前数据漏扫。原查询还忽略数据库错误、最多扫描 1000 条。

采用现有 Owner 聊天管理通道承载隐私操作，复用历史数据适配器、敏感模式匹配、分级和报告功能。消息由实际持有方读取、按原版本修改，Core 负责处理扫描结果；不建立第二套业务存储或远程同步副本。

授权依据为当前 Space、发起设备、目标设备、Core 和权限/模式版本。扫描历史与回执按此范围隔离；消息回执包括真实 Owner、记录角色、历史来源、版本、原文指纹与执行范围。脱敏提交重新校验来源与版本。设备端只放行正文替换为固定脱敏标记、清空推理正文、设置脱敏状态和更新时间的操作，其他字段必须与原记录一致。

参考：[OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)、[RFC 9110 条件请求](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.1)。

## 功能迁移与清理

| 原功能 | 修复后路径 |
| --- | --- |
| messages / import_items 范围筛选 | Owner 统一读取，新旧记录按消息和会话 source 分类 |
| 旧消息与旧导入 | 现有聊天历史适配器，按 Space、Owner、角色读取 |
| 当前聊天与新导入 | kernel_device_owned_resources 的 message 资源 |
| 停用角色历史 | 从实际消息记录发现角色，不依赖仅启用角色列表 |
| 风险模式、等级、摘要、筛选、导出 | 复用现有规则与客户端报告能力 |
| 手动确认脱敏 | 保留确认文字，新增版本和指纹检查及持有方确认 |
| 记忆脱敏与派生索引处理 | 保留统一 MemoryConsumer 路径 |
| 扫描历史 | 保留最近 50 次，并隔离调用者与授权范围；清除历史同时清除回执 |
| 结果缓存 | 只保留历史结果引用的回执，过期回执要求重扫 |

已删除 system 隐私服务的旧消息 SQL 扫描函数、handler 中直接更新旧 messages 的脱敏分支、无上下文的 PrivacyScan/PrivacyMask 服务入口及无效 ORM 字段映射。旧表数据兼容读写只保留在现有 Owner 历史适配层，并使用版本条件更新。

本地模式扫描本机持有数据；云端代算模式通过现有 TLS/设备数据通道扫描并写回设备；协同模式覆盖云端数据和原设备历史数据。历史记录的脱敏在原设备执行，Core 不保存其业务副本。

扫描每页最多 64 条，分页使用整体记录指纹校验。每来源最多 256 页，总读取最多 32768 条；超过上限、分页间数据改变、无可用角色建立授权范围、设备离线或查询失败均返回错误，不返回完整扫描成功。扫描结果 complete=true 才可在客户端显示完成。脱敏必须确认数量与去重后的请求数量一致。跨存储批次逐条执行，已成功项立即更新历史标记，后续冲突会返回失败并要求重扫，不承诺跨存储事务回滚。

个人扫描/脱敏及其历史使用当前设备数据权限；生命周期删除接口维持原有权限。手机页面将删除统计与扫描结果独立加载，删除统计不可用时显示未知，不再阻断扫描结果。电脑端不可用统计同样显示未知。

## 验证

只执行本次涉及模块的定向验证，无全量测试。

- cmd/server 的 3 项新增测试通过：真实 TLS/真实 SQLite 的手机与 Core 多模式扫描及脱敏，旧消息与新导入，其他设备隔离，模式切换及旧版本回执拒绝，存储失败返回错误；1101 条记录分页、停用角色、扫描中途变更拒绝；本地独立核心持久化脱敏。
- coordination 新增授权测试的 5 个场景通过：固定脱敏允许；任意正文、会话字段、消息角色改写和版本过期拒绝。已有 SourceCommit 定向回归通过。
- 配置操作过期范围与失败回滚、统一记忆隐私消费定向回归通过。
- 电脑端新增 2 项测试通过，vue-tsc --noEmit 通过。
- 手机端新增 2 项测试通过。3 个涉及文件静态检查无错误，有页面原有的 1 条多余导入和 2 条弃用属性提示。
- go vet ./cmd/server ./internal/system ./internal/devicemesh/coordination ./internal/devicemesh/business 通过。
- 源码冻结清单已更新，核心通过项目 build-amitiacore.mjs 从当前源码重新编译。

## 本地完整服务验证

使用项目内 AmitiaData/node/bin/node 和项目原有 start-all-services.sh 启动完整项目，未修改端口配置。开发服务依照既有脚本动态分配端口：本次前端 63002、核心实际端点 63008、兼容入口 18899、Qdrant 63005/63006、SurrealDB 63007，未使用 3000。

前端、/api/health、/livez、/readyz、Qdrant /healthz 和 SurrealDB /health 均 HTTP 200；health.ready=true；readyz 为 ready，6 ready、0 degraded、0 blocking、0 failed。

使用本地正式凭据实际调用 /api/privacy/scan：消息与导入范围扫描 2 条，默认消息/记忆/导入范围扫描 8 条，均 code=200、status=completed、complete=true。仅输出统计，不记录敏感正文或凭据。脱敏写入只在隔离测试库执行，未修改用户真实内容。

本轮未构建 Android Release/Windows 安装包、未装机、未发布、未提交或推送 Git。TLS 多核心联测为本地生产协议模拟，不等同于线上部署或手机真机验收。
