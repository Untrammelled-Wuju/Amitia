# CI 测试分组策略

> Amitia 扩展系统重构 第 5 步
> 版本：2026-07-25

## 分组原则

六组测试覆盖从快速检查到完整集成的所有层级。快速检查(quick)必须在每次提交前通过；桌面端测试(electron)仅在发布前执行。

每组测试使用独立的入口脚本，支持 `-Verbose` 获取详细输出和 `-Filter` 筛选特定测试。

## 分组一览

| 分组 | 入口 | 类型 | 覆盖率 | 预计耗时 | 失败阻断 |
|---|---|---|---|---|---|
| quick | `run-extension-unit.ps1` | Go单元测试 | Extension + MCP | ~30s | 阻断合并 |
| integration | `run-extension-integration.ps1` + `run-mcp-integration.ps1` | Go集成测试 | Extension DB + MCP transport | ~60s | 阻断合并 |
| security | `run-extension-security.ps1` | Go安全测试 | Archive/权限/Secret/路径 | ~30s | 阻断发布 |
| migration | `run-extension-migration.ps1` | Go迁移测试 | 旧行为基线/迁移路径 | ~30s | 不阻断 |
| frontend | `run-extension-frontend.ps1` | Vitest | 扩展中心组件 | ~30s | 不阻断 |
| electron | `run-extension-electron.ps1` | Playwright | 桌面端集成 | N/A | 阻断发布 |

## 执行命令

```powershell
# 快速检查
.\scripts\test\run-extension-unit.ps1

# 安全测试
.\scripts\test\run-extension-security.ps1

# 全部分组
.\scripts\test\run-all.ps1 -Group all

# 验证特定测试，显示详细输出
.\scripts\test\run-extension-unit.ps1 -Verbose -Filter "TestLegacy_Registry"
```

## CI环境要求

| 需求 | 版本 | 用途 |
|---|---|---|
| Go | 1.26.1 (见go.mod) | 后端测试 |
| Node.js | LTS | 前端Vitest测试 |
| pnpm | 最新 | 前端依赖管理 |
| Playwright | 最新 | Electron集成测试（待安装） |
| PowerShell | 7+ | 脚本执行 |

## 失败处理

- **quick/integration/security** 失败时需定位到具体测试，不得简单跳过或重试
- **migration** 允许因环境差异导致的已知失败，需记录Issue
- **frontend** 允许因组件重构导致的临时失败，标记后应尽快修复
- **electron** 当前环境阻塞，脚本以exit code 2标记，发布前必须解除

## 输出保留

CI失败时必须保留：

- Go test输出（含FAIL行和panic堆栈）
- 前端Vitest输出
- 后端日志（如涉及数据库）
- 进程列表（如有残留进程）

不得保留任何Secret内容在CI日志中。

## 与GitHub Actions集成

待项目建立GitHub仓库后可添加 `.github/workflows/extension-tests.yml`：

```yaml
name: Extension Tests
on: [push, pull_request]
jobs:
  quick:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26.1' }
      - run: pwsh scripts/test/run-extension-unit.ps1
  integration:
    runs-on: windows-latest
    needs: quick
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26.1' }
      - run: pwsh scripts/test/run-extension-integration.ps1
      - run: pwsh scripts/test/run-mcp-integration.ps1
  security:
    runs-on: windows-latest
    needs: integration
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.26.1' }
      - run: pwsh scripts/test/run-extension-security.ps1
  frontend:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: 'lts' }
      - run: cd front && pnpm install
      - run: pwsh scripts/test/run-extension-frontend.ps1
```
