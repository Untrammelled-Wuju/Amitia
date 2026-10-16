# U-Ai Workbench 第 60 轮：工作树删除与检查点清理顺序收口

## 本轮修复

- `gitworktree.Manager.removeLocked` 将检查点失效步骤前置于 Git 工作树删除，避免删除工作树成功后因检查点元数据损坏、符号链接或进程意外中断而遗留孤儿审批数据。
- 保持既有跨进程分配锁、合并锁、检查点运行锁及待决合并审计保护；不改变强制删除策略，也不允许删除已有任务绑定、脏目录或仍处于未决合并的工作树。
- 增加两项真实 Git 仓库回归测试：损坏检查点阻断 Git 删除且不能触及外部文件；正常孤儿检查点在工作树删除前安全失效。

## 已执行验证

- 使用 Go 1.23.2 在隔离 `gitworktree` 包执行 `go test -race -json -count=1 -timeout=180s`：116 个测试通过事件，1 个包级通过事件，0 个失败事件。
- `GO111MODULE=off go vet .` 通过。
- 相关测试二进制通过 Windows amd64、macOS amd64、Linux arm64、Android arm64 交叉编译；交叉编译不代表目标平台实际运行。
- 累计完整源码和累计完整替换文件包进行 CRC、路径安全、文件集合及字节一致性校验。

## 尚未完成的生产验收

- Go 1.26.1 完整项目构建、真实身份认证 HTTP、完整 Agent 调度与工作树合并链路。
- 真实数据库灾备/迁移恢复、断电故障、并发长时间压力及资源泄漏验收。
- Windows ConPTY、Electron 正式运行、Android/iOS Flutter Release、Chromium CDP 与 Device Mesh 真机联调。

本轮研发范围进度仍为估算值，不等于可发布或生产验收通过率。未执行的验证不可计为通过。
