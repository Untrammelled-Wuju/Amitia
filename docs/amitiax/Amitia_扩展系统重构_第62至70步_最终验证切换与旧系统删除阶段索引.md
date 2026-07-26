# Amitia 扩展系统重构：最终验证、切换与旧系统删除阶段索引

本阶段范围：第 62—70 步。

1. 第 62 步：执行新旧系统等价性验证
2. 第 63 步：执行桌面端稳定性验收
3. 第 64 步：执行安全、权限与隔离验收
4. 第 65 步：切换 Extension Kernel 为唯一入口
5. 第 66 步：删除旧 Plugin Runtime
6. 第 67 步：删除旧 Skill 兼容层
7. 第 68 步：删除旧 `.amitiax` 包解析器与安装链
8. 第 69 步：删除重复生命周期状态表与旧数据模型
9. 第 70 步：执行 Extension Kernel 最终总验收

## 本阶段完成后的最终状态

```text
Extension Kernel
├── 唯一领域模型
├── 唯一生命周期管理器
├── 唯一 Contribution Registry
├── 唯一 Dependency Resolver
├── 唯一 Runtime Supervisor
├── 唯一 Host API Gateway
├── 唯一 Permission / Scope
├── 唯一 Storage / Secret / Resource
├── 唯一 Event / Hook / Scheduler
├── 唯一 Extension Center / Detail
└── 唯一生产数据模型
```

## 必须被删除的旧主链

```text
旧 Plugin Runtime
旧泛 Skill Runtime
旧 MCP 生命周期和 Tool Registry
旧 Workflow Executor/Scheduler
旧 PackageService 与生产 Parser
旧 Tool 拼接和 Prompt 注入
旧 UI 动态注入
重复 Enabled/Run/State/Scope/Permission 表
```

## 最终发布条件

- P0 问题清零。
- P1 问题清零或经过正式风险接受。
- 三平台核心链路通过。
- 数据迁移、数据库恢复和应用更新回滚通过。
- 新旧系统不存在双执行、双调度、双连接和双写。
- Extension Kernel v1、Manifest v2、Host API v1、Runtime RPC v1、Schema UI v1 和 SDK v1 已锁定。
