# Workbench 第 25 轮：终端任务凭据跨进程一致性与诊断摘要

## 交付范围

- 在 `workbenchterminal` 内独立实现 Linux、macOS 和 Windows 的跨进程文件锁适配；更新、列表查询与保留期清理操作共享同一串行化保护。锁超时为 4 秒，失败时明确返回存储不可用而不是覆盖数据。
- 终端记录更新采用递增生命周期序列保护，旧事件不会覆盖较新的记录；已结束、已撤销或已关闭的状态保持终态。锁定后使用临时文件、内容 fsync、原子重命名；Linux/macOS 对目录执行 fsync。
- 新增只涉及当前认证主体、当前对话和任务的 `TaskReceiptSummary`：总数、运行中、已退出、已中断、撤销、关闭、最新状态及最后退出码。摘要与原有明细来自同一次授权读取。
- Web/Electron 与 Flutter 均校验摘要和实际记录的一致性并展示状态。保留 `terminal-lifecycle-metadata-not-test-proof` 证据类别，不将退出码为零转换成任务或测试成功。
- 新增独立进程写入、跨实例乱序、锁超时、摘要分类测试，不把静态编译等同于多设备实机验收。

## 架构与安全边界

`task_receipt_store.go` 负责持久化、最小必要元数据与列表；`task_receipt_lock_{linux,darwin,windows}.go` 负责 OS 文件锁；`task_receipt_sync_{linux,darwin,windows}.go` 负责提交后的 OS 持久化处理；`task_receipt_summary.go` 负责纯数据摘要；`task_receipt_handler.go` 继续执行本机可信主体鉴权及绑定校验。数据不包含用户输入、原始终端文本或测试日志。

跨进程锁只覆盖同一共享文件系统中使用同一约定的 Runtime；不防止其他无合作进程任意修改文件。Windows 的重命名与文件锁仅完成交叉编译，未完成 Windows 真机压力测试。运行凭据以认证 Session 区分归属，尚未提供跨登录会话恢复权限。终端记录不构成 Task Runtime 状态凭证，父 Agent 的可信身份自动关联仍待后续完成。

## 本轮验收

- 隔离 Go 终端模块：`go test -race -count=1 ./...` 通过，包含真实子进程并发写入。
- Windows amd64、macOS arm64：隔离终端模块交叉编译通过，未实机执行。
- Web TypeScript/Vue 脚本解析及客户端接线检查通过。
- JSON Schema 有效样本接受、伪造诊断结果拒绝。
- 累计替换包以最初原工程为基线构造，并逐文件对比最终完整源码。
- 未执行完整 Go 1.26.1 构建、Flutter analyze/Release、Electron/Android 真机集成；当前环境缺少相应 SDK 与完整依赖。
