# A18 Android Runtime Candidate 构建与冻结 - 审计报告

## 概述

| 字段 | 值 |
|------|-----|
| Step | 18 |
| 任务 | 构建并冻结 Android Runtime Candidate |
| 构建日期 | 2026-08-14 |
| 构建时段 | 05:20 - 06:20 (UTC+8) |
| Candidate ID | android-runtime-candidate-1.0.0 |
| Candidate Identity (SHA256) | 44C0E975B1CF5815C88DB8071227AEFA363721DAE88B05F225EA648A09E58A2C |
| APK 大小 | 138,503,280 bytes (132.09 MB) |
| 结果 | PASS |

---

## 构建环境

### 工具链

| 组件 | 版本 |
|------|------|
| Flutter | 3.38.7 |
| Dart | 3.10.7 |
| Android Gradle Plugin | 8.11.1 |
| Gradle | 8.14 |
| Kotlin | 2.2.20 |
| Build Tools | 36.1.0 |
| Android Platform | android-36 |
| compileSdk | 35 |
| minSdk | 21 |
| NDK | 27.0.12077973 |

### 构建路径

由于Windows中文路径编码问题（gen_snapshot 无法处理中文路径中的Dart kernel文件），通过纯英文junction路径 `D:\amitia-junction` 执行构建。

| 原始路径 | Junction路径 |
|-----------|-------------|
| D:\桌面\跟进项目\U-Ai\mobile_app | D:\amitia-junction |

### Git 状态

| 字段 | 值 |
|------|-----|
| Commit | fda2e9e3f7c795ae06d4192126521de5d35f05a1 |
| Branch | develop |
| Dirty | Yes |

Dirty files 包含 Step 18 修改的配置文件（TrustedRuntimePackageSource.kt、AndroidRuntimeModule.kt、build.gradle.kts 等）以及其他模块的历史修改。

---

## Frozen 输入

### Step 6 Frozen PRoot

| 字段 | 值 |
|------|-----|
| 文件名 | libamitia_proot.so |
| SHA256 | B1403A384B92D09B4A01D1130C4E227302D00C186488BD245692882D76BAEA4E |
| 大小 | 2,423,368 bytes |
| ABI | arm64-v8a |
| ELF类型 | ELF64 AArch64 |
| 来源 | android/amitia-runtime/src/main/jniLibs/arm64-v8a/libamitia_proot.so |
| 嵌入位置 | lib/arm64-v8a/libamitia_proot.so |

### Step 7 Frozen Runtime Package

| 字段 | 值 |
|------|-----|
| 文件名 | amitia-runtime-1.0.0-linux-arm64.zip |
| SHA256 | 3F061598A5C0B815CDB1D536694D9E251652BE13F301FB215F1D1AAE0C5F7F57 |
| 大小 | 361,628,971 bytes (344.5 MB) |
| 版本 | 1.0.0 |
| Guest OS | linux |
| Architecture | arm64 |
| 来源 | runtime/build/out/runtime-package/linux-arm64/ |
| 嵌入位置 | assets/runtime-package/amitia-runtime-1.0.0.zip |

---

## 构建产物

### APK 内容清单

#### Native Libraries (lib/)

| 文件 | 大小 |
|------|------|
| lib/arm64-v8a/libamitia_proot.so | 2,423,368 |
| lib/arm64-v8a/libapp.so | 8,127,408 |
| lib/arm64-v8a/libdatastore_shared_counter.so | 7,112 |
| lib/arm64-v8a/libflutter.so | 11,107,920 |

ABI 独占性: **arm64-v8a ONLY** ✓

#### Assets

| 文件 | 大小 |
|------|------|
| assets/runtime-package/amitia-runtime-1.0.0.zip | 361,628,971 |
| assets/flutter_assets/... | (Flutter assets) |

其他assets：baseline.prof、Shader文件、字体等。

---

## 验证结果

### 1. ABI 独占性检查

| 项目 | 预期 | 实际 | 结果 |
|------|------|------|------|
| arm64-v8a | ✓ | ✓ | PASS |
| armeabi-v7a | ✗ | ✗ | PASS |
| x86_64 | ✗ | ✗ | PASS |
| x86 | ✗ | ✗ | PASS |
| armeabi | ✗ | ✗ | PASS |

**修复过程**: 初始构建包含3种ABI（arm64-v8a, armeabi-v7a, x86_64）。通过以下修复解决：
1. 在 `android/app/build.gradle.kts` 的 `packaging.jniLibs` 中添加 excludes 排除非目标ABI
2. 使用 `flutter build apk --release --target-platform android-arm64` 限制Flutter引擎目标

### 2. PRoot 完整性检查

| 项目 | 值 |
|------|-----|
| 预期 SHA256 | B1403A384B92D09B4A01D1130C4E227302D00C186488BD245692882D76BAEA4E |
| APK 实际 SHA256 | B1403A384B92D09B4A01D1130C4E227302D00C186488BD245692882D76BAEA4E |
| Match | True |
| PRoot 数量 | 1 (arm64-v8a only) |

**修复过程**: 初始构建中PRoot被Gradle strip debug symbols（SHA不匹配）。通过在 `packaging.jniLibs.keepDebugSymbols.add("**/libamitia_proot.so")` 禁用strip。

