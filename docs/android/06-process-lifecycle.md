# Amitia Android 进程生命周期设计文档（Phase 2）

> 文档编号：06
> 模块：`android/runtime/process`
> 阶段：Phase 2 Runtime 实现层
> 依据：`AndroidAPP/stage.md` 第八节、第十节、第十八节、第二十二节；`docs/android/03-runtime-dependency-audit.md` 第十二节
> 生成时间：2026-07-26

---

## 1. 概述

本文档描述 Amitia Android Runtime 中三个原生 Linux ARM64 进程（Go 后端、Qdrant、SurrealDB）的完整生命周期模型，覆盖状态机、重启策略、日志滚动、崩溃恢复、资源释放、系统杀进程恢复、前台服务保活与 PID/端口冲突处理。

进程管理核心类：`LinuxProcessManagerImpl`，通过 Hilt @Singleton 注入，提供统一的启动、停止、监控、日志与重启能力。

---

## 2. 进程生命周期模型

### 2.1 受管进程清单

| 进程名 | 二进制 | 端口 | 角色 | 重启策略 |
|---|---|---|---|---|
| `surrealdb` | `surreal_linux_aarch64` | 18000 | 图数据库 | ALWAYS |
| `qdrant` | `qdrant_linux_aarch64` | 19178 | 向量数据库 | ALWAYS |
| `amitia-backend` | `amitia-backend-arm64` | 18899 | Go 后端核心 | ONCE |

### 2.2 生命周期阶段

```
[创建] ──→ RUNNING ──┬─→ STOPPED (正常退出 exit=0)
                     ├─→ CRASHED (异常退出 exit≠0) ──┬─→ 重启 ──→ RUNNING
                     │                                └─→ 不重启 ──→ CRASHED (终态)
                     └─→ 用户主动 stop ──→ STOPPED
                                              │
                                              └─→ releaseAll ──→ [销毁]
```

### 2.3 ManagedProcess 数据模型

每个进程封装为 `ManagedProcess`（private data class）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | String | 进程标识（如 `qdrant`） |
| `command` | List<String> | 原始命令（重启用） |
| `process` | Process | java.lang.Process 实例 |
| `pid` | Int | 进程 PID（Android 26+ `process.pid()`） |
| `startedAt` | Long | 启动时间戳 |
| `env` | Map<String,String> | 环境变量快照 |
| `workDir` | File | 工作目录 |
| `restartPolicy` | RestartPolicy | 重启策略 |
| `statusFlow` | MutableStateFlow<ProcessStatus> | 状态广播 |
| `monitorJob` | Job | 监控协程句柄 |
| `outLog` | File | stdout 日志文件 |
| `errLog` | File | stderr 日志文件 |
| `crashCount` | Int | 崩溃计数（可变） |
| `lastExitReason` | String? | 最后退出原因（可变） |

进程表：`ConcurrentHashMap<String, ManagedProcess>`，启动互斥：`Mutex`。

---

## 3. ProcessStatus 状态机

### 3.1 状态定义

```kotlin
enum class ProcessStatus { RUNNING, STOPPED, CRASHED, UNKNOWN }
```

| 状态 | 含义 | 触发条件 |
|---|---|---|
| `RUNNING` | 进程运行中 | `process.start()` 成功后 |
| `STOPPED` | 正常停止 | `exitCode == 0` 或用户主动 `stop()` |
| `CRASHED` | 异常崩溃 | `exitCode != 0` 且非用户主动停止 |
| `UNKNOWN` | 进程不存在或状态未初始化 | 查询不存在的进程名 |

### 3.2 状态转换图（文字）

```
UNKNOWN ──start()──→ RUNNING
RUNNING ──exit(0)──→ STOPPED
RUNNING ──exit(≠0)──→ CRASHED ──restart()──→ RUNNING
RUNNING ──stop()──→ STOPPED
RUNNING ──forceStop()──→ STOPPED
STOPPED ──start()──→ RUNNING
CRASHED ──restart()──→ RUNNING
CRASHED ──(无重启)──→ CRASHED (终态)
任意 ──releaseAll──→ [从进程表移除]
```

