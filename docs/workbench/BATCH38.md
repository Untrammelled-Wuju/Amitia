# U-Ai Workbench — Batch 38

## 本轮范围

本轮以 Batch 37 累计完整源码为基线，聚焦工作台视图生命周期和任务终端运行期保留。仍执行已批准的控制面与资源执行面分离原则：标签关闭仅解除观察，终止进程或销毁 Chromium 会话必须由显式操作决定。新增检查不代替 Go 1.26.1 完整服务联调。

## 代码变更

- Web Dock 关闭标签、批量关闭未固定标签时仅修改 Store，不再直接结束本地浏览器会话或向终端发送终止事件；浏览器观察控制权仍由视图自身的 deactivation/unmount 生命周期处理。
- Web Terminal 显式“结束会话”按钮仍调用后端 DELETE；异步打开终端期间视图销毁，不再自动结束已创建的终端。
- Flutter Terminal 延迟完成的终端打开请求不再因页面已经销毁而结束后台进程；“结束”按钮仍为显式终止操作。
- Go Terminal Manager 保留仍在执行的任务专属终端，不因 25 分钟无客户端读取而关闭；已完成任务和普通空闲终端仍按空闲 TTL 回收，所有会话仍受到 4 小时绝对 TTL 限制。
- 新增终端 Manager 隔离 race 回归、跨平台目标交叉编译和 Web Dock 实际逻辑测试，并将它们加入定向验收门禁。

## 验收范围

- Go 1.23 本地隔离编译与 race：使用已有真实 Manager/Session 代码、虚拟进程，覆盖归属隔离、输入及关闭、异常尺寸、常规回收、任务运行保留、任务退出回收和绝对生命周期。
- Node + TypeScript：实际转译 Dock 脚本并执行关闭操作；浏览器、终端和 Flutter 显式终止契约进行静态源检查。
- Windows amd64、macOS amd64、Linux arm64：只对上述隔离 Manager/Session 标准库核心执行交叉编译，不能代表系统 PTY、ConPTY 真机验证。
- 继续执行已有 Git Worktree、终端缓冲区、短期 WebSocket 凭据、流重连、字节游标和认证脚本测试。

## 阻塞与后续

完整后端要求 Go 1.26.1，当前环境只有 Go 1.23.2 且无法连接到下载域名，也缺少完整项目依赖、Flutter SDK 和 Electron 安装依赖。没有后端进程启动、真实 API 认证、Android Release、Windows ConPTY 和 Device Mesh 端到端证据，85% 集成里程碑仍未通过。
