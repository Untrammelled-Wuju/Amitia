# Amitia Android Linux Runtime 设计文档（Phase 2）

> 文档编号：05
> 模块：`android/runtime`
> 阶段：Phase 2 Runtime 实现层
> 依据：`AndroidAPP/stage.md` 第八节、第十节、第二十二节；`docs/android/03-runtime-dependency-audit.md`
> 生成时间：2026-07-26

---

## 1. 概述

本文档描述 Amitia Android 应用内嵌 Linux Runtime 的整体设计，覆盖状态机、RootFS 管理、进程管理、启动与停止顺序、健康检查、目录布局、PRoot 集成预留、Android 生命周期集成与内存耗电策略。

Linux Runtime 在 Android 应用私有目录内运行 Go 后端、Qdrant、SurrealDB 三个原生 Linux ARM64 进程，通过 127.0.0.1 loopback 端口对外提供服务，不依赖 root 权限，不依赖 Termux。

端口分配（仅 127.0.0.1，避开 3000）：

| 服务 | 端口 | 健康检查路径 |
|---|---|---|
| Go 后端 | 18899 | `/api/health` |
| Qdrant | 19178 | `/healthz` |
| SurrealDB | 18000 | `/health` |

---

## 2. 整体架构

### 2.1 分层架构（文字图）

```
┌─────────────────────────────────────────────────────────────┐
│                     Android Application                     │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              UI Layer (Compose / ViewModel)           │  │
│  │              通过 RuntimeManager 观察 StateFlow        │  │
│  └───────────────────────────┬───────────────────────────┘  │
│                              │                              │
│  ┌───────────────────────────▼───────────────────────────┐  │
│  │             RuntimeManager (门面)                      │  │
│  │   start() / stop() / restart() / repair() / refresh() │  │
│  └───────────────────────────┬───────────────────────────┘  │
│                              │                              │
│  ┌───────────────────────────▼───────────────────────────┐  │
│  │            BootstrapSequence (编排层)                  │  │
│  │   启动顺序 / 停止顺序 / Degraded 降级判定              │  │
│  └───┬───────────┬───────────┬───────────┬───────────────┘  │
│      │           │           │           │                  │
│  ┌───▼────┐ ┌────▼─────┐ ┌───▼────┐ ┌────▼──────────────┐  │
│  │Rootfs  │ │Process   │ │Health  │ │RuntimeStateMachine│  │
│  │Manager │ │Manager   │ │Checker │ │(StateFlow)        │  │
│  └───┬────┘ └────┬─────┘ └───┬────┘ └────┬──────────────┘  │
│      │           │           │           │                  │
│  ┌───▼───────────▼───────────▼───────────▼──────────────┐  │
│  │              RuntimeDirectories (目录布局)             │  │
│  │   runtime/  +  amitia-data/  (物理隔离)               │  │
│  └───────────────────────────────────────────────────────┘  │
│                              │                              │
│  ┌───────────────────────────▼───────────────────────────┐  │
│  │             Linux ARM64 进程 (PRoot 预留)              │  │
│  │   amitia-backend-arm64 / qdrant / surreal             │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 模块依赖

```
RuntimeManager
  └── BootstrapSequence
        ├── RuntimeDirectories
        ├── LinuxRootfsManager (Impl)
        │     └── RootfsIntegrityChecker
        ├── LinuxProcessManager (Impl)
        │     └── LogRotator
        ├── HealthChecker (Impl)
        │     └── OkHttpClientProvider
        └── RuntimeStateMachine