### 3.3 状态查询

- `status(name)`：实时查询，若 `statusFlow=RUNNING` 但 `process.isAlive=false` 则修正为 `STOPPED`
- `observeStatus(name)`：返回 `Flow<ProcessStatus>`，进程存在时转发 `statusFlow`，不存在时发射单次 `UNKNOWN`
- `crashCount(name)`：返回累计崩溃次数
- `lastStartedAt(name)`：返回最近启动时间戳
- `lastExitReason(name)`：返回最后退出原因字符串

---

## 4. RestartPolicy 策略

### 4.1 策略定义

```kotlin
enum class RestartPolicy { NEVER, ONCE, ALWAYS }
```

| 策略 | 行为 | 适用进程 |
|---|---|---|
| `NEVER` | 崩溃后不重启 | 一次性脚本、初始化任务 |
| `ONCE` | 首次崩溃重启一次，再次崩溃不再重启 | Go 后端（避免无限崩溃循环） |
| `ALWAYS` | 总是重启（无限重试） | SurrealDB、Qdrant（数据服务需高可用） |

### 4.2 重启决策矩阵

| 当前状态 | crashCount | Policy=NEVER | Policy=ONCE | Policy=ALWAYS |
|---|---|---|---|---|
| CRASHED | 1 | 不重启 | 重启 | 重启 |
| CRASHED | 2 | 不重启 | 不重启 | 重启 |
| CRASHED | N | 不重启 | 不重启 | 重启 |

### 4.3 重启流程

`handleProcessExit(name, exitCode)` 内部执行：

1. 若 `exitCode == 0`：`statusFlow.value = STOPPED`，`lastExitReason = "exit=0"`，返回
2. 若 `exitCode != 0`：
   - `crashCount++`
   - `lastExitReason = "exit=$exitCode"` 或异常信息
   - `statusFlow.value = CRASHED`
   - 广播 `LogEmitted(WARN)` 告警
   - 判定 `shouldRestart`：
     - `ALWAYS` → true
     - `ONCE` → `crashCount <= 1`
     - `NEVER` → false
   - 若 `shouldRestart`：`delay(1000ms)` 后调用 `restartInternal(name)`

### 4.4 重启间隔

固定 1000ms 延时，避免崩溃风暴。未来可扩展为指数退避（1s → 2s → 4s → 8s，上限 30s）。

### 4.5 重启实现

`restartInternal(name)`：

1. 从进程表移除旧记录（保留 `command`、`env`、`workDir`、`restartPolicy`）
2. 调用 `start()` 用相同参数重新启动
3. 重启失败：广播 `ErrorOccurred`，不再次重试（避免递归）

---

## 5. 日志滚动机制

### 5.1 日志文件布局

```
<files>/runtime/logs/
├── surrealdb.out.log      # stdout
├── surrealdb.out.log.1    # 滚动备份
├── surrealdb.err.log      # stderr
├── surrealdb.err.log.1    # 滚动备份
├── qdrant.out.log
├── qdrant.out.log.1
├── qdrant.err.log
├── qdrant.err.log.1
├── amitia-backend.out.log
├── amitia-backend.out.log.1
├── amitia-backend.err.log
└── amitia-backend.err.log.1
```

### 5.2 异步读取

每个进程启动后，`LinuxProcessManagerImpl` 在 `CoroutineScope(SupervisorJob() + Dispatchers.IO)` 中启动两个读取协程：

- `readStreamToLog(process.inputStream, outLog, name, "stdout")`
- `readStreamToLog(process.errorStream, errLog, name, "stderr")`

读取协程用 `BufferedReader.readLine()` 逐行读取，每行通过 `LogRotator.writeLine` 写入文件。`redirectErrorStream(false)` 保证 stdout 与 stderr 分离。

监控协程在 `waitFor()` 前先 `join()` 两个读取协程，确保进程退出前日志已刷新。

### 5.3 滚动策略

