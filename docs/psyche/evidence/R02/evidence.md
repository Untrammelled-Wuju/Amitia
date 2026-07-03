# R02 修复证据

**生成时间：** 2026-07-03
**修复步骤：** R02 - 恢复V3.1原B/R编号和唯一问题台账

## 执行内容

1. 确认B-01—B-20为V3.1不可重定义原始编号
2. 确认R-01—R-20为V3.1不可重定义原始风险编号
3. 旧全部CLOSED报告标记为HISTORICAL_INVALID，注明失效原因
4. 建立V9审计83项问题映射到原B/R、168步骤和修复步骤
5. 问题状态统一为OPEN/IN_PROGRESS/CODE_DONE/TESTED/ACCEPTED/REOPENED

## 修改文件

- `docs/B-R-closure-check.md` - 重写，旧CLOSED标记为HISTORICAL_INVALID
- `docs/psyche/repair-matrix.md` - 新建，V9 83项问题→修复步骤映射
- `docs/psyche/known-defects.md` - 保持原有B-01—B-20编号不变

## 验证

- B-01—B-20标题与V3.1一致：确认
- 83项审计问题均映射到主修步骤：确认(repair-matrix.md)
- 问题状态从CLOSED回退为OPEN：确认

## 残余风险

- 旧CLOSED证据文件未删除，作为历史记录保留
- 前端B-19问题(TypeScript类型统一)需要前端源码确认
