# Qdrant Linux ARM64 Runtime Builder

基于官方 Qdrant Release 构建可复现、可校验、可离线安装的 Linux ARM64 Runtime 产物。

## 固定版本

| 项目 | 值 |
| --- | --- |
| Qdrant | `1.19.0` |
| Release Tag | `v1.19.0` |
| Release Commit | `74f3e85b9473c62560006c043e13737ce6b48412` |
| 平台 | linux |
| 架构 | arm64 |
| Rust Target | `aarch64-unknown-linux-musl` |
| Libc | musl |
| 官方资产 | `qdrant-aarch64-unknown-linux-musl.tar.gz` |

资产 SHA 与许可证 SHA 均来自 [`qdrant.lock.json`](./qdrant.lock.json)。

## 文件清单

```text
qdrant.lock.json      # 版本锁（Tag、Commit、资产 SHA）
LICENSE-QDRANT        # Apache 2.0，来源 qdrant/qdrant@v1.19.0
update_lock.py        # 重新生成锁文件（从官方 GitHub Release）
build.py              # 在线/离线构建脚本
verify.py             # 静态 / Runtime 验证脚本
smoke_test.py         # ARM64 Runtime 冒烟测试
elf_inspector.py      # ELF Header 静态检查（纯标准库）
test_build.py         # 构建脚本单元测试
test_smoke.py         # Smoke Test 单元测试
fixtures/             # 测试用小型Fixture目录
```

## 如何重新生成锁文件

仅限离线状态变化或版本变更时使用：

```bash
python update_lock.py
python update_lock.py --tag v1.19.0
python update_lock.py --output qdrant.lock.json
```

仅允许 `v1.19.0`，修改版本须显式传 `--allow-version-change`。

## 在线构建

```bash
python build.py --clean
```

## 离线构建

指定并使用 SHA 校验后的缓存或源资产：

```bash
python build.py --offline --source-archive /path/to/qdrant-aarch64-unknown-linux-musl.tar.gz
```

## 自定义输出

```bash
python build.py --cache-dir /tmp/cache --output-dir /tmp/out --work-dir /tmp/work
## Static 验证

检查产物结构、权限、ELF、SHA 与归档：

```bash
python verify.py --mode static
python verify.py --mode static --report /tmp/report.json
```

## ARM64 Runtime 验证

仅在真实 Linux ARM64 环境中执行：

```bash
python verify.py --mode runtime --distribution runtime/out/qdrant/linux-arm64/qdrant
```

## 页面大小

Linux ARM64 Runtime 验证在真实环境中输出：

```text
Page size: 4096
```

或：

```text
Page size: 16384
```

4KB 与 16KB 测试结果分开记录，未执行测试不会标记为通过。第 41 步补充 Android PRoot 16KB 真机验收。

## Smoke Test

真实 Linux ARM64 环境执行：

```bash
python smoke_test.py --distribution runtime/out/qdrant/linux-arm64/qdrant
python smoke_test.py --distribution runtime/out/qdrant/linux-arm64/qdrant \
  --report runtime/out/qdrant/linux-arm64/test-reports/smoke.json
```

标准流程包括：配置生成、启动、`/readyz`、创建 Collection、Upsert、Query、SIGTERM 停
止、使用相同 Storage 重启、持久化验证。

## 持久化验证

Smoke Test 第二次重启验证：

- Collection 仍存在
- Point 仍可读取
- Query 返回一致结果
- 删除临时目录无残留

## 产物结构

```text
runtime/out/qdrant/linux-arm64/
├── qdrant/
│   ├── bin/
│   │   └── qdrant        # 0755
│   └── LICENSE            # 0644
├── qdrant-runtime.json    # 0644
├── file-manifest.json     # 0644
├── SHA256SUMS             # 0644
└── amitia-qdrant-v1.19.0-linux-arm64-musl.tar.xz
```

## 第 9 步路径合同

最终产物安装至 `<runtime-root>/qdrant/bin/qdrant`。第 9 步 Resolver 解析为：

```text
amitia://runtime/qdrant/bin/qdrant
```

路径不得包含版本号。

## 第 18 步注入方式

第 18 步 Ubuntu ARM64 rootfs 可直接消费本产物根目录结构。

## 第 20 步统一 Runtime

第 20 步将本产物与 Go 后端、Node、SurrealDB 组装为统一 Runtime 包。

## 第 39 步 CI 接入

CI 中执行 `build.py --offline` 并传 `--source-archive`，配合已缓存资产或内部镜像库。

## 第 41 步 Android PRoot 验收

需要在 Android PRoot 环境重新执行 smoke_test.py 与 verify.py。

## 禁止提交

以下文件位于 `.gitignore`，禁止提交：

```text
.cache/
.work/
__pycache__/
*.pyc
test-reports/
```

## 禁止行为

- 使用 `latest`、nightly、dev 版本
- 使用第三方镜像或现场编译
- 修改 Go 后端
- 把版本号写入运行路径
- 把数据或配置写入发布产物

## 最终产物不含

- 配置文件
- 数据目录
- Storage
- Snapshot
- WAL
- 临时数据
- PID 文件

## 许可证

Qdrant 官方 Apache 2.0，见 `LICENSE-QDRANT`。