`LogRotator`（@Singleton）：

- 阈值：5MB（`DEFAULT_MAX_SIZE = 5L * 1024 * 1024`）
- 滚动动作：删除 `.1` → 当前文件重命名为 `.1` → 新建当前文件 → 写入新行
- 锁机制：`ConcurrentHashMap<String, Any>` 按文件绝对路径加锁
  - 同文件写入串行化（避免并发写损坏）
  - 不同文件并行写入（无全局锁瓶颈）

### 5.4 日志查询

`logs(name, tailLines=200)`：

1. 从进程表获取 `outLog` 与 `errLog`
2. 分别调用 `LogRotator.readTail(file, lines)` 读取尾部
3. 合并返回（stdout 在前，stderr 在后）

`readTail` 用 `File.readLines().takeLast(lines)` 实现，适合小文件（5MB 内约 5 万行，性能可接受）。

### 5.5 日志保留策略

- 仅保留 `.1` 一份滚动备份（即最多 10MB 每进程每流）
- 不做按时间清理（避免后台任务耗电）
- `releaseAll` 时不删除日志文件（供事后诊断）

---

## 6. 崩溃恢复

### 6.1 崩溃检测

监控协程 `process.waitFor()` 返回后：

| 退出码 | 判定 | 处理 |
|---|---|---|
| 0 | 正常退出 | `STOPPED`，不重启 |
| 非 0 | 崩溃 | `CRASHED`，按 RestartPolicy 决定重启 |
| 异常 | 监控协程抛异常 | `CRASHED`，`lastExitReason = e.message` |

### 6.2 退出原因记录

`lastExitReason` 字符串格式：

- `"exit=$exitCode"` — 正常退出码
- `"stopped"` — 用户主动 stop
- `"force-stopped"` — 用户 forceStop
- `"监控异常: ${e.message}"` — 监控协程异常

### 6.3 崩溃计数与限流

- `crashCount` 单调递增，重启后不清零
- `ONCE` 策略下 `crashCount > 1` 不再重启，避免崩溃风暴
- `ALWAYS` 策略不限次数，但每次重启前 `delay(1000ms)`

### 6.4 数据完整性

崩溃不影响数据完整性：

- SQLite：WAL 模式自动恢复（`modernc.org/sqlite` 默认开启）
- Qdrant：存储目录持久化，重启后自动加载
- SurrealDB：`surrealkv` 存储引擎持久化，重启后自动恢复

---

## 7. 应用退出资源释放

### 7.1 releaseAll 流程

`releaseAll()` 在应用退出时调用：

1. 快照进程表所有 name
2. 逐个 `stop(name, timeoutMs=3000)` 优雅停止
3. 优雅停止失败 → `forceStop(name)` 强制停止
4. `processes.clear()` 清空进程表
5. 广播 `LogEmitted(INFO)` 报告释放数量

### 7.2 调用时机

| 时机 | 调用方 | 行为 |
|---|---|---|
| `Activity.onDestroy` | RuntimeManager | releaseAll |
| `Application.onTerminate` | RuntimeManager | releaseAll |
| `repair()` | BootstrapSequence | releaseAll 后重新启动 |

### 7.3 协程作用域

`LinuxProcessManagerImpl` 持有 `CoroutineScope(SupervisorJob() + Dispatchers.IO)`：

- `SupervisorJob` 保证单个子协程失败不影响其他
- 监控协程与读取协程均在此 scope 内
- `releaseAll` 不取消 scope（scope 生命周期与 @Singleton 相同，即与应用相同）

---

## 8. 系统杀进程恢复

### 8.1 系统杀进程场景

| 场景 | 触发 | 影响 |
|---|---|---|
| 内存压力 | Android Low Memory Killer | Runtime 进程被杀 |
| Doze 模式 | 长期后台 | 网络与 CPU 受限，进程可能冻结 |
| 电池优化 | 用户手动 | 后台进程被限制 |
| 应用被杀 | 系统回收 | 全部 Runtime 进程终止 |

