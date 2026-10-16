# Workbench 第 10 轮：受控 Git Worktree 生命周期

## 开发基线与范围

- 基线：`U-Ai_Workbench_Batch09_累计完整源码包.zip`。
- 目标：在原有 GitController + Workspace Service 基础上，加入 Git Worktree 列表、独立创建、Workspace 自动挂载与非强制删除。
- 范围：桌面 Web/Electron 的 Git 工作台与 Flutter 工具箱 Git 页面均可调用同一个 Go API。未实现远程跨设备 Worktree 操作，也未实现 Agent 根据任务自动创建 Worktree。
- 额外约束：只创建在应用私有 `dataRoot/workspaces/agent-worktrees/` 下；不接受客户端自定义目标路径。

## API

位于既有受认证 `/api/workspaces/git` 路由下，所有接口 `POST`：

1. `/worktrees`：`{ "workspaceUri": "amitia://workspace/@..." }`，返回 `{ "entries": [GitWorktree] }`；非托管条目也列出，但不能通过该 API 删除。
2. `/worktrees/create`：`{ "workspaceUri": "...", "name": "...", "branch": "feature/...", "ref": "HEAD" }`。`branch` 省略时创建 detached HEAD。成功返回 GitWorktree，包含自动注册的 `workspaceUri`。
3. `/worktrees/remove`：`{ "workspaceUri": "...", "id": "32 字符随机标识" }`。仅清理私有受管路径，拒绝锁定或未提交变更的 Worktree。不提供 `force`。

目标目录由受认证的 Workspace 挂载解析而来。创建后调用现有 `WorkspaceService.RegisterLocalMount`，复用文件浏览、编辑、Agent 工具的工作区授权模型。客户端不得以 Git 返回的原生路径作为额外授权凭证。

## 失效与安全策略

- Git 不存在、仓库不可用、仓库根不匹配时拒绝创建。
- 限制每个 Git 共用仓库下由 U-Ai 托管的 Worktree 最多 16 个。
- 检查 commit-ish 格式和可解析的提交，分支名称经 Git 规范检查；禁止参数选项注入。
- 通过 `git worktree add/remove` 调用，不运行远程 clone、不强制删除、不自动执行构建脚本或仓库 Hook。
- 忽略系统和用户级 Git 配置，屏蔽 hooks 和 fsmonitor，并拒绝仓库中配置的外部 clean/smudge/process filter。
- 对跟踪的子模块和符号链接采取保守拒绝策略；防止受管父目录被符号链接重定向。
- 服务端拒绝只读挂载的创建及删除操作。
- 已注册挂载的数据不会因为关闭 Workbench 标签而删除；删除操作需明确确认。
- Worktree 删除不会自动删除 Git 分支，避免意外丢失提交引用。失败时应保留原目录和未提交修改。

## 已执行验证

- 真实 Git 仓库本地集成测试（Go 1.23 隔离纯标准库包）：创建、列出、指定分支、不同工作树访问、未提交修改拒绝、安全 ID、数量限额、取消上下文、外部 filter 和符号链接攻击；`go test -race -count=1` 通过。
- 5 个改动 Go 文件：`go/parser` 和 `gofmt` 通过。
- Web Workbench TypeScript 脚本语法与前后端路由对照共 16 项静态断言通过。
- Flutter 新增服务接口及工具箱调用位置已做静态核查，未运行 `flutter analyze`。

## 尚未验收

- 完整 Go 1.26.1 工程编译、路由级 HTTP 集成测试、SQLite 实际持久化和 Electron 运行。
- Flutter SDK 未安装，Android/iOS Release 与真机未测。
- 大型仓库磁盘配额、磁盘空间清理、Agent 生命周期归属、跨设备 Worktree 授权及任务结束自动回收。
- Git 外部属性/工作区配置的各平台全覆盖安全审计，仍需进一步压力和安全测试后才能生产发布。
