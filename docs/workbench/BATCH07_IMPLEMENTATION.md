# Workbench 第七轮：Device Mesh 跨设备只读浏览器观察

## 已实现

本轮引入显式设备目标的只读浏览器读取闭环：客户端向 Cloud Core 的 `POST /api/device-mesh/v1/workbench/browser/read` 提交目标设备 ID、对话 ID、Agent BrowserRef 和读取类型。Core 从当前已认证的 Actor 获取 Space 与源设备身份，强制要求源设备具备浏览器控制相关权限、来源与目标均为该 Space 内已信任设备、目标处于在线状态。Core 使用现有 Device Mesh Runtime Invocation 机制将请求转发至目标 Runtime，而不是构造未经校验的代理 HTTP 请求。

目标 Device Agent 对 RPC 的 Space、DeviceID、RuntimeID、有效配对凭据进行再次核验；只接受 `state`、`dom`、`screenshot` 三种读取类型。会话解析需要全局 Agent 浏览器短期随机引用、相同 Space 与对话，并在读取结束时再次核验是否撤销。截屏使用 JPEG，限定原始大小为 2 MiB；DOM 截断在 1 MiB UTF-8 字符边界。任何远程导航、输入、控制租约或关闭操作均未开放。

Web/Electron 与 Flutter 工作区浏览器标签识别 `targetDeviceId` 与 `browserRef`，分别通过 Core 的上述入口展示状态、DOM、截图。远程标签不得回退请求本地 Chromium；禁用地址栏操作、键鼠触控、控制权获取、本地 iframe 预览。关闭远程标签仅退出观察视图，不关闭 Agent 原有会话。设备掉线、凭据失效、关闭标签后给出真实错误并允许前台恢复重试。

## 安全不变量

1. 身份由 Cloud Core 认证中间件提供，客户端不能通过请求体伪造 Space 或来源设备。
2. 浏览器引用只能由实际持有 Chromium 的 Device Agent 在对应 Space 和 Conversation 内解析。
3. RPC 操作按明确白名单处理；新增操作默认为拒绝。
4. Device Mesh 断线或目标设备未受信任时拒绝读取，不尝试访问当前设备文件或浏览器。
5. 服务端在读取后再次核验观察许可，避免过期或撤销引用留下有效快照。
6. 不能将静态代码检查等同于实际远程设备运行验收。

## 尚未完成

跨设备浏览器写操作必须另行实现正式的授权票据、源目标双侧控制权租约、操作审计与可中止机制，目前禁止。Agent 事件中自动填写真实 `targetDeviceId` 的来源链路尚未打通；目前仅对协议明确包含目标设备和已知观察引用的标签生效，不会自动猜测设备。远端 Device Agent 必须已经有正在执行的 Agent Chromium 会话并发布有效引用，本轮并不支持凭空启动另一台机器上的新浏览器。Go 1.26.1、Flutter SDK、前端安装依赖及两台真实配对设备的集成环境缺失，正式编译和真实 Device Mesh 双端联调仍待验证。

## 下一轮

完善 Agent 会话事件的来源设备显式标识及可信转发；补充基于主体和目标设备授权的控制租约、全局浏览器工具互斥、远程观察令牌生命周期、设备重连后索引恢复，并进行双端真实运行测试。

## 数据留存约束

Browser Read RPC 属于短时只读观察，不得写入 Device Mesh 的持久化 ExecutionJournal；亦不应进入 Runtime Dispatcher 的 24 小时成功结果缓存。第七轮对上述两个路径均设置精确的 Handler 白名单例外，并保留现有设备凭据 Guard 的二次校验。没有此约束时，历史截图可能超过 1 MiB 持久化执行记录上限，且撤销观察引用后仍有风险读到旧缓存。
