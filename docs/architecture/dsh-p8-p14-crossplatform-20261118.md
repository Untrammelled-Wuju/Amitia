# DSH 兼容插件：P8 历史回滚、P14 事务恢复与残留审计、跨平台能力矩阵（2026-11-18）

目标版本 `@deepseek-ai/dsh@0.2.2-rc.2`。本文只回答计划文档中编号 P8、P14 与「跨平台能力矩阵」三项，全部结论附文件行号或命令输出；本轮未改 `plugins-code/dsh-compatible-host/src/**`、未改后端业务逻辑、未重启服务、未安装插件、未做任何 Git 操作。

环境事实（本轮实测）：

```text
$ uname -m && sw_vers -productVersion
arm64
26.2
$ ps -Ao pid,comm | grep -iE "amitiacore|Electron"
9660 …/desktop/node_modules/.pnpm/electron@43.0.0/node_modules/electron/dist/Electron.app/Contents/MacOS/Electron
10058 …/desktop/resources/core/macos-arm64/AmitiaCore
$ adb devices
List of devices attached
```

即：桌面 macOS arm64 在运行（未重启），无 Android 设备在线，Windows x64 未构建未运行。

## 一、结论摘要

| 项 | 结论 | 等级 |
| --- | --- | --- |
| P8 历史回滚 | 宿主扩展的历史版本回滚**已存在且持久**（`/api/extensions/versions/rollback*` + `extension_package_rollback_points`，实测 306 个回滚点、0 悬空）；缺的是 DSH **子包**（`plugin_data/dsh-tree`）的历史版本与备份覆盖，且该项不能在允许改动范围内补齐 | 复用既有链路 + 明确缺口 |
| P14 事务恢复 | 真实恢复链是 `package_*_saga` + `RecoverPackageOperations`；本轮跑了 5 个恢复用例（3 个新增、2 个既有）全部通过。真实数据里存在 1 条**永不终结**的 `requires_recovery` 陈旧操作，已在日志中重复 209 次报错 | 已执行演练 + 发现 1 个真实缺陷 |
| 跨平台矩阵 | `conversation.*`、`skill.list`、`directory.list/create`、plugin-host/task-host 子进程在 core 内**平台同源**（无 GOOS 分支）；`directory.pick` 三平台均缺；差异集中在 Node 二进制布局、扩展 UI 入口类型（web_module vs mobile_web_runtime）与目录选择实现位置 | macOS 有代码级证据；Windows/Android 仅代码级，无实机 |

## 二、P8：DSH 插件历史回滚

### 2.1 计划原文要求

`docs/architecture/dsh-plugin-compatibility-development-plan-20261113.md:702-727`：包管理器需支持「启用、停用、更新、回滚和卸载」，验收含「更新失败可恢复原状态」；当前状态原文为「基础安装、路径校验、staging、原子切换和失败保留旧版本已完成；可持久恢复的历史版本回滚仍未完成」。

### 2.2 项目里已有的三条候选链路

| 链路 | 代码位置 | 覆盖对象 | 持久性 | 能回滚吗 |
| --- | --- | --- | --- | --- |
| A. 扩展包代次 + 回滚点 | `backend/internal/extension/kernel/package_generation_store.go:97,195,261,570`、`package_rollback_preview.go`、`package_rollback_uninstall_saga.go`、`internal/extension/package_distribution_api.go:29-31` | 整个 Amitiax 扩展（含 DSH 宿主本身）的版本、定义、贡献、配置 | `extension_package_rollback_points`（SQLite，`persistence/sqlite/schema.go:1764`）+ `installations/<ext>/generations/<generation-id>` | 是，preview → confirm → execute 三段式 |
| B. 插件数据备份 | `backend/internal/extension/plugin_data_backup.go:62-104,290-326,370-407`（组件 `extension.plugin_data.v1`） | 扩展 `data.pluginData.include` 声明的目录 | 随全量备份归档（`.amitia-backup`） | 只能整体恢复备份，不能单包回滚 |
| C. DSH 子包安装器 | `plugins-code/dsh-compatible-host/src/runtime/package-manager.mjs:212-341` | `plugin_data/dsh-tree`（npm workspace） | 仅 `package.json` + `package-lock.json` | 否：失败时只回写 `package.json`（:266-271），无历史版本目录、无 staging、无原子切换 |

