# U-Ai Workbench Batch 47：浏览器写入代次与 Device Mesh 观察恢复

本轮以 Batch 46 累计源码为基线，继续完善 Browser / Terminal / Device Mesh 阶段中与浏览器控制、配对和断线有关的可靠性。未重写现有 Chromium、PTY 或设备调度引擎。

## 实现范围

1. 后端 `BeginManualTabMutation` 现在必须同时校验观察引用、所有者、随机租约代次及当前有效期；只有当前代次能够进入单写者门禁。
2. Browser 本地 `navigate`、`action`、`input` 均从请求体接收 `leaseId`，Agent 观察标签未提交有效代次时拒绝变更。独立浏览器标签不需要 Agent 接管租约。
3. Web/Electron 工作台每次提交变更时固定当前租约令牌；请求晚到或失败时，用页面生命周期和租约代次阻止旧响应写回新的页面状态。
4. Flutter 的导航、操作和输入使用提交前捕获的租约代次，异步响应还会核验当前代次。远端设备的浏览器观察仍保持只读。
5. Device Runtime 在远端观察完成后对比当前凭据 ID 和凭据密钥，拒绝配对凭据已轮换的旧观察结果；Cloud Core 在返回结果前核验当前设备连接是否仍为请求开始时的连接，断线重连后需发起新请求。
6. 修正一条旧版测试对并发 Agent 写入的错误假设，使测试符合单标签单写者规则，且验证旧完成回调不能解除新一代 Agent 写入锁。

## 本轮定向验证

定向回归使用真正的工作区 Go 源码、Web Vue 脚本逻辑和 Flutter/Device Mesh 源码约束，不包含假装启动完整服务的模拟认证响应。实际运行结果保存在交付验收 JSON 中。

- Browser 原源码配隔离契约桩的 Go race 测试覆盖令牌缺失、旧令牌重放、跨观察者写入、Agent/人工互斥和释放后的原子并发边界。
- Web 源码转译执行测试覆盖导航、动作、输入实际携带租约代次，以及旧响应不能覆盖最新代次。
- Flutter 源码级校验确认使用捕获的租约 ID、响应核验和已有生命周期隔离；尚未经 Flutter Analyzer 或 Android/iOS Release 验证。
- Device Mesh 源码级校验覆盖配对凭据轮换和断线重连后旧结果隔离；尚未进行真实多设备联网测试。
- 后端源码语法检查覆盖 Workbench 相关 Go 文件，不等价于完整模块类型检查。

## 尚未完成的生产验收

完整后端依赖编译和启动、实际 Chromium/CDP 用户与 Agent 接管、真实双设备在途断线重连、Flutter Release、Electron 启动以及 Windows ConPTY 验收均未通过本轮生产门禁。研发实现进度与生产验收通过率仍分别记录。
