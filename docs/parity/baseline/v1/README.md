# B8 Parity Baseline v1.0.0

**Baseline ID:** PARITY-2026-08-07-V1  
**Status:** FROZEN  
**Generated:** 2026-08-07T14:00:00+08:00  
**Source:** B7三方能力映射结果

## 概述

本目录包含B8阶段冻结的Parity Baseline全集，共28个文件。基于B7生成的506个MAP候选能力组，确定了每个能力的Scope State、验收Profile、验收维度与最低证据等级。

## 能力统计

| 分类 | 数量 |
|------|------|
| 总计MAP组 | 506 |
| REQUIRED | 262 |
| REQUIRED_PLATFORM_EQUIVALENT | 4 |
| PRESERVE_AMITIA | 240 |
| EXCLUDED | 0 |
| REVIEW_REQUIRED | 0 |

## Profile分布

| Profile | 数量 | 最低证据等级 |
|---------|------|-------------|| PROFILE_TOOL | 371 | L4_AUTOMATED_TEST |
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

## 文件清单

| # | 文件名 | 说明 |
|---|--------|------|
| 1 | parity_baseline.json | 冻结的完整Baseline(主文件) |
| 2 | parity_capability_catalog.json | 能力目录全集 |
| 3 | parity_capability_matrix.md | 能力矩阵(可读格式) |
| 4 | inclusion_decisions.json | 纳入决策记录 |
| 5 | exclusion_decisions.json | 排除决策记录 |
| 6 | review_resolution.json | Review决议记录 |
| 7 | amitia_preservation_baseline.json | Amitia保留能力基线 |
| 8 | operit_required_baseline.json | Operit所需能力基线 |
| 9 | openminis_required_baseline.json | OpenMinis所需能力基线 |
| 10 | shared_required_baseline.json | 双方共用所需能力基线 |
| 11 | platform_equivalence_rules.json | 平台等价规则 |
| 12 | platform_requirement_matrix.json | 平台需求矩阵 |
| 13 | acceptance_profiles.json | 验收Profile定义 |
| 14 | acceptance_dimension_schema.json | 验收维度Schema |
| 15 | evidence_level_schema.json | 证据等级Schema |
| 16 | source_evidence_schema.json | 来源证据Schema |
| 17 | test_evidence_schema.json | 测试证据Schema |
| 18 | runtime_evidence_schema.json | 运行时证据Schema |
| 19 | capability_acceptance_contracts.json | 能力验收合同 |
| 20 | completion_formula.json | 完成率计算公式 |
| 21 | regression_policy.json | 防回归政策 |
| 22 | baseline_version.json | 基线版本信息 |
| 23 | upstream_delta_policy.json | 上游差异策略 |
| 24 | B8_summary.json | B8执行摘要 |
| 25 | B9_input_manifest.json | B9输入清单 |
| 26 | verification.log | 验证日志 |
| 27 | README.md | 本文件 |
| 28 | B8_Parity_Baseline冻结报告.md | B8冻结报告(主文档) |

## 验收维度

- **F** = Functional Behavior (功能行为)
- **C** = Call Chain (调用链)
- **I** = Integration (集成)
- **P** = Permission (权限)
- **E** = Error Handling (错误处理)
- **R** = Recovery (恢复)
- **T** = Automated Test (自动化测试)
- **D** = Device/Product Runtime (设备/产品运行时)
- **S** = Security (安全)
- **X** = Cross-platform Consistency (跨平台一致性)

## 证据等级

| Level | 名称 | 说明 |
|-------|------|------|
| L0 | Declaration | 仅声明存在 |
| L1 | Implementation | 代码已实现 |
| L2 | Registered | 已注册到系统 |
| L3 | Integrated | 通过集成测试 |
| L4 | Automated Test | 通过自动化测试套件 |
| L5 | Runtime Verified | 真机/产品环境验证通过 |

## 平台覆盖

- Android (来自Operit)
- iOS (来自OpenMinis)
- Web (来自Amitia)
- Desktop (来自Amitia)
- Flutter (来自Amitia)
- Backend (来自Amitia)

## 下一步

下一阶段为**B9**，执行能力验收测试并填充证据等级。
输入清单见 B9_input_manifest.json。
