# Amitia 运行时依赖审计（Phase 0 / Task 0.5）

> 来源：`AndroidAPP/stage.md` 第六节、第九节、第二十二节
> 审计范围：Go 后端、SQLite、Qdrant、SurrealDB 在 Android Linux ARM64 + PRoot 环境下的运行时依赖
> 引用：每项结论均给出 `file_path:line`
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. Go 后端 Linux ARM64 兼容性

### 1.1 交叉编译可行性

| 项 | 结论 | 引用 |
|---|---|---|
| Go 版本 | `go 1.26.1`（支持 `GOOS=linux GOARCH=arm64`） | `backend/go.mod:3` |
| 模块路径 | `github.com/u-ai/backend` | `backend/go.mod:1` |
| CGO 是否启用 | **否** | `backend/pkg/database/mysql/db.go:22`（`glebarez/sqlite` 纯 Go） |
| SQLite 驱动 | `github.com/glebarez/sqlite` v1.11.0（基于 `modernc.org/sqlite`，纯 Go SSA 解析） | `backend/go.mod:8`、`backend/pkg/database/mysql/db.go:22` |
| 是否依赖 cgo 库 | 否 | `backend/go.mod` 全文扫描未见 cgo 依赖 |
| 是否依赖 Windows 系统 DLL | 仅 `golang.org/x/sys/windows`，由 `//go:build windows` 标签隔离 | `backend/internal/mcp/transport/process_windows.go:1, 10` |

### 1.2 交叉编译命令预期

```bash
cd backend
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o amitia-backend-arm64 ./cmd/server
```

预期结果：

- 生成 `amitia-backend-arm64` Linux ARM64 二进制
- 体积预估 30-50 MB（Go 静态链接 + glebarez/sqlite + modernc.org/sqlite）
- 无任何外部 .so 依赖（纯静态）

### 1.3 平台相关代码兼容性

| 文件 | 平台分支 | Linux ARM64 兼容性 | 引用 |
|---|---|---|---|
| `backend/cmd/server/main.go:38-56` | **无平台分支（硬编码 Windows）** | **不兼容** | `main.go:46, 51` |
| `backend/pkg/database/qdrant/manager.go:34-60, 78-89` | `runtime.GOOS` switch | 兼容（已含 `linux` 分支） | `manager.go:43-47, 78-89` |
| `backend/pkg/database/surrealdb/manager.go:44-70, 88-95` | `runtime.GOOS` switch | 部分兼容（Linux 未区分 ARM64） | `manager.go:53-57, 88-95` |
| `backend/internal/mcp/transport/process_windows.go` | `//go:build windows` | 兼容（构建标签隔离） | `process_windows.go:1` |
| `backend/internal/mcp/transport/process_unix.go` | `//go:build !windows` | 兼容（自动选用） | `process_unix.go:1` |

### 1.4 平台抽象需求

依据 stage.md 9.1 节，需将平台相关逻辑抽象为：

```go
type RuntimePlatform interface {
    KillExistingProcess(port int) error
    ExecutableSuffix() string
    QdrantBinaryName() string
    SurrealBinaryName() string
}

type DesktopRuntime struct{}        // Windows 实现（保留 cmd/taskkill + .exe）
type AndroidEmbeddedRuntime struct{} // Linux ARM64 实现（lsof/fuser + kill + 无后缀）
type ServerRuntime struct{}         // 远程部署实现
```

**禁止**：复制一份完整后端形成 Android 专属分叉（stage.md 9.1 节）。

### 1.5 环境变量依赖

| 变量 | 用途 | Android 注入方式 | 引用 |
|---|---|---|---|
| `CONFIG_PATH` | 配置文件目录 | 通过 `os.Setenv` 或 PRoot 环境注入 Android 私有目录 | `backend/cmd/server/main.go:61-66` |
| `AMITIA_RUN_MODE` | 运行模式标记 | 注入 `android-embedded` | `desktop/src/main/core-manager.ts:130`（参考） |
| `AMITIA_DATA_DIR` | 数据根目录 | 注入 `<files>/amitia-data/` | `desktop/src/main/core-manager.ts:131`（参考） |
| viper `AutomaticEnv()` | 所有配置项可通过环境变量覆盖 | 注入 `SERVER_PORT=18899`、`QDRANT_PORT=19178`、`SURREALDB_PORT=18000`、`SURREALDB_DATAPATH=<absPath>` 等 | `backend/config/config.go:144` |

### 1.6 文件资源依赖

| 资源 | 用途 | Android 路径 | 引用 |
|---|---|---|---|
| `<DataDir>/sql.sql` | 初始建表脚本 | `<files>/amitia-data/data/sql.sql` | `backend/cmd/server/main.go:291` |
| `<RuntimeRoot>/qdrant/qdrant.zip` | Qdrant 二进制压缩包 | `<files>/runtime/rootfs/qdrant/qdrant.zip` 或预先解压 | `backend/pkg/database/qdrant/manager.go:116` |
| `<RuntimeRoot>/surrealdb/surreal.zip` | SurrealDB 二进制压缩包 | `<files>/runtime/rootfs/surrealdb/surreal.zip` 或预先解压 | `backend/pkg/database/surrealdb/manager.go:206` |
| `<RuntimeRoot>/config/config.yml` | 配置文件（可选，可用环境变量覆盖） | `<files>/amitia-data/config/config.yml` | `backend/cmd/server/main.go:62-66`、`backend/config/config.go:93-97` |

### 1.7 Go 后端兼容性结论

| 项 | 结论 |
|---|---|
| 交叉编译 | ✅ 可行（无 CGO，纯 Go） |
| Linux ARM64 二进制 | ✅ 可生成（需 P0-1 修复后） |
| 平台抽象 | ⚠️ 需重构 `killExistingServer` |
| 配置加载 | ✅ 兼容（viper + 环境变量） |
| 文件路径 | ✅ 兼容（filepath.Join + util.RuntimeRoot） |
| SQLite | ✅ 兼容（glebarez/sqlite 纯 Go） |
| 网络监听 | ✅ 兼容（127.0.0.1） |

---

## 2. SQLite Linux ARM64 兼容性

### 2.1 驱动与底层

| 项 | 值 | 引用 |
|---|---|---|
| 驱动 | `github.com/glebarez/sqlite` v1.11.0 | `backend/go.mod:8` |
| 底层实现 | `modernc.org/sqlite`（纯 Go SSA 解析 SQLite 文件格式，无 cgo） | 间接依赖 |
| SQLite 版本 | 由 `modernc.org/sqlite` 内嵌（通常为 SQLite 3.40+） | — |
| ARM64 支持 | ✅ 纯 Go 实现天然支持 ARM64 | `glebarez/sqlite` 文档 |

### 2.2 数据库连接

| 项 | 值 | 引用 |
|---|---|---|
| 连接初始化 | `mysql.NewSQLite(dataDir)` | `backend/cmd/server/main.go:77`、`backend/pkg/database/mysql/db.go:14` |
| 数据库文件 | `<DataDir>/app.db` | `backend/pkg/database/mysql/db.go:17` |
| 连接池 | `SetMaxIdleConns(10)`、`SetMaxOpenConns(1)`、`SetConnMaxLifetime(time.Hour)` | `backend/pkg/database/mysql/db.go:32-34` |
| 写权限校验 | 启动时通过 `os.MkdirAll` 创建目录 | `backend/pkg/database/mysql/db.go:15` |

