# Amitia Step 22 - Final Cleanup / Release Gate

**审计时间：** 2026-08-14
**Source Commit：** ac7b946b0a660f06522b474ff02f85317ea88356
**Latest Tag：** v3.1.0-develop

---

## 审计结果汇总

| 类别 | 数量 |
|------|------|
| 高危风险项 | 4 |
| 中危风险项 | 4 |
| 低危风险项 | 3 |
| 通过检查项 | 127/138 |

---

## 关键项目状态

```
Step18 Candidate ID = NOT_APPLICABLE (本步骤为首次桌面端Release Gate审计)
Step19 Result = NOT_APPLICABLE
Step20 Result = NOT_APPLICABLE
Step21 Result = NOT_APPLICABLE

Release Candidate ID = ac7b946-desktop-release-gate

Step18 APK SHA = NOT_APPLICABLE (桌面端Release)
Step21 Tested APK SHA = NOT_APPLICABLE
Release APK SHA = NOT_APPLICABLE
Validated APK == Released APK = NOT_APPLICABLE

Step18 AAB SHA = NOT_APPLICABLE
Release AAB SHA = NOT_APPLICABLE
Validated AAB == Released AAB = NOT_APPLICABLE

Runtime Package SHA = NOT_APPLICABLE (非Android App Runtime)
Release Runtime Package SHA Match = NOT_APPLICABLE

PRoot SHA = NOT_APPLICABLE (桌面端无PRoot组件)
Release PRoot SHA Match = NOT_APPLICABLE

Source Commit = ac7b946b0a660f06522b474ff02f85317ea88356
Release Source Commit Match = PASS

Runtime Version = NOT_APPLICABLE
Release Runtime Version Match = NOT_APPLICABLE

App Version Name = v3.1.0-develop
App Version Code = N/A

Signing Certificate SHA = PENDING_VERIFY
Release Signing Certificate Match = PENDING_VERIFY

Candidate Rebuilt After Validation = NOT_APPLICABLE
Candidate Resigned After Validation = NOT_APPLICABLE
Candidate Patched After Validation = NOT_APPLICABLE
Runtime Package Replaced After Validation = NOT_APPLICABLE
PRoot Replaced After Validation = NOT_APPLICABLE

RuntimeController Authority Count = 1
RuntimeState Authority Count = 1
RuntimeInstaller Authority Count = 1
RuntimeManifestStore Authority Count = 1
ActiveRuntimeManager Authority Count = 1
RuntimeHostLayout Authority Count = 1
GuestLayout Authority Count = 1
MountContract Authority Count = 2 (同名不同职责，PASS)
PRoot Resolver Authority Count = 1
RuntimeService Authority Count = 1
StartupDetector Authority Count = 1
Generation Authority Count = 1
RuntimeBridge Authority Count = 1
BackendConnection Source Count = 1
BackendTransport Authority Count = 1
RuntimeStatusProjection Authority Count = 1
Business Gate Authority Count = 1
Recovery Authority Count = 1
Go LocalCredentialStore Authority Count = 1
Go RuntimeHost Authority Count = 1
Go ProcessSupervisor Authority Count = 1
Go RuntimeOrchestrator Authority Count = 1

Android Second Backend Count = 0

Workspace Runtime Production Source = 0
LegacyBundled Runtime Production Source = 0
Downloads Runtime Source = 0
External Storage Runtime Source = 0
Latest/Newest Runtime Resolver = 0
Arbitrary packageUri Production = 0

PRoot Assets Fallback = 0
PRoot filesDir Fallback = 0
PRoot PATH Fallback = 0
Termux PRoot Fallback = 0
PRoot Network Download = 0

Runtime Online Download = 0
Runtime apt install = 0
Runtime npm install = 0

pkill Runtime Usage = 0
killall Runtime Usage = 0
Global Process Scan/Kill Usage = 0 (仅管理自身子进程，PASS)

Runtime WorkManager Auto Restart = 0
Runtime AlarmManager Auto Restart = 0
Runtime Boot Auto Start = 0
RuntimeService START_STICKY = 0
RuntimeService Exported = false (PASS)

Flutter Arbitrary Shell = 0
Flutter Arbitrary Mount = 0
Flutter Arbitrary Env = 0
Flutter Raw Token Input = 0
Flutter Generation Authority = 0

Token In PRoot Argv = 0
Token In Guest Env = 0 (有测试验证禁止)
Token In Runtime Snapshot = 0
Token In Runtime Event = 0
Token In URL = 1 (FAIL - desktop与front通过URL query param传输token)
Token In Android Log = 0
Token In Flutter Log = 0
Token In Go Log = 1 (FAIL - QQ侧车token输出到console.log)

Android Persistent Token Duplicate = 0
Flutter Persistent Token Duplicate = 0

API Secret Leak = 1 (FAIL - 火山引擎AccessKey打印到appLog)
Signing Secret Leak = 0
Host Absolute Path UI Leak = 0 (Android路径有脱敏保护)

External Storage Runtime/Data Dependency = 0
Root Dependency = 0
Termux Dependency = 0
Shizuku Dependency = 0
ADB Normal-Use Dependency = 0

arm64-v8a Enabled = 1
x86 Enabled = 0
x86_64 Enabled = 0
armeabi-v7a Enabled = 0

Fake Runtime Release Enabled = 0
Mock BackendTransport Release Enabled = 0
Auth Bypass Release Enabled = 0
Test Package Source Release Enabled = 0
Crash Injection Production API = 0
Debug Runtime Path Override = 0

Feature Direct BackendTransport Bypass = 0
Feature Direct BackendConnection Bypass = 0
Runtime READY Business Gate = 0
BackendService Non-null Business Gate = 0
readyz Business Gate = 0

Runtime Polling = 0
Connection Polling = 0
Per-Request Token Fetch = 0
WebSocket Reconnect Storm = 0 (有指数退避)
Recovery Unbounded Retry = 0
Structural Failure Recovery Loop = 0

Recovery Auto Version Switch = 0
Recovery Runtime Install/Download = 0
Recovery Persistent Data Delete = 0

Candidate Build Entry Count = NOT_APPLICABLE
Candidate Validator Count = NOT_APPLICABLE
Latest/Newest Artifact Selection = 0

Build Record Validation = NOT_APPLICABLE
SHA256SUMS Validation = NOT_APPLICABLE
Candidate Validator Result = NOT_APPLICABLE

Release APK Explicit Path = NOT_APPLICABLE
Release APK SHA Verified = NOT_APPLICABLE
Release AAB Explicit Path = NOT_APPLICABLE
Release AAB SHA Verified = NOT_APPLICABLE

Android debuggable = false (PASS)
Manifest Security Audit = PASS
Network Security Config Audit = WARN (未配置network_security_config.xml)
Backup/Token Rules Audit = WARN (未配置dataExtractionRules)

Symlink Escape Audit = PASS
Archive Traversal Audit = PASS
Same Version Different Bytes Test = PASS
Runtime Root Drift Fail-Closed = WARN (RUNTIME_ROOT_MODIFIED已定义但未使用)
Invalid Active Runtime No-Fallback = PASS
Missing Runtime Package No-Network-Repair = PASS
Missing PRoot No-Download-Fallback = PASS
Missing Token Android No-Generation = PASS
Transport Error No-Reinstall = PASS
Business Error No-Reinstall = PASS

PRoot License = NOTICE (需手动确认第三方PRoot许可)
Node License = MIT (PASS)
Qdrant License = Apache-2.0 (PASS)
Ubuntu Rootfs Notice = NOTICE (需包含rootfs notice)
Third Party Notices = PASS (THIRD_PARTY_NOTICES.md存在)
Project License = AGPL-3.0 (PASS)

Release Metadata = PENDING_COMPLETE
Release Gate Report = GENERATED

Runtime Core Test Waiver Count = 0
Non-Core Release Waiver Count = 0

Validated Bytes == Released Bytes = NOT_APPLICABLE

Final Result = FAIL
```

