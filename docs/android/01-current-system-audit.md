# Amitia 现状系统审计（Phase 0 / Task 0.3）

> 来源：`AndroidAPP/stage.md` 第六节、第二十九节步骤 1-7
> 审计范围：`backend/`（Go 后端）、`front/`（Vue3 Web 前端）、`desktop/`（Electron 桌面端）
> 审计方法：静态扫描源码，所有结论均给出 `file_path:line` 引用，状态严格区分「已真实实现 / 部分实现 / 仅 UI / 后端缺失 / 已废弃 / 无法确认」
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. 后端入口

| 项 | 值 | 引用 |
|---|---|---|
| Go 后端入口包 | `package main` | `backend/cmd/server/main.go:3` |
| 入口函数 | `func main()` | `backend/cmd/server/main.go:58` |
| 模块路径 | `github.com/u-ai/backend` | `backend/go.mod:1` |
| 应用名 | `U-Ai`（默认 `config.AppCfg.App.Name`） | `backend/config/config.yml:14`、`backend/config/config.go:106` |
| 应用版本 | `26.1.7`（`config.AppCfg.App.Version`） | `backend/config/config.yml:15` |
| 部署模式 | `desktop-local`（`config.AppCfg.App.DeployMode`） | `backend/config/config.yml:16` |
| 启动横幅 | `fmt.Printf` 输出后端监听信息 | `backend/cmd/server/main.go:128-136` |

入口流程概览（`backend/cmd/server/main.go:58-260`）：

1. 解析 `runtimeRoot`（`main.go:59`，来自 `util.RuntimeRoot()`）
2. 加载 `CONFIG_PATH` 环境变量并初始化配置（`main.go:61-67`，调用 `config.InitConfig`）
3. 解析 `DataDir` 与 `Surreal.DataPath` 为绝对路径（`main.go:69-70`）
4. 初始化日志（`main.go:72`）
5. 创建 SQLite 数据库并执行启动迁移（`main.go:77-84`）
6. 启动 Qdrant 与 SurrealDB（`main.go:102-104`）
7. 装配 AppServices（`main.go:107`）
8. 启动 QQ 管理器（`main.go:138-139`，连接 `http://127.0.0.1:19877`）
9. 启动 OutboxWorker / DeliveryWorker / DesktopPetWorker / ProcessingWorker（`main.go:188-195`）
10. 启动主动消息 Cron（`main.go:196-202`）
11. 启动 DataLifecycle 与 Reconciliation 后台 goroutine（`main.go:204-218`）
12. 终止端口占用旧进程并启动 HTTP 服务（`main.go:178, 220-228`）

---

## 2. 后端启动命令

### 2.1 直接运行（开发/远程部署）

`backend/cmd/server/main.go` 编译为 `server.exe`（Windows）或 `server`（Linux）后直接执行：

```
./server.exe   # Windows
./server       # Linux
```

参考 `README.md:67-72`：

```
./surrealdb/surreal.exe start --user root --pass root rocksdb:data.db
./qdrant/qdrant.exe
./server.exe
```

### 2.2 Electron 桌面端启动

桌面端通过 `desktop/src/main/core-manager.ts:103` 使用 `child_process.spawn` 拉起 `AmitiaCore.exe`（即重命名后的 `server.exe`，见 `AGENTS.md` 桌面端构建规则）：

```ts
coreProcess = spawn(corePath, [], { cwd: dataDir, env, windowsHide: true });
```

注入的环境变量（`desktop/src/main/core-manager.ts:128-131`）：

| 变量 | 值 | 用途 |
|---|---|---|
| `CONFIG_PATH` | `<AmitiaDataDir>/config` | 配置文件目录 |
| `AMITIA_RUN_MODE` | `desktop` | 运行模式标记 |
| `AMITIA_DATA_DIR` | `<AmitiaDataDir>` | 数据根目录 |

### 2.3 Web 模式启动

Web 模式下用户需手动启动后端二进制并保持运行，前端通过 `front/src/runtime/runtime-adapter.ts` 的 `resolveApiUrl` 解析后端地址（`front/src/composables/useChatSSE.ts:71`、`front/src/composables/useWebChatSSE.ts:122`）。前端开发服务器固定监听 `127.0.0.1:5178`（`desktop/src/main/window.ts:12`、`README.md:71`）。

---

## 3. Go 版本

| 项 | 值 | 引用 |
|---|---|---|
| Go 版本 | `go 1.26.1` | `backend/go.mod:3` |
| 依赖管理 | Go Modules | `backend/go.mod:1` |
| 主要依赖 | gin v1.12.0、glebarez/sqlite v1.11.0、gorm v1.31.1、viper v1.21.0、logrus v1.9.4、google/uuid v1.6.0、golang-jwt/jwt/v5 v5.3.1、redis/go-redis/v9 v9.19.0 | `backend/go.mod:5-15` |

---

## 4. CGO 使用情况

| 项 | 值 | 引用 |
|---|---|---|
| CGO 是否启用 | **否**（SQLite 驱动为纯 Go 实现） | `backend/pkg/database/mysql/db.go:22` 使用 `github.com/glebarez/sqlite` |
| SQLite 驱动底层 | `modernc.org/sqlite`（纯 Go，无 cgo） | `backend/go.mod:8`（间接由 glebarez/sqlite 拉入） |
| 交叉编译友好性 | 高（无 cgo 即可 `GOOS=linux GOARCH=arm64 go build`） | 由驱动选择决定 |
| 是否调用任何 cgo 库 | 未在 `go.mod` 直接依赖任何 `cgo` 包；`golang.org/x/sys/windows` 仅在 Windows 构建时使用 | `backend/internal/mcp/transport/process_windows.go:1` (`//go:build windows`) |

> 结论：Amitia Go 后端为纯 Go 实现，无 CGO 依赖，可直接通过 `GOOS=linux GOARCH=arm64` 交叉编译。

---

## 5. SQLite 驱动

| 项 | 值 | 引用 |
|---|---|---|
| 驱动包 | `github.com/glebarez/sqlite` v1.11.0 | `backend/go.mod:8`、`backend/pkg/database/mysql/db.go:22` |
| 底层实现 | `modernc.org/sqlite`（纯 Go SSA 解析 SQLite 文件，无 cgo） | 间接依赖 |
| ORM | `gorm.io/gorm` v1.31.1 | `backend/go.mod:13`、`backend/pkg/database/mysql/db.go:23` |
| 连接初始化 | `mysql.NewSQLite(dataDir)` | `backend/cmd/server/main.go:77`、`backend/pkg/database/mysql/db.go:14` |
| 连接池参数 | `SetMaxIdleConns(10)`、`SetMaxOpenConns(1)`、`SetConnMaxLifetime(time.Hour)` | `backend/pkg/database/mysql/db.go:32-34` |
| 业务库 | `app.db`（单文件，GORM 管理） | `backend/pkg/database/mysql/db.go:17` |
| 测试库 | 多处使用 `:memory:` 或临时目录（`backend/internal/delivery/store_sqlite_test.go:14`、`backend/internal/migration/initial_sql_test.go:14` 等） | 全包测试代码 |

> 注意：`pkg/database/mysql` 包名虽叫 `mysql`，但实际只连接 SQLite。无 MySQL 或 Redis 实际依赖（`redis/go-redis/v9` 在 `go.mod:11` 但代码未实际使用，见后续 6.1）。

---

## 6. SQLite 文件路径

### 6.1 数据目录解析

| 项 | 值 | 引用 |
|---|---|---|
| 配置项 | `storage.dataDir`（默认 `data`） | `backend/config/config.yml:7`、`backend/config/config.go:102` |
| 运行时解析 | `config.AppCfg.Storage.DataDir = util.ResolveRuntimePath(runtimeRoot, ...)` | `backend/cmd/server/main.go:69` |
| 目录创建 | `os.MkdirAll(dataDir, 0755)` | `backend/pkg/database/mysql/db.go:15` |
| SQLite 文件 | `<DataDir>/app.db` | `backend/pkg/database/mysql/db.go:17` |
| 建表脚本 | `<DataDir>/sql.sql` | `backend/cmd/server/main.go:291` |

### 6.2 Electron 桌面端数据目录

| 项 | 值 | 引用 |
|---|---|---|
| 数据根目录 | `<安装目录>/../AmitiaData`（packaged）或 `process.cwd()/../AmitiaData`（dev） | `desktop/src/main/path-manager.ts:13-19` |
| 子目录 | `config`、`data`、`logs`、`uploads`、`qdrant`、`surrealdb`、`memory`、`runtime` | `desktop/src/main/path-manager.ts:25-35` |
| 写权限校验 | 启动时写测试文件验证 | `desktop/src/main/path-manager.ts:50-57` |

### 6.3 静态资源目录（HTTP 暴露）