### 2.3 迁移兼容性

| 项 | 值 | 引用 |
|---|---|---|
| 迁移框架 | `migration.Runner` + `DefaultMigrations()` | `backend/internal/migration/runner.go:62-80`、`backend/internal/migration/migrations.go:77-96` |
| 初始建表 | `migration.ApplyInitialSQLFile(db, sqlPath)` | `backend/cmd/server/main.go:298` |
| 迁移记录表 | `migration_records` | `backend/internal/migration/runner.go:18-29` |
| 备份机制 | `CreatePreMigrationBackup`（仅对已有库） | `backend/cmd/server/main.go:268-272`、`backend/internal/migration/backup_test.go:46-65` |
| 测试覆盖 | 多个测试用 `glebarez/sqlite` 在临时目录或 `:memory:` 验证迁移 | `backend/internal/migration/backup_test.go:14`、`backend/internal/migration/extensions_test.go:15` |

### 2.4 Android 路径与权限

| 项 | Android 路径 | 引用 |
|---|---|---|
| 数据根目录 | `<files>/amitia-data/sqlite/` | stage.md 第八节 8.1 |
| 数据库文件 | `<files>/amitia-data/sqlite/app.db` | — |
| 建表脚本 | `<files>/amitia-data/data/sql.sql`（需打包随 APK 分发） | `backend/cmd/server/main.go:291` |
| 写权限 | Android 应用私有目录默认可写 | `backend/pkg/database/mysql/db.go:15` |

### 2.5 并发与文件锁

| 项 | 值 | 引用 |
|---|---|---|
| SQLite 文件锁 | 现代 SQLite 默认 WAL 模式（`modernc.org/sqlite` 默认开启） | — |
| Go 后端连接池 | `SetMaxOpenConns(1)`（避免并发写冲突） | `backend/pkg/database/mysql/db.go:33` |
| Linux 文件锁 | 通过 `flock` 系统调用（`modernc.org/sqlite` 内置） | — |
| Android 兼容 | ✅ Linux ARM64 原生支持 `flock` | — |

### 2.6 异常退出与损坏检测

| 项 | 值 | 引用 |
|---|---|---|
| WAL 模式 | 默认开启（提升并发与崩溃恢复） | `modernc.org/sqlite` 默认 |
| 异常退出恢复 | SQLite 自动检查点（checkpoint） | — |
| 损坏检测 | `sqlDB.Ping()` 启动时验证 | `backend/pkg/database/mysql/db.go:38-40` |
| 备份恢复 | `migration.Runner.CreatePreMigrationBackup` 提供迁移前备份 | `backend/cmd/server/main.go:268` |

### 2.7 SQLite 兼容性结论

| 项 | 结论 |
|---|---|
| Linux ARM64 兼容性 | ✅ 完全兼容（纯 Go 驱动） |
| 数据持久化 | ✅ Android 私有目录可写 |
| 文件锁 | ✅ Linux 原生支持 |
| 并发访问 | ✅ `SetMaxOpenConns(1)` 避免冲突 |
| 异常退出恢复 | ✅ WAL 模式自动恢复 |
| 数据迁移 | ✅ 现有 SQLite 文件可直接复制到 Android |

### 2.8 Phase 3 实测验证结论（2026-07-26）

| 项 | 结论 | 证据 |
|---|---|---|
| 驱动版本 | `github.com/glebarez/sqlite v1.11.0` + `modernc.org/sqlite v1.23.1`（间接依赖） | `backend/go.mod`、`backend/go.sum` |
| 是否纯 Go | ✅ 是（基于 modernc.org/sqlite 的 SSA 解析实现，无 cgo） | 依赖树扫描未见 `mattn/go-sqlite3` 等 cgo 驱动 |
| Linux ARM64 交叉编译 | ✅ 通过 | `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o amitia-backend-arm64 ./cmd/server` 退出码 0，产物 54.63 MB |
| Windows 编译验证 | ✅ 通过 | `go build -o test-server-windows.exe ./cmd/server` 退出码 0，产物 59.18 MB（已删除） |
| ARM64 二进制可运行 | ⏳ 待 Phase 7 真机验证 | 当前仅完成交叉编译产物，未在 ARM64 Linux/PRoot 环境实机启动 |
| SQLite 数据库读写 | ⏳ 待 Phase 7 真机验证 | 需在 Android 真机 PRoot 中实际触发迁移与查询 |

---

## 3. Qdrant Linux ARM64 获取或编译方式

### 3.1 当前 Qdrant 版本与可执行文件

| 项 | 值 | 引用 |
|---|---|---|
| Qdrant 版本号 | **无法确认**（代码未打印版本，由 `qdrant.zip` 解压提供） | `backend/pkg/database/qdrant/manager.go:111-131` |
| Windows 二进制 | `<RuntimeRoot>/qdrant/qdrant.exe` | `backend/pkg/database/qdrant/manager.go:80` |
| Linux ARM64 二进制 | `<RuntimeRoot>/qdrant/qdrant_linux_aarch64` | `backend/pkg/database/qdrant/manager.go:83` |
| Linux x86 二进制 | `<RuntimeRoot>/qdrant/qdrant_linux_x86` | `backend/pkg/database/qdrant/manager.go:85` |
| 压缩包获取 | `qdrant.exe.zip` 或 `qdrant.zip` 自动解压 | `backend/pkg/database/qdrant/manager.go:116-128`、`AGENTS.md` Git 上传规则 |

### 3.2 启动参数

| 项 | 值 | 引用 |
|---|---|---|
| 启动命令 | `qdrant --config-path <config.yaml>` | `backend/pkg/database/qdrant/manager.go:97` |
| 配置文件生成 | 运行时生成 `<qdrantDir>/config/config.yaml` | `backend/pkg/database/qdrant/manager.go:69-75` |
| 配置内容 | `service.http_port: <port>`、`service.grpc_port: <port+1>` | `backend/pkg/database/qdrant/manager.go:74` |
| 工作目录 | `<RuntimeRoot>/qdrant` | `backend/pkg/database/qdrant/manager.go:98` |
| HTTP 端口 | 19178 | `backend/config/config.yml:23` |
| gRPC 端口 | 19179（HTTP + 1） | `backend/pkg/database/qdrant/manager.go:74` |
| 监听地址 | 127.0.0.1 | `backend/config/config.yml:22` |
| 健康检查 | `GET http://127.0.0.1:<port>/readyz` | `backend/pkg/database/qdrant/manager.go:152-167` |

### 3.3 数据目录与持久化

| 项 | 值 | 引用 |
|---|---|---|
| 数据目录 | `<qdrantDir>/storage/`（Qdrant 默认相对工作目录） | `backend/pkg/database/qdrant/manager.go:98` 隐含 |
| 集合配置 | `memory_embeddings`、`working_memory`、`user_profiles`、`episodic_memories`、`amitia_emotes`（向量维度 1536） | `backend/config/config.yml:27-42`、`backend/config/config.go:115-124` |
| 默认向量维度 | 2560（viper 默认值，与 `config.yml` 的 1536 冲突，需注意） | `backend/config/config.go:113, 116-124`、`backend/config/config.yml:25, 30, 33, 36, 39, 42` |
| 集合初始化 | `EnsureCollections()` | `backend/cmd/server/main.go:325` |
| 重启数据保留 | ✅ 数据目录持久化 | — |