### 2.2.1 另外三条被点名的能力，为什么不选

| 能力 | 实测/代码事实 | 能否承载「DSH 插件回滚」 |
| --- | --- | --- |
| 数据可携带（`backend/internal/system/dataportability/`） | 路由齐全：`GET/POST /data-portability/backups`、`POST /data-portability/backups/:name/restore/preview`、`POST /data-portability/restores/:operationId`、`DELETE …`（`backend/internal/system/data_portability_routes.go:31-37`）；实现含 `staging.go`、`staged_import.go`、`reconciliation.go`、`verification.go` | 只能做**整体备份/整体恢复**，粒度不是「某个扩展的某个版本」；扩展插件数据是挂在它下面的一个组件（`extension.plugin_data.v1`） |
| `AmitiaData/backups` | 实测 3 个归档：`backup-202bb439….amitia-backup` 15418 字节、`backup-e566bede….amitia-backup` 14583 字节、`backup-fd676cb0….amitia-backup` 15475 字节（2026-11-12） | 是整应用级备份产物，不能当版本回滚点（且实测都早于 DSH 子包安装） |
| device-mesh 业务写通道 | `Engine.Edit` 只做 `ExpectedRevision` 冲突检测（`backend/internal/devicemesh/business/edit.go:34`，`ExpectedRevision < 1` 直接拒绝）；`ReadResource` 对 conversation 取不到记录（见本计划 23.31） | 不保留任何历史副本，revision 只是单调计数器，不能作为回滚存储 |

### 2.3 结论：DSH 插件回滚应复用 A（宿主扩展）+ B（插件数据），不要新造第三套

理由（可核验）：

1. A 链路对本项目实际安装的 DSH 宿主**已经生效**，数据就在库里：

```text
$ sqlite3 AmitiaData/extensions-v2/kernel.db "SELECT extension_id, count(*), min(source_version), max(source_version) FROM extension_package_rollback_points GROUP BY extension_id;"
com.amitia.qa/desktop-pet-plugin|6|1.0.5|1.0.7
com.amitia/dsh-compatible-host|306|0.1.100|0.1.99
com.amitia/plugin-market|1|1.4.4|1.4.4
```

注意第二行的 `0.1.100 / 0.1.99` 是 TEXT 列的字典序，不是版本序；按 `created_at` 排序的真实区间是 `0.1.23 … 0.1.313`（见 2.3 第 2 点与审计脚本的 `versions=` 字段）。写文档或做门禁时不能用 SQL 的 `min/max` 判断最新回滚点。

```text
$ sqlite3 -header -column AmitiaData/extensions-v2/kernel.db "SELECT source_version, retention_state, retention_until, substr(snapshot_hash,1,12) AS snap, installed_path FROM extension_package_rollback_points WHERE extension_id='com.amitia/dsh-compatible-host' ORDER BY created_at DESC LIMIT 2;"
source_version  retention_state  retention_until              snap          installed_path
0.1.313         active           2026-12-17T15:25:56.193509Z  sha256:2b1ff  …/generations/generation-b698451b-cc95-4278-83da-ad4ac8c07803
0.1.312         active           2026-12-17T15:24:00.532445Z  sha256:89374  …/generations/generation-89a6ca25-6f5c-4ccf-b989-0f15a523c0e5
```

2. 回滚点指向的代次目录**没有悬空**（回滚不会因为树缺失而失败）：

```text
$ python3 scripts/audit/dsh-plugin-residual-audit.py   # rollback points 段
com.amitia.qa/desktop-pet-plugin       points=6 active=6 versions=1.0.5..1.0.7 dangling=0
com.amitia/dsh-compatible-host         points=306 active=306 versions=0.1.23..0.1.313 dangling=0
com.amitia/plugin-market               points=1 active=1 versions=1.4.4..1.4.4 dangling=0
```

3. 回滚点是**元数据 + 代次路径**，不含插件数据文件（`resource_snapshot_json` 仅 14 字节、`migration_state_snapshot_json` 13 字节），因此回滚 A 不会带走 `plugin_data`；`plugin_data` 由 B 负责。

4. 回滚入口在生产路由里已注册：

