# BDI 安全策略接入决策链（本地构建记录）

日期：2026-10-26

## 背景

安全设置的 BDI 配置此前只是可读写的数据：硬约束、软偏好、应对策略、情绪表达上限都保存到运行时策略里，但决策链没有读取，属于“后端做了功能、真实链路没接入”。本次把 BDI 接到既有候选生成—评分—仲裁—表达计划链路上，没有新建第二套决策系统。

## 接入方式

硬约束合并进既有的 `HardConstraintFilter`，`block` 直接过滤候选，`override` 不删除候选而是做确定性提分置顶，并在候选原因里记录 `safety_bdi/override`。软偏好写入既有 `CandidateScoringContext.UserPreferences`，复用候选与标签两个维度的偏好加权。应对策略进入既有 `ExpressionPlanInput`，由 `decision.SelectCopingStrategy` 与心理状态、关系阶段共同决定。情绪显示模式与强度上限在表达计划生成后应用：显示强度成为上限，`suppress` 直接抑制表达，`reframe` 写入元数据并降到低强度；负面情绪上限作用于 `concerned` 语气，亲密表达上限作用于 `warm`/`soft` 语气，上限为 0 时抑制表达。硬约束原因同时进入 `PlanContentPolicy.ForbiddenTopics`，随提示词的表达约束段落渲染。

历史遗留的应对策略值在读取、保存和两端界面统一归一：`active→problem_solving`、`support→seek_support`、`reframe→reappraisal`、`avoid→avoidance`、`suppress→suppression`、`accept→acceptance`。

保存前校验候选 ID、偏好维度、严重程度、应对策略、表达模式和强度范围，非法值返回 400 并给出具体字段。新增 `GET /api/safety/bdi-options` 提供候选、维度、策略、模式与取值区间，桌面端与手机端不再硬编码枚举。

## 敏感记忆主动提及过滤

复用既有记忆字段 `allowProactiveMention`，未设置视为允许。仅当本轮是主动任务且 `preventSensitiveProactiveMention` 开启时，把标记为禁止主动提及的记忆从注入资源中剔除；普通对话不受影响。桌面端设备记忆面板与手机端记忆页都增加了切换入口和状态标签。

## 界面

桌面端安全设置把硬约束与软偏好从只读表格改为可编辑表格（候选/维度下拉来自接口、可增删、可单独保存），并显示生效摘要与软偏好贡献值。手机端安全设置不再弹出裸 JSON 编辑框，改为结构化编辑器，覆盖应对策略、情绪表达、硬约束、软偏好四块，保存时携带 `expectedRevision`。

## 验证

后端 `go build ./cmd/server` 通过，`internal/safety`、`internal/decision`、`internal/interaction`、`internal/chat`、`internal/devicemesh/business`、`internal/devicemesh/coordination` 定向测试通过。前端 `vue-tsc --noEmit` 与 `vite build` 通过。手机端改动文件 `flutter analyze` 无 error/warning。

接口实测：`GET /api/safety/bdi-options` 返回候选、维度、策略、模式与区间；保存带 `severity=override` 的硬约束与旧值 `active/support` 后回读为 `problem_solving/seek_support`；未知候选返回 400「硬约束指向未知行为候选」。测试配置已还原为空配置。

完整项目已用本次源码重建本地核心（Source Gate `59b84261d7b94a33d226be60062c000ad0eed9a6e2dbd9e9d7ed9e00bd6d110c`，Core SHA-256 `0f89c0236ce6faf5f25f0854d623647240fac6604cc09b409b50a66a2a0df2d2`）并重启，`/api/health` 返回 `ready=true`、`version=26.2.0-beta.17`。启动后 `/readyz` 为 ready（6 ready、0 degraded、0 blocking、0 failed）；随后复检为 degraded（5 ready、1 degraded），降级项是既有的 `data_lifecycle` 诊断「旧删除协调器已退役」，与本次 BDI 接入无关，未在本轮改动范围内处理。

## Android 本地构建

Runtime Package 由同一份后端源码重新编译 Linux ARM64 后端并重建插件宿主、任务宿主，`sourceCommit` 为 `38b7a13bf6e6d2b2d0977f4889225c2709ae7bcc`，SHA-256 `e4546c6665e5a65245fa67be8a101893a4fc43a41ac30179dec3706d4d395703`，严格校验通过。

APK `artifacts/android-bdi-20261026/amitia-26.2.0-beta.17-release-arm64-v8a-20261026-203525.apk`，136406991 字节，SHA-256 `cdb3790fa2775a8166bdfaad98e32fba18a0a2653a23b0462c49dbc72565ca4f`。versionCode 23、versionName 26.2.0-beta.17，正式证书 SHA-256 `119d8ebe45b44a3e055dac7e71f31d8b9bec128abb2f59bb08635177e06d680b`，apksigner v2、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 哈希与构建输入一致，Release lint 与无障碍 lint 通过。

当前 macOS 主机未连接任何 Android 设备（adb 与 mDNS 均无设备），本轮未安装、未冷启动、未做真机界面验证，不能记录为真机验证通过。