### 3.4 Linux ARM64 二进制获取方式

#### 方式 A：官方预编译二进制

| 项 | 值 |
|---|---|
| 官方下载页 | https://github.com/qdrant/qdrant/releases |
| 目标资产 | `qdrant-x86_64-unknown-linux-gnu.tar.gz` 或 `qdrant-aarch64-unknown-linux-gnu.tar.gz` |
| 安装路径 | 解压后放入 `<RuntimeRoot>/qdrant/qdrant_linux_aarch64` |
| 验证 | `./qdrant_linux_aarch64 --version` |

#### 方式 B：从源码编译

```bash
git clone https://github.com/qdrant/qdrant.git
cd qdrant
# 在 Linux ARM64 主机或交叉编译工具链下
cargo build --release --target aarch64-unknown-linux-gnu
# 产物路径：target/aarch64-unknown-linux-gnu/release/qdrant
```

#### 方式 C：复用现有 `qdrant.zip`

依据 `AGENTS.md` Git 上传规则，仓库内 `qdrant.zip` 可能包含 Windows 二进制，需单独打包 Linux ARM64 版本。

### 3.5 PRoot 兼容性验证项

| 验证项 | 验证方法 |
|---|---|
| 二进制可执行 | `./qdrant_linux_aarch64 --version` 在 PRoot Linux 中运行 |
| 端口监听 | `netstat -tlnp \| grep 19178` |
| 健康检查 | `curl http://127.0.0.1:19178/readyz` 返回 200 |
| 数据持久化 | 写入向量后重启 Runtime，验证数据存在 |
| Go 后端连接 | 后端日志 `Qdrant就绪，向量检索功能已启用`（`backend/cmd/server/main.go:331`） |
| ARM64 真机 | 在 ARM64 Android 真机 PRoot 中验证 |
| 兼容降级 | 若 PRoot 限制无法运行，尝试兼容构建参数或降级版本 |
| VectorStore Adapter | 评估在不破坏接口前提下提供可替换 Adapter（stage.md 9.3 节） |

### 3.6 Qdrant 失败处理

| 场景 | 后端行为 | 引用 |
|---|---|---|
| 启动失败 | 仅记录日志，回退到关键词搜索（不退出主进程） | `backend/cmd/server/main.go:308-312` |
| 健康检查超时 | 调用 `StopQdrant()` 后回退 | `backend/cmd/server/main.go:313-318` |
| 客户端初始化失败 | 同上 | `backend/cmd/server/main.go:319-324` |
| 集合创建失败 | 同上 | `backend/cmd/server/main.go:325-330` |
| 强制终止 | 超时 5 秒后 `Process.Kill()` | `backend/pkg/database/qdrant/manager.go:146` |

### 3.7 Qdrant 兼容性结论

| 项 | 结论 |
|---|---|
| Linux ARM64 路径支持 | ✅ 已支持（`runtime.GOOS == "linux"` + `runtime.GOARCH == "arm64"` 分支） |
| 二进制获取 | ⚠️ 需从官方获取或从源码编译 |
| PRoot 兼容性 | ⚠️ 需真机验证 |
| 数据持久化 | ✅ 工作目录持久化 |
| 端口冲突 | ✅ `killExistingQdrant` 已平台抽象 |
| 失败回退 | ✅ 关键词搜索降级 |

### 3.8 Phase 3 实测验证结论（2026-07-26）

| 项 | 结论 | 证据 |
|---|---|---|
| 二进制下载 | ✅ 成功 | `https://github.com/qdrant/qdrant/releases/latest/download/qdrant-aarch64-unknown-linux-musl.tar.gz`，27.44 MB |
| 解压后大小 | ✅ 74.23 MB（111.03 MB 与 surreal 不同，Qdrant 静态 musl 较小） | `android/app/src/main/assets/qdrant_linux_aarch64` |
| ELF 头验证 | ✅ 通过 | `7F 45 4C 46 02 01 01 00 ... 03 00 B7 00`，ELFCLASS64 + ELFDATA2LSB + EM_AARCH64 (0xB7) |
| 静态链接 | ✅ musl 静态版（无 glibc 依赖，Android PRoot 友好） | 资产名 `qdrant-aarch64-unknown-linux-musl` |
| 二进制存放位置 | `android/app/src/main/assets/qdrant_linux_aarch64` | 后端 `qdrant/manager.go` 已支持该命名 |
| 后端代码兼容 | ✅ 已增强 | `backend/pkg/database/qdrant/manager.go` 新增 `IsLinuxARM64()` + fallback `qdrant` + 多 zip 包名支持 |
| PRoot 实机运行 | ⏳ 待 Phase 7 真机验证 | 当前仅完成静态文件验证 |

---

## 4. SurrealDB Linux ARM64 获取或编译方式

### 4.1 当前 SurrealDB 版本与可执行文件

| 项 | 值 | 引用 |
|---|---|---|
| SurrealDB 版本号 | **无法确认**（代码未打印版本，由 `surreal.zip` 解压提供） | `backend/pkg/database/surrealdb/manager.go:201-221` |
| Windows 二进制 | `<RuntimeRoot>/surrealdb/surreal.exe` | `backend/pkg/database/surrealdb/manager.go:90` |
| Linux 二进制 | `<RuntimeRoot>/surrealdb/surreal`（**未区分 ARM64/x86**） | `backend/pkg/database/surrealdb/manager.go:92` |
| 压缩包获取 | `surreal.exe.zip` 或 `surreal.zip` 自动解压 | `backend/pkg/database/surrealdb/manager.go:206-218`、`AGENTS.md` Git 上传规则 |

### 4.2 启动参数

| 项 | 值 | 引用 |
|---|---|---|
| 启动命令 | `surreal start --log info --user <user> --pass <pass> --bind <addr> <storage>` | `backend/pkg/database/surrealdb/manager.go:112-118` |
| 日志级别 | `info` | `backend/pkg/database/surrealdb/manager.go:113` |
| 用户名 | `root`（默认） | `backend/config/config.yml:54`、`backend/config/config.go:132` |
| 密码 | `root`（默认） | `backend/config/config.yml:55`、`backend/config/config.go:133` |
| 绑定地址 | `127.0.0.1:18000` | `backend/pkg/database/surrealdb/manager.go:103` |
| 存储引擎 | `surrealkv:<absPath>`（基于路径前缀） | `backend/pkg/database/surrealdb/manager.go:105-110` |
| 工作目录 | `<RuntimeRoot>/surrealdb` | `backend/pkg/database/surrealdb/manager.go:119` |
| 健康检查 | `GET http://127.0.0.1:<port>/health` | `backend/pkg/database/surrealdb/manager.go:246-261` |

### 4.3 数据目录与持久化

