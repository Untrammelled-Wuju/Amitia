# U-Ai Workbench 第 63 轮：取消请求后的 Worktree 安全回滚

## 研发范围与实现

- `gitworktree.Manager.EnsureTask` 在创建工作树后、持久化任务绑定前检测调用方取消；不向调用方报告一个已经取消的分配为成功。
- 任务绑定失败或调用方取消时，回滚使用保留调用上下文值但不继承取消信号的独立、最多五秒清理上下文。回滚继续沿用持有合并锁时的内部删除路径，不重复加锁。
- 所有原有安全门禁继续有效：仅删除受管理、未绑定、干净、没有未决合并、检查点可安全清理的工作树。若已绑定或清理失败，保留实际资源，并同时报告原始失败和回滚失败。
- 新增真实临时 Git 仓库测试：普通取消、截止时间已过、绑定资源不得误删，并回归既有绑定失败回滚锁测试。

## 已实际执行的验证

- Go 1.23.2，隔离模块：`GO111MODULE=off go test -race -json -count=1 -timeout=180s .`，共 125 个测试通过事件、1 个包级通过事件、0 个失败事件。
- `GO111MODULE=off go vet .` 通过。
- `windows/amd64`、`darwin/amd64`、`linux/arm64`、`android/arm64` 的独立 Git Worktree 测试二进制交叉编译通过。
- 发布判定器 8 个正反例测试通过；实际前置报告保持 blocked，13 项生产验收证据未齐备。
- 累计 ZIP 在本轮交付前执行 CRC、路径安全、历史文件保留及与原始基线的逐文件叠加重建。

## 尚未完成的生产验收

完整 Go 1.26.1 服务编译和认证调用、真实 Coding Agent 调度及批准合并、SQLite/SurrealDB/Qdrant 灾备、Electron/Flutter 正式构建、Chromium/CDP、Windows ConPTY、Device Mesh 真机，以及长时间并发压力与发布签名均未经过完整真实运行验证。研发范围估算与生产验收状态分开记录；本轮不输出 SHA-256。
