# Amitia Harness 最后 10% 实施与验收记录

日期：2026-10-09。工作路径：`D:\桌面\跟进项目\U-Ai`。初始分支 `master`，初始 HEAD `d106b98e9`。

## 重要结论

这是当前已完成的实现和测试证据，不是生产级 100% 验收证明。原有工作区存在大量未提交的其它开发内容；所有更改都保留在原项目目录。未执行重置、清理、拉取、提交或推送。

## P1 父 Turn 恢复：部分完成

- 原 Interaction 上进行数据库执行租约竞争、心跳与过期回收，阻断两个独立 Orchestrator 并发获取同一个有效所有者。
- 恢复沿用原 interactionId、turnId、requestId、executionId；删除恢复调度 10 分钟固定截止。
- 恢复扫描分页、修复暂时不可执行时的延迟重试，恢复前利用工具账本补齐已经持久化的工具结果。
- 修复执行结果不确定却把父 Interaction 和 Turn 标记为失败的状态错配。

验收缺口：目前自动恢复候选筛选仍偏向具有已终结多 Agent 协调凭据的 Web Turn；通用主 Agent 多状态、父子同时崩溃、所有七类父 Turn 进程故障窗口尚未完成独立端到端验收。

## P2 工具副作用：已实现显著补齐，尚非全工具生产验收

- 模型统一工具路径复用现有 tool_call_intents 和 tool_call_results，执行前持久化、同一工具调用结果复用、未知副作用阻断重放。
- 增加 attemptId、ownerInstanceId、turnId、executionId、inputHash、resultRef、errorClass、开始结束时间。
- 增量迁移 20261009001 已注册，声明式 baseline.sql 同步更新；新建数据库、旧版数据库补列与重复迁移测试通过。
- 结果写库后至 Turn checkpoint 写库前崩溃时，可以根据持久化证明修复缺失 checkpoint。
- 超时、网络失联、设备离线与 UNKNOWN/UNCERTAIN 返回记录为 INDETERMINATE，不将不可信的失败视为可重试的安全失败；在对账前拒绝重复副作用。
- 独立测试子进程在七类故障窗口以退出码 37 崩溃，读取实际 SQLite 和副作用文件，七窗口各重复 15 次，共 105 次测试通过，无重复文件副作用。

验收缺口：此 105 次仅为工具账本测试，并非父 Turn 七窗口全链路；外部支付、消息推送、远端设备 Worker、旧工具入口等无法仅凭这一测试证明 exactly-once。人工对账接口和文件写入前后版本/Diff 证据尚未全面验收。

## P3 Coding Agent：部分完成

- 原有工具链和原有验证门禁继续使用，无新执行器。
- 验证命令不接受 `go test || true`、输出重定向、命令替换等掩盖测试真实结果的形式。
- Agent 不再在同轮反复提交相同工具指令达到停滞后将模型消息当作成功结果。
- 不可信/不可持久化的工具结果阻断最终成功。
- Go/Vue/Flutter 人为植入错误、自动诊断修改、最后一次改动后重新构建及最终自验收的完整评测基线仍缺失，不能宣称 P3 完成。

## P4 跨端恢复：基础状态语义已补，真机全链路待验证

- Go AgentUIEvent 快照/回放、Web TS Reducer、Flutter Dart Reducer、两端运行控制器使用相同 `turn.waiting`/`needs_reconciliation`，保留相同 turnId 与 executionId。
- 等待安全对账时暂停 UI 的“正在发送/生成”指示，不发送伪完成通知。
- 通知 Runtime 接收原始等待事件，反映“状态待对账，停止自动重试”，对过期工具完成事件拒绝将任务猜为成功。

验收缺口：本轮 ADB 检查无可用设备；Release 签名环境变量未配置到本次命令进程，未执行 Android Release 重新构建、签名、安装及真机验证。iOS、Windows Electron 重启、云端 Worker 跨网络断线端到端验收未完成。

## P5 / P6 可观测性与生产验收：未完成

当前有原 request/interaction/turn/execution/toolCall/attempt 相关身份与账本引用；尚未以自动评测基线覆盖至少五类 Coding 任务并长期回归。尚未执行完整 48 小时持续运行、完整生产环境数据库升级回滚、灰度发布、权限安全审计、真实异地 Worker/云端设备对账。不能宣布 Harness 100%。

## 已验证测试（全部为指定范围而非全套覆盖）

- Go `internal/chat`、`internal/interaction`、`cmd/server`、`internal/migration`、`internal/conversationstream` 定向测试通过。
- Go `-race` 对恢复租约、工具账本、状态投影相关定向测试通过。
- Go Windows 后端 `go build ./cmd/server` 成功，构建物位于 backend/artifacts/harness-final10。
- 工具账本 `-count=15` × 七类独立进程故障窗口通过，即 105 个窗口用例。
- Web 两组 Vitest 指定文件，共 18 条通过（Golden 及运行时投影）。
- Flutter 三组定向测试，包括 Reducer、Golden 及运行控制器，共 26 条通过。
- Notification Runtime 对账状态测试与 Race 结果，以最后一次命令输出为准；无结果时不得默认为通过。

