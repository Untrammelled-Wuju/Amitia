# Workbench Batch 02 开发记录

本轮基于 `U-Ai_Workbench_Batch01_完整源码包.zip` 继续修改，未重建独立 Workspace、Git 或 Browser 后端。

## 已落地

- Web/Electron：多标签固定、位置移动 Store、关闭全部未固定标签、键盘标签切换、会话草稿迁移、布局持久化、标签组件缓存。
- Web 资源视图：目录分页、文本分段读取、Git 状态和提交详情、Diff 着色、Diff 大文本渐进显示。
- Web 浏览器：HTTP(S) 地址校验、导航状态保存、刷新、系统浏览器回退；此视图仍是隔离 iframe，不提供 CDP 自动化。
- AgentUIEvent：消费 `workbench.open` 事件并转换为统一 `open-tab-v1` 命令，按会话、操作 ID 和焦点策略约束；事件发布端尚未接入。
- Flutter：文件浏览点击进入独立文件标签、Git Diff 进入独立 Diff 标签、多浏览器各自地址栏、多页面状态保活、浏览器前进后退、固定标签、持久化队列、跨设备标签安全阻断。
- 协议：新增 `open-event-v1.schema.json`，与 `open-tab-v1.schema.json` 组合使用。

## 验证边界

已执行：TypeScript 转译语法检查、Workbench Store 独立执行回归、Agent 事件适配器回归、JSON Schema 解析、压缩包目录与一致性检查。

未执行：Vue SFC 模板编译、Vue 完整类型检查、Vitest 原生运行、Flutter Analyze、Android Release 签名构建与真机安装、完整服务重启、跨设备 Device Mesh 授权测试、Browser Runtime 实际 CDP 联调。当前环境没有项目 node_modules、Flutter SDK 与 Windows 发布环境。

## 后续关键项

1. 将后端 Agent 事件管线实际产出 `workbench.open`，补事件持久化、重放及授权。
2. 提供会话绑定的 Browser Runtime 视图 API，完成 CDP 页面与用户工作区同页可视化及控制权租约。
3. 增加 Device Mesh 目标设备授权和资源代理，不能直接将目标 URI 路由到本机。
4. 补齐 Flutter 端 AI 命令消费、移动底部抽屉和分栏适配。
5. 交互 PTY、多 Agent 视图及终端流恢复。
6. 在目标构建环境执行 Vue 类型检查、Flutter Release、真机和 Electron 端到端测试。
