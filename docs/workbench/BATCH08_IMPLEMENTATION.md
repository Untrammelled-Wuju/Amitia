# Batch 08 — Agent 浏览器来源路由、引用验证与多端恢复

## 基线与边界

本轮以 `U-Ai_Workbench_Batch07_累计完整源码包.zip` 为开发基线，针对已经存在的 BrowserAgent / Workbench / Device Mesh 进行增量改造；不更换浏览器引擎，不建立第二条设备调用通道。

## 实现

- BrowserAgent 发布 `workbench.open` 时，附带由初始化后的 ToolFacade 提供的 `originDeviceId`。该值不是模型输入；空身份不被猜测。既有 `browserRef`、对话及 Space 作用域不变。
- Web/Electron 通过云端部署状态、当前 UI 设备身份和 `originDeviceId` 解析观察路由。云端不同设备走现有 `/api/device-mesh/v1/workbench/browser/read`，相同设备及本地部署保持本机 API。云端身份查询失败不回退到错误的本地 Runtime；分发队列保留事件先后顺序。显式给定 `targetDeviceId` 时仍由服务端校验配对与权限。
- Flutter 读取现有部署模式和加密保存的云端配对身份，识别事件来源。云端身份无效或过期时拒绝自动打开，避免误向本机 Runtime 请求另一台设备的观察引用；源设备与本机相同则保留本地路径。
- Web 浏览器视图显式退出后不再自动重建观察或会话；提供手动“重新连接”。远程观察断线采用有限指数退避，避免每次截图失败都不断请求。
- Flutter 远程截图、DOM 或状态读取失败后进入受限重试并增加退避上限；手动退出状态不因 App 切前后台而自行重建。
- 对 Agent 浏览器引用使用统一 URL-safe 32 字符正则校验，避免把仅长度正确但内容不合法的引用继续发送到 BrowserProvider。
- 修复 open-event-v1 / open-tab-v1 JSON Schema 缺失 `browserRef` 的兼容问题，明确 `originDeviceId`、`shared`、`readOnly` 字段，并为设备 ID 定义格式限制。

## 现有约束与未完成

- 此轮只改进已经存在的 **跨设备只读浏览器观察**。**没有**实现跨设备输入、自动会话转移、工作区跨 Core 数据迁移或新 Chromium 运行环境。
- 浏览器 Agent 未启用、Agent 完成后关闭非 KeepSession 浏览器、观察引用过期、设备离线、跨设备授权撤销后，仍需要用户明确重新开启或修复 Runtime；不会伪造可用会话。
- 对于无设备身份的历史事件，维持兼容行为，绝不凭 URL 或由模型提供的设备 ID 推断授权。
- 本轮没有完整 Go 1.26.1 项目构建、Electron/Flutter Release、双设备 Chromium 联调；相应验收仍阻塞。

## 验证

- Go 1.23 parser/gofmt：变更 Go 文件通过。
- BrowserAgent 引用注册表隔离单测：`GO111MODULE=off` 的最小 GOPATH 镜像下启用 `-race` 通过；不代表完整应用 Go 构建通过。
- Web TypeScript 脚本解析通过，设备目标解析函数定向断言通过。
- JSON Schema Draft 2020-12：正反例验证通过。
- Flutter/Dart 新增目标解析单测；因 SDK 不可用，未执行。

## 下轮优先级

完成带错误码的远程观察授权失效反馈、浏览器会话心跳及跨设备重连验收；扩展当前独立 browser 工具调用的并发仲裁，再继续 Git Worktree/PTY/文件预览。所有跨设备控制写操作继续默认关闭，直至审批与受信设备双重验证完成。