```text
backend/internal/extension/package_distribution_api.go:29  group.POST("/versions/rollback/preview", …)
backend/internal/extension/package_distribution_api.go:30  group.POST("/versions/rollback/confirm", …)
backend/internal/extension/package_distribution_api.go:31  group.POST("/versions/rollback", …)
backend/internal/extension/router.go:27                   extensions := group.Group("/extensions")
backend/cmd/server/router.go:741                          extension.RegisterRouter(apiGroup, …)  // apiGroup = r.Group("/api")
```

即 `POST /api/extensions/versions/rollback/preview|confirm|rollback`。

### 2.4 缺口（有证据，未修）

| 缺口 | 证据 | 影响 |
| --- | --- | --- |
| DSH 子包目录 `dsh-tree` 不在插件数据备份范围 | `plugins-code/dsh-compatible-host/amitia-extension.json:2949-2953` 只有 `include: ["dsh-data"]`；`plugin_data_backup.go:250-268` 的 `pluginDataPathDeclared` 对不在 include 前缀下的路径直接拒绝 | 备份/恢复整条链路会跳过所有已安装的 DSH 子包（实测 `dsh-tree` 有 43 个顶层 node_modules 项、依赖 100+ 个 `@deepseek-ai/dsh-*` 包） |
| 子包没有版本历史 | `ls -a plugin-data/com.amitia__dsh-compatible-host/dsh-tree` = `.amitia-npm  node_modules  package-lock.json  package.json`（无 snapshots/backup 目录）；`package-manager.mjs` 无 staging/原子切换 | 子包更新失败只能回写 `package.json`，`node_modules` 可能半更新 |
| 子包回滚需要插件侧状态同步 | 插件自身安装记录在 `dsh-data/storage`，只回滚 npm 树会与记录不一致；本轮禁止改 `src/**` | 不能在宿主侧单方面补齐 |
| 回滚点不覆盖 `plugin_data` | 见 2.3 第 3 点 | 回滚宿主版本后，子包与 `dsh-data` 保持新状态（需人工判断兼容性） |

### 2.5 下一步（按可执行顺序）

1. 把 `plugins-code/dsh-compatible-host/amitia-extension.json` 的 `data.pluginData.include` 从 `["dsh-data"]` 扩到 `["dsh-data", "dsh-tree/package.json", "dsh-tree/package-lock.json"]`，只备份清单文件而不备份 `node_modules`（避免 512 MiB 上限与巨量文件）。改完必须重新打包并重装插件才能生效，本轮按约束未做。
2. 子包级历史回滚放到插件侧实现（记录 `dsh-data/storage` 的已安装版本 + `package-lock.json` 快照，回滚时按快照 `npm ci`），并在 `host-services.mjs` 暴露为一个可失败的显式动作，不允许空成功。
3. 宿主版本回滚的验收补一条「回滚后子包兼容性判定」：读 `dsh-data` 里的子包版本，与回滚目标的 `dsh` ABI Catalog 比对，不一致时明确失败而不是静默启动。

## 三、P14：事务恢复与残留审计

### 3.1 项目里存在两套 journal，只有一套在生产链路上

| 系统 | 表 | 本轮实测行数 | 是否有执行入口 |
| --- | --- | --- | --- |
| A. `update` 包 journal（`JournalManager`/`RollbackExecutorV2`/`RecoveryManager`） | `extension_lifecycle_journal`、`extension_rollback_plans`、`extension_rollback_steps`、`extension_side_effect_assessments` | 0 / 0 / 0 / 0 | 只有 `GET /api/extensions/recovery/scan`（`backend/internal/extension/update_api.go:37,100-112`）与 `GET /rollbacks*`、`GET /journal/:operationId`，**没有 ExecuteRecovery 路由** |
| B. 包安装/更新/回滚 saga + `RecoverPackageOperations` | `extension_package_operations`、`extension_package_operation_steps`、`extension_package_operation_leases`、`package_consistency_findings` | 328 / 3462 / 0 / 0 | 启动时自动执行（`backend/cmd/server/services.go:920`、`internal/extension/runtime.go:45`） |

结论：P14 的「事务恢复」就是 B；A 是并行存在但从未写入的死链（0 行），且 `ExecuteRecovery` 在 `executor == nil` 时会「跳过回滚但仍写 completed 步骤」（`update/recovery.go:171-183`），具备假成功风险，不应作为 P14 的验收对象。

