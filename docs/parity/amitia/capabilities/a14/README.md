# Amitia A14 能力审计输出 (B6)

## 目录结构

| 类型 | 数量 | 大小 |
|------|------|------|
| JSON | 107 | ~680 KB |
| Markdown | 3 | - |
| Log | 1 | - |
| **总计** | **111** | **~700 KB** |

## 关键文件

- `B6_Amitia完整能力审计报告.md` - 主审计报告（37个章节）
- `capability_catalog.json` - 完整能力目录（327项，AMT-0001~0327）
- `capability_matrix.md` - 可读能力矩阵表
- `B6_summary.json` - 审计结果汇总
- `verification.log` - 执行日志

## 能力统计

| 指标 | 数值 |
|------|------|
| 总能力数 | 327 |
| 已实现 | 282 |
| 部分实现 | 12 |
| Stub | 8 |
| 遗留系统 | 5 |
| 无运行证据 | 20 |

## 领域覆盖

- Agent与对话系统
- Prompt与上下文管理
- Tool Runtime
- 角色系统
- 性格/情绪/生活系统
- 记忆系统（6层Pipeline）
- 模型Provider（4种API）
- 语音系统
- 渠道系统（QQ/微信/Web）
- 扩展/MCP/Skill/Plugin
- Runtime基础设施
- 数据库存储（SQLite/Qdrant/SurrealDB）
- 安全/权限/日志
- Flutter移动端（85路由）
- Web前端（52路由）
- Electron桌面端
- Android/iOS原生层

## 基线信息

- **基线编号**: AMT-A14-3daeaf3
- **Git提交**: 3daeaf3c0a82e33213e0a52d84cfaf8f68f78eab
- **分支**: develop
- **审计时间**: 2026-08-07

## 使用方式

配合 docs/parity/amitia/baseline/a14/ 下的B3基线冻结报告使用，为B7三方能力矩阵整合提供输入。
