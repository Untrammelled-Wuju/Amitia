# B9P7: B10-B154系统复用矩阵与施工边界冻结

## 任务定位
基于B9P1-B9P6已经确认的Canonical系统和B9P4映射，将B10-B154每一步分类为明确的施工模式（REUSE/EXTEND/ADAPTER_ONLY/NEW_PROVIDER/MIGRATION_ONLY/INTEGRATION_ONLY/VALIDATION_ONLY/FORBIDDEN_DUPLICATE），冻结后续施工边界。

## 前置补丁状态
- B9P1-B9P6: 全部PASS

## 核心输出
- `step_reuse_matrix.json` - 145步完整复用矩阵（最核心文件）
- `step_reuse_matrix.md` - 复用矩阵Markdown版
- `step_dependency_corrections.json` - 前置条件修正（B9→B9P8）
- `step_responsibility_corrections.json` - 施工责任修订
- `step_forbidden_actions.json` - 每步禁止动作清单
- `step_canonical_targets.json` - 每步Canonical Target路径
- `rewritten_batch_plan.md` - 修订版批次施工摘要
- `future_prompt_guard.json` - 未来执行Prompt Guard
- `parallel_execution_matrix.json` - 并行/串行执行规则

## 施工模式分布
- EXTEND: 72步
- NEW_PROVIDER: 51步
- REUSE: 1步
- ADAPTER_ONLY: 3步
- MIGRATION_ONLY: 2步
- INTEGRATION_ONLY: 5步
- VALIDATION_ONLY: 11步

## 关键修订原则
1. 现有Extension Kernel为唯一Canonical底座
2. 禁止创建任何平行系统
3. 迁移步骤必须单向(Legacy→Canonical)
4. 所有步骤前置条件修正为"B9P8 PASS"

## 后续步骤
B9P7完成后，B9P8负责总验收+冻结最终Resolved Manifest。