### 3.2 恢复演练（已真实执行）

#### 3.2.1 既有故障矩阵（模拟「写 journal 后崩溃」）

```text
$ cd backend && AMITIA_RUN_PACKAGE_FAULT_MATRIX=1 go test ./internal/extension/kernel -run 'TestPackageFaultMatrixRealJournalCracksRecoverDeterministically' -v
=== RUN   TestPackageFaultMatrixRealJournalCracksRecoverDeterministically
=== RUN   TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/install
=== RUN   TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/update
=== RUN   TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/rollback
=== RUN   TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/uninstall
--- PASS: TestPackageFaultMatrixRealJournalCracksRecoverDeterministically (1.94s)
    --- PASS: TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/install (0.59s)
    --- PASS: TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/update (0.58s)
    --- PASS: TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/rollback (0.35s)
    --- PASS: TestPackageFaultMatrixRealJournalCracksRecoverDeterministically/uninstall (0.43s)
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel	2.937s
```

该用例做的是：建一条 `in_progress` 且无已完成步骤的操作 → `RecoverPackageOperations` 必须报错并把它置为 `requires_recovery`/`recovery_manual` → 再次恢复必须得到同一结果且**步骤数不增长**（重放不产生重复副作用）。判定点见 `package_fault_matrix_test.go:88-98`（第二次恢复必须同状态同步骤数）。

#### 3.2.2 新增演练（指针原子切换被打断）

新增文件 `backend/internal/extension/kernel/p14_transaction_recovery_drill_test.go`，用真实的 `PackageGenerationStore` 做三件事：在 `current.json` 删除后崩溃、恢复后拒绝陈旧 CAS、崩溃残留的 staging 只保留不属于本次操作的那份。

```text
$ cd backend && go test ./internal/extension/kernel -run 'TestP14|TestRecoverPackageOperations' -v
=== RUN   TestP14GenerationPointerCrashRecoversPreviousGeneration
--- PASS: TestP14GenerationPointerCrashRecoversPreviousGeneration (0.10s)
=== RUN   TestP14StaleGenerationSwitchIsRejectedAfterRecovery
--- PASS: TestP14StaleGenerationSwitchIsRejectedAfterRecovery (0.09s)
=== RUN   TestP14InterruptedPrepareLeavesOnlyItsOwnStaging
--- PASS: TestP14InterruptedPrepareLeavesOnlyItsOwnStaging (0.02s)
=== RUN   TestRecoverPackageOperationsScansIncompleteWithClaimsAndHash
--- PASS: TestRecoverPackageOperationsScansIncompleteWithClaimsAndHash (0.34s)
=== RUN   TestRecoverPackageOperationsPreservesUnownedStaging
--- PASS: TestRecoverPackageOperationsPreservesUnownedStaging (0.37s)
PASS
ok  	github.com/u-ai/backend/internal/extension/kernel	1.920s
```

三个新用例各自证明的语义（对应 `package_generation_store.go` 的实现点）：

| 用例 | 模拟的中断 | 恢复行为 | 残留检查 |
| --- | --- | --- | --- |
| `TestP14GenerationPointerCrashRecoversPreviousGeneration` | `current.json` 已被删除、新指针未写（:691-760 的两步替换的中间态） | `ReadCurrent` 回落到 `previous.json` 并重新落盘（:617-689），`ResolveCurrentGeneration` 校验树哈希通过 | `generations/` 仍为 2 个目录（指针崩溃不丢树）、`staging/` 为空 |
| `TestP14StaleGenerationSwitchIsRejectedAfterRecovery` | 恢复后又收到基于旧值 `SwitchCurrent` | 返回 `ErrPackageGenerationCAS`（:295-300），`current.json` 不变 | 无指针污染 |
| `TestP14InterruptedPrepareLeavesOnlyItsOwnStaging` | `PrepareGeneration` 已建 staging 但未 commit | 同请求重入复用同一 staging（:152-166），`CommitGeneration` 后搬走 | 只剩一个**不属于本次操作**的 staging 目录，未被误删（对应 `TestRecoverPackageOperationsPreservesUnownedStaging` 的既有约束） |