| 路由 | 物理目录 | 引用 |
|---|---|---|
| `/audio` | `./data/tts_cache` | `backend/cmd/server/router.go:96` |
| `/exports` | `./data/exports` | `backend/cmd/server/router.go:97` |
| `/voice` | `./data/voice_msg` | `backend/cmd/server/router.go:98` |
| `/images` | `./data/images` | `backend/cmd/server/router.go:99` |
| `/videos` | `./data/videos` | `backend/cmd/server/router.go:100` |
| `/avatars` | `./data/avatars` | `backend/cmd/server/router.go:101` |
| `/emote-assets` | `<DataDir>/emotes` | `backend/cmd/server/router.go:102` |

---

## 7. 数据库迁移方式

### 7.1 启动迁移入口

| 项 | 值 | 引用 |
|---|---|---|
| 入口函数 | `applyDatabaseStartupMigrations(db)` | `backend/cmd/server/main.go:81`、`backend/cmd/server/main.go:262` |
| 迁移执行器 | `migration.Runner{DB: db, SkipBackup: existingDatabase}` | `backend/cmd/server/main.go:267` |
| 已有库检查 | `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'` | `backend/cmd/server/main.go:284` |
| 预迁移备份 | `CreatePreMigrationBackup()`（仅对已有库） | `backend/cmd/server/main.go:268-272` |
| 初始建表 | `initDatabase(db)` → `migration.ApplyInitialSQLFile(db, sqlPath)` | `backend/cmd/server/main.go:273`、`backend/cmd/server/main.go:298` |
| 版本化迁移 | `migRunner.Apply(migration.DefaultMigrations())` | `backend/cmd/server/main.go:276` |

### 7.2 迁移框架结构

| 文件 | 职责 | 引用 |
|---|---|---|
| `backend/internal/migration/runner.go` | `Migration` 结构体、`Runner.Apply`、`EnsureTable`、`hasPendingMigrations`、`applyOne` | `runner.go:1-312` |
| `backend/internal/migration/migrations.go` | `DefaultMigrations()` 返回全部版本化迁移 | `migrations.go:77-96` |
| `backend/internal/migration/runtime_queue.go` | 单条迁移示例：`RuntimeQueueMigration()` | `runtime_queue.go:1-33` |
| `backend/internal/migration/legacy_data_migration.go` | 旧数据迁移 | （`docs/extension-kernel/inventories/source-files.md:175` 引用） |

### 7.3 已注册的版本化迁移

`backend/internal/migration/migrations.go:77-96` 中 `DefaultMigrations()` 返回以下迁移（部分摘录）：

- `TemporalRelationshipTimeMigration()`
- `CanonicalSingleUserMigration()`
- `CharacterBaseColumnMigration()`
- `MCPClientMigration()`
- `UserProfileCharacterScopeMigration()`
- `ImageGenConfigMigration()`
- `DesktopPetActionDefinitionsMigration()`
- `ImageGenConfigEnabledMigration()`
- `DesktopPetGenerationTasksMigration()`
- `DesktopPetTaskExecutionFieldsMigration()`
- `DesktopPetActionExecutionFieldsMigration()`
- `DesktopPetGenerationFramesMigration()`
- `DesktopPetGenerationCallLogsMigration()`
- `DesktopPetProcessingTasksMigration()`
- `DesktopPetProcessingActionsMigration()`
- `DesktopPetProcessedFramesMigration()`
- `DesktopPetPackagesMigration()`
- `DesktopPetInstallationsMigration()`
- `DesktopPetRuntimeSettingsMigration()`
- `ExtensionsMigration()`、`PluginRuntimeMigration()`、`ExtensionWorkshopMigration()`、`ExtensionAgentSkillsMigration()`、`ExtensionPackagesMigration()` 等（见 `migration/extensions_test.go:24-39`、`migration/extension_packages_test.go:24-37`）

每条迁移包含 `Version`、`Name`、`AcceptedChecksums`、`Up` 回调（`backend/internal/migration/runner.go:11-16`），并通过 `migration_records` 表跟踪状态（`runner.go:18-29`）。

### 7.4 备份机制

| 项 | 值 | 引用 |
|---|---|---|
| 备份目录 | `<DataDir>/migration_backups`（默认） | `backend/internal/migration/backup_test.go:46` |
| 备份记录表 | `backup_records`、`backup_contents` | `backend/internal/migration/backup_test.go:60-65, 112` |
| 备份内容 | 主库 + WAL + SHM + 元数据 | `backup_test.go:113`（验证 4 条 content 记录） |
| 跳过备份条件 | `Runner.SkipBackup=true` 或无 pending 迁移 | `runner.go:70-80` |

> Android 迁移策略：迁移机制为纯 Go 代码 + 文件操作，Linux ARM64 完全兼容；只需把 `<DataDir>/sql.sql` 与 `migration_records` 表数据迁移即可。

---

## 8. Qdrant 现状

### 8.1 版本与可执行文件

| 项 | 值 | 引用 |
|---|---|---|
| Qdrant 版本号 | **无法确认**（代码未打印版本，由 `qdrant.zip`/`qdrant.exe.zip` 解压提供） | `backend/pkg/database/qdrant/manager.go:111-131` |
| Windows 可执行文件 | `<RuntimeRoot>/qdrant/qdrant.exe` | `backend/pkg/database/qdrant/manager.go:80` |
| Linux x86 可执行文件 | `<RuntimeRoot>/qdrant/qdrant_linux_x86` | `backend/pkg/database/qdrant/manager.go:85` |
| Linux ARM64 可执行文件 | `<RuntimeRoot>/qdrant/qdrant_linux_aarch64` | `backend/pkg/database/qdrant/manager.go:83` |
| 二进制获取 | 解压 `qdrant.exe.zip` 或 `qdrant.zip`（自动解压机制） | `backend/pkg/database/qdrant/manager.go:116-128`、`AGENTS.md` Git 上传规则 |
| 平台分支 | `runtime.GOOS` switch（windows/linux） | `backend/pkg/database/qdrant/manager.go:78-89` |

### 8.2 启动参数

| 项 | 值 | 引用 |
|---|---|---|
| 启动命令 | `qdrant --config-path <config.yaml>` | `backend/pkg/database/qdrant/manager.go:97` |
| 配置文件生成 | 运行时生成 `<qdrantDir>/config/config.yaml` | `backend/pkg/database/qdrant/manager.go:69-75` |
| 配置内容 | `service.http_port: <port>`、`service.grpc_port: <port+1>` | `backend/pkg/database/qdrant/manager.go:74` |
| 工作目录 | `<RuntimeRoot>/qdrant` | `backend/pkg/database/qdrant/manager.go:98` |
| stdout/stderr | 重定向到 `qdrantWriter`，前缀 `[Qdrant]` | `backend/pkg/database/qdrant/manager.go:21-32, 99-100` |

### 8.3 端口与数据目录

| 项 | 值 | 引用 |
|---|---|---|
| HTTP 端口 | `19178`（`config.qdrant.port`） | `backend/config/config.yml:23`、`backend/config/config.go:111` |
| gRPC 端口 | `19179`（HTTP 端口 + 1） | `backend/pkg/database/qdrant/manager.go:74` |
| 监听地址 | `127.0.0.1` | `backend/config/config.yml:22` |
| 数据目录 | `<qdrantDir>/storage/`（Qdrant 默认，相对工作目录） | `backend/pkg/database/qdrant/manager.go:98` 隐含；`.gitignore:35-37` 排除 `qdrant/storage/` |
| 健康检查 | `GET http://127.0.0.1:<port>/readyz`，60 次重试，每次 500ms | `backend/pkg/database/qdrant/manager.go:152-167` |
| 集合 | `memory_embeddings`、`working_memory`、`user_profiles`、`episodic_memories`、`amitia_emotes`（向量维度均 1536，默认值 2560） | `backend/config/config.yml:27-42`、`backend/config/config.go:115-124` |

### 8.4 旧进程清理

| 项 | 值 | 引用 |
|---|---|---|
| Windows 清理 | `taskkill /F /IM qdrant.exe` | `backend/pkg/database/qdrant/manager.go:44` |
| Linux 清理 | `pkill -9 qdrant` | `backend/pkg/database/qdrant/manager.go:46` |
| 端口等待 | 最多 10 秒等待旧进程释放端口 | `backend/pkg/database/qdrant/manager.go:50-59` |

### 8.5 停止与异常处理

| 项 | 值 | 引用 |
|---|---|---|
| 停止函数 | `StopQdrant()` | `backend/pkg/database/qdrant/manager.go:133` |
| 优雅停止 | `SIGTERM` + 5 秒等待 | `backend/pkg/database/qdrant/manager.go:137-148` |
| 强制终止 | 超时后 `Process.Kill()` | `backend/pkg/database/qdrant/manager.go:146` |
| 启动失败处理 | 仅记录日志并回退到关键词搜索（不退出主进程） | `backend/cmd/server/main.go:308-312` |

---

## 9. SurrealDB 现状

### 9.1 版本与可执行文件