---

## 高危风险项 (FAIL)

### 1. [HIGH] QQ侧车Token日志泄漏

| 项目 | 值 |
|------|-----|
| 文件 | `backend/qq-sidecar/src/qqbot-client.ts:317` |
| 文件 | `backend/qq-sidecar/bundle.mjs:297` (同步副本) |
| 问题 | `console.log(`[QQBot] Identify token: ${payload.d.token.substring(0, 40)}...`)` 输出完整token前40字符到控制台日志 |
| 违规 | Token in Log ≠ 0 |
| 修复建议 | 删除该日志语句或改为输出脱敏版本 (如 `****` + 后4字符) |

### 2. [HIGH] 火山引擎AccessKey日志泄漏

| 项目 | 值 |
|------|-----|
| 文件 | `backend/internal/realtime/proxy.go:107` |
| 问题 | `appLog.Info("volc headers: ... AccessKey=" + realtimeAccessToken)` 明文打印AccessKey到应用日志 |
| 违规 | Secret Leak ≠ 0 |
| 修复建议 | 使用现有脱敏机制掩盖AccessKey |

### 3. [HIGH] 火山引擎App Key硬编码

| 项目 | 值 |
|------|-----|
| 文件 | `backend/internal/realtime/proxy.go:101` |
| 问题 | `volcanoHeaders.Set("X-Api-App-Key", "PlgvMymc7f3tQnJ6")` App Key硬编码在源码中 |
| 违规 | 密钥管理最佳实践违反 |
| 修复建议 | 将Key移至配置文件或SecretStore，从环境变量读取 |

### 4. [HIGH] Token通过URL query param传输

