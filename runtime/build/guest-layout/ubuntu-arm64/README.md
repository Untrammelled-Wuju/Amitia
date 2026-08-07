# Amitia Guest Layout - Ubuntu ARM64

本目录负责生成 Amitia Runtime 在 Ubuntu 24.04.4 ARM64 Guest 内的固定目录布局与 Mount Contract。

## 各目录职责

| 路径 | 职责 | 持久化等级 |
|------|------|-----------|
| `/opt/amitia` | 不可变程序根目录 | Immutable |
| `/etc/amitia` | 可变配置根目录 | Persistent Critical |
| `/var/lib/amitia` | 持久化数据根目录 | Persistent Critical |
| `/var/cache/amitia` | 可安全重建缓存 | Rebuildable |
| `/var/log/amitia` | 运行日志 | Persistent Diagnostic |
| `/run/amitia` | 单次运行临时状态 | Ephemeral |
| `/var/lib/amitia/workspaces` | Task Workspace | Persistent Critical |

## 不可变和可变目录

### 不可变（Runtime 更新可替换）

- `/opt/amitia`
- 不允许存储用户数据、配置、缓存、日志、临时文件。

### 可变（Runtime 更新不得覆盖）

- `/etc/amitia`
- `/var/lib/amitia`
- `/var/log/amitia`
- `/var/cache/amitia`

## Guest 环境变量

```text
AMITIA_RUNTIME_ROOT=/opt/amitia
AMITIA_CONFIG_ROOT=/etc/amitia
AMITIA_DATA_ROOT=/var/lib/amitia
AMITIA_CACHE_ROOT=/var/cache/amitia
AMITIA_LOG_ROOT=/var/log/amitia
AMITIA_RUN_ROOT=/run/amitia
AMITIA_TEMP_ROOT=/run/amitia/tmp
AMITIA_WORKSPACE_ROOT=/var/lib/amitia/workspaces
AMITIA_HOME=/home/amitia
HOME=/home/amitia
LANG=C.UTF-8
LC_ALL=C.UTF-8
TZ=Etc/UTC
```

## Resource URI 映射

```text
amitia://runtime/       → /opt/amitia/
amitia://config/        → /etc/amitia/
amitia://data/          → /var/lib/amitia/
amitia://workspace/     → /var/lib/amitia/workspaces/
```

## Qdrant 目录合同

| 用途 | 路径 |
|------|------|
| 程序 | `/opt/amitia/qdrant` |
| 配置 | `/etc/amitia/providers/qdrant/config.yaml` |
| 数据根 | `/var/lib/amitia/providers/qdrant` |
| Storage | `/var/lib/amitia/providers/qdrant/storage` |
| Snapshots | `/var/lib/amitia/providers/qdrant/snapshots` |
| Migration | `/var/lib/amitia/providers/qdrant/migration` |

## Node 可变目录合同

```text
AMITIA_NODE_HOME=/var/lib/amitia/node/home
AMITIA_NODE_PREFIX=/var/lib/amitia/node/prefix
AMITIA_NPM_CACHE=/var/cache/amitia/node/npm
AMITIA_NODE_TMP=/run/amitia/tmp/node
```

## Mount ID 和 Guest Target

| Mount ID | Guest Target | 持久化等级 |
|----------|-------------|-----------|
| `runtime` | `/opt/amitia` | Immutable |
| `config` | `/etc/amitia` | Persistent Critical |
| `data` | `/var/lib/amitia` | Persistent Critical |
| `cache` | `/var/cache/amitia` | Rebuildable |
| `logs` | `/var/log/amitia` | Persistent Diagnostic |
| `run` | `/run/amitia` | Ephemeral |

## 挂载顺序

1. Runtime Root
2. Config Root
3. Data Root
4. Cache Root
5. Log Root
6. Run Root
7. /proc
8. /dev
9. /sys
10. DNS 和环境文件
11. 用户明确授权的额外 Workspace

## 权限和所有权

### 用户

| 用户 | UID | GID | Home |
|------|-----|-----|------|
| root | 0 | 0 | /root |
| amitia | 1000 | 1000 | /home/amitia |

### 不可变目录

- UID/GID: 0/0
- Mode: 0755

### 可变目录

- UID/GID: 1000/1000
- Mode: 0750（普通）、0700（Temp/Locks/Sockets）

## 第 18 步集成验证

第 18 步提供的 Ubuntu Base RootFS 本步骤不修改。集成验证必须在临时副本执行：

1. 复制或重新安全解压 RootFS 到临时目录
2. 在临时目录应用本 Overlay
3. 测试完成后删除临时目录

禁止直接修改第 18 步正式输出目录。

## 第 20 步如何组装组件

第 20 步把组件放进本步骤固定目录：

- Go 后端 → `/opt/amitia/backend/amitia-server`
- Node → `/opt/amitia/node/bin/node`
- Qdrant → `/opt/amitia/qdrant/bin/qdrant`
- Plugin Host → `/opt/amitia/plugin-host/dist/index.js`
- Task Host → `/opt/amitia/task-host/dist/index.js`

## 第 25 步如何映射 Android Host 路径

第 25 步决定 Android Host 上的物理目录。本步骤只定义 Guest 目标。

## 第 28 步如何建立 Bind

第 28 步按照 Mount Contract 建立真实 PRoot Bind。

## 哪些目录可以清理

- `/run/amitia/*` - Runtime 完整停止后可以清理
- `/var/cache/amitia/*` - 缓存清理流程可以清理

## 哪些目录绝对不能自动删除

- `/etc/amitia`
- `/var/lib/amitia`
- `/var/log/amitia`

## 版本升级边界

- 可替换：`/opt/amitia`
- 不得替换：`/etc/amitia`、`/var/lib/amitia`、`/var/cache/amitia`、`/var/log/amitia`
- 升级前必须重新创建：`/run/amitia`

## 不提交生成目录

以下目录不提交到 Git：

- `runtime/out/guest-layout/`
- `**/__pycache__/`
- `**/.work/`