| 项 | 值 | 引用 |
|---|---|---|
| SurrealDB 版本号 | **无法确认**（代码未打印版本，由 `surreal.zip`/`surreal.exe.zip` 解压提供） | `backend/pkg/database/surrealdb/manager.go:201-221` |
| Windows 可执行文件 | `<RuntimeRoot>/surrealdb/surreal.exe` | `backend/pkg/database/surrealdb/manager.go:90` |
| Linux 可执行文件 | `<RuntimeRoot>/surrealdb/surreal`（不区分 ARM64/x86） | `backend/pkg/database/surrealdb/manager.go:92` |
| 二进制获取 | 解压 `surreal.exe.zip` 或 `surreal.zip` | `backend/pkg/database/surrealdb/manager.go:206-218` |
| 平台分支 | `runtime.GOOS` switch（windows/linux） | `backend/pkg/database/surrealdb/manager.go:88-95` |

### 9.2 启动参数

| 项 | 值 | 引用 |
|---|---|---|
| 启动命令 | `surreal start --log info --user <user> --pass <pass> --bind <addr> <storage>` | `backend/pkg/database/surrealdb/manager.go:112-118` |
| 日志级别 | `info` | `backend/pkg/database/surrealdb/manager.go:113` |
| 用户名 | `root`（默认） | `backend/config/config.yml:54`、`backend/config/config.go:132` |
| 密码 | `root`（默认） | `backend/config/config.yml:55`、`backend/config/config.go:133` |
| 绑定地址 | `127.0.0.1:18000` | `backend/pkg/database/surrealdb/manager.go:103` |
| 存储引擎 | `surrealkv:<absPath>`（基于路径前缀） | `backend/pkg/database/surrealdb/manager.go:105-110` |
| 工作目录 | `<RuntimeRoot>/surrealdb` | `backend/pkg/database/surrealdb/manager.go:119` |
| stdout/stderr | 重定向到 `surrealWriter`，前缀 `[SurrealDB]` | `backend/pkg/database/surrealdb/manager.go:31-42, 120-121` |

### 9.3 端口与数据目录

| 项 | 值 | 引用 |
|---|---|---|
| 端口 | `18000`（`config.surrealdb.port`） | `backend/config/config.yml:51`、`backend/config/config.go:129` |
| 监听地址 | `127.0.0.1` | `backend/config/config.yml:50` |
| Namespace | `uai` | `backend/config/config.yml:52`、`backend/config/config.go:130` |
| Database | `memory_graph` | `backend/config/config.yml:53`、`backend/config/config.go:131` |
| 数据路径配置 | `data/graph.db`（相对 `RuntimeRoot`） | `backend/config/config.yml:56` |
| 实际存储路径 | `surrealkv:<RuntimeRoot>/data/graph.db` | `backend/pkg/database/surrealdb/manager.go:106-109` |
| 健康检查 | `GET http://127.0.0.1:<port>/health`，60 次重试，每次 500ms | `backend/pkg/database/surrealdb/manager.go:246-261` |

### 9.4 监控与自愈

| 项 | 值 | 引用 |
|---|---|---|
| 监控协程 | `StartSurrealMonitor()`，10 秒轮询 | `backend/cmd/server/main.go:104`、`backend/pkg/database/surrealdb/manager.go:143-192` |
| 健康判定 | `isSurrealAlive(port)` → `/health` 返回 200 | `backend/pkg/database/surrealdb/manager.go:132-141` |
| 自动重启 | 检测到进程退出或健康检查失败时重启 | `backend/pkg/database/surrealdb/manager.go:166-182` |
| 重启回调 | `surrealRestartFn`（重建 graph 服务） | `backend/pkg/database/surrealdb/manager.go:27-29`、`backend/cmd/server/main.go:114-120` |
| 锁 | `surrealMu sync.Mutex` 保护重启流程 | `backend/pkg/database/surrealdb/manager.go:23`、`backend/pkg/database/surrealdb/manager.go:73-74, 159, 169-185, 225-226` |

### 9.5 旧进程清理

| 项 | 值 | 引用 |
|---|---|---|
| Windows 清理 | `taskkill /F /IM surreal.exe` | `backend/pkg/database/surrealdb/manager.go:54` |
| Linux 清理 | `pkill -9 surreal` | `backend/pkg/database/surrealdb/manager.go:56` |
| 端口等待 | 最多 10 秒等待旧进程释放端口 | `backend/pkg/database/surrealdb/manager.go:60-69` |

### 9.6 停止

| 项 | 值 | 引用 |
|---|---|---|
| 停止函数 | `StopSurreal()`（先停止监控再终止进程） | `backend/pkg/database/surrealdb/manager.go:223-244` |
| 优雅停止 | `SIGTERM` + 5 秒等待 | `backend/pkg/database/surrealdb/manager.go:231-242` |
| 强制终止 | 超时后 `Process.Kill()` | `backend/pkg/database/surrealdb/manager.go:240` |

---

## 10. 后端依赖的环境变量

### 10.1 显式读取的环境变量

| 变量 | 用途 | 引用 |
|---|---|---|
| `CONFIG_PATH` | 配置文件目录（若为相对路径会拼接到 `runtimeRoot`） | `backend/cmd/server/main.go:61-66` |
| `AMITIA_RUN_MODE` | 运行模式标记（由 Electron 注入，后端代码未直接读取但供日志/调试使用） | `desktop/src/main/core-manager.ts:130` |
| `AMITIA_DATA_DIR` | 数据根目录（由 Electron 注入，后端代码未直接读取，但 `util.RuntimeRoot()` 可能消费） | `desktop/src/main/core-manager.ts:131` |

### 10.2 viper AutomaticEnv

| 项 | 值 | 引用 |
|---|---|---|
| 自动绑定 | `v.AutomaticEnv()`，所有配置项均可通过环境变量覆盖（前缀默认空） | `backend/config/config.go:144` |
| 可覆盖项示例 | `SERVER_PORT`、`SERVER_HOST`、`STORAGE_DATADIR`、`QDRANT_PORT`、`SURREALDB_PORT`、`SURREALDB_DATAPATH` 等 | `backend/config/config.go:99-134` |

### 10.3 间接依赖的运行时变量

| 变量 | 用途 | 引用 |
|---|---|---|
| `GIN_MODE` | Gin 框架运行模式（由 `config.server.mode=release` 间接设置） | `backend/cmd/server/router.go:46-48` |

---

## 11. 后端依赖的文件资源

### 11.1 启动期必需文件

| 文件 | 用途 | 引用 |
|---|---|---|
| `<DataDir>/sql.sql` | 初始建表脚本，缺失则启动失败 | `backend/cmd/server/main.go:291-297` |
| `<RuntimeRoot>/qdrant/qdrant.exe` 或 `qdrant.zip` | Qdrant 可执行文件或压缩包 | `backend/pkg/database/qdrant/manager.go:80, 116` |
| `<RuntimeRoot>/surrealdb/surreal.exe` 或 `surreal.zip` | SurrealDB 可执行文件或压缩包 | `backend/pkg/database/surrealdb/manager.go:90, 206` |
| `<runtimeRoot>/config/config.yml` | 配置文件（若未指定 `CONFIG_PATH`） | `backend/cmd/server/main.go:62-66`、`backend/config/config.go:93-97` |

### 11.2 运行期生成的文件

| 文件 | 用途 | 引用 |
|---|---|---|
| `<DataDir>/app.db` | SQLite 主库 | `backend/pkg/database/mysql/db.go:17` |
| `<DataDir>/migration_backups/` | 迁移备份目录 | `backend/internal/migration/backup_test.go:46` |
| `<RuntimeRoot>/qdrant/config/config.yaml` | Qdrant 配置（运行时生成） | `backend/pkg/database/qdrant/manager.go:69-75` |
| `<RuntimeRoot>/qdrant/storage/` | Qdrant 数据目录 | `.gitignore:35-37` |
| `<RuntimeRoot>/data/graph.db/` | SurrealDB 数据目录 | `backend/config/config.yml:56`、`backend/pkg/database/surrealdb/manager.go:106-109` |
| `<runtimeRoot>/logs/` | 日志目录 | `backend/cmd/server/main.go:72` |

### 11.3 Electron 桌面端资源白名单

`desktop/electron-builder.yml:11-26` 中 `extraResources` 严格白名单：

| 资源 | 路径 | 备注 |
|---|---|---|
| `AmitiaCore.exe` | `resources/core/AmitiaCore.exe` | Go 后端编译产物重命名 |
| `node.exe.zip`、`node.zip` | `resources/core/` | Node 运行时（侧车依赖） |
| `sidecar/bundle.mjs`、`sidecar/launcher.mjs` | `resources/core/sidecar/` | 微信侧车 |
| `qq-sidecar/bundle.mjs`、`qq-sidecar/launcher.mjs` | `resources/core/qq-sidecar/` | QQ 侧车 |
| `resources/bridge`、`resources/migrations`、`resources/data`、`resources/config-template`、`resources/qdrant`、`resources/surrealdb` | 各自目标 | 配置/数据/Qdrant/SurrealDB 资源 |