### 3.3 残留审计（已真实执行）

审计脚本：`scripts/audit/dsh-plugin-residual-audit.py`（只读打开 `kernel.db`，不改任何状态）。

```text
$ python3 scripts/audit/dsh-plugin-residual-audit.py
extensions_root=/Users/untrammelled/Desktop/Project/U-AI/AmitiaData/extensions-v2
kernel_db=/Users/untrammelled/Desktop/Project/U-AI/AmitiaData/extensions-v2/kernel.db

== package operations ==
status completed          302
status failed             25
status requires_recovery  1
incomplete_operations=1
  INCOMPLETE package-operation-e786b4d3-bd3f-434f-91ba-e254028f7b0b ext=com.amitia/dsh-compatible-host 0.1.83->0.1.84 status=requires_recovery step=recovery_manual recovery_required=1 error=PACKAGE_RECOVERY_REQUIRED updated=2026-11-18T05:09:49.753835Z

== package operation steps ==
status completed          3462
non_terminal_steps=0

== package operation leases ==
lease_rows=0

== update package lifecycle journal ==
extension_lifecycle_journal          rows=0
extension_rollback_plans             rows=0
extension_rollback_steps             rows=0
extension_side_effect_assessments    rows=0
started_journal_operations=0

== consistency findings ==
findings=0 unresolved=0

== installation pointers ==
com.amitia/dsh-compatible-host         db_version=0.1.314   db_generation=generation-ab60876e-ac82-4031-8fb9-09f727534802 pointer=generation-ab60876e-ac82-4031-8fb9-09f727534802 match
    pointer_version=0.1.314   previous_version=0.1.313   generations=292 staging=[]
com.amitia/plugin-market               db_version=1.4.5     db_generation=generation-e5e5bb3e-f1c2-4539-8efe-e0b72c731bb7 pointer=generation-e5e5bb3e-f1c2-4539-8efe-e0b72c731bb7 match
    pointer_version=1.4.5     previous_version=1.4.4     generations=2 staging=[]

== dsh container plugin data ==
dsh-tree present=True
dsh-data present=True
dsh-tree lockfile present=True
dsh-tree node_modules top-level=43

== dsh manifest plugin data include ==
data.pluginData.include=["dsh-data"]
platforms=["windows", "linux", "macos", "android"]

== summary ==
findings=1
  incomplete_operation:package-operation-e786b4d3-bd3f-434f-91ba-e254028f7b0b
```

（完整输出 111 行，`exit_code=1`；上表按段落摘录，省略了 23 个 builtin 扩展的无代次行。）

### 3.4 真实缺陷：陈旧 `requires_recovery` 操作永不终结

这条 2026-11-15 的 0.1.83→0.1.84 更新，**8 个持久化步骤全部 completed**：

```text
$ sqlite3 -header -column AmitiaData/extensions-v2/kernel.db "SELECT step_name, status, started_at FROM extension_package_operation_steps WHERE operation_id='package-operation-e786b4d3-bd3f-434f-91ba-e254028f7b0b' ORDER BY started_at;"
step_name                           status     started_at
confirmation.authority_evidence     completed  2026-11-15T07:08:36.220918Z
update.validate_and_diff            completed  2026-11-15T07:08:36.22658Z
update.create_rollback_point        completed  2026-11-15T07:08:36.233898Z
update.commit_target_generation     completed  2026-11-15T07:08:36.553584Z
update.quiesce_previous_generation  completed  2026-11-15T07:08:36.706092Z
update.execute_migrations           completed  2026-11-15T07:08:36.706729Z
update.switch_current_pointer       completed  2026-11-15T07:08:36.736307Z
update.commit_update_state          completed  2026-11-15T07:08:36.741863Z
```

但操作记录停在 `requires_recovery`，错误原文是「当前代次与目标代次不一致」：

```text
error_detail = update generation reconciliation failed: unexpected current generation generation-ab60876e-ac82-4031-8fb9-09f727534802
stable_generation = generation-7be0f557-f62b-40de-b237-61164f1a3f5b
target_generation = generation-6a1e7649-6f93-418a-9c46-f00264353786
```