```

所有 @Singleton 类通过 Hilt 构造注入，接口绑定在 `RuntimeModule` 中声明。

---

## 3. 状态机设计

### 3.1 状态定义

| 状态 | Phase | 含义 |
|---|---|---|
| `NotInstalled` | IDLE | RootFS 未安装，需要用户授权安装 |
| `Installing(progress, message)` | INSTALLING | 正在解压 RootFS 资产 |
| `Installed` | INSTALLED | RootFS 已就绪，尚未启动服务 |
| `Starting(stage, progress)` | STARTING | 正在按顺序启动服务 |
| `Running(uptimeMs, services)` | RUNNING | 全部核心服务健康 |
| `Degraded(reason, services)` | DEGRADED | 部分非致命服务不可用（如 SurrealDB） |
| `Stopping(stage)` | STOPPING | 正在按顺序停止服务 |
| `Stopped` | STOPPED | 全部停止，可重新启动 |
| `Failed(error, retryable, requiresUserAction)` | FAILED | 致命错误，需重试或用户介入 |
| `Updating(progress, message)` | UPDATING | RootFS 升级中 |

### 3.2 状态转换矩阵（文字图）

```
NotInstalled ──→ Installing ──→ Installed ──→ Starting ──┬─→ Running
       │              │             │             │       ├─→ Degraded
       │              │             │             │       └─→ Failed
       │              └─→ Failed    │             └─→ Failed
       │                            │
       │                            └─→ Updating ──→ Installed
       │
       └─(已安装)──→ Starting

Running ──→ Stopping ──→ Stopped
Running ──→ Degraded
Running ──→ Failed

Degraded ──→ Running (恢复)
Degraded ──→ Failed
Degraded ──→ Stopping

Stopped ──→ Starting (重启)
Stopped ──→ NotInstalled (卸载)

Failed ──→ Starting (重试)
Failed ──→ Stopped
Failed ──→ Installing (重装)