> Android 内嵌模式必须遵循 `AGENTS.md` 规则：surreal.exe / qdrant.exe 通过 `surreal.zip` / `qdrant.zip` 发布并自动解压。

---

## 12. 后端是否使用绝对 Windows 路径

**结论：否，后端未使用任何硬编码绝对 Windows 盘符路径。**

| 检查项 | 结果 | 引用 |
|---|---|---|
| 配置文件路径 | 使用 `filepath.Join(runtimeRoot, "config")` | `backend/cmd/server/main.go:63` |
| 数据目录解析 | `util.ResolveRuntimePath(runtimeRoot, config.AppCfg.Storage.DataDir)` | `backend/cmd/server/main.go:69` |
| SurrealDB 数据路径 | `util.ResolveRuntimePath(workDir, storagePath)` | `backend/pkg/database/surrealdb/manager.go:107` |
| Qdrant 目录 | `filepath.Join(workDir, "qdrant")` | `backend/pkg/database/qdrant/manager.go:68` |
| 日志目录 | `filepath.Join(runtimeRoot, "logs")` | `backend/cmd/server/main.go:72` |
| 静态资源路由 | `filepath.Join(config.AppCfg.Storage.DataDir, "emotes")` | `backend/cmd/server/router.go:102` |

> 所有路径均通过 `filepath.Join` 与 `util.RuntimeRoot()` 拼接，跨平台兼容。

---

## 13. 后端是否调用 Windows 专属命令

**结论：是，存在三处 Windows 专属命令调用，需平台抽象。**

### 13.1 `main.go` 端口占用检测与终止

| 行 | 命令 | 平台 | 引用 |
|---|---|---|---|
| 46 | `cmd /c netstat -ano \| findstr :<port> \| findstr LISTENING` | Windows 专属 | `backend/cmd/server/main.go:46` |
| 51 | `taskkill /F /PID <pid>` | Windows 专属 | `backend/cmd/server/main.go:51` |

```go
out, _ := exec.Command("cmd", "/c", "netstat -ano | findstr :"+port+" | findstr LISTENING").Output()
fields := strings.Fields(string(out))
for _, f := range fields {
    if pid, err2 := strconv.Atoi(f); err2 == nil {
        if pid != os.Getpid() {
            exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
        }
    }
}
```

> 此函数 `killExistingServer`（`backend/cmd/server/main.go:38-56`）在 Linux/Android 上无法运行，需平台抽象为 Linux 使用 `lsof`/`fuser` + `kill`，或在内嵌模式下直接通过 PID 文件管理。

### 13.2 Qdrant 旧进程清理

| 行 | 命令 | 平台 | 引用 |
|---|---|---|---|
| 44 | `taskkill /F /IM qdrant.exe` | Windows | `backend/pkg/database/qdrant/manager.go:44` |
| 46 | `pkill -9 qdrant` | Linux/Unix | `backend/pkg/database/qdrant/manager.go:46` |

> 已通过 `runtime.GOOS` 分支处理，Linux/Android 路径已存在。

### 13.3 SurrealDB 旧进程清理

| 行 | 命令 | 平台 | 引用 |
|---|---|---|---|
| 54 | `taskkill /F /IM surreal.exe` | Windows | `backend/pkg/database/surrealdb/manager.go:54` |
| 56 | `pkill -9 surreal` | Linux/Unix | `backend/pkg/database/surrealdb/manager.go:56` |

> 已通过 `runtime.GOOS` 分支处理，Linux/Android 路径已存在。

### 13.4 MCP Transport 进程管理

| 文件 | 构建标签 | 内容 | 引用 |
|---|---|---|---|
| `backend/internal/mcp/transport/process_windows.go` | `//go:build windows` | 使用 `windows.CREATE_NEW_PROCESS_GROUP`、`CreateJobObject`、`SetInformationJobObject`、`AssignProcessToJobObject`、`TerminateJobObject`、`CloseHandle` | `process_windows.go:1, 13-15, 17-39, 41-46, 48-52` |
| `backend/internal/mcp/transport/process_unix.go` | `//go:build !windows` | 使用 `syscall.SysProcAttr{Setpgid: true}` 与 `syscall.Kill(-pid, SIGKILL)` | `process_unix.go:1, 7-9, 11-13` |

> 已通过 Go 构建标签完全隔离，Linux/Android 自动使用 `process_unix.go` 实现，无需修改。

### 13.5 总结

| 位置 | 状态 | Android 兼容性 |
|---|---|---|
| `main.go:killExistingServer` | **未平台抽象**（硬编码 Windows 命令） | 需重构为平台接口 |
| `qdrant/manager.go:killExistingQdrant` | 已平台抽象（`runtime.GOOS` switch） | 兼容 |
| `surrealdb/manager.go:killExistingSurreal` | 已平台抽象（`runtime.GOOS` switch） | 兼容 |
| `mcp/transport/process_*.go` | 已平台抽象（`//go:build` 标签） | 兼容 |

---

## 14. 后端是否存在平台相关代码

### 14.1 已识别的平台相关代码汇总

| 文件 | 平台分支方式 | 影响 | 引用 |
|---|---|---|---|
| `backend/cmd/server/main.go:38-56` | 无（硬编码 Windows） | 阻塞 Linux/Android 启动 | `main.go:46, 51` |
| `backend/pkg/database/qdrant/manager.go:34-60, 78-89` | `runtime.GOOS` switch | 已兼容 | `manager.go:43-47, 78-89` |
| `backend/pkg/database/surrealdb/manager.go:44-70, 88-95` | `runtime.GOOS` switch | 已兼容 | `manager.go:53-57, 88-95` |
| `backend/internal/mcp/transport/process_windows.go` | `//go:build windows` | 已兼容 | `process_windows.go:1` |
| `backend/internal/mcp/transport/process_unix.go` | `//go:build !windows` | 已兼容 | `process_unix.go:1` |

### 14.2 未发现的平台相关代码

- 无 `golang.org/x/sys` 在非 Windows 路径上的直接调用
- 无 `os/signal` 平台特定信号（仅使用 `os.Interrupt` + `syscall.SIGTERM`，`main.go:74`）
- 无 Windows 服务注册代码
- 无系统托盘原生代码（托盘逻辑在 Electron 中：`desktop/src/main/tray.ts`）

### 14.3 Electron 桌面端 Windows 专属代码

| 文件 | 行 | 内容 | 引用 |
|---|---|---|---|
| `desktop/src/main/core-manager.ts:168-180` | Windows 进程终止 | `taskkill /PID <pid> /T /F`、`taskkill /F /IM qdrant.exe`、`taskkill /F /IM surreal.exe` | `core-manager.ts:170, 175, 179` |
| `desktop/src/main/core-manager.ts:181-187` | Linux 进程终止 | `coreProcess.kill("SIGTERM")`、`pkill -9 qdrant`、`pkill -9 surreal` | `core-manager.ts:182, 184, 186` |
| `desktop/src/main/core-manager.ts:18, 23` | Electron 路径获取 | `app.isPackaged`、`process.resourcesPath` | `core-manager.ts:19, 21` |

> Android 客户端不通过 Electron 启动后端，而是通过内嵌 Linux Runtime 直接启动 Go 二进制，Electron 代码不参与 Android 路径。

---

## 15. Web 和 Electron 如何启动后端

### 15.1 Web 模式

| 项 | 值 | 引用 |
|---|---|---|
| 后端启动 | 用户手动执行 `./server.exe` 或 `./server`（无进程管理） | `README.md:67-72` |
| 前端启动 | `pnpm run dev`（Vite 开发服务器） | `front/package.json:8` |
| 前端监听 | `127.0.0.1:5178` | `desktop/src/main/window.ts:12`、`README.md:71` |
| API 解析 | `resolveApiUrl()` 解析后端地址 | `front/src/composables/useChatSSE.ts:71`、`front/src/composables/useWebChatSSE.ts:122` |
| 部署模式 | `config.app.deployMode` 默认 `desktop-local` | `backend/config/config.yml:16` |

### 15.2 Electron 桌面端启动

| 步骤 | 文件 | 行 | 内容 |
|---|---|---|---|
| 1. 数据目录初始化 | `desktop/src/main/index.ts` | `73-86` | `ensureAmitiaDataDir()` |
| 2. 进入主应用 | `desktop/src/main/index.ts` | `90` | `enterMainApp()` |
| 3. 部署模式读取 | `desktop/src/main/index.ts` | `92-93` | `configStore.getDeploymentConfig()` |
| 4. Runtime Manager 初始化 | `desktop/src/main/index.ts` | `94-95` | `runtimeManager.initialize()` |
| 5. 数据/配置校验 | `desktop/src/main/index.ts` | `115-122` | `ensureDataAndConfig()` |
| 6. 启动核心进程 | `desktop/src/main/index.ts` | `125-134` | `startCore()` + `waitForCoreReady()` |
| 7. 创建主窗口 | `desktop/src/main/index.ts` | `136` | `createMainWindow()` |
| 8. 创建托盘 | `desktop/src/main/index.ts` | `137-140` | `createAppTray()` |
| 9. 注册更新管理器 | `desktop/src/main/index.ts` | `146` | `registerUpdateManager()` |
| 10. 启动桌面宠物 | `desktop/src/main/index.ts` | `148-167` | `DesktopPetManager` + `ChatStateSubscriber` + `CharacterWatcher` |