| 项 | 值 | 引用 |
|---|---|---|
| 数据路径配置 | `data/graph.db`（相对 `RuntimeRoot`） | `backend/config/config.yml:56` |
| 实际存储路径 | `surrealkv:<RuntimeRoot>/data/graph.db` | `backend/pkg/database/surrealdb/manager.go:106-109` |
| Namespace | `uai` | `backend/config/config.yml:52`、`backend/config/config.go:130` |
| Database | `memory_graph` | `backend/config/config.yml:53`、`backend/config/config.go:131` |
| 重启数据保留 | ✅ 数据目录持久化 | — |

### 4.4 Linux ARM64 二进制获取方式

#### 方式 A：官方预编译二进制

| 项 | 值 |
|---|---|
| 官方下载页 | https://github.com/surrealdb/surrealdb/releases |
| 目标资产 | `surreal-aarch64-unknown-linux-gnu.tar.gz` |
| 安装路径 | 解压后放入 `<RuntimeRoot>/surrealdb/surreal`（注意：当前代码不区分 ARM64） |
| 验证 | `./surreal version` |

#### 方式 B：从源码编译

```bash
git clone https://github.com/surrealdb/surrealdb.git
cd surrealdb
# 在 Linux ARM64 主机或交叉编译工具链下
cargo build --release --target aarch64-unknown-linux-gnu
# 产物路径：target/aarch64-unknown-linux-gnu/release/surreal
```

#### 方式 C：复用现有 `surreal.zip`

依据 `AGENTS.md` Git 上传规则，仓库内 `surreal.zip` 可能包含 Windows 二进制，需单独打包 Linux ARM64 版本。

### 4.5 代码修改需求（P0-2）

**当前问题**：`backend/pkg/database/surrealdb/manager.go:88-95` 的 Linux 分支不区分 ARM64/x86：

```go
case "linux":
    surrealPath = filepath.Join(surrealDir, "surreal")
```

**建议修改**（参考 Qdrant 模式 `qdrant/manager.go:78-89`）：

```go
case "linux":
    if runtime.GOARCH == "arm64" {
        surrealPath = filepath.Join(surrealDir, "surreal_linux_aarch64")
    } else {
        surrealPath = filepath.Join(surrealDir, "surreal_linux_x86")
    }
```

### 4.6 PRoot 兼容性验证项

| 验证项 | 验证方法 |
|---|---|
| 二进制可执行 | `./surreal version` 在 PRoot Linux 中运行 |
| 端口监听 | `netstat -tlnp \| grep 18000` |
| 健康检查 | `curl http://127.0.0.1:18000/health` 返回 200 |
| 存储引擎可用 | 写入图数据后查询 |
| 数据目录持久化 | 写入数据后重启 Runtime，验证数据存在 |
| 端口可访问 | Go 后端日志 `SurrealDB就绪，图谱功能已启用`（`backend/cmd/server/main.go:348`） |
| 认证配置 | 用户名/密码 `root`/`root` 验证 |
| Go 后端可连接 | `graph.NewClient(cfg)` 成功（`backend/cmd/server/main.go:355`） |
| 应用重启后数据存在 | 重启 Runtime 后查询数据 |
| 异常退出后可恢复 | 强制终止后重启，验证数据完整 |
| ARM64 真机 | 在 ARM64 Android 真机 PRoot 中验证 |

### 4.7 SurrealDB 失败处理

| 场景 | 后端行为 | 引用 |
|---|---|---|
| 启动失败 | 仅记录日志，图谱功能不可用（不退出主进程） | `backend/cmd/server/main.go:337-340` |
| 健康检查超时 | 调用 `StopSurreal()` 后回退 | `backend/cmd/server/main.go:341-345` |
| 进程异常 | 10 秒轮询监控，自动重启（`StartSurrealMonitor`） | `backend/pkg/database/surrealdb/manager.go:143-192` |
| 重启回调 | 重建 graph 服务（`surrealRestartFn`） | `backend/pkg/database/surrealdb/manager.go:27-29`、`backend/cmd/server/main.go:114-120` |
| 强制终止 | 超时 5 秒后 `Process.Kill()` | `backend/pkg/database/surrealdb/manager.go:240` |

### 4.8 SurrealDB 兼容性结论

| 项 | 结论 |
|---|---|
| Linux ARM64 路径支持 | ⚠️ **未区分 ARM64/x86（P0-2）**，需补充分支 |
| 二进制获取 | ⚠️ 需从官方获取或从源码编译 |
| PRoot 兼容性 | ⚠️ 需真机验证 |
| 数据持久化 | ✅ 数据目录持久化 |
| 自动重启 | ✅ 监控协程已实现 |
| 端口冲突 | ✅ `killExistingSurreal` 已平台抽象 |
| 失败回退 | ✅ 图谱功能不可用（不谎报全部正常） |

### 4.9 Phase 3 实测验证结论（2026-07-26）

| 项 | 结论 | 证据 |
|---|---|---|
| 二进制下载 | ✅ 成功 | `https://github.com/surrealdb/surrealdb/releases/download/v3.2.0/surreal-v3.2.0.linux-arm64.tgz`，45.97 MB |
| 解压后大小 | ✅ 111.03 MB（gnu 动态版，依赖 glibc） | `android/app/src/main/assets/surreal_linux_aarch64` |
| ELF 头验证 | ✅ 通过 | `7F 45 4C 46 02 01 01 00 ... 03 00 B7 00`，ELFCLASS64 + ELFDATA2LSB + EM_AARCH64 (0xB7) |
| 版本 | SurrealDB v3.2.0（GitHub Release latest） | GitHub API tag_name |
| 后端代码兼容 | ✅ 已修复 P0-2 | `backend/pkg/database/surrealdb/manager.go` 新增 `IsLinuxARM64()` + ARM64/x86 分支 + fallback `surreal` + 多 zip 包名支持 |
| 二进制存放位置 | `android/app/src/main/assets/surreal_linux_aarch64` | 与后端代码命名约定一致 |
| PRoot 实机运行 | ⏳ 待 Phase 7 真机验证 | 当前仅完成静态文件验证；若 glibc 不兼容 PRoot，需改用 musl 版本或源码编译 |
| 启动参数兼容 | ✅ 跨平台一致 | `surreal start --log info --user root --pass root --bind 127.0.0.1:18000 surrealkv:<dataPath>` |

---

## 5. RootFS 体积

### 5.1 RootFS 组成预估

| 组件 | 预估体积 | 说明 |
|---|---|---|
| 精简 Ubuntu/Debian ARM64 用户空间 | 50-100 MB | 最小化安装（不含 GUI） |
| Go 后端二进制 | 30-50 MB | 静态链接 |
| Qdrant Linux ARM64 二进制 | 30-50 MB | Rust 静态链接 |
| SurrealDB Linux ARM64 二进制 | 30-50 MB | Rust 静态链接 |
| SQLite 数据文件 | < 10 MB | 取决于数据量 |
| Python Runtime（预留） | 50-100 MB | stage.md 3.2 节预留 |
| Node.js Runtime（预留） | 30-50 MB | stage.md 3.2 节预留 |
| Shell + 基础工具 | 5-10 MB | bash、coreutils 等 |
| 配置文件 | < 1 MB | config.yml + sql.sql |
| **总计** | **230-420 MB** | 不含未来 MCP/Skill/AmitiaX |