Updating ──→ Installed
Updating ──→ Failed
```

非法转换抛 `IllegalStateException`，并通过 `RuntimeEvent.ErrorOccurred` 广播。

### 3.3 事件广播

`RuntimeStateMachine` 内部维护 `MutableStateFlow<RuntimeState>` 与 `MutableSharedFlow<RuntimeEvent>`。事件类型：

- `StateChanged(from, to)` — 状态切换
- `ProgressUpdated(stage)` — 进度更新
- `ServiceHealthChanged(name, state)` — 服务健康变化
- `LogEmitted(level, tag, message)` — 运行时日志
- `ErrorOccurred(error, retryable, requiresUserAction, cause)` — 错误事件

UI 层通过 `observe()` 与 `observeEvents()` 订阅，避免轮询。

---

## 4. RootFS 管理设计

### 4.1 目录与版本

- RootFS 根目录：`<files>/runtime/rootfs/`
- 版本目录：`<files>/runtime/versions/`
- 版本文件：`<files>/runtime/rootfs/.amitia-rootfs-version`
- 完整性清单：`<files>/runtime/rootfs/manifest.json`

### 4.2 资产格式

RootFS 资产以 **ZIP** 格式打包（`rootfs.zip`），通过 Android `assets` 分发。选择 ZIP 而非 tar.xz 的原因：

1. Java 标准库原生支持 `ZipInputStream`，无需引入第三方依赖（Apache Commons Compress）
2. 解压性能在 Android 上足够（分块读写）
3. APK 内 assets 已压缩，ZIP 重复压缩开销可接受

### 4.3 安装流程

1. 清理旧 RootFS 目录
2. 用 `ZipInputStream` 遍历 entries，逐文件解压到 `rootfsDir`
3. 对每个 entry 执行路径穿越校验（拒绝 `..` 与越界路径）
4. 对 `bin/`、`sbin/` 下的二进制自动 `setExecutable(true, false)`
5. 解压完成后，遍历所有文件计算 SHA-256，生成 `Manifest`
6. 写入 `manifest.json`（kotlinx.serialization）与版本文件
7. 全程在 `Dispatchers.IO` 执行，通过 `progress(Float, String)` 回调上报

### 4.4 完整性校验

`RootfsIntegrityChecker` 提供：

- `sha256(file: File): String` — 分块哈希（64KB buffer）
- `verifyManifest(rootfsDir, manifest): RootfsIntegrity` — 返回 `valid / missingFiles / corruptedFiles`

`RootfsIntegrity` 数据类包含三个字段，便于 UI 展示缺失与损坏清单。

### 4.5 升级流程（原子替换）

1. 解压新资产到临时目录 `runtime/.tmp-rootfs-upgrade-<ts>/`
2. 在临时目录生成 manifest 并校验完整性
3. 校验失败：删除临时目录，返回失败
4. 校验通过：将当前 `rootfsDir` 重命名为 `rootfs.bak-<ts>`，临时目录重命名为 `rootfsDir`
5. 删除备份目录
6. 更新版本文件

整个过程用户数据目录 `amitia-data/` 不受影响。

### 4.6 清理流程

- `cleanup(requireConfirmation=true)`：仅统计大小与文件数，通过日志广播待清理清单，不删除
- `cleanup(requireConfirmation=false)`：删除 `rootfsDir`，保留 `amitia-data/`

---

## 5. 进程管理设计

### 5.1 进程模型

每个受管进程封装为 `ManagedProcess`，包含：

| 字段 | 说明 |
|---|---|
| `name` | 进程名（如 `surrealdb`、`qdrant`、`amitia-backend`） |
| `command` | 原始命令列表（用于重启） |
| `process` | `java.lang.Process` 实例 |
| `pid` | `process.pid()`（Android 26+ 支持） |
| `startedAt` | 启动时间戳 |
| `env` | 环境变量快照 |
| `workDir` | 工作目录 |
| `restartPolicy` | NEVER / ONCE / ALWAYS |
| `statusFlow` | `MutableStateFlow<ProcessStatus>` |
| `outLog` / `errLog` | 日志文件 |
| `crashCount` | 崩溃计数 |
| `lastExitReason` | 最后退出原因 |

进程表用 `ConcurrentHashMap<String, ManagedProcess>` 管理，启动互斥用 `Mutex` 保护。

### 5.2 启动流程

1. 互斥检查同名进程是否已 RUNNING，若是则返回失败
2. `ProcessBuilder().command(cmd).directory(workDir).redirectErrorStream(false)`
3. 注入环境变量
4. `start()` 后获取 `pid()`
5. 启动两个协程异步读取 stdout / stderr 到日志文件（`<name>.out.log` / `<name>.err.log`）
6. 启动监控协程：`waitFor()` 后判定退出码，按 `restartPolicy` 决定重启
7. 若设置 `timeoutMs`，启动延时协程，超时 `destroy()`

### 5.3 停止流程

1. 取消监控协程
2. `process.destroy()`（Linux 发送 SIGTERM）
3. `withTimeoutOrNull(timeoutMs)` 等待退出
4. 仍存活则 `destroyForcibly()`（SIGKILL）
5. 更新 `statusFlow` 为 `STOPPED`

`forceStop` 跳过优雅阶段，直接 `destroyForcibly()`。

### 5.4 日志滚动

`LogRotator` 按 `logFile.absolutePath` 维护锁映射（`ConcurrentHashMap<String, Any>`），保证同文件写入串行化、不同文件并行写入。

滚动策略：单文件超过 5MB 时，删除 `.1` → 当前重命名为 `.1` → 新建当前 → 写入。`readTail` 提供尾部行读取用于日志面板。

详细设计见 `06-process-lifecycle.md`。

---

## 6. 启动顺序（stage.md 第十节）

`BootstrapSequenceImpl.start()` 严格按以下顺序：

| 步骤 | Stage | 进度 | 动作 | 失败处理 |
|---|---|---|---|---|
| 1 | preparing | 0.05 | 检查 RootFS 是否已安装 | 未安装则 transition(Installing) → install |
| 2 | installing | 0.1-0.3 | 解压 RootFS 资产 | 失败 → Failed(retryable=true) |
| 3 | checking-bin | 0.32 | 检查三个二进制存在性与可执行位 | 失败 → Failed(requiresUserAction=true) |
| 4 | checking-dirs | 0.35 | `ensureAllCreated` + `validateIsolation` | 失败 → Failed(retryable=true) |
| 5 | checking-config | 0.4 | 检查 `config.yml` 存在 | 失败 → Failed(requiresUserAction=true) |
| 6 | starting-surrealdb | 0.45-0.55 | 启动 SurrealDB + 健康轮询（30s） | 非致命 → Degraded |
| 7 | starting-qdrant | 0.6-0.7 | 启动 Qdrant + 健康轮询（30s） | 非致命 → Degraded |
| 8 | starting-backend | 0.75-0.95 | 启动 Go 后端 + 健康轮询（60s） | 致命 → Failed |
| 9 | running/degraded | 1.0 | transition(Running) 或 transition(Degraded) | — |

健康轮询参数：

- 间隔：500ms
- 单次检查超时：2000ms
- SurrealDB 总超时：30s
- Qdrant 总超时：30s
- Go 后端总超时：60s

禁止使用固定延时等待就绪，必须通过 `waitForHealthy` 轮询端口或 HTTP。

### 6.1 命令构建

- **SurrealDB**：`surreal start --log info --user root --pass root --bind 127.0.0.1:18000 surrealkv:<path>`
- **Qdrant**：`qdrant --config-path <config.yaml>`（自动生成配置含 http_port=19178, grpc_port=19179）
- **Go 后端**：`amitia-backend-arm64`（通过环境变量注入配置）

### 6.2 后端环境变量

| 变量 | 值 |
|---|---|
| `CONFIG_PATH` | `<amitia-data>/config` |
| `STORAGE_DATADIR` | `<amitia-data>/sqlite` |
| `QDRANT_HOST` / `QDRANT_PORT` | `127.0.0.1` / `19178` |
| `SURREALDB_HOST` / `SURREALDB_PORT` | `127.0.0.1` / `18000` |
| `SURREALDB_DATAPATH` | `<amitia-data>/surrealdb` |
| `AMITIA_EMBEDDED` | `android` |
| `LOCAL_AUTH_TOKEN` | 随机 UUID（每次启动重新生成） |
| `GOGC` | `50`（降低 GC 阈值控内存） |

---

## 7. 停止顺序（stage.md 第十节）

`BootstrapSequenceImpl.stop()` 严格按以下顺序：

| 步骤 | Stage | 进度 | 动作 | 超时 |
|---|---|---|---|---|
| 1 | stopping-reject | 0.1 | transition(Stopping, "reject")，停止接受新请求 | — |
| 2 | stopping-backend | 0.3 | `processManager.stop("amitia-backend")` | 10s |
| 3 | stopping-qdrant | 0.6 | `processManager.stop("qdrant")` | 8s |
| 4 | stopping-surrealdb | 0.85 | `processManager.stop("surrealdb")` | 8s |
| 5 | stopped | 1.0 | transition(Stopped) + 日志刷新 | — |

停止顺序与启动顺序相反：先停依赖方（后端），再停被依赖方（Qdrant、SurrealDB）。每个进程先 SIGTERM 等待优雅退出，超时后 SIGKILL。

---

## 8. 健康检查策略

### 8.1 检查方式

| 方法 | 实现 | 用途 |
|---|---|---|
| `checkPort(host, port, timeoutMs)` | `java.net.Socket` 连接 | 端口可达性 |
| `checkHttp(url, timeoutMs, expectedStatus)` | OkHttpClient GET | 服务就绪性 |
| `checkProcess(pid)` | `/proc/<pid>` 存在性 | 进程存活（PRoot 内） |
| `waitForHealthy(name, check, interval, timeout)` | 轮询 + 超时 | 启动就绪等待 |

### 8.2 健康检查 URL

- SurrealDB：`http://127.0.0.1:18000/health`
- Qdrant：`http://127.0.0.1:19178/healthz`
- Go 后端：`http://127.0.0.1:18899/api/health`

