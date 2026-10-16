# Workbench Batch04：Agent 浏览器共享观察与移动端 Chromium 视图

## 基线

源码基线：`U-Ai_Workbench_Batch03_完整源码包.zip`，基于最初提供的 `U-Ai-source-20261014-1052.zip` 的前三轮累计改动。

## 本轮变更

1. 新增 `workbenchbrowser.AgentTabBridge`，为由后端浏览器工具创建的 Agent Tab 生成不可预测的短期引用；按 SpaceID、ConversationID 限制引用，支持撤销、过期清理和容量上限。
2. BrowserAgent 成功导航到起始 URL 后，向现有 ConversationStream 发布持久化 `workbench.open` 事件。事件默认后台打开标签，不打断聊天；当同进程 BrowserProvider 和引用注册可用时，事件携带只读同页观察引用。
3. 浏览器控制器新增 `POST /api/workbench/browser/observe`。服务端验证设备身份、对话范围与短期引用，绑定到原 Agent Chromium 会话和 Tab，不另外创建 Chromium 页面。
4. 观察状态独立于所有者状态：GET 状态、截图与 DOM 可观察；浏览器导航、前进后退、脚本输入、点击和滚动在服务端被拒绝；关闭观察者标签只解除绑定，不能关闭 Agent 的原会话。
5. 观察许可撤销后，后续读取会被阻断，避免旧标签永久持有原始会话地址；创建独立浏览器与观察同一 ViewID 时拒绝直接覆写。
6. 后端调整会话创建和关闭的锁范围，支持闲置回收、观察配额、点击坐标/滚动量/输入文本长度限制。
7. Web/Electron 工作区增加 `browserRef` 的持久化、Agent 消息协议映射、只读 Chromium 截图观察与网页 URL 同步。
8. Flutter 工作区新增真正的设备 Chromium 视图，支持受认证截图流、导航、点击、输入、滚动和 DOM；提供显式 WebView 轻量预览切换。
9. Flutter 的 Agent 工作区事件保留 `browserRef`，多标签隔离并保留背景打开规则；在观察模式禁用修改操作。
10. 增加服务端 Scope、撤销、过期测试，以及前端 Agent 引用协议与标签隔离测试。

## 安全条件

- 仅接受受认证 API 请求；Agent 浏览器引用不能单独跳过设备及对话认证。
- 观察令牌随机生成，最长保留 6 小时；当 Agent BrowserAgent 不保留会话时，执行结束立即撤销。
- 默认只读，观察者的 HTTP 修改入口服务端返回 403。
- 未授权跨设备目标仍不使用当前设备代替执行。
- 暂不开放操作控制权接管，避免同一页面出现两个并发控制者。
- 观察者不可用时明确报告错误，不宣称已共享实际页面。

## 限制与未验收项

- 只在浏览器执行链与 Workbench API 共用同一个 BrowserProvider 的 Runtime 中允许真实同页观察。Cloud Core 与远程设备间的 Agent Tab 转发仍未开发。
- 浏览器引用/页面映射保持在内存，不支持进程重启后恢复原生 Chrome CDP 连接。持久化 UI Tab 可以重新发现错误，但不能恢复失效的 Agent 控制句柄。
- 目前只对具有起始 URL 的 BrowserAgent 执行自动发布，不覆盖任意手写 browser_* 工具组合。
- Flutter Android/iOS Release 构建及真机、Go 单元测试、Electron 启动和真实 Browser Runtime 联调仍须在完整环境中运行。
- 当前实现不等同于 Codex 全量对标完成，更不代表生产级验收。

## 建议后续研发

- 统一 Agent 浏览器会话登记和恢复协议；使用可续订的观察租约替代单次 token。
- 加入严格授权的用户接管流程（原控制者暂停、显式请求、租约释放和审计）。
- Device Mesh 跨设备资源目标路由和双端签发能力票据。
- 实际 Browser CDP/Flutter 截图同步与异常恢复测试、Android Release 构建安装、Windows Electron 端到端回归。
