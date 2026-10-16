# U-Ai Workbench Batch 44：Browser / Device Mesh 控制与生命周期收口

## 开发范围

以第 43 轮累计完整源码为基线，将 Browser 观察与跨设备读取路径的可复现并发、类型与恢复缺陷收口。继续采用研发范围估算与生产验收分离的口径，不因缺少正式构建环境停滞研发，也不把隔离回归当作生产端到端验收。

## 本轮落地

- Agent 同页手动控制改为单写者，禁止同一观察者在前一个输入指令仍执行时并行下发第二个变更命令；Agent 操作与用户操作的既有互斥规则保持不变。
- 远程浏览器网关不再回传设备未经规范化的原始 JSON。按照 state/dom/screenshot 读取类型验证远端结果，拒绝不匹配的字段、无效 JPEG/Base64、过大 DOM 或截图、伪装成成功的错误响应；成功后只返回受限的结构化结果。
- Web 浏览器视图添加生命周期请求取消和代次校验。标签切换、视图销毁、显式停止后不再采用旧截图、旧 DOM、旧控制权回包覆盖当前状态；重新激活允许发起新代次截图请求，而不会被旧请求长期阻挡。
- 新增实际 Go 源码链接隔离 race 测试和 Web Vue script 原源码异步测试，纳入研发验证门禁。

## 验收限制

- Browser Go 源码测试使用最小 Browser 契约桩和 Go 1.23.2 运行，真实源码中的 control_lease/agent_bridge/remote_read 逻辑参与 race 测试；不代表真实 Chromium、Gin Gateway 或完整后端构建通过。
- Web 视图测试执行从真实 Vue 组件提取的 TypeScript setup 逻辑并模拟延迟请求；不代表完整 Vue 类型检查、Electron 启动或真实浏览器 UI 验收。
- Device Mesh 远端可信设备、断线配对、不同终端同时操作及 Chromium CDP 同页共享仍需真实环境联调；不修改设备配对、Git 远端或发布产物。
