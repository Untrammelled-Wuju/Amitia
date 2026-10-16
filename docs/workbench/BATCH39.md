# Workbench 第 39 轮：终端启动并发安全与构建前置验证

## 本轮代码

本轮在现有 Workbench Terminal Manager 上修复进程启动与 Manager 全局锁强耦合的问题：启动过程在锁外执行，同时先在锁内预留全局和用户配额；启动结束后释放预留名额，并对 Manager 是否已经关闭执行二次检查。这样，缓慢的终端启动不阻止其他已运行终端的读取与操作，未完成的并发启动也不能绕过会话配额。

Manager 正在关闭时不接受新启动。关闭过程中才成功返回的进程必须关闭；启动器部分成功又返回错误时，也必须关闭该进程。运行中进程的主动停止与关闭标签分离规则保持不变。

新增独立的错误类别 `ErrClosed` 与 `ErrUnavailable`，HTTP 层分别返回服务正在关闭和进程启动器不可用，不再混用测试证据或凭据存储失败的信息。项目现有的身份、任务绑定和终端配额规则仍由原有 Handler 和 Manager 负责。

## 定向验收范围

- Go race 测试验证终端启动阻塞时其他读取仍可进行。
- Go race 测试验证同一用户最多六个并发待启动终端，全局最多四十八个待启动终端。
- Go race 测试验证关闭期间启动成功的进程不会变为孤儿进程，失败和部分失败均释放配额与资源。
- Linux 环境使用真实 `/bin/sh` 子进程走完 Manager 启动、stdin 输入、stdout 输出与退出码 7 的验证，不把模拟进程测试结果当作真实执行证据；该检查仍不包含 PTY、HTTP 认证和完整服务。
- 定向跨平台交叉编译仅用于检测本次涉及的标准库 Manager 代码可以在 Windows、macOS、Linux arm64 和 Android arm64 下编译，不等于真实 PTY、ConPTY、移动端或完整后端可运行。
- `scripts/workbench/go-source-syntax-gate.go` 用标准库解析 Workbench 相关 Go 文件；只能发现语法错误，不能替代类型检查、依赖链接和完整服务构建。

## 未完成的生产验收

当前环境使用 Go 1.23.2，项目声明 Go 1.26.1，无法获取完整外部模块依赖；因此本轮不宣称后端整体编译成功、HTTP 认证及 Coding Agent 运行链路通过。Web/Electron、Flutter Release、Windows ConPTY、真实 Browser/CDP 与 Device Mesh 也仍需完整环境运行验证。下一阶段应直接使用满足版本的 Go 工具链完成 `cmd/server` 编译，再按真实本机认证请求运行已经提供的 API smoke，并继续 Coding Worker 的服务级审批合并联调。
