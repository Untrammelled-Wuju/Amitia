# 模型用途分配接入审计与单轨修复

审计基线：12bf013；日期：2026-10-21，Asia/Shanghai。

## 重大问题

P1：模型用途配置与实际推理脱节。model_scenario_routes 只有配置读写，没有执行读取；Owned 聊天、摘要、自动记忆和候选提取直接读取默认模型。网页提交 routes 对象，而 Handler 要求数组；数据库列 model_config_id 直接通过 map 返回，客户端读取 modelConfigId。空表返回空目录导致分配界面没有可操作项。手机 ModelConfigService 的 routes/updateRoutes 没有页面调用。

同一问题内还发现 Compressor 自行实现 OpenAI Chat 和 Ollama HTTP 请求，忽略主系统协议选择与用途分配；旧标题生成、SQL 写库和系统事件回调已无正式生成入口，但仍保留实现和旧链路测试。

确认此项后停止扩展审计，不能据此估算全项目剩余问题数量。

## 行业常规方案与实施

采用单一配置事实源、统一能力目录、职责明确的模型选择策略、请求级授权、原子持久化及真实边界集成测试。复用原 model_scenario_routes、模型仓库、协议适配器、Owned 执行与所有者提交链路，不建立第二套模型注册或执行系统。

选择顺序：聊天会话显式模型 > 对应用途模型 > Core 默认模型。摘要和记忆计算分别选择自身用途，不被聊天模型选择覆盖。指定的模型不存在时报告错误，不悄悄改用其他模型。单项配置更新保留其他行，清除使用 0；事务内校验用途、重复项、非整数和不存在的模型，整批失败整批回滚；模型删除与关联用途清除在原事务内执行。

用途目录由后端统一输出 label、description、scenario、modelConfigId。当前正式用途为 chat、summary、memory_extract。安全改写、导入解析没有可迁移的模型路由执行实现，删除其误导性前端配置声明，不增加伪实现；安全系统和原文本导入解析仍保留。历史数据库基线与已发布迁移不改写，历史未知配置行不参与执行。

电脑端修复数组契约并使用后端目录；手机在现有文本模型配置页增加用途分配，沿用 CoreConfigurationIntent 与现有 transport。普通云端设备不能读写模型配置，统筹管理员可操作，旧 Core 或旧权限版本的表单不能继续保存。本地、云端统筹开启及关闭继续复用现有 Core 计算／设备所有者模式。

压缩和任务交接摘要删除独立 HTTP 客户端及响应解析，接入已有模型适配器；保留压缩阈值、最近历史、前次摘要合并和原子提交。本地 MNN/llama.cpp 使用原本地运行时，远端协议使用原适配器。旧 Ollama 实现的独立 num_ctx/num_predict 预算迁入公共 Ollama 适配器，Generate 和 Stream 共用。参数含义参考 [Ollama 官方文档](https://docs.ollama.com/modelfile)。

旧 SQL 标题生成、写入、修订、发布器与系统注册回调清理；标题 JSON 约束、小预算、重试与规范化继续被 Owned 标题复用。所有者提交、并发手工改名保护与模式切换拒绝沿用原业务引擎。

## 已执行验证

- 后端定向测试覆盖配置真实 HTTP、目录 JSON 字段、默认回退、显式选择、局部更新、事务回滚、删除清除以及无效模型拒绝。
- TestModelScenarioActualHTTPInference：真实 TCP 模型服务 + Owned 引擎、SQLite 所有者提交，核对实际调用次序 chat、explicit、memory、memory、summary。
- TestModelScenarioRealTLSCoreConfigurationAndModeSwitch：两个独立 Core/设备身份、真实 TLS 配对和签名认证；普通设备被拒，授权管理员能保存，统筹开关两种模式均调用 cloud-assigned 模型，同时分别提交到 Core 和设备所有者；旧权限表单被拒。
- 压缩阈值、历史原子提交、标题 JSON/预算/重试及 Owned 标题所有者、并发改名、模式切换测试通过；Ollama Generate/Stream 的输出与上下文预算测试通过。
- 配置事务撤权、取消和 HTTP 请求上下文定向测试通过。
- 网页配置授权与用途契约 12 项通过；网页 typecheck 通过。
- 手机相关页面测试 4 项通过，含云端管理员分配、普通设备拒绝及切换 Core 后旧编辑表单拒绝。页面定向 analyze 无错误、无警告，保留 7 条既有信息级提示。

以上使用受控模型响应验证协议、路由和数据归属，不代表外部供应商模型质量或手机真机验收。后续核心、Runtime、Release APK、完整服务健康与制品验证结果单独补充；未执行生产服务器发布或 Git 推送。

## 本地构建与运行验收结果

源码基线：f9622bcb8b621a26e66968e9748a1b4883fc4641，提交日志 fix(model): connect scenario routing across clients and core。本次只提交本轮源码、定向测试、更新说明和冻结清单，不包含制品、依赖或运行数据；未推送 Git。

使用同一份源码重新编译 macOS ARM64 核心与 Linux ARM64 后端，重新编译两个运行时宿主，重新生成 Runtime Package。

- macOS 核心 SHA-256：8a80138513bbceebac3f67a4388b0a73da45fd06e87a624212875588fdd18080
- Android 后端 SHA-256：b7b11fbf550b00aadef888f8cd1a99dbc153855765a94a35c417e853cbf84885
- Runtime Package SHA-256：a7011fd2f8ea7dd2976cd38385d996d6fac5133fe3cc83b4e21a975053419529
- APK：artifacts/apk/amitia-26.2.0-beta.11-release-arm64-v8a-20261021-180011.apk，161091839 字节
- APK SHA-256：743af721f2213e1d6bafc0d6a81ffeaee8a07a2dc794bb7204bc02c22b390120
- 版本：26.2.0-beta.11，versionCode 17，未调整版本。
- 正式签名证书与钥匙串保存的 Release 证书一致，apksigner、zipalign、包名、单 ABI、Runtime 源码提交及包内 Runtime/后端/宿主哈希验证全部通过。
- Flutter clean、pub get、Release config-only 成功；Release lint 和 assembleRelease 串行执行且退出码均为 0。单 worker、关闭并行、Kotlin 进程内编译、JVM 自动内存配置且禁用自动堆转储。

本机 Gradle 新制品实际输出在 mobile_app/build/app/outputs/flutter-apk/app-release.apk；mobile_app/android/app/build/outputs/flutter-apk/app-release.apk 是 10 月 20 日旧文件，未用于归档或验收。旧输出没有被手工改写。

完整项目已重新启动，前端、核心 health、模型目录、Qdrant healthz 和 SurrealDB health 均 HTTP 200；/api/health.ready=true。/readyz 为 HTTP 200、blockingCount=0、failedCount=0，但 state=degraded、readyCount=5、degradedCount=1，因此不能宣称全环境完全健康。启动日志还保留 SurrealDB 表已存在告警；尚未确认该告警与这一项降级的因果关系，本轮不扩展到第二项系统整改。

实际扩展包与受信服务列表均为空，没有已安装渠道插件可验证。SDK 完整路径 adb 未检测到真机，未执行安装、debug 包检查或冷启动验证。未构建桌面 NSIS、未发布服务器更新、未推送 Git。

原始验证记录位于 artifacts/audits/model-scenario-20261021/；记录时间 2026-10-21 18:01:49 +0800。