核心进程启动参数（`desktop/src/main/core-manager.ts:100-145`）：

```ts
const corePath = getCorePath();       // resources/core/AmitiaCore.exe
const dataDir = getAmitiaDataDir();   // <installDir>/../AmitiaData
const env = {
    ...process.env,
    CONFIG_PATH: path.join(dataDir, "config"),
    AMITIA_RUN_MODE: "desktop",
    AMITIA_DATA_DIR: dataDir,
};
coreProcess = spawn(corePath, [], { cwd: dataDir, env, windowsHide: true });
```

### 15.3 远程模式

| 项 | 值 | 引用 |
|---|---|---|
| 部署模式配置 | `DeploymentModeConfig`（`mode: "local" | "remote"`） | `desktop/src/shared/types`、`desktop/src/main/index.ts:93` |
| 远程模式行为 | 跳过 `startCore()`，直接 `notifyStatus(runtimeManager, "ready")` | `desktop/src/main/index.ts:155-157` |
| 远程地址配置 | `ConfigStore` 管理 | `desktop/src/main/config-store.ts`（推断） |

---

## 16. 所有真实 API

### 16.1 API 路由总览

入口：`backend/cmd/server/router.go:45` `setupRouter(ctx, services)`

所有 API 路由组在 `r.Group("/api")`（`router.go:52`）下注册：

| 模块 | 注册函数 | 引用 |
|---|---|---|
| User | `user.RegisterUserRouter` | `router.go:54` |
| Character | `character.RegisterCharacterRouter` | `router.go:55` |
| Chat | `chat.RegisterChatRouterWithDelivery` | `router.go:56` |
| Memory | `memory.RegisterMemoryRouter` + 2 个内联 | `router.go:57-61` |
| Profile | `profile.RegisterProfileRouter` | `router.go:62` |
| Proactive | `proactive.RegisterProactiveRouterWithCompanion` + `RegisterRemindersRouter` | `router.go:63-64` |
| Episodic | `episodic.RegisterEpisodicRouter` | `router.go:65` |
| WorldBook | `worldbook.RegisterWorldBookRouter` | `router.go:66` |
| Feedback | `feedback.RegisterFeedbackRouter` | `router.go:67` |
| Graph | `graph.RegisterGraphRouter` | `router.go:68` |
| Agent | `agent.RegisterAgentRouter` | `router.go:69` |
| System | `system.RegisterSystemRouter` | `router.go:70` |
| Companion | `companion.RegisterCompanionRouter` | `router.go:71` |
| QQ | `qq.RegisterQQRouter` | `router.go:72` |
| TTS | `tts.RegisterTtsRouter` | `router.go:73` |
| ASR | `asr.RegisterAsrRouter` | `router.go:74` |
| Realtime | `realtime.RegisterRealtimeRouter` | `router.go:75` |
| Vision | `vision.RegisterVisionRouter` | `router.go:76` |
| Embedding Config | `embedding_config.RegisterEmbeddingConfigRouter` | `router.go:77` |
| ImageGen | `imagegen.RegisterImageGenRouter` | `router.go:78` |
| DesktopPet | `desktoppet.RegisterDesktopPetRouter` | `router.go:79` |
| Processing | `processing.RegisterProcessingRouter` | `router.go:80` |
| Installation | `installation.RegisterRoutes` | `router.go:81` |
| Psyche API | `system.RegisterPsycheAPIRouter` | `router.go:82` |
| Psyche Snapshot | `system.RegisterPsycheSnapshotRouter` | `router.go:83` |
| Health | `system.RegisterHealthRouter` | `router.go:84` |
| Voice Entry | `system.RegisterVoiceEntryRouter` | `router.go:87` |
| Safety | `safety.RegisterSafetyRouter` | `router.go:88` |
| Delivery | `delivery.RegisterSubmitRouter` | `router.go:89` |
| Extension | `extension.RegisterRouter` | `router.go:90` |
| Emote | `emote.RegisterRouter` | `router.go:91` |
| MCP API | `mcpapi.RegisterRouter` | `router.go:92` |
| Temporal | `temporal.RegisterRouter` | `router.go:93` |
| Mood | `mood.RegisterMoodRouter` | `router.go:94` |

### 16.2 关键路由示例（来自 `backend/internal/system/router.go:115-248`）

| 路径 | 方法 | 用途 | 引用 |
|---|---|---|---|
| `/api/messages/stream` | GET | 消息流（SSE 长轮询） | `system/router.go:225`、`backend/internal/system/stream_handler.go:27` |
| `/api/messages/events` | GET | 消息事件总线（SSE） | `system/router.go:231`、`backend/internal/system/messages_events_handler.go:24` |
| `/api/proactive-sse` | GET | 主动消息 SSE | `system/router.go:226` |
| `/api/web-chat/send-stream` | POST | Web 聊天流式回复 | `system/router.go:246`、`backend/internal/system/stream_handler.go:202` |
| `/api/web-chat/conversations` | GET/POST | 会话列表/创建 | `system/router.go:236-237` |
| `/api/web-chat/conversations/:id/messages` | GET | 会话消息列表 | `system/router.go:238` |
| `/api/web-chat/send` | POST | 非流式发送 | `system/router.go:244` |
| `/api/health/circuit-breakers` | GET | 熔断器健康报告 | `backend/internal/system/health_voice_router.go:18` |
| `/api/health/data-lifecycle` | GET | 数据生命周期统计 | `backend/internal/system/health_voice_router.go:43` |
| `/api/health/reconciliation` | GET | 对账引擎状态 | `backend/internal/system/health_voice_router.go:48` |
| `/api/voice/session` | POST | 创建语音会话 | `backend/internal/system/health_voice_router.go:77` |
| `/api/voice/turn` | POST | 处理一轮语音对话 | `backend/internal/system/health_voice_router.go:96` |

### 16.3 DesktopPet Processing 路由（来自 `backend/internal/desktoppet/processing/router.go:16-31`）