原因是 `stable_generation`/`target_generation` 停留在 0.1.84 那一次，而实例此后已滚到 0.1.314（`current_generation_id=generation-ab60876e…`）。恢复器每次启动都会重新判定失败，日志里已经累积 209 条同源 error：

```text
$ for f in AmitiaData/logs/electron.2026111*.log; do echo "$f $(grep -c 'package operation recovery failed' $f)"; done
AmitiaData/logs/electron.20261115.002.log 51
AmitiaData/logs/electron.20261115.003.log 4
AmitiaData/logs/electron.20261116.001.log 85
AmitiaData/logs/electron.20261116.002.log 5
AmitiaData/logs/electron.20261116.003.log 33
AmitiaData/logs/electron.20261117.001.log 12
AmitiaData/logs/electron.20261117.002.log 1
AmitiaData/logs/electron.20261117.003.log 0
AmitiaData/logs/electron.20261117.004.log 17
AmitiaData/logs/electron.20261118.001.log 1
```

最近一条（含原始错误与随后降级）：

```text
[2026-11-18T05:09:49.754Z] … {"@level":"error","@message":"package operation recovery failed: package-operation-e786b4d3-…: update generation reconciliation failed: unexpected current generation generation-ab60876e-…","@timestamp":"2026-11-18 13:09:49"}
[2026-11-18T05:09:49.754Z] … {"@level":"warning","@message":"Core continuing with package operations quarantined for recovery","@timestamp":"2026-11-18 13:09:49"}
```

判定：

1. **正面结论**：Core 启动不被残留操作阻塞（`services.go:920-923` 只记 error + warn 后继续），符合 P14 验收第 1 条「容器崩溃不影响核心启动」。
2. **缺陷**：恢复器没有「操作已被后续更新超越」的判定与终结路径，导致 (a) 每次启动重复报错，(b) `extension_package_operations` 永久留一条非终态记录，(c) 任何「无残留」验收都无法通过。
3. **修复方向**（需改后端业务逻辑 + 重建 Core + 重启，本轮按约束未做）：在 `recoverPackageOperation` 里先做「代次已超越」判定——若该操作的 `target_generation` 不在当前 `current/previous` 且所有步骤 completed，则按 `superseded` 终结（保留原始 error_detail 到操作记录，不吞错），而不是继续判 `requires_recovery`。

### 3.5 P14 下一步

1. 实现 3.4 的 `superseded` 终结判定，并为它补一个「已超越操作 → 终结 → 二次扫描无残留」的用例。
2. 把 `scripts/audit/dsh-plugin-residual-audit.py` 接入扩展内核自检（当前是独立只读脚本），让「非终态操作 / 非终态步骤 / 租约 / 指针不一致 / 悬空回滚点 / staging 残留」成为可回归的失败项。
3. 清理 `update` 包那条从未写入的 journal 死链（0 行数据、无 Execute 路由、`executor == nil` 时假成功），避免后续误接到 P14 验收上。

## 四、跨平台能力矩阵

### 4.1 平台基线

| 事实 | 证据 |
| --- | --- |
| 插件声明平台 = windows / linux / macos / android（无 iOS） | `plugins-code/dsh-compatible-host/amitia-extension.json:35-40` |
| 与 Catalog 的 ios `not-declared` 一致 | `plugins-code/dsh-compatible-host/catalog/platform-support.v1.json` |
| 运行时模块只有一个 javascript 入口，全平台同一份代码 | `amitia-extension.json:52`（`"adapter": "javascript"`，`entry: dist/index.js`） |
| UI 入口按平台分派 | `amitia-extension.json:464-505`：`electron_macos`/`electron_windows`/`web` → `web_module` + `modules/dsh-compatible-ui/desktop/index.mjs`；`android`/`ios`/`mobile` → `mobile_web_runtime` + `modules/dsh-compatible-ui/mobile/index.mjs` |
| Android 运行时形态 = PRoot + linux 客户机 | `backend/pkg/platform/android_proot_linux.go:26`（`HostPlatformAndroid, RuntimeKindProot, GuestPlatformLinux`） |
| Node 二进制按平台解析 | `backend/internal/scriptruntime/nodeenv/candidates.go:17-53`（`node.exe` vs `node`，`node.exe.zip` vs `node.zip`）、`:55-104`（runtime 包内 URI 候选） |
| 桌面端打包 plugin-host/task-host | `desktop/electron-builder.yml:28-31` |
| Android Runtime 包含 node/plugin-host/task-host | `device-runtime-manifest.json`（`runtime.backend`、`runtime.node`、`runtime.plugin-host`、`runtime.task-host`，`packageId=amitia.runtime.android`）；`runtime/scripts/build-fresh-runtime-package-android-arm64.py:42-50,239-245,284-305` |