### 8.3 OkHttpClient 配置

`OkHttpClientProvider` 提供共享 client：

- connect/read/write 超时：2s
- `retryOnConnectionFailure(false)`
- `followRedirects(false)`
- 支持 `clientWithTimeout(timeoutMs)` 派生新 client

### 8.4 禁止固定延时

严禁使用 `delay(5000)` 等待服务就绪。必须通过 `waitForHealthy` 轮询，间隔 500ms，超时由各服务决定。

---

## 9. 目录布局（stage.md 第八节）

### 9.1 物理目录树

```
<files>/
├── runtime/                          # Runtime 系统（可替换）
│   ├── rootfs/                       # RootFS（ZIP 解压）
│   │   ├── .amitia-rootfs-version    # 版本字符串
│   │   ├── manifest.json             # 完整性清单
│   │   ├── bin/                      # 可执行二进制
│   │   │   ├── amitia-backend-arm64
│   │   │   ├── qdrant_linux_aarch64
│   │   │   └── surreal_linux_aarch64
│   │   └── ...
│   ├── bin/                          # 当前活动二进制（符号链接或复制）
│   ├── logs/                         # 进程日志
│   │   ├── surrealdb.out.log
│   │   ├── surrealdb.err.log
│   │   ├── qdrant.out.log
│   │   ├── qdrant.err.log
│   │   ├── amitia-backend.out.log
│   │   └── amitia-backend.err.log
│   ├── tmp/                          # 临时文件
│   └── versions/                     # 版本记录
│       └── <version>.txt
└── amitia-data/                      # 用户数据（持久保留）
    ├── config/
    │   └── config.yml
    ├── sqlite/
    │   └── app.db
    ├── qdrant/
    │   ├── config.yaml
    │   └── storage/
    ├── surrealdb/
    │   └── graph.db/
    ├── uploads/
    ├── models/
    ├── extensions/
    └── backups/
```

