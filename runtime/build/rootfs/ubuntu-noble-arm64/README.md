# Ubuntu 24.04.4 ARM64 PRoot 基础 RootFS

## 固定版本

| 项目 | 值 |
| --- | --- |
| Distribution | Ubuntu Base |
| Release | `24.04.4` |
| Codename | `noble` |
| Architecture | `arm64` |
| Base Archive | `ubuntu-base-24.04.4-base-arm64.tar.gz` |
| Base Archive SHA-256 | `04207713ece899c3740823d33690441ad3a7f0ded1101aca744e2b0f37ac7ff2` |
| APT Snapshot | `20260212T150000Z` |
| Guest Platform | `linux` |
| Runtime Kind | `proot` |
| Default Locale | `C.UTF-8` |
| Default Timezone | `Etc/UTC` |

## 产物目录

构建成功后生成 `runtime/out/rootfs/ubuntu-noble-arm64/`：

```
rootfs/
rootfs-runtime.json
package-manifest.json
file-manifest.json
SHA256SUMS
test-reports/
amitia-ubuntu-base-24.04.4-arm64.tar.xz
```

## 目录结构

```
runtime/build/rootfs/ubuntu-noble-arm64/
├── rootfs.lock.json
├── packages.requested.json
├── packages.lock.json
├── update_lock.py
├── prepare_apt_cache.py
├── build.py
├── verify.py
├── proot_runner.py
├── filesystem.py
├── archive.py
├── test_build.py
├── test_archive.py
├── test_proot.py
├── README.md
├── overlays/
│   ├── etc/
│   │   ├── hostname
│   │   ├── hosts
│   │   ├── nsswitch.conf
│   │   ├── profile.d/
│   │   │   └── amitia-runtime.sh
│   │   └── apt/
│   │       └── apt.conf.d/
│   │           ├── 01autoremove-amitia
│   │           └── 99amitia-runtime
│   └── usr/
│       └── sbin/
│           └── policy-rc.d
├── .cache/
│   ├── base/
│   │   └── ubuntu-base-24.04.4-base-arm64.tar.gz
│   └── apt/
│       ├── archives/
│       │   └── *.deb
│       ├── lists/
│       └── cache-manifest.json
└── .work/
```

## 构建环境要求

必须安装：
- Python 3.8+
- `proot`
- 非 ARM64 环境额外需要 `qemu-aarch64-static`

禁止要求：
- root 权限
- chroot
- Docker
- systemd-nspawn
- LXC
- KVM
- 完整虚拟机

## 在线缓存准备

```shell
python update_lock.py
python prepare_apt_cache.py
```

`update_lock.py` 根据固定 APT Snapshot 解析完整依赖闭包，生成 `packages.lock.json`。

`prepare_apt_cache.py` 下载 Base 归档和全部 `.deb` 文件到本地缓存。

## 在线构建

```shell
python build.py --clean
```

从 Ubuntu 官方归档下载 Base，使用 PRoot + QEMU 构建完整 RootFS。

## 离线构建

```shell
python build.py --clean --offline
```

只使用已验证缓存，禁止任何网络访问。

## WSL 构建

在 WSL2 Ubuntu 中：

```shell
sudo apt install proot qemu-user-static
python build.py --clean
```

## Linux ARM64 原生构建

在 ARM64 Linux 主机上：

```shell
sudo apt install proot
python build.py --clean
```

## Linux x86_64 + QEMU 构建

```shell
sudo apt install proot qemu-user-static qemu-user
python build.py --clean
```

## 静态验证

```shell
python verify.py --mode static

验证 RootFS 结构、关键文件、用户配置、安全清理状态。
```

## PRoot Runtime 验证

```shell
python verify.py --mode runtime

在 Linux 环境中使用 PRoot（+QEMU）执行 ARM64 用户态，测试：
- 基础命令执行
- 架构识别（aarch64）
- 页面大小检测
- C.UTF-8 中文读写
- /tmp 写入
- /home/amitia 写入
- /proc 读取
- /dev/null 和 /dev/urandom
- 时区 UTC
- 回环网络配置
```

## Node 临时兼容验证

```shell
python verify.py --mode runtime --node-distribution runtime/out/node/linux-arm64/node

将第 15 步 Node 产物临时绑定到 /tmp 验证启动能力。

RootFS 归档不包含 Node。验证后自动解除绑定。
```

## Qdrant 临时兼容验证

```shell
python verify.py --mode runtime --qdrant-distribution runtime/out/qdrant/linux-arm64/qdrant

将第 17 步 Qdrant 产物临时绑定启动版本命令验证。

RootFS 归档不包含 Qdrant。验证后自动解除绑定。
```

## 可复现构建验证

连续执行两次离线构建，对比归档 SHA-256：

```shell
python build.py --clean --offline
python build.py --clean --offline
sha256sum runtime/out/rootfs/ubuntu-noble-arm64/amitia-ubuntu-base-24.04.4-arm64.tar.xz
```

两次 SHA 必须一致。

## 单元测试

```shell
python -m unittest test_build.py test_archive.py test_proot.py
```

覆盖范围：
- 锁文件验证
- Package 锁验证
- 离线缓存验证
- Overlay 白名单
- 清理流程
- 发布原子性
- 归档安全性
- 归档确定性
- PRoot 命令构建
- PRoot 环境隔离