| 项目 | 值 |
|------|-----|
| 文件 | `desktop/src/main/ui-host-sse.ts:78` |
| 文件 | `front/src/composables/useUIHostSSE.ts:167` |
| 问题 | `/api/proactive-sse?clientId=electron-ui-host&token=${encodeURIComponent(token)}` token通过URL参数传输，可被代理日志、浏览器历史记录记录 |
| 违规 | Token in URL ≠ 0 |
| 修复建议 | 改为Header传输或使用POST body |

---

## 中危风险项 (WARN)

| # | 类别 | 问题 | 文件 | 建议 |
|---|------|------|------|------|
| 1 | Network Security | 未配置network_security_config.xml | AndroidManifest | 添加显式配置锁定runtime仅localhost通信 |
| 2 | Backup | 未配置dataExtractionRules | AndroidManifest | 排除runtime/surreal/qdrant数据目录 |
| 3 | Drift Detection | RUNTIME_ROOT_MODIFIED错误码已定义但未实际使用 | RuntimeManifestError.kt | 启动时校验treeSha256与实际文件系统哈希 |
| 4 | Token Storage | Token存储于localStorage，未使用OS级安全存储 | front/src/runtime/runtime-adapter.ts | 迁移至OS Keychain/DPAPI |

---

## 低危风险项 (INFO)

| # | 类别 | 说明 |
|---|------|------|
| 1 | Manifest权限 | FOREGROUND_SERVICE_SPECIAL_USE、POST_NOTIFICATIONS为应用正常功能所需 |
| 2 | LocalToken | Flutter端通过revealForTransport()受控暴露，有generation过期门闸 |
| 3 | Path暴露 | /data/user/等路径已脱敏验证，proot路径不出现在用户可见错误中 |

---

## 静态搜索最终集合 (Section 52)

| 搜索模式 | Production命中 | 状态 |
|----------|---------------|------|
| V2/New/Manager2 duplicate authorities | 0 | PASS |
| Runtime download (curl/wget/downloadRuntime) | 0 | PASS |
| latest/newest resolver | 0 | PASS |
| Workspace/External Storage Runtime source | 0 | PASS |
| Termux/System PRoot | 0 | PASS |
| System Node/Qdrant | 0 | PASS |
| pkill/killall/pgrep/process scan | 0 | PASS |
| arbitrary shell | 0 | PASS |
| arbitrary mount | 0 | PASS |
| arbitrary env | 0 | PASS |
| Token in argv | 0 | PASS |
| Token in env (guest) | 0 | PASS |
| Token in Snapshot | 0 | PASS |
| Token in Event | 0 | PASS |
| Direct Feature BackendTransport | 0 | PASS |
| Direct Feature BackendConnection | 0 | PASS |
| Debug/Test bypass | 0 | PASS |
| Crash injection production exposure | 0 | PASS |
| x86/x86_64 ABI | 0 | PASS |
| External Storage Runtime/Data | 0 | PASS |
| Token in URL | 1 | **FAIL** |
| Token in Log (QQ-sidecar) | 1 | **FAIL** |
| Secret Leak (Volcano AccessKey) | 1 | **FAIL** |

---

## 步骤22最终PASS硬门检查 (Section 53)

以下核心硬门失败导致最终结论为FAIL：

- [ ] Release APK SHA与Step21 Tested APK SHA一致 → NOT_APPLICABLE (桌面端)
- [ ] Token in Log = 0 → **FAIL** (QQ侧车token泄漏)
- [ ] Secret Leak = 0 → **FAIL** (火山引擎AccessKey泄漏)
- [ ] Token in URL = 0 → **FAIL** (URL参数传输)

---

## 步骤22最终结论

```
步骤22最终结论：FAIL

理由：
1. 存在3项高危Secret/Token泄漏（Token in Log、Secret Leak、Token in URL）
2. 存在1项硬编码Secret（火山引擎App Key）
3. 4项中危问题建议修复（Network Security Config、Backup规则、Runtime Root Drift、Token Storage）

修复后需重新执行审计。
```

---

## 修复优先级建议

### P0 (Release Blocking)
1. 删除 `qq-sidecar/src/qqbot-client.ts:317` 的token日志并重新bundle
2. 脱敏 `realtime/proxy.go:107` 的AccessKey日志输出
3. 移除 `realtime/proxy.go:101` 的硬编码App Key，改为环境变量读取
4. 将 `ui-host-sse.ts:78` 和 `useUIHostSSE.ts:167` 的token传输改为Header

### P1 (建议Release前修复)
5. 添加 `network_security_config.xml`
6. 添加 `dataExtractionRules`
7. 实现 RUNTIME_ROOT_MODIFIED 启动校验
8. 将前端token存储迁移至OS级安全存储

### P2 (后续迭代)
9. Provider API Key加密存储迁移
10. 添加backup排除规则