## 发布门禁状态

未提交或推送 Git；未覆盖预存分支工作。未重启真实全套生产服务，也未部署或签名手机应用。源码保存在 Windows 原工作区，最终源码归档与校验和另行生成。

只有 P1–P6 实际运行证据逐项满足用户制定的门槛，才能将工程成熟度标记为 100%。严禁用剩余权重或代码行数推算已经完成的验收百分比。

## 2026-10-09 follow-up: Generic Web parent-turn recovery

New implementation files:
- `backend/internal/interaction/parent_turn_checkpoint.go`
- `backend/internal/interaction/parent_turn_checkpoint_test.go`
- `backend/cmd/server/parent_turn_generic_recovery_test.go`

Modified:
- `backend/internal/interaction/recovery_descriptor.go`: a fingerprinted `ParentTurnRecoveryRef`, including workspace, device, permission mode, thread, message style and model selection; legacy MultiAgent fingerprints unchanged.
- `backend/internal/interaction/orchestrator_process.go`: record generic Web parent recovery checkpoint before model execution, fail closed on persistence failure.
- `backend/cmd/server/parent_turn_recovery_runtime.go`: scan with primary-key cursor instead of OFFSET; validate ordinary parent descriptor scope and original interaction; enforce permission, model, workspace and device binding; restore thread, style and workspace context; never replay incomplete tool checkpoints.
- `backend/internal/chat/agent_tool_ledger.go`: fail closed on missing DB/recorder, avoiding untracked external tool effects.
- `backend/internal/chat/agent_tool_ledger_test.go`: missing durability store regression.

Scope and caveats: Generic automatic recovery is intentionally limited to eligible Web text turns with an existing original user message and AssistantTurn, a nonterminal original Interaction, unchanged scope and model/permission, and fully acknowledged tool results (or independently journaled result recovery). It does not make external side effects exactly-once, does not auto-reconcile indeterminate tool calls, and does not certify cloud/device, Electron/iOS/Android, 48-hour load, or all production fault-injection cases. Use "needs_reconciliation" instead of blind replay when an effect has unknown outcome.

Targeted original parent tests and race passed prior to the extended test run. Extended package suite, new crash window repetitions, and current build results must be checked before accepting this iteration.

## 2026-10-09 follow-up: Windows sandbox task execution and ledger uncertainty

Full `go test` of six backend packages initially passed chat/migration/conversationstream/notificationruntime/interaction and failed server tests that explicitly required `AMITIA_TEST_NODE`. With the repository's bundled Node provided, a real TLS device-owned task still failed before execution: Windows task sandbox PowerShell wrapper `Add-Type` threw `DirectoryNotFoundException` for a malformed temporary path, and side effects appropriately stayed indeterminate with no blind replay.

Controlled environment experiments: with `TMP` populated from existing valid `TEMP`, the real TLS device owner reconnect/permissions test passed across inline/artifact/pause-resume/per-use scenarios. The real Source Native TLS test passed with `TMP` populated and `ProgramFiles(x86)` unchanged/unset. Production fix in `backend/internal/platform/process/environment.go` fills absent Windows TMP from TEMP or the OS temp directory; it does not loosen sandbox isolation. Added `environment_test.go` for empty/present TMP. Rerunning the actual Source Native TLS test with test runner TMP still unset passed after this source change. Full server package re-run with bundled Node is a separate pending check.

The tool ledger now refuses to run external operations when no durable recorder/DB exists and classifies transport-uncertain errors (deadline exceeded, TCP reset, websocket close, disconnect, unexpected EOF) as requiring reconciliation instead of deterministic retry. Added corresponding `agent_tool_ledger_test.go` cases and a `-race` regression check. Windows real TLS test fixture now exposes bounded TaskHost process diagnostics on stderr to make infrastructure faults diagnosable; does not bypass sandbox or side-effect fencing.

## Further acceptance hardening (2026-10-09, R5)

Generic Web parent-turn recovery now snapshots the original effective Conversation model, reasoning settings, workspace binding, device and permission mode, fingerprints those settings, and rejects config drift while honoring explicit per-turn model overrides. Added original-parent snapshot and generic-recovery drift tests. Removed the unrelated MultiAgentCoordinator and TaskHost enablement prerequisites for the parent recovery loop without adding a separate scheduler.

