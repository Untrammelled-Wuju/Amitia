# U-Ai Workbench 第 61 轮：删除证据完整性和正式发布判定

## 已修改源码

- `gitworktree.Manager.removeLocked` 在失效孤儿检查点前，通过受保护 Git 执行环境检查工作树是否含有未提交或未跟踪的文件。脏工作树直接拒绝删除，保留完整检查点及工作树。检查点安全性校验与既有待决合并锁保持不变。
- `remove_dirty_checkpoint_test.go` 使用真实临时 Git 仓库验证未跟踪文件和已经暂存文件的删除拒绝；清理脏状态后再执行删除，确保检查点才被失效。
- `scripts/workbench/release-readiness.mjs` 提供正式发布证据判定器。读取现有 `integration-gate.mjs` 的 JSON 报告，要求全部关键服务、前端、桌面、移动与实际设备验收通过；缺少数据、未通过或 Go 版本不符时保持阻塞，不使用独立测试推断生产通过。
- `release-readiness.test.mjs` 通过八项正反例测试，覆盖缺失真实验收、重复证据、错误工具链、缺少 Go 测试事件和研发测试代替生产验收的误判场景。

## 本轮执行过的检查

- 隔离 Go 1.23.2 `gitworktree` 全包 `go test -race -json -count=1 -timeout=180s .`：119 个测试通过事件、1 个包通过事件、0 个失败事件。
- `GO111MODULE=off go vet .` 通过。
- Git Worktree 目标测试代码通过 Windows amd64、macOS amd64、Linux arm64、Android arm64 交叉编译，不代表目标系统运行成功。
- `node --test scripts/workbench/release-readiness.test.mjs`：8 项通过。
- 真实运行 `integration-gate.mjs --preflight-only` 后输入新发布判定器：13 项正式验收要求因缺失可验证证据而保持阻塞。该结果不是编译失败，也不意味着应用可发布。
- 累计替换包与完整源码包实施 CRC、安全路径、历史文件保留以及原始底包逐字节覆盖重建校验。

## 未完成的生产验收

- Go 1.26.1 后端完整构建与真实认证接口、Coding Agent 完整执行到用户审批和安全合并。
- Web/Electron 真实类型检查与启动、Flutter Android/iOS Release 构建与实际设备联调。
- Chromium/CDP、Windows ConPTY、Device Mesh 配对、离线恢复和多端实际联调。
- 完整数据库备份恢复、真实长时间性能和故障演练、发布签名与安装包验收。

研发范围估算与正式发布准备状态分开判断。本轮源码未进行生产发布，不输出最终 SHA-256。