### 9.2 隔离规则

`RuntimeDirectories.validateIsolation()` 校验 `amitiaDataRoot` 的规范路径不以 `rootfsDir` 开头，确保：

- RootFS 升级/清理不会触碰用户数据
- 用户数据在 RootFS 损坏时仍可恢复
- 备份策略可独立作用于两个目录

### 9.3 目录创建

`ensureAllCreated()` 在启动时 mkdirs 所有目录并校验可写，任一失败返回 `Result.failure`。

---

## 10. PRoot 集成预留

当前 Phase 2 实现**不依赖 PRoot**，直接通过 `ProcessBuilder` 启动原生 Linux ARM64 二进制。PRoot 集成预留点：

1. **BootstrapSequence 命令构建**：`buildSurrealdbCommand` 等方法可在 Phase 3 包装为 `proot -r <rootfs> -b /proc -b /dev -b /sys -- <原始命令>`
2. **RootfsIntegrityChecker**：manifest 已记录完整文件列表，PRoot 模式下可校验 `/proc`、`/dev` 绑定
3. **LinuxProcessManager**：进程监控与日志滚动逻辑与 PRoot 无关，PRoot 透明
4. **HealthChecker**：HTTP/端口检查与 PRoot 无关（loopback 共享 Android 网络栈）

PRoot 不可用时的降级路径：

- 优先尝试原生运行（当前实现）
- 二进制无法原生运行 → 评估 proot-rs
- 仍不可用 → 进入 `Failed` 状态，提示用户设备不兼容

---

## 11. 与 Android 应用生命周期集成

### 11.1 进程保活

- **Foreground Service**：显示常驻通知，保持 Linux Runtime 运行
- **START_STICKY**：系统杀进程后自动重启服务
- **通知权限**：引导用户开启通知权限
- **电池优化白名单**：引导用户加入白名单

### 11.2 生命周期事件映射

| Android 事件 | Runtime 行为 |
|---|---|
| `onCreate` | 注入 RuntimeManager，恢复持久化状态 |
| `onStart` | 按需启动 Runtime（AlwaysOn 模式） |
| `onStop` | OnDemand 模式下停止 Runtime |
| `onDestroy` | `processManager.releaseAll()` |
| `onLowMemory` | 降级运行，关闭非致命服务 |
| `onTrimMemory(TRIM_MEMORY_RUNNING_LOW)` | 触发 GC，清理日志临时文件 |
| 系统杀进程 | 数据已持久化，重启后恢复 |

### 11.3 状态持久化

`RuntimeStateMachine` 的当前状态通过 DataStore 持久化，应用重启后恢复到 `Stopped` 或 `Failed`（不自动 Running，需用户确认）。

