# Operit v1.12.0 完整能力审计报告

> B4 级 | 静态源代码审计 | 生成时间: 2026-08-07

## 摘要

| 指标 | 数值 |
|------|------|
| **应用程序** | Operit AI v1.12.0 |
| **包名** | com.ai.assistance.operit |
| **跟踪文件数** | 2,656 |
| **扫描文件数** | ~2,400 |
| **排除文件数** | ~256 |
| **原子能力总数** | **365** |
| **使用分类数** | 26 |
| **内置能力** | 365 |
| **代理可调用** | 235 |
| **用户可见** | 292 |

## 方法

静态源代码审计（无动态验证），证据最高等级 E3。
扫描12个域，覆盖 365 个原子能力。

## 分类能力分布

| 分类 | 数量 |
|------|------|
| 01_AGENT_AND_CHAT | 14 |
| 02_TASK_AND_PLANNING | 1 |
| 03_TOOL_RUNTIME | 24 |
| 04_ANDROID_DEVICE_CONTROL | 85 |
| 05_LINUX_AND_SHELL | 13 |
| 06_FILE_AND_NETWORK | 38 |
| 07_BROWSER_AND_SEARCH | 23 |
| 08_MEDIA_AND_VISION | 12 |
| 09_WORKSPACE_AND_REMOTE | 3 |
| 10_MCP | 3 |
| 11_SKILL | 3 |
| 12_TOOL_PACKAGE | 3 |
| 13_WORKFLOW_AUTOMATION | 4 |
| 14_HOOK_EVENT_SCHEDULE | 13 |
| 15_MODEL_PROVIDER | 19 |
| 16_LOCAL_MODEL | 2 |
| 17_VOICE | 24 |
| 18_MEMORY | 26 |
| 19_CHARACTER | 3 |
| 20_PERMISSION_AND_SECURITY | 19 |
| 21_UI_AND_OVERLAY | 9 |
| 22_IMPORT_EXPORT_BACKUP | 5 |
| 23_UPDATE_AND_RECOVERY | 3 |
| 24_RUNTIME_AND_INFRASTRUCTURE | 9 |
| 25_DEVELOPER_AND_DIAGNOSTICS | 4 |
| 26_OTHER | 3 |

## 核心子系统

1. **代理框架** - EnhancedAIService + 108工具 + XML/Native工具调用
2. **Shell执行** - 5级权限 + PRoot Linux沙盒 + PTY伪终端
3. **Android控制** - 6个Shell执行器 + MediaProjection + 蓝牙17工具
4. **MCP协议** - Bridge-Client代理 + TCP Socket桥接
5. **工作流** - DAG拓扑排序 + Kahn算法 + 6种节点
6. **模型** - 23+提供者 + MNN/llama.cpp本地推理
7. **浏览器** - WebView + 20+浏览器工具 + Playwright风格
8. **语音** - 4个STT + 8个TTS + VAD + 唤醒词
9. **记忆** - ObjectBox + HNSWLib向量检索 + 知识图谱
10. **角色** - SillyTavern兼容 + 工具访问控制

## 文件清单

本审计共生成 **38个文件**。

详细分类能力和证据见 `capability_catalog.json`。

---
*STATIC_SOURCE_SCAN | 365 capabilities | 26 categories*
