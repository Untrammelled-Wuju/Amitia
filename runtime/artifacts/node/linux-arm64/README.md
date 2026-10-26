# Node Runtime Lock - Linux ARM64

本目录包含 Linux ARM64 Node Runtime 的固定版本信息。

| 项目 | 值 |
| --- | --- |
| Node.js | 26.12.1 |
| 平台 | linux |
| 架构 | arm64 |
| 归档文件名 | node-v26.12.1-linux-arm64.tar.xz |
| 官方 SHA-256 | `42aa04922591ee5d4dbc7740d452621acf28508239daa512f5ee8c47ee224fdb` |

## 来源

- 官方下载: https://nodejs.org/dist/v26.12.1/node-v26.12.1-linux-arm64.tar.xz
- SHA 校验源: https://nodejs.org/dist/v26.12.1/SHASUMS256.txt

## 引用关系

prepare 脚本读取本目录的 `node-runtime-lock.json` 作为唯一版本输入。

## 版本升级流程

1. 从 Node.js 官方获取目标版本的 SHA-256
2. 更新本 lock 文件的 `version`、`archiveFileName`、`sourceUrl`、`sha256` 字段
3. 重新执行 prepare 脚本
4. 重新生成 Build Record
