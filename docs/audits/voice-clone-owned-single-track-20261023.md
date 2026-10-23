# 复刻音色 Owner 单轨修复与生产模拟

日期：2026-10-23。

## 原问题

语音复刻同时存在两套事实源和三套调用路径：

- 旧 `/api/tts/synthesize` 根据 `characterId`、`voiceId`、`speakerId` 或活动配置隐式选择音色，不要求当前 Owner 的页面权限版本。
- 旧 `/api/tts/play/:messageId` 按消息 ID 播放，未要求页面权限版本。
- 新 `/api/device-mesh/v1/business/speech` 有 Owner、角色、权限版本和确认保存链路，但只能使用角色已保存的音色。
- 电脑端和手机端的复刻音色管理页可以创建、列出和删除音色，但试听没有把明确的 `speakerId` 送入新 Owner 语音链。
- 手机云端模式在多角色时隐式使用第一个角色，界面显示试听成功，实际授权角色可能与用户理解不一致。

这属于新系统没有包含旧系统必要能力、客户端虚假接入和新旧路径混用。

## 行业标准方案

采用单轨契约、明确能力选择、强 Owner 授权、Expand-Migrate-Contract 和精确数据清理：

1. `kernel_device_owned_resources` 中的 Owner 语音回执和音频继续作为唯一持久化事实源。
2. 新语音接口增加可选 `speakerId`。未指定时沿用角色保存音色；显式指定时只使用该复刻音色，不静默回退。
3. 显式复刻音色必须同时满足当前 Space、活动 TTS 配置、Provider 能力、角色执行范围、角色 revision、ACL 和 Owner ACK。
4. TTS 配置、角色和复刻音色的未保存表单预览改为独立管理端入口；正式设备试听统一经过 Owner 语音链。
5. 电脑端配置页、角色页和播放条删除通用合成调用；手机端角色试听改走明确配置入口，复刻音色试听改走 Owner 语音链。
6. 手机多角色试听必须明确选择角色；单角色自动选择；配置中的当前角色只作为弹窗预选值。
7. 删除旧通用 Handler、按消息播放 Handler、Service 方法、路由和前端调用；为旧路由保留 `404` 回归断言。
8. 生产模拟数据按 Owner、kind、resource_id、speakerId 和配置主键精确清理，不使用宽泛删除条件。

## 新接口边界

Owner 正式朗读：

- `POST /api/device-mesh/v1/business/speech`
- 新增可选字段 `speakerId`
- 角色、文本、预期执行范围和 Owner 确认仍然必填

未保存表单预览：

- `POST /api/tts/configs/:id/preview`
- `POST /api/tts/characters/:characterId/preview`
- `POST /api/tts/voice-clones/:speakerId/preview`
- `POST /api/tts/preview`

以上四个入口只允许共享 Core 管理端使用，绑定普通设备不能借此绕过 Owner 链路。

## 功能迁移

| 原功能 | 新位置 |
| --- | --- |
| 指定配置试听 | `/api/tts/configs/:id/preview` |
| 指定角色试听 | `/api/tts/characters/:characterId/preview` |
| 指定复刻音色试听 | `/api/tts/voice-clones/:speakerId/preview` 或 Owner `speakerId` |
| 未保存配置试听 | `/api/tts/preview` |
| 角色正式朗读 | Owner Speech |
| 复刻音色正式试听 | Owner Speech 显式 `speakerId` |
| 旧消息播放 | 已删除，不保留兼容回退 |

旧实现删除：

- `backend/internal/tts/legacy_play_handler.go`
- 通用 `/api/tts/synthesize`
- 按消息播放 `/api/tts/play/:messageId`
- `SynthesizeWithActive`
- 手机 `TTSService.synthesize`
- 电脑端和手机端对旧通用合成接口的调用

## 跨模式

- 本地 Core：Owner 为当前设备，显式复刻音色仍需角色和 Owner ACK。
- 云端统筹 Core：Owner 为 Core，角色 revision、ACL、Space 和活动配置按 Core 数据校验。
- 云端非统筹设备：仍使用真实设备 Owner，不得通过绑定设备身份调用共享 Core 管理入口。
- Core、Provider、权限或模式切换：旧 execution scope 被拒绝，迟到音频不保存、不播放。
- 复刻音色不属于当前 Space、配置已切换或 Provider 不支持复刻：明确失败，不替换为角色默认音色。

## 定向验证

- 后端 TTS 与 Owner business 定向测试通过。
- 三 Core 真实 Owner 语音测试通过，覆盖当前角色、Owner 确认和 Provider 切换取消。
- 普通绑定设备不能创建、删除或绕过共享 Core 复刻音色。
- 后端 `go vet` 与 `go build ./cmd/server` 通过。
- 电脑端 `vue-tsc --noEmit` 通过。
- 电脑端 Owner 语音和角色权威测试 19 项通过。
- 前端生产构建通过。
- 手机端相关文件和测试定向分析无 error、warning 或 info。
- 手机端 Owner 语音、配置归属、单角色自动选择和多角色明确选择共 13 项测试通过。
- `git diff --check` 通过。
- 运行源码中旧通用语音路由、Service 方法和旧播放 Handler 引用为 0；测试仅保留旧路由必须返回 `404` 的断言。

## 真实生产环境模拟

使用当前正式本地凭据、完整 Electron、Core、Qdrant、SurrealDB 和隔离 TTS Provider 执行。隔离 Provider 使用正式火山 SSE 请求与响应协议，避免产生外部供应商费用和真实音色变更。

1. 创建唯一临时复刻音色 `audit-clone-20261023` 和唯一 TTS 配置 `900001`。
2. 通过正式语音复刻接口创建音色，并使用真实活动配置绑定。
3. 调用 Owner 语音接口时发送明确 `speakerId=audit-clone-20261023`。
4. 隔离 Provider 确认收到相同 Speaker，返回正式 SSE 音频数据。
5. Owner 语音接口返回业务码 200、`saved=true`，音频 MIME 为 `audio/mpeg`。
6. 音频 SHA-256 为 `94bc04280638b725cf9cd07ab18aed7568816844e9f63d27451f643f3a127ee9`。
7. Owner ACK 为 `space_2chhqxu5ffnncw2shczebcyld4`。
8. 资源 ID 为 `speech/34d506b3f39d8924c58c808f851b87c5`，回执版本为 `tool-result=1`、`checkpoint=2`。
9. 旧 `/api/tts/synthesize` 与 `/api/tts/play/x` 均返回 `404`。
10. 按精确 Space、配置 ID、speakerId、Owner 和资源 ID 清理临时数据；TTS 配置、复刻音色、Owner 资源、inbox 和 outbox 测试残留均为 0。
11. `app.db` 与 `extensions-v2/kernel.db` 的 `quick_check` 均为 `ok`。

当前源码重建结果：

- Core SHA-256：`0697b7bc6587485f35d38fbd76fbe9f64975839e6cbbe38eda95544afdcef75c`
- Source Gate：`d68df58ce44d1ae0556b57edee05fd197b7afa699486f3d5879304147500467d`
- 冻结范围：11092 项

## 验证边界

Provider 侧使用正式协议模拟，没有调用真实付费音色复刻服务，也没有修改用户已有音色。本轮未构建 Android Release APK 或桌面正式安装包，未重新安装手机端；手机端完成了源码、定向测试和静态分析。当前未启动独立 Cloud Core 群集，跨模式由同一 Owner 契约、三 Core 业务测试和权限矩阵覆盖。未修改管理端、版本号或发布配置，未拉取、提交或推送 Git。
