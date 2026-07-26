# Amitia 扩展系统重构：开发者生态与扩展中心阶段索引

本阶段范围：第 56—61 步。

1. 第 56 步：实现 TypeScript Plugin SDK
2. 第 57 步：实现 Plugin CLI
3. 第 58 步：实现开发模式与热重载
4. 第 59 步：实现 Extension Developer Console
5. 第 60 步：重建扩展中心
6. 第 61 步：重建 Extension Detail Page

## 本阶段完成后的开发与管理主链

```text
开发者
→ TypeScript SDK
→ Plugin CLI
→ Development Workspace
→ Hot Reload
→ Developer Console
```

```text
用户
→ Extension Center
→ Extension Detail Page
→ Lifecycle Plan
→ Extension Kernel
```

## 核心约束

- SDK 不暴露内部 RPC、Repository、Electron 或数据库。
- CLI 不直接写安装目录和数据库。
- 开发模式不绕过 Manifest、Permission、Scope 和 Runtime。
- Developer Console 不允许直接修改业务真值。
- 扩展管理只保留一个 Extension Center。
- Skill、MCP、Workflow、Plugin 只作为 Contribution 展示。
- 单个 Extension 只保留一个正式详情页。
- 安装、启停、更新、回滚、卸载仍统一通过 Lifecycle Manager。

下一阶段从第 62 步开始，进入最终验证、唯一入口切换和旧系统删除阶段。