### 3. Runtime Package 完整性检查

| 项目 | 值 |
|------|-----|
| 预期 SHA256 | 3F061598A5C0B815CDB1D536694D9E251652BE13F301FB215F1D1AAE0C5F7F57 |
| APK 实际 SHA256 | 3F061598A5C0B815CDB1D536694D9E251652BE13F301FB215F1D1AAE0C5F7F57 |
| Match | True |
| Runtime Package 数量 | 1 |

### 4. Step 18 Gradle Task 检查

gradle task `copyFrozenRuntimePackage` 通过环境变量 `FROZEN_RUNTIME_PACKAGE_PATH` 和 `FROZEN_RUNTIME_PACKAGE_SHA256` 接收冻结输入。

### 5. 构建后 source 目录检查

`android/app/src/main/assets/runtime-package/` 已通过 `cleanFrozenRuntimePackage` 清理。构建产物中嵌入的 Runtime Package 直接来自环境变量指向的 Step 7 产物。

---

## 静态审计

### 1. x86 ABI 污染

| 检查项 | 结果 |
|--------|------|
| APK 中是否存在 x86/x86_64/armeabi-v7a .so 文件 | 否 |
| Result | PASS |

### 2. 动态下载

| 检查项 | 结果 |
|--------|------|
| 是否存在从网络下载 runtime/proot 的代码 | 否 |
| 是否存在未授权的远程获取逻辑 | 否 |
| Result | PASS |

注: `HttpRuntimeHealthProbe.kt` 用于向 loopback 地址发送健康检查探针，不属于动态下载。

### 3. Debug Bypass

| 检查项 | 结果 |
|--------|------|
| 是否存在 debug 模式下绕过冻结合约的代码 | 否 |
| Result | PASS |

### 4. Token/Secret 泄漏

| 检查项 | 结果 |
|--------|------|
| 是否存在硬编码 API Key | 否 |
| 是否存在硬编码密码/Secret | 否 |
| Result | PASS |

注: `SensitiveValueRedactor.kt` 和 `RuntimeEnvironmentBuilder.kt` 中出现的敏感词仅用于沙箱环境变量屏蔽和日志脱敏，非真实凭证。

### 5. 重复组件

| 检查项 | 结果 |
|--------|------|
| APK 中 PRoot 组件数量 | 1 |
| APK 中 Runtime Package 嵌入数量 | 1 |
| Kotlin 代码中 RuntimePackageSource 定义数量 | 1 |
| Result | PASS |

---

## 关键修复记录

### 修复1: 中文路径编码问题

**问题**: `gen_snapshot` 处理中文路径时出现乱码（`D:\����\������Ŀ\U-Ai\...`），导致 "Unable to read file: app.dill"。

**解决方案**: 创建 NTFS junction 将项目映射到纯英文路径 `D:\amitia-junction`。

### 修复2: Cross-module internal 可见性

**问题**: `TrustedRuntimePackageSource`、`RuntimePackageReference`、`RuntimeHostLayout`、`AndroidRuntimeModule.runtimeHostLayout` 声明为 `internal`，但 `RuntimeBridgeHandler` (app 模块) 需要访问它们。`internal` 在 Kotlin 中是模块级可见性，跨模块 (amitia-runtime → app) 无法访问。

**解决方案**: 移除这些声明中的 `internal` 修饰符。

### 修复3: ABI 过滤

**问题**: 初始构建的 APK 包含 arm64-v8a、armeabi-v7a、x86_64 三种 ABI 的 native libraries。

**解决方案**: 
1. 在 `build.gradle.kts` 的 `packaging.jniLibs` 中添加 `excludes.add("lib/armeabi-v7a/**")` 等
2. 使用 `--target-platform android-arm64` 约束 Flutter 引擎目标

### 修复4: PRoot 符号 strip

**问题**: Gradle 默认 strip native 库符号，导致 APK 中的 PRoot SHA256 与 Step 6 冻结输入不匹配。

**解决方案**: 在 `packaging.jniLibs.keepDebugSymbols.add("**/libamitia_proot.so")` 保留 PRoot。

---

## Build Record

完整的构建记录位于:
- `android/app/build/outputs/candidate/candidate-build-record.json`
- `android/app/build/outputs/candidate/SHA256SUMS`

---

## 最终判定

| 类别 | 结果 |
|------|------|
| ABI 独占 (arm64-v8a only) | PASS |
| PRoot 完整性 (SHA256 match) | PASS |
| Runtime Package 完整性 (SHA256 match) | PASS |
| PRoot 数量 = 1 | PASS |
| Runtime Package 数量 = 1 | PASS |
| 静态审计: x86 ABI | PASS |
| 静态审计: 动态下载 | PASS |
| 静态审计: Debug bypass | PASS |
| 静态审计: Token 泄漏 | PASS |
| 静态审计: 重复组件 | PASS |
| **总体** | **PASS** |

---

## 后续步骤

Step 19-21 只能验证此冻结 Candidate，不得重建：
- Step 19: 静态验证 APK 内容
- Step 20: 真实设备或模拟器启动测试
- Step 21: 运行时完整性校验
