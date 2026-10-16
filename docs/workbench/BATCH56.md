# Workbench 第 56 轮：Terminal 权限撤销与只读数据围栏

## 范围与设计

基于第 55 轮累计完整源码继续研发。本轮只收紧 `workbenchterminal` 已有的终端会话生命周期，不新增另一套终端服务，也不更改 REST/WebSocket 路由、身份字段或数据库结构。

关闭、主动撤销、空闲回收或 Manager 关闭后，即使其他 goroutine 已经取得旧会话对象，也不能继续读取其输出、导出终端转录或读取生命周期记录。服务端关闭会话时，必须先标记旧会话不可用，再从 Manager 删除。已建立的流式观察者会被唤醒，得到不含输出内容的终止状态。

## 本轮实际代码变更

- `backend/internal/workbenchterminal/manager.go`：关闭、回收及 Manager 退出时，在对象从索引中消失之前标记撤销；`Read` 返回会话自身的加锁校验结果。
- `backend/internal/workbenchterminal/session.go`：把 Snapshot 的输出读取与撤销状态检查放在同一把锁内；撤销时清零保留的输出字节；取消后到达的新进程输出不再进入缓存。
- `backend/internal/workbenchterminal/stream_notify.go`：流观察使用同一受锁保护的 Snapshot 语义，撤销后只返回无数据的终止状态。
- `backend/internal/workbenchterminal/export.go`：从旧会话指针导出数据时，若已撤销则返回不存在。
- `backend/internal/workbenchterminal/lifecycle.go`：生命周期查询在锁内核验撤销状态，避免旧对象继续泄露元数据。
- `backend/internal/workbenchterminal/session_read_revocation_test.go`：新增撤销、关闭、过期回收、服务停止、并发读取的回归测试，以及输出缓存清零检查。
- `scripts/workbench/integration-gate.mjs`：更新 Terminal Manager 隔离测试文件列表，确保独立门禁覆盖导出及生命周期查询。

## 已执行的定向验证

- Linux：使用真实源码组合执行 `GO111MODULE=off go test -race -count=1`，包含既有 Manager 测试和新增读写围栏测试，通过。
- Go 标准库语法检查：Workbench 相关 126 个 Go 文件通过解析；这不是完整 Go 服务编译。
- `integration-gate.mjs` Node 语法检查通过。
- Windows amd64、macOS amd64、Linux arm64、Android arm64：上述隔离测试集合交叉编译通过，未在这些目标设备上实际运行。

## 尚未完成的生产级验收

完整 Go 1.26.1 后端构建启动、真实 Gin 路由和身份认证、Windows ConPTY、Chromium/CDP、Device Mesh 真机、Electron/Flutter Release、压力测试、长任务连续恢复、数据库迁移及正式发布验收均未据此判为通过。研发进度仅代表估算的源码实现范围，不等于正式环境验收率。
