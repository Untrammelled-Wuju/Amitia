# Android 角色头像修复构建与安装记录

- 记录时间：2026-10-25T22:31:33+08:00。
- 版本：26.2.0-beta.15，versionCode 21，保持项目当前版本。
- 源码基线：`b15ba9f2189790a0da9670587f2ac3dbaafa6bc2`，提交日志 `chore(release): freeze beta.15 Android role avatar sources`，分支 `develop`。
- 最近本地安装源码基线：`be597440a35f980ac442d4164a8985144d49ca0d`；已发布平台源码基线仍为 `affb5ac4b6ecfce79778e59edc3fea2bdabd7a17`。
- 分别保存与本地安装基线及已发布平台基线的源码差异；范围包括用户此前确认的当前全部源码改动，排除设计临时文件、产物、依赖和运行数据。
- APK：`artifacts/apk/amitia-26.2.0-beta.15-release-arm64-v8a-20261025-222815.apk`，160535843 字节。
- APK SHA-256：`f151042ea8e7ec3fd31d1734855decbe522ec10801bb12b26fcef47754b7fba8`。
- Runtime Package SHA-256：`04703f5582ba46255d139d38235099468524d75a22c4141c4297d5fbc4e9732a`。
- 从本次源码重新编译 Linux ARM64 后端、plugin-host、task-host，并重新生成 Runtime Package。后端构建信息中的 vcs.revision 与源码基线一致。
- 正式签名、原安装证书匹配、16 KB zipalign、单 arm64-v8a ABI、内嵌 Runtime、三个组件哈希及 sourceCommit 校验通过。
- assembleRelease、Release Vital lint、无障碍模块 Release lint 通过。
- 已覆盖安装到 2407FRK8EC（JV9LT8JB8TVKNJ65），安装后 APK 哈希与本次制品一致。
- 首次启动完成 Runtime 更新，二次冷启动进入消息页；验证日志未发现 FATAL EXCEPTION。
- 真机确认消息列表不再显示对话标题首字，当前两条对话显示默认角色图标。实际图片头像、停用角色读取、角色切换及头像更新由定向组件测试验证。
- 手机 /api/health 与 /livez HTTP 200，health.ready=true，版本 26.2.0-beta.15；/readyz HTTP 200、degraded（3 ready、1 degraded、0 blocking、0 failed）。本次未进行实际模型对话。
- 相关手机端四个测试文件共 57 项通过，表情插件接口 61 项通过；网页端 typecheck/build 通过。
- go mod verify、go vet ./... 和 Linux ARM64 编译通过。相对最近本地安装基线无 Go 源码变更，不重复运行后端包测试。
- 上次构建记录中的后端和其他手机模块失败用例未在本次重新验证，本次定向检查不能作为全项目测试通过的结论。
- 本次代码变化为手机端、网页端和表情插件；按纯前端修改规则未重启本地完整服务。已有本地 Core health/livez/readyz HTTP 200，readyz 为 ready（6 ready）。
- 完整验证记录及日志：`artifacts/android-avatar-20261025/`。
- 本次仅本地构建和真机安装，未发布更新通道、未推送 Git。
- 本地 AGENTS.md 同步更新记录，遵循现有 .git/info/exclude 保持不跟踪；本文件单独提交保留记录。
- 本记录提交不是下一次源码差异对比的目标源码提交。