### 5.2 体积控制策略

依据 stage.md 第二十二节：

| 策略 | 实施 |
|---|---|
| 精简 RootFS | 使用 `debootstrap --variant=minbase` 或 Alpine Linux（更小，约 5 MB 基础） |
| 二进制静态链接 | Go 与 Rust 都默认静态链接 |
| 压缩分发 | RootFS 使用 `tar.gz` 或 `squashfs` 压缩，预期压缩后 80-150 MB |
| 分离用户数据 | `files/runtime/rootfs/` 与 `files/amitia-data/` 分离（stage.md 8.1 节） |
| 按需下载 | 首次启动时下载 RootFS 包，APK 本身只含 Android 端代码 |
| Hash 校验 | RootFS 更新包校验 Hash（stage.md 第二十一节） |

### 5.3 APK 体积预估

| 项 | 预估体积 |
|---|---|
| Android 原生代码 + 资源 | 10-30 MB |
| 内嵌 RootFS（首次下载） | 0（首次启动时下载） |
| PRoot 二进制 | 5-10 MB |
| **APK 总体积** | **20-50 MB** |

---

## 6. 后端及数据库预计内存

### 6.1 内存预估

| 组件 | 预估内存 | 说明 |
|---|---|---|
| Go 后端 | 50-150 MB | 取决于会话数与缓冲 |
| SQLite | 10-50 MB | `SetMaxIdleConns(10)` + `SetMaxOpenConns(1)` + 缓存 |
| Qdrant | 100-300 MB | 取决于向量数量与 HNSW 索引 |
| SurrealDB | 50-150 MB | 取决于图数据规模 |
| PRoot 转译开销 | 10-30% | PRoot 用户空间转译会增加内存与 CPU 开销 |
| Linux Runtime 进程开销 | 10-20 MB | bash + init 进程 |
| **总计** | **250-700 MB** | 不含 Android 系统开销 |

### 6.2 内存控制策略

| 策略 | 实施 |
|---|---|
| Qdrant 内存限制 | 通过 `config.yaml` 设置 `service.max_workers`、`storage.performance_mmap_threshold` 等 |
| SurrealDB 内存限制 | 通过启动参数限制 |
| Go 后端 GOGC | 设置 `GOGC=50` 降低 GC 阈值 |
| 运行策略 | AlwaysOn / OnDemand / RemoteOnly（stage.md 第二十二节） |
| 默认策略 | 不盲目 AlwaysOn，根据实测决定 |

### 6.3 Android 资源限制

| 限制 | 影响 | 缓解 |
|---|---|---|
| 应用堆内存限制（通常 256-512 MB） | Go 后端 + 数据库可能超限 | 使用 `largeHeap=true` + Native 进程不受 Java 堆限制 |
| 低内存设备（< 4 GB RAM） | 整体可能不可用 | 默认推荐 6 GB+ RAM 设备 |
| Doze 模式 | 后台 Linux 进程可能被冻结 | Foreground Service 维持运行 |
| 系统杀进程 | Runtime 数据需保护 | 数据目录与 RootFS 分离，重启可恢复 |

---

## 7. 端口冲突风险

### 7.1 当前端口占用

| 服务 | 端口 | 监听地址 | 引用 |
|---|---|---|---|
| Go 后端 HTTP | 18899 | 127.0.0.1 | `backend/config/config.yml:2-3` |
| Qdrant HTTP | 19178 | 127.0.0.1 | `backend/config/config.yml:22-23` |
| Qdrant gRPC | 19179 | 127.0.0.1 | `backend/pkg/database/qdrant/manager.go:74` |
| SurrealDB | 18000 | 127.0.0.1 | `backend/config/config.yml:50-51` |
| 微信侧车 | 19876 | 127.0.0.1 | `backend/internal/system/wechat_bridge_service.go:63` |
| QQ 侧车 | 19877 | 127.0.0.1 | `backend/cmd/server/main.go:138` |
| 前端 Dev | 5178 | 127.0.0.1 | `desktop/src/main/window.ts:12` |

### 7.2 Android 端口冲突风险

| 风险 | 评估 | 缓解 |
|---|---|---|
| 其他 Android 应用占用 18899 | 低（Android 不常用此端口） | 启动时检测，失败则提示用户 |
| 其他 Android 应用占用 19178/18000 | 低 | 同上 |
| 与系统服务冲突 | 低（系统服务通常用 < 1024 端口） | — |
| 局域网暴露 | ✅ 仅监听 127.0.0.1 | `backend/config/config.go:100` 默认 127.0.0.1 |
| 端口 3000 | ✅ 已避开 | `AGENTS.md` 项目规则 |

### 7.3 端口冲突检测与处理

| 项 | 值 | 引用 |
|---|---|---|
| Go 后端端口占用检测 | `killExistingServer(addr)` | `backend/cmd/server/main.go:38-56`（需平台抽象） |
| Qdrant 端口占用检测 | `killExistingQdrant(port)` | `backend/pkg/database/qdrant/manager.go:34-60`（已平台抽象） |
| SurrealDB 端口占用检测 | `killExistingSurreal(port)` | `backend/pkg/database/surrealdb/manager.go:44-70`（已平台抽象） |
| Android 内嵌模式行为 | 通过 PID 文件管理（无需 kill 旧进程，因为是首次启动） | Phase 2 实现 |

---

## 8. Android 后台限制

### 8.1 Android 后台限制概述

| 限制 | 影响 | 引用 |
|---|---|---|
| Doze 模式（Android 6+） | 后台进程网络与 CPU 受限 | stage.md 第十八节 |
| App Standby | 长期未使用的应用被限制 | — |
| 后台进程限制（Android 8+） | 后台服务无法长期运行 | stage.md 第十八节 |
| 系统杀进程（内存压力） | Linux Runtime 可能被杀 | stage.md 第二十二节 |
| 电池优化 | 后台耗电高的应用被限制 | stage.md 第二十二节 |

### 8.2 缓解策略

依据 stage.md 第十八节与第二十二节：

| 策略 | 实施 |
|---|---|
| Foreground Service | 显示常驻通知，保持 Linux Runtime 运行 |
| 运行策略 | AlwaysOn（前台服务）/ OnDemand（按需启动）/ RemoteOnly（仅远程） |
| 节能模式 | 提供低功耗模式（降低 Cron 频率、关闭主动消息） |
| 系统杀进程恢复 | 记录最后状态，重启后恢复 Runtime 状态 |
| 不高频轮询 | 禁止使用高频轮询维持假活跃 |
| 默认策略 | 根据实测决定，不盲目 AlwaysOn |
| 通知权限 | 引导用户开启通知权限 |
| 通知隐私 | 敏感消息不显示完整内容 |
| 电池优化白名单 | 引导用户加入电池优化白名单 |

### 8.3 进程保活实现

| 项 | 实施 |
|---|---|
| Foreground Service 类型 | `mediaPlayback` / `dataSync` / `connectedDevice`（根据 Android 14+ 规范） |
| 常驻通知 | 显示当前角色头像 + 状态文本 + 控制按钮 |
| 重启策略 | `START_STICKY` + 自启动广播 |
| 崩溃恢复 | 捕获 Native 崩溃 + 自动重启 |
| 日志记录 | 记录系统杀进程时间与原因 |

