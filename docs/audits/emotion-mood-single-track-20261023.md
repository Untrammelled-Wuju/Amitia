# 情绪与心情单轨修复及生产验收

记录时间：2026-10-23。范围：本轮审计确认的一项重大问题——当前心理状态、心情历史和多套派生情绪状态混用。发现该项后停止扩展审计。

## 原问题与实证

项目同时存在以下事实源和入口：

- Owner `psyche` 保存当前 Valence、Arousal、Dominance、心境、压力、疲劳和需求等正式运行状态。
- 旧 `/api/moods` 从 `messages.mood` 聚合并提供按会话查询、单条删除和按会话删除。
- `moods`、`emotion_states`、`affect_states` 保存多套历史或派生状态。
- 手机端保留 `MoodService`、`MoodDto`、Provider 和旧接口回退。
- 聊天后处理原先存在旧心情恢复链路，新 Owner 链路没有正式情绪历史，导致后端已有当前状态但双端只能看到摘要，历史也没有统一归属。

旧删除接口尤其暴露了契约缺口：如果只删除新接口而不迁移删除能力，用户将无法清理心情历史。

## 采用的行业通用方案

复用现有 Owner 运行状态、Device Mesh 数据端口、角色权限、统筹授权和可撤销管理事务，不新建第二套心理状态系统。

1. `psyche` 仍是当前状态的唯一事实源。
2. 新增 Owner `emotion` 资源保存每轮有界历史快照，快照记录角色、会话、消息、请求、来源、阈值、置信度、PAD、压力、疲劳、心境、标签和恢复信息。
3. 聊天后处理读取正式 `mood_detection_enabled` 与 `mood_detection_threshold`，只有启用且达到阈值才写历史。
4. 历史写入与聊天后处理使用相同请求和 Owner 提交边界，重复请求幂等。
5. 历史删除使用 Owner 乐观并发编辑，必须匹配页面加载时的 Core、Owner、模式版本和资源 revision；删除写墓碑，不允许复活或篡改内容。
6. 迁移采用 Expand-Migrate-Contract：旧表只作为一次性幂等迁移来源，不参与正式读写。迁移完成后可删除旧运行代码，但保留用户旧记录作为兼容备份。
7. 本地、非统筹设备、统筹 Core 和远端 Owner 离线均使用同一资源契约，不提供旧表或本机 `psyche` 假回退。

参考通用实践：SQLite/业务数据使用唯一所有者、乐观并发和软删除墓碑；OAuth/设备数据采用 owner-scoped resource 与 idempotent request；状态展示与不可变历史事件分离，避免用派生表反推当前状态。

## 功能等价迁移

| 旧能力 | 新正式入口 |
| --- | --- |
| `/api/moods` 聚合列表 | `/api/device-mesh/v1/business/data?kind=emotion&characterId=...` |
| 按会话查询心情 | Owner `emotion` 查询支持 `conversationId` 过滤 |
| 删除单条心情 | `/api/device-mesh/v1/business/resources/edit`，`kind=emotion`、`deleted=true`，校验 revision 与 executionScope |
| 按会话/角色删除 | 双端基于同一 Owner 列表逐条执行带 revision 的墓碑删除，并提供“清空”操作 |
| 旧心情恢复选项 | 新聊天历史快照的 `recoveryApplied`、`recoveredMood`、`recoveredLevel` |
| 检测开关与阈值 | `mood_detection_enabled`、`mood_detection_threshold`，双端真实读写 |

电脑端和手机端均已接入真实 Owner 历史，具备单条删除和清空；手机端不再保留 `MoodService` 或回退接口。

## 旧代码清理

已删除：

- 后端 `internal/mood` 路由、Handler、Service。
- 后端 `internal/affect`、`internal/emotionstate` 运行包。
- 聊天旧 `mood_recovery.go` 与 `owned_mood_recovery.go`。
- 旧 `/api/moods` 注册及新聊天写 `moods` 的逻辑。
- 手机端 `MoodService`、`MoodDto`、Provider 注册和旧 `/api/moods` 调用。

