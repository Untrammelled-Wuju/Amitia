# 数据库迁移安全基线

对应实施计划：V3.1 第4步。

## 目标

在任何心理运行时表、状态版本表或作用域迁移执行前，必须先完成 SQLite 备份、完整性检查、关键表计数和恢复演练。备份失败时迁移不得继续。

## 当前实现

新增源码模块：`backend/internal/migration`。

`BackupManager.CreatePreMigrationBackup` 执行以下步骤：

1. 检查源数据库存在且非空。
2. 对源库执行 `PRAGMA integrity_check`。
3. 记录关键表行数。
4. 读取 `schema_version` 和 `user_version`。
5. 使用 `VACUUM INTO` 生成一致性备份副本。
6. 计算备份 SHA256。
7. 打开备份副本执行恢复演练式校验。
8. 写入 `.manifest.json` 元数据。

## 关键表

第4步默认迁移前至少检查：

| 表 | 原因 |
|---|---|
| `characters` | 角色与人格配置 |
| `conversations` | 会话作用域 |
| `messages` | 聊天事实 |
| `memories` | 长期记忆 |
| `memory_events` | 记忆审计 |
| `retrieval_logs` | 检索审计 |
| `user_profiles` | 用户画像 |
| `episodic_memories` | 情景记忆 |
| `proactive_messages` | 主动消息投递记录 |
| `active_message_task` | 主动任务 |

## 验收证据

执行：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./internal/migration'
```

通过条件：

- 能创建备份数据库。
- 能创建 manifest。
- manifest 包含 checksum、schema version、user version、关键表行数。
- 恢复演练通过。
- 非法表名被拒绝。

## 接入边界

本步新增独立迁移安全模块，不改变现有 `/api/storage/backup` 行为。第5步幂等迁移框架接入启动流程时，必须先调用该模块并在失败时阻止迁移。