---

## 9. 文件权限问题

### 9.1 Android 应用私有目录权限

| 路径 | 权限 | 引用 |
|---|---|---|
| `<files>/`（应用私有目录） | 默认可读写，无需权限 | Android 规范 |
| `<files>/runtime/rootfs/` | Linux 文件系统权限（PRoot 内 0755/0644） | stage.md 8.1 节 |
| `<files>/amitia-data/sqlite/` | Linux 文件系统权限（PRoot 内 0755/0644） | stage.md 8.1 节 |
| `<files>/amitia-data/qdrant/` | 同上 | stage.md 8.1 节 |
| `<files>/amitia-data/surrealdb/` | 同上 | stage.md 8.1 节 |
| `<files>/amitia-data/uploads/` | 同上 | stage.md 8.1 节 |
| `<files>/amitia-data/models/` | 同上 | stage.md 8.1 节 |
| `<files>/amitia-data/extensions/` | 同上 | stage.md 8.1 节 |
| `<files>/amitia-data/backups/` | 同上 | stage.md 8.1 节 |

### 9.2 文件权限风险

| 风险 | 评估 | 缓解 |
|---|---|---|
| Linux 文件权限与 Android 权限不匹配 | 中 | PRoot 内使用标准 Linux 权限，Android 通过 RootFS 隔离 |
| 二进制可执行权限 | 高 | 解压后需 `chmod +x`（`util.UnzipFile` 是否保留权限需验证） |
| 跨进程文件访问 | 低 | 仅 Go 后端与数据库进程访问，无跨应用共享 |
| 外部存储访问 | 不需要 | 全部数据在私有目录 |

### 9.3 路径校验

依据 stage.md 第二十一节：

| 项 | 实施 |
|---|---|
| 文件路径校验 | 防止路径穿越（`..`）攻击 |
| 上传类型校验 | 限制 MIME 类型 |
| Deep Link 校验 | 防止恶意 URL 注入 |
| Bridge 请求权限校验 | PermissionBroker 校验 |
| 二进制 Hash 校验 | RootFS 更新包与二进制校验 |

---

## 10. `/proc`、`/dev`、网络和 DNS 需求

### 10.1 `/proc` 文件系统

| 项 | 需求 | PRoot 兼容性 |
|---|---|---|
| `/proc/self/` | 进程信息（PID、状态等） | PRoot 转译 |
| `/proc/<pid>/status` | 进程状态查询 | PRoot 转译 |
| `/proc/loadavg` | 系统负载 | PRoot 提供 |
| `/proc/meminfo` | 内存信息 | PRoot 提供 |
| `/proc/cpuinfo` | CPU 信息 | PRoot 提供 |

### 10.2 `/dev` 设备文件

| 项 | 需求 | PRoot 兼容性 |
|---|---|---|
| `/dev/null` | 标准输出重定向 | PRoot 提供 |
| `/dev/zero` | 零字节填充 | PRoot 提供 |
| `/dev/urandom` | 随机数生成 | PRoot 提供 |
| `/dev/random` | 随机数生成 | PRoot 提供 |
| `/dev/pts/` | 伪终端（如需 Shell） | PRoot 提供 |
| `/dev/net/` | 网络设备（不需要，PRoot 复用 Android 网络） | — |

### 10.3 网络需求

| 项 | 需求 | Android 兼容性 |
|---|---|---|
| IPv4 loopback | 127.0.0.1 监听 | ✅ Android 支持 |
| IPv6 loopback | ::1 监听（后端默认不用） | ✅ Android 支持 |
| TCP 客户端 | Go 后端调用外部 API（OpenAI、豆包等） | ✅ 通过 Android 网络栈 |
| HTTP 客户端 | 调用 LLM API、TTS API、ASR API | ✅ 通过 Android 网络栈 |
| 端口监听 | 18899、19178、18000 仅监听 127.0.0.1 | ✅ Android 不限制 loopback 监听 |

### 10.4 DNS 需求

| 项 | 需求 | Android 兼容性 |
|---|---|---|
| 域名解析 | Go 后端调用 `api.openai.com`、`openspeech.bytedance.com` 等 | ✅ 通过 Android DNS |
| `/etc/resolv.conf` | PRoot Linux 可能需要配置 | PRoot 通常自动配置 |
| `/etc/hosts` | 可能需要本地 hosts（如自定义 LLM 端点） | PRoot 可配置 |

### 10.5 文件系统挂载

| 挂载点 | 需求 | PRoot 实现 |
|---|---|---|
| `/proc` | 进程信息 | PRoot 自动挂载 |
| `/sys` | 系统信息 | PRoot 自动挂载（部分） |
| `/dev` | 设备文件 | PRoot 自动挂载 |
| `/tmp` | 临时文件 | PRoot 使用 `TMPDIR` 或绑定挂载 |
| `/etc` | 配置文件 | RootFS 内置 |

---

## 11. PRoot 环境兼容性

### 11.1 PRoot 技术路线选择

依据 stage.md 3.2 节，优先评估以下方案：

| 方案 | 优点 | 缺点 | 评估 |
|---|---|---|---|
| PRoot（C 语言版） | 成熟、广泛使用 | 性能开销较大（系统调用转译） | 优先评估 |
| proot-rs | Rust 实现、性能较好 | 较新，社区较小 | 次选 |
| libproot | 可嵌入 | 集成复杂 | 备选 |
| 自有 Linux Runtime | 完全控制 | 开发成本高 | 必要时使用 |
| NDK/JNI 桥接 | 原生性能 | 复杂、维护成本高 | 必要时结合 |

### 11.2 PRoot 验证项

| 验证项 | 验证方法 | 预期结果 |
|---|---|---|
| RootFS 解压 | `tar -xzf rootfs.tar.gz -C /data/data/<pkg>/files/runtime/rootfs/` | 文件完整、权限正确 |
| PRoot 启动 | `proot -r <rootfs> -b /proc -b /dev -b /sys /bin/sh` | 进入 Shell |
| 二进制执行 | `proot -r <rootfs> ./qdrant_linux_aarch64 --version` | 输出版本号 |
| 端口监听 | 启动 Qdrant/SurrealDB/Go 后端 | 端口可访问 |
| 性能 | 跑基准测试 | 性能开销 < 30% |
| 内存 | 监控内存占用 | 不超过设备限制 |
| ARM64 真机 | 在 ARM64 Android 真机验证 | 全部功能可用 |

### 11.3 PRoot 失败时的备选方案

依据 stage.md 3.2 节与 9.3 节：

| 失败场景 | 备选方案 |
|---|---|
| PRoot 性能不足 | 评估 proot-rs 或自有 Runtime |
| Qdrant 无法在 PRoot 中运行 | 评估直接原生运行（非 PRoot） |
| Qdrant 原生运行也失败 | 提供 VectorStore Adapter（可替换实现） |
| SurrealDB 无法运行 | 进入 Degraded 状态，不谎报全部能力正常 |
| 整体 PRoot 不可用 | 评估 Termux 集成（虽然 stage.md 不希望依赖 Termux） |

