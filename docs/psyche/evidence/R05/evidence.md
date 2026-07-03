# R05 证据：阶段门G0 基线与治理验收

**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 基线验证
- SHA-256: 92d4959ca48c329504c5904e6102201a8c8e2abe378efa21037b23ccd041fea3
- 代码基线: Amitia-develop (6).zip
- 83项审计问题已全部映射到主修步骤

## 边界守卫
- scripts/audit/check_forbidden_changes.ps1: 禁止修改release/WorkDone/server.exe
- scripts/audit/allowed_dirs.json: 允许修改目录清单
- AGENTS.md: 已更新修复边界约束

## 证据完整性
- R01-R04证据已就绪
- 83项问题覆盖矩阵: docs/psyche/repair-matrix.md
- 问题台账: docs/B-R-closure-check.md, docs/psyche/known-defects.md

## 验收
✅ release无直接修改
✅ 83项问题全部有主修步骤
✅ 基线哈希与当前代码一致