| 路径 | 方法 | 用途 |
|---|---|---|
| `/api/desktop-pets/packages` | GET | 列出桌宠包 |
| `/api/desktop-pets/packages/:packageId/download` | GET | 下载桌宠包 |
| `/api/desktop-pets/generation-tasks/:taskId/process` | POST | 创建处理任务 |
| `/api/desktop-pets/processing-tasks/:processingTaskId` | GET | 查询处理任务 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/cancel` | POST | 取消处理任务 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/actions/:actionKey/retry` | POST | 重试动作 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/package` | POST | 打包 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/actions/:actionKey/switch-attempt` | POST | 切换尝试 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/actions/:actionKey/exclude` | POST | 排除动作 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/events` | GET | 处理事件流（SSE） |
| `/api/desktop-pets/processing-tasks/:processingTaskId/actions/:actionKey/frames/:frameIndex/processed-image` | GET | 处理后帧图片 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/actions/:actionKey/frames/:frameIndex/source-image` | GET | 源帧图片 |
| `/api/desktop-pets/processing-tasks/:processingTaskId/actions/:actionKey/preview` | GET | 动作预览 |

### 16.4 Installation 路由（来自 `backend/internal/desktoppet/installation/router.go:9-23`）

| 路径 | 方法 | 用途 |
|---|---|---|
| `/api/desktop-pets/packages/:packageId/install` | POST | 安装桌宠包 |
| `/api/desktop-pets/installations` | GET | 列出安装 |
| `/api/desktop-pets/installations/:installationId` | GET | 查询安装 |
| `/api/desktop-pets/installations/:installationId/enable` | POST | 启用 |
| `/api/desktop-pets/installations/:installationId/disable` | POST | 禁用 |
| `/api/desktop-pets/installations/:installationId/default-action` | PATCH | 更新默认动作 |
| `/api/desktop-pets/installations/:installationId/settings` | PATCH | 更新运行时设置 |
| `/api/desktop-pets/installations/:installationId/recenter` | POST | 重置位置 |
| `/api/desktop-pets/installations/:installationId/actions/:actionKey/play` | POST | 播放动作 |
| `/api/desktop-pets/installations/:installationId` | DELETE | 卸载 |

### 16.5 ASR 路由（来自 `backend/internal/asr/asr.go:145-159`）

| 路径 | 方法 | 用途 |
|---|---|---|
| `/api/asr/upload` | POST | 上传音频文件 |
| `/api/asr/uploads/:file` | GET | 提供已上传文件 |
| `/api/asr/submit` | POST | 提交 ASR 任务 |
| `/api/asr/query` | GET | 查询 ASR 结果 |
| `/api/asr/configs` | GET/POST | 列出/创建配置 |
| `/api/asr/configs/:id` | GET/PUT/DELETE | 单条配置 |
| `/api/asr/configs/:id/activate` | POST | 激活配置 |
| `/api/asr/configs/:id/test` | POST | 测试配置 |

### 16.6 静态资源路由

来自 `backend/cmd/server/router.go:96-102`：

| 路径 | 物理目录 |
|---|---|
| `/audio` | `./data/tts_cache` |
| `/exports` | `./data/exports` |
| `/voice` | `./data/voice_msg` |
| `/images` | `./data/images` |
| `/videos` | `./data/videos` |
| `/avatars` | `./data/avatars` |
| `/emote-assets` | `<DataDir>/emotes` |

---

## 17. 所有实时协议

### 17.1 SSE 端点清单

| 端点 | 事件名 | 用途 | 引用 |
|---|---|---|---|
| `GET /api/messages/stream?conversationId=<id>&since=<msgId>` | `message`、`error` | 消息流（长轮询 500ms） | `backend/internal/system/stream_handler.go:27-109`、`front/src/composables/useChatSSE.ts:70-87` |
| `GET /api/messages/events?channel=web` | `connected`、`message_created`、`message_updated`、`conversation_updated` | 消息事件总线 | `backend/internal/system/messages_events_handler.go:24-75`、`backend/internal/system/message_event_bus.go:14-17`、`front/src/composables/useWebChatSSE.ts:121-134` |
| `GET /api/proactive-sse` | `proactive_message`、`ping` | 主动消息推送 | `backend/internal/system/router.go:226`、`front/src/composables/useChatSSE.ts:90-119` |
| `POST /api/web-chat/send-stream` | `message_start`、`token`、`voice_audio`、`message_end` | Web 聊天流式回复 | `backend/internal/system/stream_handler.go:202-388` |
| `GET /api/desktop-pets/processing-tasks/:id/events` | `connected`、`ping`、`processing.progress`、`processing.action`、`processing.action.progress`、`processing.completed`、`processing.task.created`、`processing.task.cancel_requested`、`processing.action.retry`、`processing.package.created`、`processing.action.switch_attempt`、`processing.action.excluded` | 桌宠处理任务事件流 | `backend/internal/desktoppet/processing/handler.go:243-280`、`front/src/composables/useProcessingTask.ts:223-318` |
| `GET /api/desktop-pets/tasks/:id/events` | `connected`、`ping`、`<evt.EventType>` | 桌宠任务事件流 | `backend/internal/desktoppet/handler.go:184-221` |
| `GET /api/reminders/stream` | `status`、`changed` | 提醒流（5 秒轮询） | `backend/internal/system/stream_handler.go:135-171` |
| `GET /api/proactive/reminders/stream` | `<eventName>`、`ping` | 主动提醒流（30 秒心跳） | `backend/internal/proactive/handler_stream.go:9-40` |

### 17.2 SSE 事件字段示例

#### `POST /api/web-chat/send-stream` 响应事件

来自 `backend/internal/system/stream_handler.go:310-388`：

**event: message_start**（`stream_handler.go:315`）
```json
{
  "conversationId": "<convId>",
  "messageId": "<msgId>",
  "role": "assistant",
  "channel": "web",
  "createdAt": "2006-01-02 15:04:05"
}
```

**event: token**（`stream_handler.go:365, 371`）— 当无 TTS 时
```json
{
  "id": "<msgId>",
  "conversationId": "<convId>",
  "role": "assistant",
  "content": "<line>",
  "createdAt": "2006-01-02 15:04:05"
}
```

**event: voice_audio**（`stream_handler.go:360`）— 当 TTS 成功时
```json
{
  "messageId": "<msgId>",
  "conversationId": "<convId>",
  "role": "assistant",
  "content": "<line>",
  "createdAt": "2006-01-02 15:04:05",
  "audioUrl": "<url>",
  "duration": <float>
}
```

**event: message_end**（`stream_handler.go:387`）
```json
{
  "messageId": "<lastMsgId>",
  "status": "completed",
  "conversationId": "<convId>",
  "finalContentLength": <int>
}
```

#### `GET /api/messages/events` 事件

来自 `backend/internal/system/messages_events_handler.go:50-69`：

```json
{
  "conversationId": "<convId>",
  "messageId": "<msgId>",
  "channel": "web",
  "direction": "<direction>",
  "role": "<role>",
  "content": "<content>",
  "createdAt": "<createdAt>",
  "status": "<status>",
  "data": <optional data>
}
```

事件类型来自 `backend/internal/system/message_event_bus.go:14-17`：
- `message_created`
- `message_updated`
- `conversation_updated`

### 17.3 WebSocket

| 项 | 值 | 引用 |
|---|---|---|
| 是否使用 | **无法确认**（代码搜索未见 `c.Websocket` 或 `gorilla/websocket`，但 `config.Chat.MergeWindowMs` 暗示有合并窗口逻辑） | `backend/config/config.yml:18` |
| Realtime 模块 | `realtime.RegisterRealtimeRouter` 注册（具体实现未审计） | `backend/cmd/server/router.go:75` |

> 待审计：`backend/internal/realtime/` 模块的 WebSocket 实现细节，需在 Phase 4 进一步确认。

### 17.4 流式 HTTP

`POST /api/web-chat/send-stream` 是流式 HTTP 响应（SSE 格式但 POST 方法，`backend/internal/system/stream_handler.go:202-388`）：

```
POST /api/web-chat/send-stream HTTP/1.1
Content-Type: application/json
Authorization: Bearer <token>

