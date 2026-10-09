# AI 能力发现与安装

聊天中的 `find_capability` 和 `acquire_capability` 共用正式安装服务。发现、安装、配置、连接验证是独立阶段；只有验证成功的能力才能报告可用。

## 发现来源

- 已安装的 Skill、MCP 和扩展进入本地发现。
- Skill 搜索 GitHub 仓库及 `openai/skills`，下载使用固定提交的完整技能目录，保留 `SKILL.md`、引用和脚本资源。
- MCP 搜索官方 Registry：`https://registry.modelcontextprotocol.io/v0.1/servers`。支持固定版本的 npm、PyPI 包及远程 HTTP/SSE 服务。
- 插件搜索 GitHub 上带原生 Amitia manifest 的 `.amitiax` 发布包，其他产品的插件需要先适配 Amitia。
- 本地 Skill 默认读取运行时根目录和扩展根目录下的 `skills`；本地插件包读取运行时根目录及其父目录下的 `plugin`。
- `AMITIA_SKILL_DIRS`、`AMITIA_PACKAGE_DIRS` 可追加目录，使用平台路径列表分隔符。
- 私有目录可通过 `AMITIA_SKILL_CATALOG_URL`、`AMITIA_EXTENSION_CATALOG_URL`、`AMITIA_MCP_CATALOG_URL` 配置。没有配置时不请求不存在的自有目录接口。

## 聊天调用

先用 `capabilityId`、`query`、`preferredKind` 搜索，检查候选来源、安装描述和来源错误，再用相同 `capabilityId` 和返回的 `candidateId` 安装。候选缓存按空间和能力隔离，有效期十分钟。

用户给出来源时可传 `sourceUri`：Skill 支持本地目录、`SKILL.md`、ZIP、GitHub 目录或文件地址。MCP 必须提供真实的 `install` 描述；插件还需真实 `extensionId` 和包版本。不得猜测接口地址、凭据或包身份。

## 状态与恢复

Skill 安装进入 SQLite 和正式资源存储，并进入 Agent Skill 目录；沿用项目现有全局作用域规则。MCP 配置进入正式服务器存储，环境配置进入加密凭据存储，使用项目运行时解析器连接后执行工具发现。缺少运行时、凭据、目录参数或授权时反馈实际错误。

原生插件通过正式预览、确认、安装事务和安装记录验证。安装成功但尚未配置或启用运行时的包返回 `installed_only`，同时返回 `installed=true`、`enabled=false` 和后续配置说明，不能当成可执行工具使用。

安装失败清理本次新增的 MCP 配置、凭据和连接，不删除已有同身份服务器。审批恢复保留原始候选来源，校验空间；含环境配置的审批只保存在内存，重启后要求重新提供配置，避免凭据进入明文审批记录。

## 参考实现

- [Codex Skill Installer](https://github.com/openai/skills/blob/main/skills/.system/skill-installer/scripts/install-skill-from-github.py)：目录导入和资源完整性。
- [DeepSeek Harness MCP](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/README.md)：配置驱动的连接与工具发现。
- [OpenClaw ClawHub](https://github.com/openclaw/openclaw/blob/main/skills/clawhub/SKILL.md)：搜索、安装、更新使用独立流程。
- [Hermes MCP](https://github.com/hermes-agent-org/hermes/blob/main/website/docs/guides/use-mcp-with-hermes.md)：保存实际配置并在连接后发现工具。
- [MCP Registry 格式](https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/server-json/generic-server-json.md)：包版本、传输方式和必需参数的标准元数据。
