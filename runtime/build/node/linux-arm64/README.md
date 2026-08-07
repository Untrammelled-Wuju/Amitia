# Linux ARM64 Node Runtime 构建

## 固定版本

| 项目       | 值                                                                 |
| --------- | ------------------------------------------------------------------ |
| Node.js   | 24.19.0                                                            |
| LTS 代号   | Krypton                                                            |
| npm       | 11.17.0                                                            |
| N-API     | 137                                                                |
| 平台       | linux                                                              |
| 架构       | arm64                                                              |
| 归档       | `node-v24.19.0-linux-arm64.tar.xz`                                |
| SHA-256   | `01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc` |

来源: Node.js 官方发行服务器 `https://nodejs.org/dist/v24.19.0/`

## 产物目录

构建成功后生成 `runtime/out/node/linux-arm64/`：

```
node/
  bin/node
  bin/npm -> ../lib/node_modules/npm/bin/npm
  bin/npx -> ../lib/node_modules/npm/bin/npx
  lib/node_modules/npm/
  include/node/
  LICENSE
  README.md
node-runtime.json
file-manifest.json
SHA256SUMS
amitia-node-v24.19.0-linux-arm64.tar.xz
```

## 在线构建

```shell
python build.py --clean
```

从 `nodejs.org` 下载官方归档，校验 SHA-256，解压裁剪后重新打包。

## 离线构建

```shell
python build.py --offline
```

或指定本地源归档：

```shell
python build.py --offline --source-archive D:/path/to/node-v24.19.0-linux-arm64.tar.xz
```

## 自定义路径

```shell
python build.py --cache-dir D:/cache/node --output-dir D:/out/node --work-dir D:/work/node
```

## 静态验证

```shell
python verify.py --mode static
```

## ARM64 Runtime 验证

在 Ubuntu 24.04 ARM64 中：

```shell
python verify.py --mode runtime
```

验证 Node 版本、npm 版本、npx 版本、`test-runtime.mjs` 运行时测试。

## 单元测试

```shell
python -m unittest test_build.py
```

## 路径合同（第 6 步消费）

最终安装布局：

```
<runtime-root>/node/
  bin/node
  bin/npm
  bin/npx
  include/node/
  lib/node_modules/npm/
```

第 6 步 `nodeenv.Resolver` 通过以下路径访问：

```
amitia://runtime/node/bin/node
amitia://runtime/node/lib/node_modules/npm/bin/npm-cli.js
amitia://runtime/node/lib/node_modules/npm/bin/npx-cli.js
```

## 下游消费

- 第 16 步: Node Runtime 启动脚本消费本产物
- 第 18 步: 将本产物注入 Ubuntu rootfs 或 Runtime 组装目录
- 第 20 步: 节点、Go 后端、Qdrant 和 rootfs 统一组装
- 第 24 步: Flutter 集成验证
- 第 39 步: CI 接入

## 版本升级流程

1. 更新 `node.lock.json` 中的 version、npmVersion、napiVersion、archiveName、archiveRoot、archiveSha256
2. 从官方获取新 SHA-256（`https://nodejs.org/dist/v<version>/SHASUMS256.txt`）
3. 删除旧缓存 `runtime/build/node/linux-arm64/.cache/`
4. 重新构建并验证

## 原生 npm 模块限制

- 纯 JavaScript npm 包可正常使用
- 预编译 ARM64 glibc Native Addon 可能可用
- 需要本地编译的 Native Addon 依赖后续 Ubuntu 工具链（gcc/g++/make/Python）
- 本产物不包含编译工具链，不保证所有 npm 包可安装

## 不提交 Git 的文件

- `runtime/build/node/linux-arm64/.cache/`（下载缓存）
- `runtime/build/node/linux-arm64/.work/`（临时工作目录）
- `runtime/build/node/linux-arm64/__pycache__/`、`*.pyc`
- `runtime/out/`（构建产物）

## 安全解压

构建脚本拒绝以下归档成员：

- 绝对路径
- Windows 盘符
- `..` 路径穿越
- 绝对符号链接
- 越界符号链接
- 越界硬链接
- 字符/块设备、FIFO
- 多顶层目录
- 错误归档根

## 可复现打包

同一输入连续构建两次，SHA-256 一致：

- 固定 uid=0 / gid=0
- 固定 mtime=0 (Unix Epoch 0)
- 固定用户名 root / 组 root
- 目录 0755 / 普通文件 0644 / node 0755
- XZ 压缩等级 5
- 成员按 UTF-8 路径升序