### 8.2 恢复策略

1. **状态持久化**（Phase 4 实现）：将 `RuntimeStateMachine.current` 写入 DataStore
2. **重启检测**：应用启动时读取持久化状态
   - 若为 `Running` / `Degraded`：说明上次被异常终止，恢复为 `Stopped`（不自动启动，需用户确认）
   - 若为 `Stopped` / `Failed`：保持原状态
3. **数据校验**：启动前 `rootfsManager.verify()` 校验 RootFS 完整性
4. **进程表清空**：应用重启后进程表为空（内存态），需重新 `start()`

### 8.3 Foreground Service 保活

Foreground Service（Phase 4 实现）提供：

- 常驻通知，显示当前 Runtime 状态
- `START_STICKY`：服务被杀后系统自动重启
- 通知控制按钮：停止 / 重启 Runtime
- 服务类型：`dataSync`（Android 14+ 规范）

### 8.4 不高频轮询

禁止用高频轮询维持假活跃（违背省电策略）。进程保活依赖：

- Foreground Service 进程优先级提升
- `START_STICKY` 系统重启机制
- 用户主动重启（通过通知或 UI）

---

## 9. 前台服务与进程保活关系

### 9.1 进程优先级

Android 进程优先级（从高到低）：

1. 前台进程（Foreground process）
2. 可见进程（Visible process）
3. 服务进程（Service process）— **Runtime 所在层级**
4. 缓存进程（Cached process）

Foreground Service 将服务进程提升到接近前台优先级，显著降低被杀概率。

### 9.2 保活层次

```
┌─────────────────────────────────────┐
│  Android System                     │
│  ┌───────────────────────────────┐  │
│  │  Foreground Service           │  │
│  │  (AmitiaCoreService)          │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │  RuntimeManager         │  │  │
│  │  │  (Java/Kotlin 层)       │  │  │
│  │  └───────────┬─────────────┘  │  │
│  │              │ ProcessBuilder  │  │
│  │  ┌───────────▼─────────────┐  │  │
│  │  │  Linux ARM64 进程       │  │  │
│  │  │  (子进程,继承优先级)     │  │  │
│  │  │  - amitia-backend       │  │  │
│  │  │  - qdrant               │  │  │
│  │  │  - surrealdb            │  │  │
│  │  └─────────────────────────┘  │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

Linux 子进程通过 `ProcessBuilder.start()` 创建，继承父进程（Foreground Service）的优先级。Foreground Service 存活时，子进程不易被 Low Memory Killer 杀死。

### 9.3 服务生命周期与 Runtime 状态同步

| Service 事件 | Runtime 行为 |
|---|---|
| `onStartCommand` | 检查持久化状态，按需 `runtimeManager.start()` |
| 通知"停止"按钮 | `runtimeManager.stop()` + `stopForeground()` |
| 通知"重启"按钮 | `runtimeManager.restart()` |
| `onDestroy` | `runtimeManager.stop()` + `processManager.releaseAll()` |
| `onTaskRemoved` | OnDemand 模式下 `runtimeManager.stop()` |

### 9.4 START_STICKY 行为

- 系统杀进程后，会在内存允许时重启 Service
- 重启后 `onStartCommand` 再次执行
- Runtime 状态从 DataStore 恢复（不自动 Running，需用户确认或策略配置）

---

## 10. PID 管理与端口冲突处理

### 10.1 PID 获取

Android 26+（API 26）支持 `Process.pid()`：

```kotlin
val pid = try {
    process.pid()
} catch (e: Exception) {
    stateMachine.emitLog(WARN, "无法获取 $name PID: ${e.message}")
    -1
}
```

PID 失败时返回 -1，不阻塞启动。`checkProcess(pid)` 在 `pid <= 0` 时直接返回 false。

### 10.2 PID 文件

当前实现**不使用 PID 文件**（避免文件系统一致性问题）。进程表 `ConcurrentHashMap` 是内存态，应用重启后清空。

未来若需跨进程 PID 共享（如 PRoot 模式），可在 `runtime/tmp/<name>.pid` 写入 PID，启动时读取并检查存活。

### 10.3 端口冲突检测

启动前通过 `HealthChecker.checkPort` 检测端口是否被占用：

| 场景 | 检测结果 | 处理 |
|---|---|---|
| 端口空闲 | `checkPort = false` | 正常启动 |
| 端口被自己旧进程占用 | `checkPort = true` | `stop` 旧进程后重启 |
| 端口被其他应用占用 | `checkPort = true` | `Failed(requiresUserAction=true)` |

### 10.4 端口冲突处理流程

`BootstrapSequenceImpl.start()` 在启动每个服务前隐式依赖 `waitForHealthy`：

1. 启动进程
2. `waitForHealthy` 轮询 HTTP 健康检查
3. 若进程因端口冲突立即退出：`statusFlow = CRASHED`，`waitForHealthy` 超时
4. 超时后返回 `Failed` 或 `Degraded`

Go 后端通过 `killExistingServer` 平台抽象（Phase 3 修复 P0-1）处理端口占用：

- Desktop：`taskkill`
- Android：原生模式无需 kill（首次启动），PRoot 模式用 `lsof`/`fuser`

### 10.5 防重复启动

`LinuxProcessManagerImpl.start()` 互斥检查：

```kotlin
val existing = processes[name]
if (existing != null && existing.statusFlow.value == ProcessStatus.RUNNING) {
    return Result.failure(IllegalStateException("进程 $name 已在运行 pid=${existing.pid}"))
}
```

若进程存在但非 RUNNING（如 CRASHED/STOPPED）：取消旧 monitorJob，从进程表移除，重新启动。

---

## 11. 进程停止详细流程

### 11.1 优雅停止（stop）

```
stop(name, timeoutMs=5000)
  │
  ├─ 1. 取消 monitorJob（防止退出后触发重启）
  ├─ 2. process.destroy() → SIGTERM
  ├─ 3. withTimeoutOrNull(timeoutMs) { process.waitFor() }
  │     ├─ 退出 → statusFlow = STOPPED, lastExitReason = "stopped"
  │     └─ 超时 → 继续
  ├─ 4. 仍存活 → process.destroyForcibly() → SIGKILL
  ├─ 5. process.waitFor() 等待彻底退出
  └─ 6. statusFlow = STOPPED, lastExitReason = "stopped"
