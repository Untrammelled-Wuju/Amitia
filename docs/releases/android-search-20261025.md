# Android 搜索框修改构建与安装记录

- 记录时间：2026-10-25T21:53:56+08:00。
- 版本：26.2.0-beta.15，versionCode 21，保持项目当前版本。
- 源码基线：`be597440a35f980ac442d4164a8985144d49ca0d`，提交日志 `chore(release): freeze beta.15 Android sources with borderless search`，分支 `develop`。
- 对比基线：`affb5ac4b6ecfce79778e59edc3fea2bdabd7a17`。包含冻结时用户确认的全部源码改动；冻结后其他工作产生的变更未混入本次基线。
- APK：`artifacts/apk/amitia-26.2.0-beta.15-release-arm64-v8a-20261025-214805.apk`，160538831 字节。
- APK SHA-256：`2a7bd0e313c60e3ec7d8dc70b58e85a8c3885ddcc471de409b3eed41a0689fa5`。
- Runtime Package SHA-256：`b3490d2cc1aebeff99c31de025e87d6243aa0258b8acd33c47fc73af05ef39af`。
- 后端、plugin-host 和 task-host 均从本次源码重新编译，重新生成 Runtime Package；包内组件哈希和 sourceCommit 一致。
- 正式签名校验、原安装证书匹配、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime 校验通过。
- assembleRelease、Release Vital lint、无障碍模块 Release lint 通过。
- 已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），安装后 APK 哈希与本次构建一致。
- 首次启动完成运行时更新，二次冷启动进入消息页；验证日志未发现 FATAL EXCEPTION。
- 真机确认搜索框无边框，按住时无高亮，普通点击正常进入搜索页；验证后返回消息页。
- 手机 /api/health HTTP 200、ready=true、version=26.2.0-beta.15、model=configured；/livez HTTP 200；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。未进行实际模型对话。
- 本地完整项目已重建 macOS 核心并重启，前端、Core、Qdrant、SurrealDB 健康检查均 HTTP 200，readyz 为 ready（6 ready）。
- go mod verify、go vet ./...、Linux ARM64 编译和网页端 typecheck/build 通过。
- 手机端 40 个定向测试文件：263 项通过、10 项失败；失败涉及角色头像、记忆页面和工作坊布局，本次搜索跳转用例通过。
- 后端 56 个改动包定向测试：45 包通过、3 包无测试、8 包失败，35 个顶层用例失败。
- 任务宿主 16 项、QQ 渠道 25 项、表情插件接口 51 项测试通过。
- 定向测试未全部通过，本次不记录为完整测试门禁通过。
- 日志：`artifacts/android-search-20261025/`。失败明细：`backend-test-results.json`、`flutter-test-results.json`。
- 本次仅本地构建与安装，未发布更新通道、未推送 Git。
- 本地 AGENTS.md 同步更新记录，并遵循现有 .git/info/exclude 保持不跟踪；本文件单独提交保留构建记录。
- 本记录提交不是下一次源码差异对比的目标源码提交。

## 后端失败包

- `github.com/u-ai/backend/internal/devicemesh/agent`
- `github.com/u-ai/backend/internal/configwrite`
- `github.com/u-ai/backend/internal/devicemesh/executionjournal`
- `github.com/u-ai/backend/internal/extension`
- `github.com/u-ai/backend/internal/extension/kernel/task_runtime`
- `github.com/u-ai/backend/internal/extension/kernel/trusted_service`
- `github.com/u-ai/backend/cmd/server`
- `github.com/u-ai/backend/internal/extension/kernel`

