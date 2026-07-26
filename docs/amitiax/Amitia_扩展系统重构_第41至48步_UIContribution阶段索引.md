# Amitia 扩展系统重构：UI Contribution 阶段索引

本阶段范围：第 41—48 步。

1. 第 41 步：定义 UI Contribution 协议
2. 第 42 步：升级 Schema UI 系统
3. 第 43 步：实现沙箱 Web UI
4. 第 44 步：建立前端 Extension Slots
5. 第 45 步：实现 Extension Page Host
6. 第 46 步：实现聊天与消息 UI 扩展
7. 第 47 步：实现 Electron 桌面扩展点
8. 第 48 步：实现 UI 冲突、排序与布局规则

## 本阶段完成后的 UI 架构

```text
UIContributionDefinition
→ Contribution Registry
→ UI Slot / Page / Chat / Desktop Adapter
→ Schema UI / Restricted Web UI / Host-native UI
→ Permission / Scope / Runtime
→ Stable Rendering
```

## 核心安全边界

- 扩展不得动态注入 Vue 组件。
- 扩展不得访问 Pinia、DOM、Electron Main 或任意 IPC。
- 完整页面统一使用 Extension Page Host。
- 复杂 Web UI 运行在沙箱中。
- 聊天扩展不能接管消息主链。
- 桌面扩展不能运行主进程代码。
- 多扩展冲突由宿主和用户偏好稳定解决。

下一阶段从第 49 步开始，进入旧系统到 Extension Kernel 的正式迁移阶段：内置 Tool、Agent Skill、MCP、Workflow、官方 Plugin、旧 `.amitiax` 和扩展数据迁移。
