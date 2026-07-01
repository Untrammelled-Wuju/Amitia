# 幂等数据库迁移框架

对应实施计划：V3.1 第5步。

## 目标

把后续新增表、列、索引和回填纳入可审计版本管理。迁移框架必须显式检查 SQLite 当前结构，不通过吞掉重复 DDL 错误来伪装幂等。

## 当前实现

新增 `backend/internal/migration/runner.go`：

| 能力 | 说明 |
|---|---|
| `schema_migrations` | 记录 version、name、checksum、status、error_message、started_at、finished_at |
| `Runner.Apply` | 按顺序执行迁移，已 applied 的版本跳过 |
| `Step.AddColumn` | 执行前通过 `PRAGMA table_info` 检查列是否存在 |
| `Step.CreateIndex` | 执行前查询 `sqlite_master` 检查索引是否存在 |
| 事务边界 | 每个迁移在事务中构建并执行，失败时回滚 DDL 变化 |
| 失败记录 | 失败迁移写入 `schema_migrations`，状态为 `failed` |

`backend/data/sql.sql` 已加入 `schema_migrations` 表，保证新安装也拥有迁移记录表。

## 验收测试

执行：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./internal/migration'
```

覆盖：

- 新库首次执行迁移。
- 旧库通过 `AddColumn` 升级。
- 重复执行不会重复 DDL。
- 失败迁移回滚并记录 failed。
- 非法表名和列名被拒绝。

## 后续接入

第5步先建立框架与测试。启动时接入真实迁移列表需要在第6步统一服务装配前谨慎完成，并且必须先调用第4步的迁移前备份安全模块。

