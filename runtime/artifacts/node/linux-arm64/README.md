# Node Runtime Lock - Linux ARM64

本目录包含 Linux ARM64 Node Runtime 的固定版本信息。

| 项目 | 值 |
| --- | --- |
| Node.js | 24.19.0 |
| 平台 | linux |
| 架构 | arm64 |
| 归档文件名 | node-v24.19.0-linux-arm64.tar.xz |
| 官方 SHA-256 | `01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc` |

## 来源

- 官方下载: https://nodejs.org/dist/v24.19.0/node-v24.19.0-linux-arm64.tar.xz
- SHA 校验源: https://nodejs.org/dist/v24.19.0/SHASUMS256.txt

## 引用关系

prepare 脚本读取本目录的 `node-runtime-lock.json` 作为唯一版本输入。

## 版本升级流程

1. 从 Node.js 官方获取目标版本的 SHA-256
2. 更新本 lock 文件的 `version`、`archiveFileName`、`sourceUrl`、`sha256` 字段
3. 重新执行 prepare 脚本
4. 重新生成 Build Record