### 11.4 PRoot 已知限制

| 限制 | 影响 | 缓解 |
|---|---|---|
| 系统调用转译开销 | CPU 性能下降 5-30% | 优化热点路径 |
| 部分系统调用不支持 | 特定操作可能失败 | 评估兼容性 |
| 内存映射限制 | 大文件 mmap 可能失败 | 调整 Qdrant/SurrealDB 配置 |
| 进程数限制 | Android 进程数限制可能影响 | 控制子进程数量 |
| 信号处理 | PRoot 转译可能影响信号传递 | 测试 SIGTERM/SIGKILL |

---

## 12. 进程退出行为

### 12.1 正常退出流程

依据 stage.md 8.3 节：

| 步骤 | 实施 | 引用 |
|---|---|---|
| 1. 停止接受新请求 | `services.UnifiedEntry.SetOrchestratorReady(false)` | `backend/cmd/server/main.go:239` |
| 2. 关闭 Plugin Runtime | `services.Extension.Close(pluginShutdownCtx)` | `backend/cmd/server/main.go:241-243` |
| 3. 排水现有请求 | `srv.Shutdown(shutdownCtx)`（10 秒超时） | `backend/cmd/server/main.go:246-250` |
| 4. 停止 Go 后端 | `srv.ListenAndServe` 返回 | `backend/cmd/server/main.go:227` |
| 5. 停止 Qdrant | `qdrantDB.StopQdrant()` | `backend/cmd/server/main.go:97` |
| 6. 停止 SurrealDB | `surrealdbDB.StopSurreal()` | `backend/cmd/server/main.go:98` |
| 7. 刷新日志和状态 | `cleanup()` | `backend/cmd/server/main.go:93-100` |

### 12.2 异常退出处理

| 场景 | 后端行为 | Android 处理 |
|---|---|---|
| 收到 SIGTERM | 优雅关闭流程（10 秒排水） | Foreground Service 停止时发送 SIGTERM |
| 收到 SIGINT | 同上 | 不常见 |
| 强制终止（SIGKILL） | 立即退出，可能丢数据 | 系统杀进程时发生 |
| 崩溃 | 进程退出码非 0 | 捕获崩溃日志，自动重启 |
| 数据库锁死 | 启动迁移失败 → `os.Exit(1)` | 显示真实错误，提供重试 |
| Qdrant 启动失败 | 仅记录日志，回退关键词搜索 | Runtime 进入 Degraded 状态 |
| SurrealDB 启动失败 | 仅记录日志，图谱功能不可用 | Runtime 进入 Degraded 状态 |

### 12.3 进程退出码

| 退出码 | 含义 | 引用 |
|---|---|---|
| 0 | 正常退出 | — |
| 1 | 启动失败（数据库迁移失败 / 服务启动失败 / 关闭信号后错误） | `backend/cmd/server/main.go:84, 235, 257` |

### 12.4 Android 进程退出策略

| 项 | 实施 |
|---|---|
| 正常停止 | 通过 Foreground Service 停止 → 发送 SIGTERM → 等待 10 秒 → 强制停止 |
| 异常停止 | 捕获 Native 崩溃 → 记录日志 → 自动重启（最多 3 次） |
| 系统杀进程 | Android 在内存压力下可能杀进程 → 重启后从持久化状态恢复 |
| 数据保护 | 用户数据（`files/amitia-data/`）与 RootFS 分离，进程退出不影响数据 |

---

## 13. 应用升级时的数据迁移策略

### 13.1 数据分类

| 数据类型 | 路径 | 升级行为 | 引用 |
|---|---|---|---|
| RootFS | `<files>/runtime/rootfs/` | 替换（不删除用户数据） | stage.md 8.1 节 |
| 用户数据 | `<files>/amitia-data/` | 保留 | stage.md 8.1 节 |
| SQLite 数据库 | `<files>/amitia-data/sqlite/app.db` | 保留 + 自动迁移 | `backend/cmd/server/main.go:262-280` |
| Qdrant 数据 | `<files>/amitia-data/qdrant/storage/` | 保留 | `backend/pkg/database/qdrant/manager.go:98` |
| SurrealDB 数据 | `<files>/amitia-data/surrealdb/graph.db/` | 保留 | `backend/pkg/database/surrealdb/manager.go:106-109` |
| 配置文件 | `<files>/amitia-data/config/` | 保留 + 合并新配置项 | `backend/config/config.go:92-158` |
| 日志 | `<files>/runtime/logs/` | 可清理 | `backend/cmd/server/main.go:72` |
| 临时文件 | `<files>/runtime/tmp/` | 可清理 | stage.md 8.1 节 |

### 13.2 升级流程

依据 stage.md 8.1 节：

| 步骤 | 实施 |
|---|---|
| 1. 检测当前版本 | 比对 `RootFS 版本` 与目标版本 |
| 2. 下载新 RootFS | 校验 Hash |
| 3. 停止 Runtime | 优雅关闭 Go 后端 + Qdrant + SurrealDB |
| 4. 备份用户数据 | 复制 `files/amitia-data/` 到 `backups/<timestamp>/` |
| 5. 替换 RootFS | 解压新 RootFS 到 `files/runtime/rootfs/` |
| 6. 启动 Runtime | 执行启动顺序（stage.md 8.3 节） |
| 7. 数据库迁移 | Go 后端启动时自动执行 `migration.Runner.Apply` |
| 8. 验证 | 健康检查 + 数据完整性校验 |
| 9. 失败回滚 | 恢复备份 + 旧 RootFS |

### 13.3 数据库迁移兼容性

| 项 | 值 | 引用 |
|---|---|---|
| 迁移框架 | `migration.Runner` | `backend/internal/migration/runner.go:62-80` |
| 迁移记录 | `migration_records` 表跟踪状态 | `backend/internal/migration/runner.go:18-29` |
| 预迁移备份 | `CreatePreMigrationBackup`（仅对已有库） | `backend/cmd/server/main.go:268-272` |
| 旧数据兼容 | `legacy_data_migration.go` 处理旧数据 | `docs/extension-kernel/inventories/source-files.md:175` |
| 重复迁移安全 | `hasPendingMigrations` 检查，已应用迁移不重复执行 | `backend/internal/migration/runner.go:70-80` |

### 13.4 升级失败回滚

| 失败场景 | 回滚策略 |
|---|---|
| RootFS 解压失败 | 保留旧 RootFS，提示用户重试 |
| 数据库迁移失败 | 自动恢复预迁移备份（`migration_records.status='failed'`） |
| 启动失败 | 回滚到旧 RootFS + 旧数据库备份 |
| Hash 校验失败 | 拒绝安装，提示用户重新下载 |

### 13.5 跨版本兼容性

| 版本跨度 | 兼容性策略 |
|---|---|
| 小版本（26.1.7 → 26.1.8） | 数据库迁移自动处理 |
| 大版本（26.x → 27.x） | 评估数据迁移脚本，必要时提供升级工具 |
| RootFS 大版本变更 | 全新 RootFS + 数据迁移脚本 |
| 配置格式变更 | `viper.WatchConfig` + 默认值兼容（`config.go:157`） |

---