{
  "conversationId": "...",
  "characterId": "...",
  "content": "...",
  "useMemory": true
}
```

响应：`Content-Type: text/event-stream`

### 17.5 前端实现

| 文件 | 端点 | 事件监听 | 引用 |
|---|---|---|---|
| `front/src/composables/useChatSSE.ts` | `/api/messages/stream` | `message` | `useChatSSE.ts:70-87` |
| `front/src/composables/useChatSSE.ts` | `/api/proactive-sse` | `proactive_message` | `useChatSSE.ts:90-119` |
| `front/src/composables/useWebChatSSE.ts` | `/api/messages/events` | `message_created`、`message_updated` | `useWebChatSSE.ts:121-134` |
| `front/src/composables/useWebChatSSE.ts` | `/api/proactive-sse` | `proactive_message` | `useWebChatSSE.ts:136-170` |
| `front/src/composables/useChat.ts` | `/api/web-chat/send-stream` | `message_start`、`token`、`voice_audio`、`message_end` | `useChat.ts:94-144` |
| `front/src/composables/useProcessingTask.ts` | `/api/desktop-pets/processing-tasks/:id/events` | `connected`、`ping`、`processing.progress`、`processing.action`、`processing.action.progress`、`processing.completed` | `useProcessingTask.ts:223-318` |

### 17.6 重连策略

| 端点 | 重连策略 | 引用 |
|---|---|---|
| `/api/messages/stream` | 断开后 3 秒重连 | `front/src/composables/useChatSSE.ts:80-83` |
| `/api/messages/events` | 断开后 3 秒重连 | `front/src/composables/useWebChatSSE.ts:128-131` |
| `/api/proactive-sse` | 断开后 5 秒重连 | `front/src/composables/useChatSSE.ts:113-117`、`useWebChatSSE.ts:165-168` |
| 桌宠处理任务 | 断开后切换轮询模式（`schedulePoll`） | `front/src/composables/useProcessingTask.ts:309-313` |

---

## 18. 所有当前用户可用功能

> 状态：**已真实实现** / **部分实现** / **仅 UI** / **后端缺失** / **已废弃** / **无法确认**

### 18.1 核心业务能力

| 功能 | 后端状态 | 前端状态 | 引用 |
|---|---|---|---|
| 用户登录/Token | 已真实实现 | 已真实实现 | `backend/internal/user/`、`backend/config/config.go:34-37` |
| 角色管理（创建/编辑/删除/切换/列表/详情） | 已真实实现 | 已真实实现 | `backend/internal/character/`、`router.go:55` |
| 会话管理（创建/列表/删除/分页） | 已真实实现 | 已真实实现 | `backend/internal/chat/`、`router.go:56` |
| 聊天发送（流式 + 非流式） | 已真实实现 | 已真实实现 | `backend/internal/system/stream_handler.go:202`、`useChat.ts:94` |
| 消息流（SSE 实时推送） | 已真实实现 | 已真实实现 | `backend/internal/system/stream_handler.go:27`、`useChatSSE.ts` |
| 图片消息 | 已真实实现 | 已真实实现 | `backend/internal/system/stream_handler.go:211`、`useWebChatSend.ts` |
| 语音消息 | 已真实实现 | 已真实实现 | `backend/internal/system/stream_handler.go:360`（`voice_audio` 事件） |
| 视频消息 | 已真实实现 | 已真实实现 | `backend/internal/system/stream_handler.go:99-100`、`useWebChatSend.ts` |
| 表情包 | 已真实实现 | 已真实实现 | `backend/internal/emote/handler.go:1`、`router.go:91` |

### 18.2 记忆与图谱

| 功能 | 后端状态 | 前端状态 | 引用 |
|---|---|---|---|
| 长期记忆 | 已真实实现 | 已真实实现 | `backend/internal/memory/`、`router.go:57` |
| 情景记忆 | 已真实实现 | 已真实实现 | `backend/internal/episodic/`、`router.go:65` |
| 世界书 | 已真实实现 | 已真实实现 | `backend/internal/worldbook/`、`router.go:66` |
| 知识图谱（SurrealDB） | 已真实实现 | 已真实实现 | `backend/internal/graph/`、`router.go:68` |
| 心理状态 | 已真实实现 | 已真实实现 | `backend/internal/psyche/`、`router.go:82-83` |
| 时间线（Temporal） | 已真实实现 | 已真实实现 | `backend/internal/temporal/`、`router.go:93` |
| 情绪 | 已真实实现 | 已真实实现 | `backend/internal/mood/`、`router.go:94` |

### 18.3 主动消息与渠道

| 功能 | 后端状态 | 前端状态 | 引用 |
|---|---|---|---|
| 主动消息（Cron 调度） | 已真实实现 | 已真实实现 | `backend/internal/proactive/`、`router.go:63`、`backend/cmd/server/main.go:196-202` |
| 提醒 | 已真实实现 | 已真实实现 | `backend/internal/system/stream_handler.go:135`、`router.go:64` |
| 微信渠道 | 已真实实现 | 已真实实现 | `backend/internal/system/wechat_bridge_service.go`、`router.go:115-130`、`AGENTS.md` 侧车 19876 |
| QQ 渠道 | 已真实实现 | 已真实实现 | `backend/internal/qq/`、`router.go:72`、`backend/cmd/server/main.go:138`（19877） |

### 18.4 模型与生成

| 功能 | 后端状态 | 前端状态 | 引用 |
|---|---|---|---|
| 模型配置（LLM） | 已真实实现 | 已真实实现 | `front/src/views/model-config/`、`router.go`（推断） |
| 嵌入模型配置 | 已真实实现 | 已真实实现 | `backend/internal/embedding_config/`、`router.go:77` |
| 图像生成 | 已真实实现 | 已真实实现 | `backend/internal/imagegen/`、`router.go:78` |
| 视觉理解 | 已真实实现 | 已真实实现 | `backend/internal/vision/`、`router.go:76` |
| ASR（语音识别） | 已真实实现 | 已真实实现 | `backend/internal/asr/asr.go:140-159` |
| TTS（语音合成） | 已真实实现 | 已真实实现 | `backend/internal/tts/`、`router.go:73`、`backend/internal/system/stream_handler.go:278-308` |

### 18.5 桌宠与扩展

| 功能 | 后端状态 | 前端状态 | 引用 |
|---|---|---|---|
| 桌宠生成 | 已真实实现 | 已真实实现 | `backend/internal/desktoppet/`、`router.go:79` |
| 桌宠处理（帧处理/打包） | 已真实实现 | 已真实实现 | `backend/internal/desktoppet/processing/`、`router.go:80` |
| 桌宠安装 | 已真实实现 | 已真实实现 | `backend/internal/desktoppet/installation/`、`router.go:81` |
| 桌宠运行时（Electron 端） | 已真实实现 | 仅桌面端 | `desktop/src/main/pet/manager.ts` |
| 扩展系统 | 已真实实现 | 已真实实现 | `backend/internal/extension/`、`router.go:90` |
| MCP 集成 | 已真实实现 | 已真实实现 | `backend/internal/mcpapi/`、`router.go:92`、`backend/internal/mcp/transport/` |

### 18.6 系统与健康

| 功能 | 后端状态 | 前端状态 | 引用 |
|---|---|---|---|
| 健康检查（熔断器） | 已真实实现 | 部分实现 | `backend/internal/system/health_voice_router.go:18-73` |
| 数据生命周期 | 已真实实现 | 仅后端 | `backend/internal/mindruntime/data_lifecycle_executor.go`、`router.go:84` |
| 对账引擎 | 已真实实现 | 仅后端 | `backend/internal/system/health_voice_router.go:48-73` |
| 语音会话 | 已真实实现 | 仅后端 | `backend/internal/system/health_voice_router.go:77-178` |
| 安全（Safety） | 已真实实现 | 仅后端 | `backend/internal/safety/`、`router.go:88` |
| 投递（Delivery） | 已真实实现 | 仅后端 | `backend/internal/delivery/`、`router.go:89` |
| 反馈 | 已真实实现 | 已真实实现 | `backend/internal/feedback/`、`router.go:67` |
| 实时（Realtime） | 无法确认 | 无法确认 | `backend/internal/realtime/`、`router.go:75`（WebSocket 细节未审计） |

### 18.7 Electron 桌面端独有功能

| 功能 | 状态 | 引用 |
|---|---|---|
| 桌宠窗口管理 | 已真实实现 | `desktop/src/main/pet/manager.ts`、`desktop/src/main/pet/window-adapter.ts` |
| 系统托盘 | 已真实实现 | `desktop/src/main/tray.ts` |
| 自动更新 | 已真实实现 | `desktop/src/main/update-manager.ts` |
| 多窗口（主/子） | 已真实实现 | `desktop/src/main/window.ts`、`front/src/types/desktop.d.ts` |
| 应用主题（深/浅色） | 已真实实现 | `desktop/src/main/branding.ts`、`desktop/src/main/index.ts:54-57` |
| 开机自启动 | 已真实实现 | `desktop/src/main/index.ts:142-144`、`desktop/src/main/tray.ts:46-56` |

---

## 19. 关键结论

### 19.1 Android 兼容性结论

1. **Go 后端可交叉编译**：`glebarez/sqlite` 纯 Go 驱动，无 CGO 依赖，可直接 `GOOS=linux GOARCH=arm64 go build`（`backend/go.mod:8`、`backend/pkg/database/mysql/db.go:22`）。
2. **Windows 专属代码仅一处阻塞**：`backend/cmd/server/main.go:38-56` `killExistingServer` 未平台抽象，需重构为 `RuntimePlatform` 接口（Windows 用 `cmd/taskkill`，Linux/Android 用 `lsof`/`fuser` + `kill` 或 PID 文件管理）。
3. **Qdrant/SurrealDB 已平台抽象**：`runtime.GOOS` switch 已支持 Linux ARM64 路径（`backend/pkg/database/qdrant/manager.go:78-89`、`backend/pkg/database/surrealdb/manager.go:88-95`），但 SurrealDB Linux 二进制未区分 ARM64/x86（`surrealdb/manager.go:92`），需补充。
4. **MCP Transport 已平台抽象**：通过 `//go:build` 标签完全隔离 Windows 与 Unix 实现（`process_windows.go:1`、`process_unix.go:1`），Linux/Android 自动选用 `process_unix.go`。
5. **配置跨平台兼容**：`viper` + `CONFIG_PATH` 环境变量 + `AutomaticEnv()` 机制支持任意路径注入（`backend/config/config.go:92-144`），Android 可通过环境变量注入私有目录路径。
6. **路径全部跨平台**：所有文件路径均通过 `filepath.Join` + `util.RuntimeRoot()` 拼接，无硬编码绝对盘符（见第 12 节）。

### 19.2 流式协议结论

1. **SSE 协议明确**：`POST /api/web-chat/send-stream` 事件为 `message_start` / `token` / `voice_audio` / `message_end`（`backend/internal/system/stream_handler.go:315, 360, 365, 371, 387`），Android 客户端必须严格对齐。
2. **事件总线协议明确**：`GET /api/messages/events` 事件为 `message_created` / `message_updated` / `conversation_updated`（`backend/internal/system/message_event_bus.go:14-17`）。
3. **桌宠处理任务事件丰富**：包含 `processing.progress` / `processing.action` / `processing.action.progress` / `processing.completed` / `processing.task.created` / `processing.task.cancel_requested` / `processing.action.retry` / `processing.package.created` / `processing.action.switch_attempt` / `processing.action.excluded`（`backend/internal/desktoppet/processing/handler.go:74-79, 112-114, 136-139, 173-178, 210-214, 236-239`）。
4. **WebSocket 待审计**：`backend/internal/realtime/` 模块的 WebSocket 实现细节未在本次审计覆盖范围内，需 Phase 4 补充。

### 19.3 端口规约