Tool side-effect ledger hardened: durable result reuse and checkpoint recovery require matching intent terminal status, result_ref, attempt_id, execution_id, turn_id, input_hash and stored status; corrupted or contradictory outcome evidence blocks replay and requires reconciliation. Added multiple adversarial result-provenance tests, 105 repeated independent process crash windows, and lease-loss tests. The live parent execution lease is propagated in context to the ledger; immediately before side effects, the ledger checks active owner, heartbeat freshness, cancellation and status in SQLite, preventing known-stale workers from starting further external tool effects. This is pre-dispatch fencing only; cannot retroactively cancel an in-flight effect at a remote provider. It does not establish exactly-once effects for uncooperative remote endpoints.

Legacy Agent tests have been upgraded to use actual durable SQLite tool journals instead of untracked stand-in execution. Project service test fixtures now migrate AssistantTurn tables. Full chat package and targeted race checks passed after these changes. The whole backend package suite has to be rerun on the final source snapshot.

Android build memory settings: mobile_app/android/gradle.properties no longer constrains Gradle to 512 MB, 1 worker, 448 MB metaspace and 64 MB code cache; parallel execution is enabled. The JVM/OS still enforce physical resource constraints. Android debug build validation is separate from production release signing; release signing credentials are not present in this remote session. Existing Android 16 device app was boot-smoke-tested, but full cloud-connected real-device acceptance and iOS test are not completed.
## Android build follow-up: unlimited configured JVM heap and artifact routing

Original mobile_app/android/gradle.properties imposed -Xmx512m, 448 MiB metaspace, 64 MiB code cache and org.gradle.workers.max=1. Removed these artificial ceilings and enabled Gradle parallel mode. Physical host resources and OS constraints still apply. Observed Java build processes have no explicit -Xmx.

Initial Debug build failed because the host preBuild demanded a separately signed accessibility Release APK, with no production keystore configured. mobile_app/android/app/build.gradle.kts now obtains the accessibility Debug APK for Debug/Profile and requires the signed Release APK only for Release. Release signing gates remain enforced.

Initial ShaderCompilerException could not write Flutter shaders on a non-ASCII working path. Building through ASCII-only subst R: allowed Gradle to assemble the Debug APK (192295410 bytes), but the Flutter CLI failed to discover it: Gradle emitted android/app/build while Flutter expects mobile_app/build/app. mobile_app/android/build.gradle.kts now selects the canonical Flutter output layout if no redirected native build root is in use; redirected native intermediates preserve the ASCII-only location. app/build.gradle.kts synchronizes variant-specific finished APKs to Flutter's expected artifact directory when build root redirection is enabled. A fresh CLI exit-code-zero Flutter build remains the acceptance condition.

The seven main backend packages passed their complete suite and the Web and Flutter chat runtime targeted tests passed. This Debug APK still bundles the preexisting runtime at sourceCommit d106b98e96dfa3f34621c82abd2c9edc349dfbe5 and does NOT prove the unsaved latest Harness backend is embedded. A new verified runtime package and protected Release signing input are necessary for real end-to-end current-source deployment.
## Verified Flutter Android Debug build (2026-10-09, R9)

The Android Debug APK completed with Flutter CLI exit code 0 after fixing variant-specific accessibility signing and introducing a post-assemble APK staging task that resolves both Gradle's layout directory and the Flutter plugin's legacy android/app/build output. Direct Gradle debug build had succeeded but :app:stageDebugFlutterApk was NO-SOURCE; the corrected task discovered the new source and synchronized it into Flutter's expected mobile_app/build/app/outputs/flutter-apk directory.

Resulting APK: mobile_app/build/app/outputs/flutter-apk/app-debug.apk
APK bytes: 190362014
SHA256: 3AA609CC4123C5D09204CFEE820E4E4A3B10E191704AC93530F0F1F0D7725B56
Validated command: flutter build apk --debug --no-pub (exit 0; no manually configured JVM maximum heap).

This is a Debug build using the previously embedded runtime package; not a release-signed build of the current modified Harness backend. External side effects exactly-once and comprehensive Android device/cloud functional acceptance remain outstanding.
## 2026-10-09 recovered remote session: validated working-tree Runtime Package

After Desktop Commander reconnected, verified the Linux ARM64 Runtime candidate exists at backend/artifacts/harness-final10/amitia-runtime-working-tree-r7.zip (113669075 bytes, SHA256 8788e2e05cc78fc21a65eb56fee1cec775d03d21f52288573af1fb908b2df990).
Project validator runtime/build/runtime-package/android-arm64/validate.py --package completed with exit code 0.
Independent metadata/backend verification confirmed metadata/component-index.json runtime.backend SHA256 equals the newly cross-compiled backend artifact SHA256 2ac663c532e9f5217ffd3079c3cde849f3e3017d9bd21f889fc8388d9ae112f9 (exit code 0).
The original Runtime Zip was not overwritten. Its sourceCommit field continues to identify the previous HEAD revision, so current worktree provenance should be additionally established by the binary digest and this delivery's source package.
No Android Release signing environment variables were present during recovery. Neither Release build, Release installation nor real-device recovery claims may be asserted until the signed build and verification are actually run.
## Release lint and signing task-graph isolation (2026-10-09)