## 路径合同（第 19-20 步消费）

最终 RootFS 包含标准 Linux 目录结构：
- `/bin`、`/sbin`、`/usr`（merged-usr）
- `/etc`（含固定配置）
- `/dev`、`/proc`、`/sys`（空挂载点）
- `/run`、`/tmp`（1777 权限）
- `/root`（root 用户 Home）
- `/home/amitia`（UID 1000）

第 19 步定义并创建：
- `/opt/amitia`（Runtime 程序）
- `/var/lib/amitia`（数据目录）
- `/var/log/amitia`（日志目录）
- `/var/cache/amitia`（缓存目录）
- `/run/amitia`（运行时状态）
- `/workspace`（用户工作区）
- `/data`（用户数据）
- `/extensions`（插件目录）
- `/plugins`（内置插件）
- `/skills`（技能包）

第 20 步组装：
- Ubuntu RootFS
- Amitia Go 后端
- Node Runtime
- Qdrant Runtime
- Runtime Manifest
- 启动脚本

## 下游消费

- 第 15 步: 验证 Node 产物能否在 RootFS 内运行
- 第 17 步: 验证 Qdrant 产物能否在 RootFS 内启动
- 第 19 步: 在 RootFS 内部署 Amitia 运行目录结构
- 第 20 步: 组装完整 Runtime 包
- 第 24 步: Flutter 集成验证

## 静态搜索验收

执行以下搜索确保生产逻辑符合规范：

```shell
# 禁止动态 latest / Daily Build / Docker / Podman
rg -n 'latest|daily/current|docker pull|podman|debootstrap|mmdebstrap' \
  runtime/build/rootfs/ubuntu-noble-arm64 --glob '!README.md' --glob '!test_*.py'

# 禁止依赖 systemd 等服务
rg -n 'systemctl|service |sudo |sshd|snapd|cloud-init' \
  runtime/build/rootfs/ubuntu-noble-arm64 --glob '!README.md' --glob '!test_*.py'

# 禁止使用 shell 执行
rg -n 'shell=True|os\.system|subprocess.*shell' \
  runtime/build/rootfs/ubuntu-noble-arm64 --glob '!test_*.py'

# 禁止在 Overlay 中包含 Node/Qdrant/Amitia
rg -n '/opt/amitia|/var/lib/amitia|/var/log/amitia|node/bin/node|qdrant/bin/qdrant' \
  runtime/build/rootfs/ubuntu-noble-arm64/overlays
```

## 安全清理

RootFS 构建完成后：
1. 清除所有 SetUID/SetGID 位
2. 删除 Machine ID 内容
3. 删除 SSH 主机密钥
4. 删除 Shell 历史
5. 清空 APT 缓存
6. 清空包管理日志
7. 删除构建机器信息
8. 拒绝设备节点
9. 拒绝 World-Writable 普通文件

## 服务启动抑制

`/usr/sbin/policy-rc.d` 统一返回 101，阻止：
- APT 安装时启动 SysV 服务
- 运行时通过 SysV 脚本启动守护进程

## PRoot 兼容性

- 不依赖 systemd
- 不依赖 snapd
- 不依赖 D-Bus
- 不依赖 udev
- 不依赖 cron
- 不依赖 rsyslog

## 版本升级流程

1. 更新 `rootfs.lock.json` 中的 `release`、`codename`、`baseArchiveName`、`baseArchiveSha256`
2. 更新 `aptSnapshot`
3. 从 https://cdimage.ubuntu.com/ubuntu-base/releases/ 获取新 Base 归档 SHA-256
4. 删除旧缓存
5. 重新构建并验证

## Package 升级流程

1. 更新 `packages.requested.json`
2. 运行 `update_lock.py`
3. 运行 `prepare_apt_cache.py`
4. 重新构建并验证

## 与第 15 步的边界

本步骤只验证 Node 产物（`node/bin/node`）是否能在 RootFS 内运行。不注入、不修改、不复制 Node 到最终 RootFS。

## 与第 17 步的边界

本步骤只验证 Qdrant 产物（`qdrant/bin/qdrant`）是否能在 RootFS 内启动版本命令。不注入、不配置、不启动 Qdrant 正式服务。

## 与第 19 步的边界

第 19 步负责Runtime程序目录、配置目录、数据目录、日志目录、缓存目录、Workspace、用户数据挂载、Node 目录、Qdrant 目录和 Go 后端目录。

本步骤只构建通用 Ubuntu 基础系统，不创建任何 Amitia 专用目录。

## 与第 20 步的边界

第 20 步负责将 RootFS、Go 后端、Node 和 Qdrant 组装成完整 Runtime 包，包括 Runtime Manifest 和启动脚本。

本步骤不生成完整 Runtime 包。

## 生成文件不提交 Git

- `runtime/build/rootfs/ubuntu-noble-arm64/.cache/`（下载缓存）
- `runtime/build/rootfs/ubuntu-noble-arm64/.work/`（临时工作目录）
- `runtime/out/`（构建产物）
- `__pycache__/` / `*.pyc`
- `*.partial`

## 联网测试边界

默认测试完全离线。网络测试需显式启用：

```shell
python verify.py --mode runtime --enable-network-test
```
