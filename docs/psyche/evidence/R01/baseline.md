# R01 基线证据

**生成时间：** 2026-07-03
**代码基线：** 02dd1d79
**修复步骤：** R01

## 环境版本

| 组件 | 版本 |
|---|---|
| Go | go1.26.1 windows/amd64 |
| Node | v22.12.0 |
| SQLite | 3.44.3 |
| 操作系统 | Windows |

## Git状态

- HEAD: 02dd1d79230d94c36910f4512eb2b692a68c5bd1
- 最近提交:
  1. 02dd1d79 fix: 修复 defineModel const 声明导致的编译警告; 同步附属修改
  2. a8adbedd fix runtime persistence and health gaps
  3. b75714d3 fix interaction scopes and profile bounds
  4. 6ebf2dda fix interaction cancellation race outcomes
  5. 84b02425 fix chat prompt ir integration and startup migration
- 未跟踪文件: debug_check.go, debug_test.go
- 修改但未暂存: release/server.exe

## 数据库基线

- 表数量: 56
- 索引数量: 105
- 迁移记录数: 11
- PRAGMA integrity_check: ok
- PRAGMA foreign_key_check: 无违规

### 关键表行数

| 表名 | 行数 |
|---|---|
| messages | 60 |
| memories | 5 |
| conversations | 2 |
| characters | 1 |
| interaction_records | 0 |
| interaction_outbox_records | 0 |
| deletion_tombstones | 0 |
| psyche_states | 0 |
| relationship_states | 0 |

## 功能开关状态

- Qdrant: 存在(backend/qdrant目录)
- SurrealDB: 存在(backend/surrealdb目录)
- 微信: 存在(wechat-chat-extractor目录)
- QQ: 存在(backend/qq-sidecar目录)
- 语音: 存在(backend/internal/realtime目录)
- 主动消息: 存在(backend/internal/proactive目录)

## 验证结论

- 测试命令输出格式: 基线已记录
- BLOCKED_BY_ENV项: 无
- 所有关键组件版本已记录
- 数据库完整性已验证