### 4.2 能力 × 平台

“代码级”= 同一份 core 代码路径无平台分支，已在 macOS 运行态可验证；“实机”= 有真实设备/进程运行证据。

| DSH 宿主能力 | macOS arm64 | Windows x64 | Android arm64 | 证据 |
| --- | --- | --- | --- | --- |
| `host.conversation.list/read/search/create/delete` | 可用（代码级） | 可用（代码级） | 可用（代码级，PRoot 内设备本地会话） | `host_api/types.go:58-63`、路由 `host_api_routes.go:2736,2830,2882,4876,4883`；`host_api_routes.go` 内 `runtime.GOOS` 出现 0 次 |
| `host.conversation.message.send/append/turn.interrupt/await` | 可用 | 可用 | 可用 | `host_api/types.go:64-67`、路由 `host_api_routes.go:4855,4862,4869,4890`、`capability_matrix.go:37` |
| `host.conversation.archive/pin` | 可用（已在原生页真实点击验证） | 可用（代码级） | 可用（代码级） | 计划文档 23.33/23.34；`types.go:68-69`、路由 `host_api_routes.go:4897,4911` |
| `host.conversation.scope` | 可用 | 可用 | 可用 | `host_api/types.go:62`、路由 `host_api_routes.go:2788` |
| `host.skill.list` | 可用（实测返回 `{"items":[]}`） | 可用（代码级） | 可用（代码级） | `host_api/types.go:72`、路由 `host_api_routes.go:4904`、处理器 `:1415`（`skillListRouteHandler`）、权限 `permission_mapping.go:66`；实测见计划 23.35 |
| `host.directory.list` | 可用（本机 fs） | 可用（本机 fs） | **语义不同**：解析 PRoot 客户机 fs，非 SAF 文档树 | `host_api/types.go:70`、路由 `host_api_routes.go:4918`、处理器 `:1545`、适配器 `kernel_host_adapters.go:1265`（`os.ReadDir`） |
| `host.directory.create` | 可用 | 可用 | 同上语义差异 | `host_api/types.go:71`、路由 `host_api_routes.go:4925`、处理器 `:1583`、适配器 `kernel_host_adapters.go:1308`（`os.Mkdir`） |
| `host.directory.pick`（交互式选择） | **缺** | **缺** | **缺** | `backend` 全仓无 `host.directory.pick`；插件侧明确失败：`src/runtime/host-services.mjs:363-369` |
| └ 平台内已有的目录选择实现 | Electron 原生对话框（未暴露给扩展） | Electron 原生对话框（未暴露） | SAF 选择器（仅 workspace 层） | `desktop/src/main/ipc-handlers.ts:190,228,284`、`desktop/src/preload/index.ts:52-59`；`backend/internal/workspace/service.go:80-97`、`backend/cmd/server/services.go:363`（仅 `AndroidNativeBridge != nil` 时装配） |
| `host.ui.dialog`（宿主内对话框） | 可用 | 可用（同一渲染进程实现） | 可用 | `front/src/composables/useUIHostSSE.ts:103,171-172`；`mobile_app/lib/core/ui_runtime/mobile_ui_host_event_client.dart:311`、`mobile_app/lib/core/services/extension_service.dart:362` |
| plugin-host 子进程（javascript 适配器） | 可用（运行中：pid 10058 的 Core 已加载宿主扩展） | 可用（代码级） | 可用（代码级，Runtime 包内 linux-arm64 node） | `javascript_main/host.go:394`（`exec.CommandContext`）、`:450-486`（`--permission` 参数构造）、`script_host/resolver.go:172-180`（`amitia://runtime/plugin-host/dist/index.js`） |
| task-host 子进程 | 可用（代码级） | 可用（代码级） | 可用（代码级） | `desktop/electron-builder.yml:31`；`device-runtime-manifest.json` 的 `runtime.task-host`；`task_runtime/process_transport.go:100-135` |
| 子进程沙箱开关 | 三平台一致：宿主 `durableSync: true` → **不加** Node `--permission` | 同左 | 同左 | `amitia-extension.json:62-70`；`javascript_main/host.go:451-453` |
| 扩展 UI 渲染形态 | `web_module`（Vue 渲染进程） | `web_module` | `mobile_web_runtime`（WebView） | `amitia-extension.json:464-505`；`mobile_app/lib/core/ui_runtime/plugin_surface_resolver.dart:17` |
| 已安装代次内实际产出的入口文件 | 桌面 + 移动入口并存 | 同左 | 同左 | `AmitiaData/extensions-v2/installations/com.amitia__dsh-compatible-host/generations/generation-ab60876e-…/modules/dsh-compatible-ui/{desktop,mobile}` |

