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

### 快速开始

```bash
# 1. 启动依赖服务
# SurrealDB (端口 8000)
./WorkDone/surrealdb/surreal.exe start --user root --pass root rocksdb:data.db

# Qdrant (端口 9178)
./WorkDone/qdrant/qdrant.exe

# 2. 启动后端 (端口 8899)
./WorkDone/server.exe

# 3. 启动前端 (端口 5178)
cd front && pnpm install && pnpm run dev
```

浏览器打开 `http://127.0.0.1:5178`，按引导完成配置即可使用。

### 项目结构

```
U-Ai/
├── front/          # Vue 3 前端
├── backend/        # Go 后端源码
├── WorkDone/       # 编译后运行目录
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

### Quick Start

```bash
# 1. Start dependencies
# SurrealDB (port 8000)
./WorkDone/surrealdb/surreal.exe start --user root --pass root rocksdb:data.db

# Qdrant (port 9178)
./WorkDone/qdrant/qdrant.exe

# 2. Start backend (port 8899)
./WorkDone/server.exe

# 3. Start frontend (port 5178)
cd front && pnpm install && pnpm run dev
```

Open `http://127.0.0.1:5178` and follow the setup wizard.

### Project Structure

```
U-Ai/
├── front/          # Vue 3 frontend
├── backend/        # Go backend source
├── WorkDone/       # Compiled runtime
│   ├── server.exe  # Backend binary
│   ├── qdrant/     # Vector database
│   └── surrealdb/  # Graph database
└── config/         # Configuration files
```

---

<p align="center">
  <sub>Made with ❤️ · 数据完全属于你 · Your Data, Your Rules</sub>
</p>
