# GameHost External E2E Tests

独立于主仓库源码的外部测试套件，通过真实的 Game Center HTTP API 验证 Mock Game Plugin 的完整生命周期。

## 独立构建

本目录是一个完全独立的 Node.js/TypeScript 项目，不依赖主仓库的任何源码。

### 前置条件

- Node.js >= 18
- 后端服务运行在 `http://127.0.0.1:18899`（默认端口）

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
| `MOCK_PLUGIN_ARCHIVE_PATH_V2` | 否 | Mock 插件 v2 归档包路径（升级测试需要） |

### 测试矩阵

- **smoke** - 基础可达性和安装测试
- **lifecycle** - 完整生命周期测试（安装→启用→运行→停止→卸载）
- **security** - 控制权限和紧急停止测试
- **fault** - 故障恢复和升级测试

## 架构

- `src/backend_driver.ts` - 后端驱动，封装 Game Center API 调用
- `src/game_center_client.ts` - HTTP 客户端
- `src/harness.ts` - 测试工具入口
- `src/waiters.ts` - 异步等待工具
- `src/matrices/*.test.ts` - 测试矩阵
