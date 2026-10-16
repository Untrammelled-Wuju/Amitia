# U-Ai Workbench 第 62 轮：Coding Worker 回滚锁重入修复

## 变更与范围

- `gitworktree.Manager.removeLocked` 统一负责取得仓库合并锁；新增内部 `removeWithMergeLock`，只由已经持有合并锁的调用点使用，避免重复获取文件锁。
- `gitworktree.Manager.EnsureTask` 在创建 Worktree 成功但任务绑定失败后，改用已持锁的内部清理路径。清理仍严格执行未绑定、无待决合并、Git 工作树干净和检查点安全失效验证。
- `ensure_rollback_lock_test.go` 以真实临时 Git 仓库验证：异常检查点存储导致绑定失败时不会陷入同锁等待；回滚失败明确报告并保留可恢复的孤儿 Worktree；元数据修复后可安全删除；持锁内部删除可以成功执行。

## 本轮实际验证

- 在 Go 1.23.2 的隔离源码环境中执行 `GO111MODULE=off go test -race -json -count=1 -timeout=180s .`：121 个 Go 测试通过事件、1 个包级通过事件、0 个失败事件。
- `GO111MODULE=off go vet .` 通过。
- Windows amd64、macOS amd64、Linux arm64 和 Android arm64 的独立 `gitworktree` 测试代码交叉编译通过；这不证明目标设备运行成功。
- `release-readiness.test.mjs` 的 8 项发布判定测试通过。
- 本轮实际执行 `integration-gate.mjs --preflight-only` 和正式发布判定器；13 项正式发布门禁仍为 blocked，未伪报上线资格。
- 累计替换包与累计完整源码包实施成员去重、CRC、目录安全、历史文件保留和原始底包逐文件重建检查。

## 尚未通过的正式验收

- 正式 Go 1.26.1 完整构建、真实认证 HTTP、Coding Agent 完整执行到用户审批及安全合并。
- Web/Electron 完整启动、Flutter Android/iOS Release、Chromium/CDP、Windows ConPTY、真实 Device Mesh 和长期稳定性测试。
- 数据库及外部服务的完整备份恢复演练、性能压力与发布签名安装。

本轮仅按已完成研发范围估算进度，正式发布仍保持阻塞。未修改历史构建产物，不执行 Git 推送，不输出最终 SHA-256。
