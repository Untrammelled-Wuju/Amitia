# R03 修复证据

**修复步骤：** R03 - 撤销release构建产物修改并建立边界守卫

## 执行内容

1. 撤销 release/server.exe 的手工修改（git checkout HEAD）
2. 建立边界守卫脚本 scripts/audit/check_forbidden_changes.ps1
3. 建立允许修改目录清单 scripts/audit/allowed_dirs.json
4. 验证边界守卫通过

## 当前状态

- release/server.exe: 已恢复原始版本
- 边界守卫检查: 通过（无禁止目录修改）
- 当前仅有 docs/B-R-closure-check.md 为合法修改

## 验证

- 模拟修改release/server.exe，CI应失败：边界守卫脚本已就绪
- 模拟修改LICENSE或WorkDone，CI应失败：边界守卫脚本已就绪
- 正式发布任务生成产物应有独立签名：后续R58/R59实现
