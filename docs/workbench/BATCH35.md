# U-Ai Workbench Batch 35：合并后崩溃恢复与验证门禁

本轮从 Batch34 累计源码继续开发。生产级阶段没有完成，不把隔离集成测试等同于完整项目构建或真机验收。

## 修复

1. Worktree 合并审计在 `pending` 或 `failed-needs-review` 时，读取流程在原仓库级排他锁内做只读 Git 状态核对。仅在批准记录、来源提交、目标分支、目标提交、审核指纹、祖先关系、目标干净状态均吻合时，将审计持久恢复为 `applied`，写入 `recoveredAt`。核对失败保持原状态，不执行 Git merge，不跳过审批。
2. RunCheckpointTest 的 JSON 测试证据要求真实测试用例 `pass`、包级 `pass`、没有 `fail` 事件、进程零退出码、没有超时或截断。不能用单条测试成功或普通输出冒充最终验收通过。
3. Linux/macOS/Windows 仓库级锁对上下文截止时间使用一致的错误分类，防止误将请求超时当作锁永久不可用。
4. 统一集成门禁的 Go Worktree 隔离测试从选中用例升级为本模块全包定向 race 测试，并检查结构化测试结果。

## 运行范围与限制

定向验证在 Linux Go 1.23.2 下，隔离运行完整 gitworktree 包的真实临时 Git 仓库测试（不依赖整个主工程的 Go Module）。跨平台仅能交叉编译确认接口存在，不能作为 Windows/macOS 设备执行的依据。Go 1.26.1 主工程编译、真实 HTTP 认证、Flutter Release、Electron、ConPTY 和 Device Mesh 真机仍须在匹配环境实测。所有生成脚本、模块修改与测试都存于累计源码包，未触碰编译产物、发布资源、远程代码仓库或真实业务数据库。