## 14. 综合兼容性矩阵

| 组件 | Linux ARM64 兼容性 | PRoot 兼容性 | 数据持久化 | 失败回退 | 阻塞 |
|---|---|---|---|---|---|
| Go 后端 | ✅（需 P0-1 修复） | ⚠️ 需验证 | ✅ | ✅ | P0-1 |
| SQLite | ✅（纯 Go） | ✅ | ✅ | ✅ | 无 |
| Qdrant | ✅（路径已支持） | ⚠️ 需验证 | ✅ | ✅（关键词搜索降级） | 二进制获取 |
| SurrealDB | ⚠️（需 P0-2 修复） | ⚠️ 需验证 | ✅ | ✅（Degraded 状态） | P0-2 + 二进制获取 |
| PRoot | — | ⚠️ 需真机验证 | — | — | 性能与兼容性 |
| Android 后台 | — | — | — | ✅（Foreground Service） | Doze 模式 |
| 文件权限 | ✅ | ✅ | ✅ | — | 二进制可执行权限 |
| 网络 | ✅ | ✅ | — | — | DNS 配置 |
| 升级迁移 | ✅ | ✅ | ✅ | ✅（备份恢复） | 数据库迁移失败处理 |

---

## 15. 关键风险与缓解

### 15.1 P0 风险

| 编号 | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-P0-1 | `killExistingServer` 硬编码 Windows 命令 | Go 后端无法在 Linux ARM64 启动 | 平台抽象（Phase 3 Task 3.1） |
| R-P0-2 | SurrealDB Linux 未区分 ARM64 路径 | Android ARM64 二进制路径错误 | 补充 `runtime.GOARCH` 分支（Phase 3 Task 3.2） |

### 15.2 P1 风险

| 编号 | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-P1-1 | Qdrant ARM64 二进制无法在 PRoot 中运行 | 向量记忆功能不可用 | 评估原生运行 / 兼容版本 / VectorStore Adapter |
| R-P1-2 | SurrealDB ARM64 二进制无法在 PRoot 中运行 | 图谱功能不可用 | 进入 Degraded 状态，不谎报全部能力正常 |
| R-P1-3 | Android 后台限制导致 Runtime 被杀 | 本地后端无法长期运行 | Foreground Service + OnDemand 策略 |
| R-P1-4 | 内存不足（< 4 GB RAM 设备） | 整体不可用 | 默认推荐 6 GB+ RAM 设备，提供 RemoteOnly 模式 |
| R-P1-5 | PRoot 性能开销过大 | 用户体验差 | 评估 proot-rs 或自有 Runtime |

### 15.3 P2 风险

| 编号 | 风险 | 影响 | 缓解 |
|---|---|---|---|
| R-P2-1 | 端口冲突（其他 Android 应用占用） | 启动失败 | 启动时检测，提示用户 |
| R-P2-2 | 二进制可执行权限丢失 | 启动失败 | 解压后 `chmod +x` |
| R-P2-3 | DNS 解析失败 | 外部 API 调用失败 | 配置 PRoot DNS |
| R-P2-4 | 升级时数据库迁移失败 | 数据不可用 | 自动恢复预迁移备份 |
| R-P2-5 | 系统杀进程后状态丢失 | Runtime 状态不一致 | 持久化状态 + 重启恢复 |

---

## 16. 验证清单

依据 stage.md 9.3 节（Qdrant 10 项验证）与 9.4 节（SurrealDB 8 项验证）：

### 16.1 Qdrant 验证清单

- [ ] 确认当前 Qdrant 版本
- [ ] 获取对应 Linux ARM64 构建
- [ ] 验证能否在 PRoot 环境中运行
- [ ] 验证存储路径
- [ ] 验证端口（19178）
- [ ] 验证健康检查（`/readyz`）
- [ ] 验证重启后数据仍存在
- [ ] 验证 Go 后端可以正常连接
- [ ] 验证 ARM64 Android 真机运行
- [ ] 评估兼容构建参数 / 降级版本 / 原生运行 / VectorStore Adapter

### 16.2 SurrealDB 验证清单

- [ ] 二进制可执行
- [ ] 当前存储引擎（`surrealkv`）可用
- [ ] 数据目录持久化
- [ ] 端口（18000）可访问
- [ ] 认证配置（`root`/`root`）正确
- [ ] Go 后端可连接
- [ ] 应用重启后数据存在
- [ ] 异常退出后可恢复

### 16.3 Android 真机验证清单（stage.md 24.4 节）

- [ ] Android 版本
- [ ] CPU 架构（ARM64）
- [ ] RootFS 解压
- [ ] PRoot 运行
- [ ] Qdrant
- [ ] SurrealDB
- [ ] Go 后端
- [ ] SQLite
- [ ] 网络
- [ ] SSE
- [ ] WebSocket
- [ ] 音频
- [ ] 图片
- [ ] 后台
- [ ] 前台服务
- [ ] 系统杀进程
- [ ] 重启恢复
- [ ] 低电量模式
- [ ] 网络切换
- [ ] 屏幕旋转
- [ ] 字体缩放
- [ ] 深色模式

---

## 17. 总结

### 17.1 兼容性总结

| 维度 | 评估 |
|---|---|
| Go 后端 Linux ARM64 | ✅ 可行（需修复 P0-1） |
| SQLite Linux ARM64 | ✅ 完全兼容（纯 Go） |
| Qdrant Linux ARM64 | ✅ 路径已支持，需获取二进制 + PRoot 验证 |
| SurrealDB Linux ARM64 | ⚠️ 路径需修复（P0-2），需获取二进制 + PRoot 验证 |
| PRoot 环境 | ⚠️ 需真机验证 |
| Android 后台限制 | ⚠️ 需 Foreground Service + OnDemand 策略 |
| 文件权限 | ✅ Android 私有目录可写 |
| 网络/DNS | ✅ Android 网络栈支持 |
| 升级迁移 | ✅ 数据库迁移框架已成熟 |

### 17.2 优先级排序

1. **P0-1**：修复 `killExistingServer` 平台抽象
2. **P0-2**：补充 SurrealDB Linux ARM64 路径分支
3. **P1-1**：获取 Qdrant Linux ARM64 二进制
4. **P1-2**：获取 SurrealDB Linux ARM64 二进制
5. **P1-3**：PRoot 兼容性真机验证
6. **P1-4**：Android Foreground Service 实现
7. **P1-5**：内存与性能优化

### 17.3 下一步行动

依据 stage.md 第二十九节执行步骤 11-19：

| 步骤 | 任务 |
|---|---|
| 11 | 确定 RootFS 和 PRoot 技术路线 |
| 12 | 创建 Android 工程 |
| 13 | 建立 Design System |
| 14 | 建立 Runtime 状态机 |
| 15 | 实现 RootFS 安装 |
| 16 | 实现 Linux 进程管理 |
| 17 | 构建 Go 后端 Linux ARM64 |
| 18 | 准备 SurrealDB Linux ARM64 |
| 19 | 准备 Qdrant Linux ARM64 |

---

**审计完成。Phase 0（Task 0.3 / 0.4 / 0.5）三份审计文档全部生成。下一步进入 Phase 1：创建 Android 工程骨架。**