| 服务 | 端口 | 监听地址 | 引用 |
|---|---|---|---|
| Go 后端 HTTP | 18899 | 127.0.0.1 | `backend/config/config.yml:2-3` |
| Qdrant HTTP | 19178 | 127.0.0.1 | `backend/config/config.yml:22-23` |
| Qdrant gRPC | 19179 | 127.0.0.1 | `backend/pkg/database/qdrant/manager.go:74` |
| SurrealDB | 18000 | 127.0.0.1 | `backend/config/config.yml:50-51` |
| 微信侧车 | 19876 | 127.0.0.1 | `backend/internal/system/wechat_bridge_service.go:63` |
| QQ 侧车 | 19877 | 127.0.0.1 | `backend/cmd/server/main.go:138` |
| 前端 Dev | 5178 | 127.0.0.1 | `desktop/src/main/window.ts:12` |

> Android 内嵌模式必须保持所有端口仅监听 `127.0.0.1`，禁止 `0.0.0.0`，且避开 3000 端口（`AGENTS.md`）。

### 19.4 与 stage.md 第二节「已确认的 Amitia 后端依赖」对照

| stage.md 声明 | 实际扫描结果 | 差异 |
|---|---|---|
| 核心业务数据库使用 SQLite | ✅ 确认（`glebarez/sqlite` + GORM） | 无差异 |
| 当前没有 Redis 依赖 | ⚠️ `go.mod:11` 声明 `redis/go-redis/v9`，但代码搜索未见实际调用 | **待确认是否有未使用的依赖** |
| 向量数据库使用独立 Qdrant 可执行程序 | ✅ 确认 | 无差异 |
| 图数据库使用 SurrealDB | ✅ 确认 | 无差异 |
| 核心后端使用 Go | ✅ 确认（Go 1.26.1） | 无差异 |
| Windows 环境中 Qdrant 可能是 `.exe` | ✅ 确认（`qdrant.exe`） | 无差异 |
| Android 内嵌 Linux 中必须使用 Linux ARM64 版本 | ⚠️ 代码已支持 `qdrant_linux_aarch64`，但 SurrealDB Linux 未区分 ARM64/x86 | **需补充 SurrealDB ARM64 路径分支** |
| SurrealDB 同样必须使用 Linux ARM64 版本 | ⚠️ 同上 | 同上 |
| Amitia 是单用户、多角色系统 | ✅ 确认（`migration.CanonicalSingleUserMigration()`、`migration.UserProfileCharacterScopeMigration()`） | 无差异 |
| Android 客户端必须连接现有角色、聊天、记忆、模型、主动消息和渠道系统 | ✅ 全部后端模块存在 | 无差异 |

### 19.5 已识别的 P0 阻塞项

| 编号 | 阻塞项 | 影响 | 缓解 |
|---|---|---|---|
| P0-1 | `main.go:killExistingServer` 硬编码 Windows 命令 | Linux/Android 启动失败 | 抽象 `RuntimePlatform` 接口，Linux 实现 `lsof`/`fuser` + `kill` 或 PID 文件管理 |
| P0-2 | SurrealDB Linux 二进制未区分 ARM64 | Android ARM64 无法直接复用现有路径分支 | 补充 `runtime.GOARCH == "arm64"` 分支（参考 Qdrant 模式） |
| P0-3 | Redis 依赖在 `go.mod` 但未实际使用 | 增加交叉编译复杂度 | 评估是否可移除该依赖 |

### 19.6 待审计项（Phase 1+ 补充）

- `backend/internal/realtime/` 模块的 WebSocket 实现细节
- `front/src/runtime/runtime-adapter.ts` 的 `resolveApiUrl` 完整逻辑
- 各业务模块（character/memory/profile 等）的详细 API 路由与字段
- Qdrant/SurrealDB 真实版本号（需运行时获取）
- `redis/go-redis/v9` 是否被间接调用

---

## 附录 A：技术栈版本汇总

| 组件 | 版本 | 引用 |
|---|---|---|
| Go | 1.26.1 | `backend/go.mod:3` |
| Gin | v1.12.0 | `backend/go.mod:7` |
| GORM | v1.31.1 | `backend/go.mod:14` |
| glebarez/sqlite | v1.11.0 | `backend/go.mod:8` |
| Viper | v1.21.0 | `backend/go.mod:9`（推断） |
| Logrus | v1.9.4 | `backend/go.mod:10`（推断） |
| Google UUID | v1.6.0 | `backend/go.mod:8`（推断） |
| JWT v5 | v5.3.1 | `backend/go.mod:8`（推断） |
| Vue | 3.5.35 | `front/package.json:21` |
| Vue Router | ^4.3.0 | `front/package.json:23` |
| Pinia | ^2.1.0 | `front/package.json:20` |
| Element Plus | ^2.7.0 | `front/package.json:19` |
| Vite | ^5.2.0 | `front/package.json:31` |
| Vitest | ^2.0.0 | `front/package.json:32` |
| Axios | ^1.7.4 | `front/package.json:17` |
| Electron | ^43.0.0 | `desktop/package.json:27` |
| electron-builder | ^26.15.3 | `desktop/package.json:28` |
| electron-updater | ^6.8.9 | `desktop/package.json:20` |

## 附录 B：端口与监听地址汇总

| 服务 | 端口 | 监听地址 | 引用 |
|---|---|---|---|
| Go 后端 | 18899 | 127.0.0.1 | `backend/config/config.yml:2-3` |
| Qdrant HTTP | 19178 | 127.0.0.1 | `backend/config/config.yml:22-23` |
| Qdrant gRPC | 19179 | 127.0.0.1 | `backend/pkg/database/qdrant/manager.go:74` |
| SurrealDB | 18000 | 127.0.0.1 | `backend/config/config.yml:50-51` |
| 微信侧车 | 19876 | 127.0.0.1 | `backend/internal/system/wechat_bridge_service.go:63` |
| QQ 侧车 | 19877 | 127.0.0.1 | `backend/cmd/server/main.go:138` |
| 前端 Dev | 5178 | 127.0.0.1 | `desktop/src/main/window.ts:12` |

## 附录 C：关键文件清单

### C.1 Go 后端

| 文件 | 职责 |
|---|---|
| `backend/cmd/server/main.go` | 入口、启动流程、Windows 专属代码 |
| `backend/cmd/server/router.go` | 路由注册 |
| `backend/cmd/server/services.go` | AppServices 装配 |
| `backend/config/config.go` | 配置结构与 viper 初始化 |
| `backend/config/config.yml` | 默认配置 |
| `backend/pkg/database/mysql/db.go` | SQLite 连接 |
| `backend/pkg/database/qdrant/manager.go` | Qdrant 启动管理 |
| `backend/pkg/database/surrealdb/manager.go` | SurrealDB 启动管理 |
| `backend/internal/migration/runner.go` | 迁移框架 |
| `backend/internal/migration/migrations.go` | 迁移列表 |
| `backend/internal/system/stream_handler.go` | SSE 流式响应 |
| `backend/internal/system/messages_events_handler.go` | 消息事件总线 |
| `backend/internal/system/message_event_bus.go` | 事件类型定义 |
| `backend/internal/system/router.go` | system 模块路由 |
| `backend/internal/system/health_voice_router.go` | 健康与语音路由 |
| `backend/internal/system/webchat_handler.go` | Web 聊天处理器 |
| `backend/internal/desktoppet/processing/handler.go` | 桌宠处理任务 |
| `backend/internal/desktoppet/handler.go` | 桌宠任务事件流 |
| `backend/internal/mcp/transport/process_windows.go` | Windows 进程管理 |
| `backend/internal/mcp/transport/process_unix.go` | Unix 进程管理 |
| `backend/internal/asr/asr.go` | ASR 服务 |
| `backend/internal/chat/handler.go` | 聊天处理器 |

### C.2 Web 前端

| 文件 | 职责 |
|---|---|
| `front/package.json` | 依赖与脚本 |
| `front/src/composables/useChatSSE.ts` | 消息流 SSE 客户端 |
| `front/src/composables/useWebChatSSE.ts` | Web 聊天 SSE 客户端 |
| `front/src/composables/useChat.ts` | 聊天发送（流式） |
| `front/src/composables/useProcessingTask.ts` | 桌宠处理任务 SSE |
| `front/src/router/index.ts` | 路由配置 |
| `front/src/runtime/runtime-adapter.ts` | API 地址解析 |

### C.3 Electron 桌面端

| 文件 | 职责 |
|---|---|
| `desktop/package.json` | 依赖与脚本 |
| `desktop/electron-builder.yml` | 构建配置 |
| `desktop/src/main/index.ts` | 主进程入口 |
| `desktop/src/main/core-manager.ts` | 核心进程管理 |
| `desktop/src/main/path-manager.ts` | 数据目录管理 |
| `desktop/src/main/tray.ts` | 系统托盘 |
| `desktop/src/main/window.ts` | 窗口创建 |
| `desktop/src/main/update-manager.ts` | 自动更新 |
| `desktop/src/main/ipc-handlers.ts` | IPC 处理器 |
| `desktop/src/main/pet/manager.ts` | 桌宠管理 |
| `desktop/src/runtime/runtime-manager.ts` | 运行时状态机 |

---

**审计完成。下一步进入 Phase 0 Task 0.4 / 0.5：生成迁移矩阵与运行时依赖审计。**