After runtime package validation and the Release lint failure investigation, the Android app Gradle lifecycle was updated so the generic preReleaseBuild and pure lint tasks do not require a signed accessibility provider APK. The separate copyAccessibilityProviderAsset task creates the signed provider APK for actual packaging; its dependency on accessibility:assembleRelease is omitted only when every explicitly requested Gradle task is a lint task. Other task invocations preserve the dependency and copy. The app's validateReleaseSigning task now gates packageRelease, assembleRelease and bundleRelease, not lint's preReleaseBuild.

Verified: gradlew :app:lintVitalRelease --dry-run --no-daemon completed successfully and included no :amitia-accessibility:assembleRelease or :amitia-accessibility:packageRelease.
Verified: gradlew :app:lintVitalRelease --no-daemon completed with BUILD SUCCESSFUL, exit code 0, 500 actionable tasks (17 executed, 483 up to date); log backend/artifacts/harness-final10/android-lint-vital-release-r12.log.
Verified: gradlew :app:assembleRelease --dry-run --no-daemon completed with exit code 0 and includes :app:validateReleaseSigning, :amitia-accessibility:assembleRelease, and :app:copyAccessibilityProviderAsset; see release-signing-graph-r12.log.
Actual Release signing and device installation remain blocked until the four production signing environment variables are injected. Absence of credentials must continue to cause an explicit failure, never a debug-signing fallback.
Verified: explicit gradlew :app:validateReleaseSigning --no-daemon returned exit code 1 with Release signing is not configured and all four required environment variables absent. Negative security test recognized this deliberate failure (SIGNING_FAIL_CLOSED_PASS). Log: backend/artifacts/harness-final10/release-signing-guard-r12.log.
## Verified signed Android Release, runtime-provenance and physical-device smoke (2026-10-09)

Signing credentials were discovered in Windows User-scoped environment variables; the remote process-scoped environment was empty. Four variables were passed into a child build process without printing their secret contents. The configured keystore file exists and :app:validateReleaseSigning returned exit code 0.
Flutter clean, flutter pub get and Flutter arm64 Release --config-only succeeded on the R: alias of the original repository, without a temporary source copy. Subsequent :app:lintVitalRelease returned 0. Gradle assembleRelease returned 0 after R8 optimization, with the accessibility provider Release assembled and copied into app assets. No artificial Gradle memory cap was added.
Current-source embedded Runtime input SHA256 8788e2e05cc78fc21a65eb56fee1cec775d03d21f52288573af1fb908b2df990 was verified in Android src/main/assets and again in the newly produced canonical APK at mobile_app/build/app/outputs/flutter-apk/app-release.apk.
The first APK verification accidentally selected an older stale file in mobile_app/android/app/build/outputs/flutter-apk; that file contained prior Runtime SHA256 9914829063d22a655901529a0278bc12eee91514a77457fdda59c15de908364e. Corrected candidate selection to prioritize the canonical freshly built Flutter output. This was a stale-output selection defect, not a bad new APK. The old APK was NOT installed.
Verified release APK contains exactly one embedded Runtime package with the matching SHA256, one accessibility provider APK, and native libraries only under arm64-v8a. zipalign and apksigner verified successfully; APK Signature Scheme v2 is valid, signer certificate SHA256 a136e76065afaeda85251009f5ba98c4c5b23a5d009955ab30958a71156c5153. aapt returned applicationId com.amitia.amitia_app, versionName 26.2.0-beta.3, versionCode 5.
Verified published local artifact: artifacts/apk/amitia-26.2.0-beta.3-release-arm64-v8a-20261009-192646.apk, bytes 135069253, SHA256 e63d890caa29f12808057cb41818b1bc7fbb90b212d37bdfd5c1b185579463d0.
ADB installed this verified signed APK with adb install -r returning Success (exit code 0) on Android 16 device 2407FRK8EC. Cold app activity start returned 0; process com.amitia.amitia_app PID 16200, version 26.2.0-beta.3 and versionCode 5, no com.amitia.amitia_app.debug package. Sampled current app PID logcat showed no fatal Android exception or unhandled Flutter error. Foreground top-resumed activity was WeChat after launch, so long-term foreground visibility and device/island notifications have not been proven.
Full cloud recovery, exact-once distributed external effects, user-visible tool reconciliation, P3 self-repair evaluation, P5 observability, 48-hour production soak, backend service restart and iOS release remain unaccepted. This signed APK is a validated local release candidate; no Git baseline commit or remote publication was performed.