仅保留：

- 启动迁移器对旧 `moods`、可选 `messages.mood`、`emotion_states`、可选 `affect_states` 的只读复制。
- 旧数据库表作为迁移来源和兼容备份，正式运行不再读取或写入。
- 旧 `/api/moods` 返回 404。

## 模式覆盖

| 模式 | 当前状态 | 历史与删除 |
| --- | --- | --- |
| 本地桌面/手机内置 Core | 当前角色 Owner `psyche` | 当前 Owner，可删除并持久墓碑 |
| 私有云、设备非统筹 | 设备 Owner `psyche` | 设备 Owner，经原设备通信端口管理 |
| 统筹 Core | Core Owner `psyche` | Core Owner，统筹权限变化后旧页面版本被拒绝 |
| Owner 离线/授权失效 | 明确不可用 | 不读 Core 旧表，不伪造成功 |

## 定向验证

- 当前状态阈值与恢复计算通过。
- Owner 情绪写入、重复写入幂等、统筹 Core Owner 通过。
- 旧 `moods`、`messages.mood`、`emotion_states` 幂等迁移通过。
- 删除单条、revision 校验、禁止内容篡改、墓碑阻止迁移复活通过。
- 后端相关包 `go vet`、定向测试和服务器编译通过。
- 电脑端 typecheck 与生产构建通过。
- 手机端定向静态分析无问题，3 项 Owner 读取与删除测试通过。
- 云设备执行门禁通过。

## 真实生产模拟

使用正式 Electron、Core、Qdrant、SurrealDB 和已配置模型完成：

1. 开启历史记录、阈值 0.2，发送真实模型消息，实际生成 Owner 快照；快照置信度 `0.4995`、`affectLabel=平静`。
2. 阈值 1.0 和关闭开关时均不新增快照。
3. 旧 `moods`、`emotion_states` 没有新写入。
4. 插入旧 `mood` 与旧 `emotion_state`，重启后 Owner 查询实际得到来源为 `owned`、`legacy-mood`、`legacy-emotion` 的记录。
5. 设置 `enabled=true`、阈值 `0.2` 后完整重启，API 仍返回相同值，证明设置持久化；完成后恢复为 `enabled=false`、阈值 `0.5`。
6. 插入一条旧 `moods` 夹具并重启迁移到 Core Owner；通过正式资源编辑接口删除，返回 revision `1 -> 2`。
7. 删除后 Owner 查询为 0；再次完整重启后仍为 0，证明旧表不会把已删除历史重新迁入。
8. 按精确角色、旧记录 ID、Owner 资源和请求 ID 清理夹具；复核旧 mood、Owner 墓碑、编辑 checkpoint、inbox 和 business request 均为 0。

最终运行状态：

- 前端：`http://127.0.0.1:55044`
- Core：`http://127.0.0.1:55050`
- Qdrant：`55047/55048`
- SurrealDB：`55049`
- 旧 `/api/moods`：HTTP 404
- `/api/health`：HTTP 200，ready=true，模型已配置
- `/readyz`：ready，6 ready、0 degraded、0 blocking、0 failed
- Core SHA-256：`6fd6c0f46b1db4e6d5ac96f654159359583938f0bb40754d7a3bf3ed052e60b6`
- Source Gate：`2d16cabe85318781d373581bc6e33ccf7860f466f74eda20be60b970a30051ba`
- 冻结清单：11080 项

## 验证边界

本轮未构建或安装新的 Android APK；手机端完成源码接入、定向测试和静态分析。桌面端运行真实开发生产配置和重建 Core，未制作或发布安装包。

旧表仍保留用户历史数据，未执行不可逆删表。正式代码已切到单轨，旧表只用于兼容迁移。未修改管理端、版本号或发布配置；未拉取 Git，未提交或推送 Git。