```

### 11.2 强制停止（forceStop）

```
forceStop(name)
  │
  ├─ 1. 取消 monitorJob
  ├─ 2. process.destroyForcibly() → SIGKILL
  ├─ 3. process.waitFor()
  └─ 4. statusFlow = STOPPED, lastExitReason = "force-stopped"
```

### 11.3 超时配置

| 进程 | stop 超时 | 原因 |
|---|---|---|
| `amitia-backend` | 10s | 需排水 HTTP 请求 + 关闭数据库连接 |
| `qdrant` | 8s | 需刷新向量索引 |
| `surrealdb` | 8s | 需 checkpoint 图数据 |
| `releaseAll` 单进程 | 3s | 批量退出时缩短超时 |

---

## 12. 监控协程实现细节

### 12.1 协程结构

```kotlin
val monitorJob = scope.launch {
    try {
        outReaderJob.join()   // 等 stdout 读完
        errReaderJob.join()   // 等 stderr 读完
        val exitCode = process.waitFor()
        handleProcessExit(name, exitCode)
    } catch (e: Exception) {
        handleProcessExit(name, -1, e.message ?: "监控异常")
    }
}
```

### 12.2 读取协程

```kotlin
suspend fun readStreamToLog(stream, logFile, name, tag) {
    try {
        BufferedReader(InputStreamReader(stream, Charsets.UTF_8)).use { reader ->
            while (true) {
                val line = reader.readLine() ?: break
                logRotator.writeLine(logFile, line)
            }
        }
    } catch (e: Exception) {
        stateMachine.emitLog(WARN, "读取 $name/$tag 流结束: ${e.message}")
    }
}
```

### 12.3 超时控制

`start` 参数 `timeoutMs` 用于一次性任务：

```kotlin
if (timeoutMs != null && timeoutMs > 0L) {
    scope.launch {
        delay(timeoutMs)
        val current = processes[name]
        if (current != null && current.process.isAlive) {
            stateMachine.emitLog(WARN, "进程 $name 超时 ${timeoutMs}ms,执行 destroy")
            current.process.destroy()
        }
    }
}
```

常驻服务（SurrealDB、Qdrant、Go 后端）启动时 `timeoutMs = null`，不设超时。

---

## 13. 异常场景处理

### 13.1 进程启动失败

| 原因 | 检测 | 处理 |
|---|---|---|
| 二进制不存在 | `ProcessBuilder.start()` 抛 IOException | `Result.failure`，不加入进程表 |
| 权限不足 | `start()` 抛 SecurityException | `Result.failure`，广播 ErrorOccurred |
| 端口已被占用 | 进程立即退出 | 监控协程检测 CRASHED，按策略重启或放弃 |
| 环境变量错误 | 进程启动后异常退出 | 同上 |

### 13.2 监控协程异常

监控协程用 try-catch 包裹，异常时调用 `handleProcessExit(name, -1, e.message)`，标记为 CRASHED。

### 13.3 日志写入失败

`LogRotator.writeLine` 异常时不传播（日志写入失败不应影响进程运行），仅在下一次 `readStreamToLog` 循环中可能再次失败并最终结束读取协程。

### 13.4 进程表一致性

`ConcurrentHashMap` 保证并发读写安全。`assignMonitorJob` 在监控协程启动后回填 `monitorJob` 字段（用 `copy` 更新 data class）。

---

## 14. 与 Go 后端退出行为的对应

依据 `03-runtime-dependency-audit.md` 第十二节，Go 后端正常退出流程：

| 后端步骤 | Runtime 对应 |
|---|---|
| 1. 停止接受新请求 | `Stopping(stage="reject")` |
| 2. 关闭 Plugin Runtime | Go 后端内部处理 |
| 3. 排水现有请求（10s） | `stop("amitia-backend", timeoutMs=10000)` |
| 4. 停止 Go 后端 | `process.destroy()` |
| 5. 停止 Qdrant | `stop("qdrant", timeoutMs=8000)` |
| 6. 停止 SurrealDB | `stop("surrealdb", timeoutMs=8000)` |
| 7. 刷新日志和状态 | `transition(Stopped)` + emitLog |

停止顺序：后端 → Qdrant → SurrealDB（与启动顺序相反）。

---

## 15. 验证清单

- [x] ProcessStatus 四状态完整定义
- [x] RestartPolicy 三策略实现
- [x] 日志滚动 5MB 阈值 + .1 备份
- [x] 崩溃检测与重启决策矩阵
- [x] releaseAll 资源释放
- [x] 系统杀进程恢复策略（持久化 + Foreground Service）
- [x] PID 获取与端口冲突处理
- [x] 防重复启动互斥
- [x] 优雅停止 + 强制停止
- [x] 监控协程 join 读取协程保证日志完整
- [x] 所有 IO 在 Dispatchers.IO
- [x] 代码无注释（遵循 AGENTS.md）

---

## 16. 后续 Phase 依赖

| 依赖项 | Phase | 说明 |
|---|---|---|
| Foreground Service 实现 | Phase 4 | AmitiaCoreService + 常驻通知 |
| 状态持久化（DataStore） | Phase 4 | 存储最后 RuntimeState |
| 指数退避重启 | Phase 5 | 优化崩溃风暴防护 |
| PID 文件跨进程共享 | Phase 3 | PRoot 模式下需要 |
| Go 后端平台抽象（P0-1） | Phase 3 | `killExistingServer` Android 分支 |
| 进程资源监控（CPU/内存） | Phase 5 | `onTrimMemory` 联动降级 |

---

**文档结束。Phase 2 进程生命周期设计完整。**
