<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="Logo/amitia-character-dark-white-1024.png" />
    <source media="(prefers-color-scheme: light)" srcset="Logo/amitia-character-light-black-1024.png" />
    <img src="Logo/amitia-character-light-black-1024.png" alt="Amitia" width="128" />
  </picture>
</p>

<h1 align="center">Amitia · 阿米提亚</h1>
<p align="center"><em>本地优先的个人 Agent 操作系统 · Local-first Personal Agent OS</em></p>

---

[中文](#中文) | [English](#english)

---

<h2 id="中文">🇨🇳 中文</h2>

### 简介

**阿米提亚（Amitia）** 是本地优先、跨设备协同的个人 Agent 操作系统。每个 Agent 可以拥有独立的模型、人格、长期记忆、技能、应用授权、团队职责和隔离电脑；数据、凭据和运行记录默认由你控制。

### 核心特性

- **Agent Profile** — 草稿、测试、发布、版本差异、回滚、停用、迁移和可携带包
- **Agent 电脑** — 本机工作区、OCI 容器、远程电脑和 Android 自动化，支持终端、快照与人工接管
- **Agent 团队** — 负责人、成员路由、共享上下文、文件、决策记录和任务交接
- **可信执行** — 持久审批、副作用对账、能力令牌、网络白名单、数据外传审计和资源租约
- **长期记忆** — Qdrant 向量检索、SurrealDB 知识图谱和团队共享记忆
- **应用与插件** — MCP、Skills、Native Bridge、渠道插件和带分类的插件市场
- **数字身份插件** — 邮箱、日历、手机号和钱包均以独立插件包发布，不进入核心
- **模型与预算** — 每 Agent/团队模型路由、Token、费用、工具次数和并发治理
- **角色模型** — 每个角色可独立覆盖模型与思考参数，未配置时回退默认模型
- **语音与主动任务** — TTS、ASR、实时通话、提醒和长期持续事项
- **跨端协同** — Web、Electron、Android 和 iOS 使用同一 Core、市场、审批与活动主链

### 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | Vue 3 + TypeScript + Vite + Element Plus |
| 后端 | Go + Gin + GORM |
| 向量库 | Qdrant |
| 图数据库 | SurrealDB |
| 嵌入模型 | Doubao Embedding Vision |

### 许可证与商业授权

Amitia 社区版本依据 GNU Affero General Public License Version 3 only（AGPL-3.0-only）发布。你可以在遵守 AGPL-3.0-only 条款的前提下使用、研究、修改、部署和分发本项目。需要闭源修改、闭源集成、OEM、白标、闭源二次分发或其他不希望遵守 AGPL-3.0-only 义务的使用方式时，可以通过 3151508592@qq.com 联系申请单独商业许可证。

详见 [LICENSE](LICENSE) 和 [COMMERCIAL-LICENSING.md](COMMERCIAL-LICENSING.md)。

源码公开地址：Gitee - https://gitee.com/Untrammelled/Amitia；GitHub - https://github.com/Untrammelled-Wuju/Amitia

### 品牌与商标

Amitia、阿米提亚、官方 Logo、角色形象和其他品牌资产不随 AGPL-3.0-only 软件许可证自动授权。详见 [TRADEMARKS.md](TRADEMARKS.md)。

### 第三方组件

第三方组件、依赖、SDK、模型、字体、图片、图标、音频和其他素材仍适用其原始许可证、服务条款或权利声明。详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

### 贡献与安全

贡献前请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 和 [CLA.md](CLA.md)。安全漏洞请按照 [SECURITY.md](SECURITY.md) 报告，不要通过公开 Issue 披露敏感漏洞。

### 快速开始

本项目运行时可直接使用编译后运行目录中的程序，目录名可以是 `release`、`WorkDone` 或其它名字。

```bash
# 在编译后运行目录中执行
./surrealdb/surreal.exe start --user root --pass root rocksdb:data.db
./qdrant/qdrant.exe
./server.exe

# 前端 (端口 5178)
cd front && pnpm install && pnpm run dev
```

浏览器打开 `http://127.0.0.1:5178`，按引导完成配置即可使用。

### 项目结构

```
U-Ai/
├── front/          # Vue 3 前端
├── backend/        # Go 后端源码
├── 运行目录/       # 编译后运行目录，名称不限
│   ├── server.exe  # 后端可执行文件
│   ├── qdrant/     # 向量数据库
│   └── surrealdb/  # 图数据库
└── config/         # 配置文件
```

---

<h2 id="english">🇬🇧 English</h2>

### Overview

**Amitia** is a local-first, cross-device Personal Agent OS. Each agent can own its model, personality, long-term memory, skills, app grants, team role, and isolated computer while keeping data and credentials under your control.

### Features

- **Agent Profiles** — drafts, testing, releases, diffs, rollback, retirement, migration, and portable packages
- **Agent Computers** — local process, OCI container, remote desktop, and Android automation with snapshots and takeover
- **Agent Teams** — leads, routing, shared context, files, decisions, and task handoff
- **Trusted Execution** — durable approvals, side-effect reconciliation, capability leases, egress policy, and resource locks
- **Long-term Memory** — Qdrant retrieval, SurrealDB knowledge graph, and team context
- **Apps and Plugins** — MCP, Skills, Native Bridge, channel plugins, and a categorized plugin marketplace
- **Identity Plugins** — email, calendar, phone, browser profiles, and wallets ship as independent extensions
- **Models and Budgets** — per-agent routing plus token, cost, tool-call, and concurrency limits
- **Voice and Autonomy** — TTS, ASR, realtime calls, reminders, routines, and configurable autonomy levels
- **Cross-device** — Web, Electron, Android, and iOS share the same Core, marketplace, approvals, and activity APIs

### Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | Vue 3 + TypeScript + Vite + Element Plus |
| Backend | Go + Gin + GORM |
| Vector DB | Qdrant |
| Graph DB | SurrealDB |
| Embedding | Doubao Embedding Vision |

### Licensing and Commercial Licensing

The Amitia Community Edition is released under the GNU Affero General Public License Version 3 only (AGPL-3.0-only). You may use, study, modify, deploy, and distribute this project subject to compliance with AGPL-3.0-only. A separate commercial license is available for closed-source modifications, closed-source integrations, OEM, white labeling, closed-source redistribution, or other use cases where you do not wish to comply with AGPL-3.0-only obligations.

See [LICENSE](LICENSE) and [COMMERCIAL-LICENSING.md](COMMERCIAL-LICENSING.md).

Source code: Gitee - https://gitee.com/Untrammelled/Amitia; GitHub - https://github.com/Untrammelled-Wuju/Amitia

### Trademarks

Amitia, 阿米提亚, official logos, character assets, and other brand assets are not automatically licensed under the AGPL-3.0-only software license. See [TRADEMARKS.md](TRADEMARKS.md).

### Third-Party Components

Third-party components, dependencies, SDKs, models, fonts, images, icons, audio, and other assets remain subject to their original licenses, terms, or notices. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

### Contributing and Security

Please read [CONTRIBUTING.md](CONTRIBUTING.md) and [CLA.md](CLA.md) before contributing. Report security vulnerabilities according to [SECURITY.md](SECURITY.md) and do not disclose sensitive vulnerabilities through public issues.

### Quick Start

```bash
./surrealdb/surreal.exe start --user root --pass root rocksdb:data.db
./qdrant/qdrant.exe
./server.exe
cd front && pnpm install && pnpm run dev
```

Open `http://127.0.0.1:5178` and follow the setup wizard.

### Project Structure

```
U-Ai/
├── front/          # Vue 3 frontend
├── backend/        # Go backend source
├── runtime/        # Compiled runtime, name agnostic
│   ├── server.exe  # Backend binary
│   ├── qdrant/     # Vector database
│   └── surrealdb/  # Graph database
└── config/         # Configuration files
```

---

<p align="center">
  <sub>Made with ❤️ · 数据完全属于你 · Your Data, Your Rules</sub>
</p>