---

## 12. 内存与耗电策略

### 12.1 内存预估

| 组件 | 预估内存 |
|---|---|
| Go 后端 | 50-150 MB |
| SQLite | 10-50 MB |
| Qdrant | 100-300 MB |
| SurrealDB | 50-150 MB |
| Linux Runtime 进程开销 | 10-20 MB |
| **总计** | **250-700 MB** |

### 12.2 内存控制

- `largeHeap=true`（app build.gradle.kts）
- Go 后端 `GOGC=50` 降低 GC 阈值
- Qdrant 通过 `config.yaml` 限制 worker 与 mmap 阈值
- SurrealDB 通过启动参数限制
- 监听 `onTrimMemory` 主动释放缓存

### 12.3 耗电策略

- **默认 OnDemand 模式**：不盲目 AlwaysOn，按需启动
- **Doze 模式感知**：系统进入 Doze 时降级为 Degraded，降低轮询频率
- **禁止高频轮询**：健康检查仅在启动阶段密集轮询，运行期改为事件驱动
- **前台服务通知**：用户可见当前状态，可手动停止以省电
- **网络复用**：所有服务监听 127.0.0.1，不唤醒基带

### 12.4 设备要求

- 推荐 6 GB+ RAM 设备
- ARM64 架构（`arm64-v8a`）
- Android 8.0+（minSdk 26）
- 低内存设备（< 4 GB）建议使用 RemoteOnly 模式

---

## 13. 关键实现决策

| 决策点 | 选择 | 原因 |
|---|---|---|
| RootFS 压缩格式 | ZIP | Java 原生支持，无第三方依赖 |
| 状态机实现 | sealed interface + StateFlow | 类型安全，Flow 响应式 |
| 进程管理 | ProcessBuilder + 协程监控 | 无需 JNI，Android 26+ 支持 pid() |
| 日志滚动 | 按文件路径加锁的 LogRotator | 并发安全，无全局锁瓶颈 |
| 健康检查 | Socket + OkHttp | 双重验证，端口与 HTTP 均可 |
| Hilt 绑定 | @Binds Module | 编译期校验，无运行时反射 |
| 二进制路径 | `runtime/bin/` | 与 rootfs 隔离，便于升级 |

---

## 14. 后续 Phase 依赖

| 依赖项 | 状态 | 说明 |
|---|---|---|
| OkHttp | 已在 libs.versions.toml | `squareup-okhttp` 4.12.0 |
| kotlinx-coroutines | 已在 libs.versions.toml | 1.8.1（本次修复版本缺失） |
| kotlinx-serialization | 已在 libs.versions.toml | 1.7.1 |
| Hilt | 已在 libs.versions.toml | 2.52 |
| RootFS 资产 (rootfs.zip) | 待 Phase 3 | 需打包 Linux ARM64 二进制 |
| PRoot 二进制 | 待 Phase 3 | 当前直接原生运行，PRoot 为备选 |
| Foreground Service | 待 Phase 4 | Android 通知与保活实现 |
| 持久化状态恢复 | 待 Phase 4 | DataStore 存储最后状态 |

---

## 15. 验证清单

- [x] 状态机 10 个状态完整定义
- [x] 状态转换矩阵覆盖所有合法路径
- [x] 非法转换抛 IllegalStateException
- [x] RootFS 安装/校验/升级/清理完整实现
- [x] 进程启动/停止/强制停止/重启完整实现
- [x] 日志滚动 5MB 阈值
- [x] 启动顺序按 stage.md 第十节
- [x] 停止顺序按 stage.md 第十节
- [x] 健康检查用端口/HTTP，无固定延时
- [x] 目录布局按 stage.md 第八节
- [x] RootFS 与用户数据隔离（validateIsolation）
- [x] 所有 IO 在 Dispatchers.IO
- [x] 所有 @Singleton 用 Hilt 注入
- [x] 代码无注释（遵循 AGENTS.md）

---

**文档结束。Phase 2 Runtime 实现层设计完整。**
