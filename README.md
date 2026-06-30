<p align="center">
  <img src="front/public/icons/icon-192.png" alt="Amitia" width="96" />
</p>

<h1 align="center">Amitia · 阿米提亚</h1>
<p align="center"><em>你的私人 AI 陪伴 · Your Private AI Companion</em></p>

---

[中文](#中文) | [English](#english)

---

<h2 id="中文">🇨🇳 中文</h2>

### 简介

**阿米提亚（Amitia）** 是一款运行在你本地的 AI 陪伴应用。所有数据存储在你自己的设备上，安全、私密、完全可控。

### 核心特性

- **智能对话** — 接入 OpenAI 兼容 API，支持 DeepSeek / Ollama 等多种模型
- **长期记忆** — 基于 Qdrant 向量数据库，AI 能记住你们之间的重要对话
- **多角色系统** — 创建和管理多个 AI 角色，每个角色有独立的性格和记忆
- **知识图谱** — SurrealDB 驱动的语义关联，让 AI 真正理解人物关系
- **桌面本地** — 数据完全本地化，无需云端，隐私无忧
- **微信/QQ 桥接** — 支持通过微信和 QQ 与 AI 对话
- **语音交互** — TTS 语音合成 + ASR 语音识别
- **主动关怀** — AI 可定时主动发起问候和提醒

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

**Amitia** is a local-first AI companion app. All data stays on your device — private, secure, and fully under your control.

### Features

- **Smart Chat** — OpenAI-compatible API, supports DeepSeek, Ollama, and more
- **Long-term Memory** — Qdrant-powered vector memory that remembers what matters
- **Multi-character** — Create and manage multiple AI personalities, each with independent memory
- **Knowledge Graph** — SurrealDB semantic relationships for deeper understanding
- **Local-first** — Zero cloud dependency, your data never leaves your device
- **WeChat/QQ Bridge** — Chat with your AI companion through WeChat or QQ
- **Voice I/O** — TTS synthesis + ASR speech recognition
- **Proactive Check-ins** — Scheduled greetings and reminders from your AI

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
