# Parity Capability Matrix

**Baseline ID:** PARITY-2026-08-07-V1  
**Generated:** 2026-08-07T14:00:00+08:00  
**Status:** FROZEN

## Overview

| Metric | Value |
|--------|-------|
| Total MAP Groups | 506 |
| Required | 262 |
| Platform Equivalent | 4 |
| Preserve Amitia | 240 |
| Excluded | 0 |

## Profile Distribution

| Profile | Count | Min Evidence Level |
|---------|-------|-------------------|| PROFILE_TOOL | 371 | L4_AUTOMATED_TEST |
| PROFILE_NATIVE | 35 | L5_RUNTIME_VERIFIED |
| PROFILE_DATA | 22 | L4_AUTOMATED_TEST |
| PROFILE_EXTENSION | 19 | L4_AUTOMATED_TEST |
| PROFILE_CORE | 13 | L4_AUTOMATED_TEST |
| PROFILE_SECURITY | 12 | L4_AUTOMATED_TEST |
| PROFILE_RUNTIME | 10 | L5_RUNTIME_VERIFIED |
| PROFILE_UI | 9 | L4_AUTOMATED_TEST |
| PROFILE_VOICE | 6 | L5_RUNTIME_VERIFIED |
| PROFILE_BACKGROUND | 5 | L5_RUNTIME_VERIFIED |
| PROFILE_MODEL | 4 | L4_AUTOMATED_TEST |

## Source Distribution

| Source Base | Count |
|------------|-------|
| REQUIRED_FROM_OPERIT | 189 |
| REQUIRED_FROM_OPENMINIS | 65 |
| REQUIRED_FROM_BOTH | 12 |
| PRESERVE_AMITIA | 240 |

## Acceptance Dimensions

| Dim | Name | Description |
|-----|------|-------------|
| F | Functional Behavior | 行为按预期执行 |
| C | Call Chain | 调用链路完整 |
| I | Integration | 上下游集成正确 |
| P | Permission | 权限正确声明 |
| E | Error Handling | 异常处理正确 |
| R | Recovery | 故障恢复能力 |
| T | Automated Test | 自动化测试 |
| D | Device Runtime | 真机/产品环境 |
| S | Security | 安全策略实施 |
| X | Cross-platform | 跨平台一致性 |

## Evidence Levels

| Level | Name | Description |
|-------|------|-------------|
| L0 | Declaration | 仅声明 |
| L1 | Implementation | 已实现 |
| L2 | Registered | 已注册 |
| L3 | Integrated | 已集成 |
| L4 | Automated Test | 自动化测试通过 |
| L5 | Runtime Verified | 真机验证通过 |

