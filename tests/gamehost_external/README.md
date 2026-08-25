# GameHost External E2E Tests

独立于主仓库源码的外部测试套件，通过真实的 Game Center HTTP API 验证 Mock Game Plugin 的完整生命周期。

## 独立构建

本目录是一个完全独立的 Node.js/TypeScript 项目，不依赖主仓库的任何源码。

### 前置条件

- Node.js >= 18
- 后端服务运行在 `http://127.0.0.1:18899`（默认端口）
- 后端必须启用 `AMITIA_EXTENSION_DEV_MODE=true`，因为外部 conformance 包故意保持 unsigned-dev 身份
- 测试使用真实用户 Access Token 和与 `mock-developer/mock-amitiax-game-plugin` 绑定的 Developer Session；不会绕过 Extension Kernel 安装安全链路

### 安装依赖

```bash
cd tests/gamehost_external
npm install
```

### 运行测试

```bash
# 运行全部测试
npm test

# 运行指定矩阵
npm run test:smoke
npm run test:lifecycle
npm run test:security
npm run test:fault
```

### 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `MOCK_PLUGIN_ARCHIVE_PATH` | 是 | Mock 插件 v1 归档包路径 |
| `MOCK_PLUGIN_ARCHIVE_PATH_V2` | 是（lifecycle/fault upgrade） | Mock 插件 v2 归档包路径 |
| `GAMEHOST_BASE_URL` | 否 | GameHost API 根地址，默认 `http://127.0.0.1:18899/api` |
| `GAMEHOST_AUTH_TOKEN` | 是 | `/api/public/auth/setup` 或 `/api/public/auth/login` 返回的用户 Access Token |
| `GAMEHOST_DEVELOPER_SESSION_ID` | 是 | 通过 `/api/extensions/dev-mode/workspaces` → trust → session 创建的 unsigned-dev 安装会话 |
| `GAMEHOST_BACKEND_RESTART_COMMAND` | 是（完整 lifecycle） | 由测试运行器提供的后端重启命令；不得在测试内探测/杀本机进程 |
| `GAMEHOST_BACKEND_CWD` | 否 | 重启命令工作目录，默认当前目录 |

### 测试矩阵

- **smoke** - 基础可达性和 canonical Extension Package 安装测试
- **lifecycle** - 完整生命周期测试（安装→启用→运行→停止→卸载）
- **security** - 控制权限和紧急停止测试
- **fault** - 故障恢复和升级测试

## 架构

- `src/backend_driver.ts` - 后端驱动，封装 Game Center API 调用
- `src/game_center_client.ts` - HTTP 客户端
- `src/harness.ts` - 测试工具入口
- `src/waiters.ts` - 异步等待工具
- `src/matrices/*.test.ts` - 测试矩阵


> `npm test` 固定使用 `--runInBand`。所有矩阵共享同一个真实后端和同一个固定 extension id，并发执行会人为制造安装/卸载竞态，因此禁止 Jest worker 并发。每个测试也会对 fixture extension 做兜底清理，防止失败用例污染后续矩阵。


## 真实 CI 门禁

`.github/workflows/gamehost.yml` 的 `external-gamehost-e2e` job 会启动真实本地 GameHost 后端、创建开发者信任会话、通过 Extension Package 正式生命周期安装 v1/v2 `.amitiax` 包，并串行执行全部 smoke/lifecycle/security/fault 矩阵。后端重启由 `scripts/restart-backend.sh` 模拟宿主异常退出，测试必须证明运行态恢复后没有重复进程、连接或其他 residue。