### 4.3 差异结论

1. core 侧 Host Service **不为平台分叉**（`host_api_routes.go` 内 `runtime.GOOS` 计数为 0），平台差异只出现在三处：Node 二进制布局、扩展 UI 入口类型、目录选择的实现位置。
2. 唯一「三平台都缺」的交互式能力是 `host.directory.pick`；桌面与 Android 各自有实现但都不在扩展 ABI 上，属于「宿主已有、ABI 未开」，与计划 23.28 把 `directoryPicker` 记为 pending 的口径一致。
3. Android 的 `host.directory.*` 与桌面不是同一路径空间（PRoot 客户机 vs 宿主 fs），同一条 DSH 调用在两端可能得到不同结果，属于**语义差异**而非缺失。
4. DSH 宿主在 Android 上走 `mobile_web_runtime`，桌面走 `web_module`，两者代码不同（`src/client-ui` vs `src/mobile-client-ui`），跨平台一致性的验证必须分开做。

### 4.4 未取证项（不得当作已验证）

| 项 | 原因 |
| --- | --- |
| Windows x64 全链路 | 本机为 macOS，未构建未运行；仅有代码级与打包配置证据 |
| Android 真机 DSH 子插件安装 / client runtime 渲染 | `adb devices` 无设备，本轮禁止安装插件 |
| Android 上 `host.directory.*` 对真实存储的可达性 | 需真机验证 PRoot 是否绑定 `/sdcard` 等路径；本轮无设备 |
| macOS 上 task-host 与 DSH 长任务的实机联调 | 本轮禁止重启服务，未构造真实长任务 |
| iOS | 插件未声明该平台，不在本轮范围 |

## 五、复现命令

```bash
# 1) P14 崩溃恢复演练（新增用例 + 既有恢复用例）
cd backend && go test ./internal/extension/kernel -run 'TestP14|TestRecoverPackageOperations' -v
# 2) P14 真实故障矩阵（写 journal 后崩溃 → 恢复 → 重放不重复）
cd backend && AMITIA_RUN_PACKAGE_FAULT_MATRIX=1 go test ./internal/extension/kernel -run 'TestPackageFaultMatrixRealJournalCracksRecoverDeterministically' -v
# 3) 残留审计（只读）
python3 scripts/audit/dsh-plugin-residual-audit.py
# 4) 历史回滚点与悬空检查（审计脚本 rollback points 段）
python3 scripts/audit/dsh-plugin-residual-audit.py | sed -n '/rollback points/,/dsh container/p'
# 5) 确认 core 侧 Host Service 无平台分支
cd backend && grep -c "runtime.GOOS" internal/extension/kernel/host_api_routes.go
```

## 六、本轮变更文件

| 文件 | 类型 | 说明 |
| --- | --- | --- |
| `docs/architecture/dsh-p8-p14-crossplatform-20261118.md` | 新增 | 本文 |
| `docs/architecture/dsh-plugin-compatibility-development-plan-20261113.md` | 追加 23.38 节 | P8/P14/矩阵结论 |
| `backend/internal/extension/kernel/p14_transaction_recovery_drill_test.go` | 新增测试 | 3 个恢复演练用例 |
| `scripts/audit/dsh-plugin-residual-audit.py` | 新增脚本 | 只读残留审计 |

未改动：`plugins-code/dsh-compatible-host/src/**`、任何后端业务逻辑、任何构建产物。
