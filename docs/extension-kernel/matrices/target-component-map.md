# 目标组件映射表

> 所有保留并抽取和改造后复用对象必须映射到 Extension Kernel 的目标组件

---

## Extension Kernel 目标组件清单

### Package Manager
| 来源对象 | ID | 来源分类 |
|---|---|---|
| 归档安全（ZIP 读写、路径校验） | PKG-001 | 保留并抽取 |
| Checksum | PKG-002 | 保留并抽取 |
| 签名验证 | PKG-003 | 保留并抽取 |
| 版本比较 | PKG-004, EXT-RT-010 | 保留并抽取 |
| 安装事务/补偿/回滚 | PKG-005, PKG-006 | 保留并抽取 |
| 恢复 | PKG-007 | 保留并抽取 |
| 文件类型限制 | PKG-009 | 保留并抽取 |
| PackageService | PKG-101 | 改造后复用 |
| Workshop Installer（统一安装路径） | WS-106 | 改造后复用 |

### Package Store
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Artifact Store | PKG-102 | 改造后复用 |

### Manifest Parser
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Schema Validator（JSON Schema） | EXT-RT-008 | 保留并抽取 |

### Dependency Resolver
| 来源对象 | ID | 来源分类 |
|---|---|---|
| 依赖解析 | PKG-103 | 改造后复用 |
| 卸载预览（依赖分析） | PKG-008 | 保留并抽取 |
| 循环检测 | WFL-006 | 保留并抽取 |
| MCP Dependency Service | MCP-008 | 改造后复用 |
| MCP 依赖声明解析 | AGT-008 | 改造后复用 |

### Runtime Supervisor
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Executor 超时控制 | EXT-RT-001 | 保留并抽取 |
| Executor Panic 恢复 | EXT-RT-002 | 保留并抽取 |
| Executor 幂等性 | EXT-RT-003 | 保留并抽取 |
| Executor 并发控制 | EXT-RT-004 | 保留并抽取 |
| Plugin Hook 超时 | PLG-001 | 保留并抽取 |
| 熔断器 | PLG-002 | 保留并抽取 |
| Plugin 并发控制 | PLG-003 | 保留并抽取 |
| Runtime Close（统一关闭） | EXT-RT-104 | 改造后复用 |
| LifecycleService | EXT-RT-109 | 改造后复用 |
| AgentSkill Round State | AGT-103 | 改造后复用 |
| 错误定位 | WFL-008 | 保留并抽取 |

### Contribution Registry
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Registry 核心结构 | EXT-RT-101 | 改造后复用 |
| Registry 作用域过滤 | EXT-RT-102 | 改造后复用 |
| Capability 定义 | EXT-RT-011 | 改造后复用 |

### Tool Registry
| 来源对象 | ID | 来源分类 |
|---|---|---|
| （新组件，来源于旧 Tool 迁移） | — | 改造后复用 |

### Agent Skill Catalog
| 来源对象 | ID | 来源分类 |
|---|---|---|
| SKILL.md / Frontmatter 解析 | AGT-001 | 保留并抽取 |
| 资源扫描与索引 | AGT-004 | 保留并抽取 |
| 资源渐进读取 | AGT-005 | 保留并抽取 |
| Token 估算 | AGT-006 | 保留并抽取 |
| OpenAI 兼容解析 | AGT-007 | 保留并抽取 |
| AgentSkillService 目录与激活 | AGT-101 | 改造后复用 |
| AgentSkillService 缓存 | AGT-102 | 改造后复用 |
| AgentSkillService Prompt 渲染 | AGT-104 | 改造后复用 |
| AgentSkillParser 兼容性分析 | AGT-105 | 改造后复用 |

### MCP Manager
| 来源对象 | ID | 来源分类 |
|---|---|---|
| MCP Client | MCP-001 | 保留并抽取 |
| MCP Transport | MCP-002 | 保留并抽取 |
| MCP OAuth | MCP-003 | 保留并抽取 |
| MCP Protocol | MCP-004 | 保留并抽取 |
| MCP Discovery | MCP-005 | 保留并抽取 |
| MCP Features | MCP-006 | 保留并抽取 |
| MCP Host | MCP-007 | 保留并抽取 |
| MCP Connection Manager | MCP-101 | 改造后复用 |
| MCP Models | MCP-103 | 改造后复用 |

### Workflow Engine
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Workflow Schema | WFL-001 | 保留并抽取 |
| Workflow Compiler | WFL-002 | 改造后复用 |
| Workflow Executor | WFL-003 | 改造后复用 |
| Value Resolver | WFL-004 | 保留并抽取 |
| 条件表达式求值 | WFL-005 | 保留并抽取 |
| JSON 变换引擎 | WFL-007 | 保留并抽取 |
| WorkflowHostAdapter | WFL-101 | 改造后复用 |
| HTTPWorkflowAdapter | WFL-102 | 改造后复用 |
| SkillWorkflowAdapter → ToolAdapter | WFL-103 | 改造后复用 |
| BuildWorkflowAdapters | WFL-104 | 改造后复用 |

### UI Contribution Registry
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Plugin Surface Schema | PLG-104 | 改造后复用 |
| Schema Surface Renderer | FE-001 | 保留并抽取 |
| SurfaceAction / SurfaceForm / SurfaceStatus / SurfaceTable | FE-002~005 | 保留并抽取 |
| ExtensionPageHeader | FE-007 | 保留并抽取 |

### Hook Pipeline
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Plugin Event Delivery | PLG-102 | 改造后复用 |

### Permission Broker
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Permission Evaluator | EXT-RT-105 | 改造后复用 |
| Plugin Host API 权限校验 | PLG-105 | 改造后复用 |
| PermissionDialog | FE-006 | 保留并抽取 |

### Scope Manager
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Plugin 命名空间隔离 | PLG-006 | 保留并抽取 |
| Registry 作用域过滤 | EXT-RT-102 | 改造后复用 |

### Storage Broker
| 来源对象 | ID | 来源分类 |
|---|---|---|
| OwnedResource Repository | EXT-RT-012 | 改造后复用 |
| Plugin State | PLG-101 | 改造后复用 |
| 插件配置 | PLG-106 | 改造后复用 |
| 副作用补偿 | EXT-RT-006 | 保留并抽取 |

### Secret Broker
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Config Crypto | EXT-RT-007 | 保留并抽取 |
| 敏感数据脱敏工具 | EXT-RT-009 | 保留并抽取 |

### Event Bus
| 来源对象 | ID | 来源分类 |
|---|---|---|
| 事件深度限制 | PLG-004 | 保留并抽取 |
| Plugin Event Delivery | PLG-102 | 改造后复用 |

### Schedule Manager
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Plugin Schedule | PLG-103 | 改造后复用 |

### Audit Store
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Executor 审计持久化 | EXT-RT-005 | 保留并抽取 |
| Plugin 执行审计 | PLG-007 | 保留并抽取 |
| Operation Audit | PKG-104 | 改造后复用 |

### Migration Manager
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Config Migration | PKG-105 | 改造后复用 |

### Developer Tooling
| 来源对象 | ID | 来源分类 |
|---|---|---|
| Workshop Session 管理 | WS-101 | 改造后复用 |
| Workshop Revision 管理 | WS-102 | 改造后复用 |
| Workshop Generator | WS-103 | 改造后复用 |
| Workshop Validation | WS-104 | 改造后复用 |
| Workshop Test Runner | WS-105 | 改造后复用 |
| Workshop Fork | WS-107 | 改造后复用 |
| Workshop UI 组件 | FE-106, 110-111 | 改造后复用 |

---

## 未映射对象说明

以下对象在当前目标组件中没有明确归属：

| 对象 | 原因 |
|---|---|
| Runtime 装配 `NewRuntime` | 属于内核级装配代码，不是独立组件 |
| Runtime `ModelTools` | 临时适配，最终删除 |
| Runtime `pluginSnapshot` | 临时适配，最终删除 |
| Service 门面 | 改造为 Extension Kernel API 层 |
| Handler / Router | 改造为 Extension Kernel HTTP API 层 |
| `protocol.go` 中 Skill 专属类型 | 最终删除，由新类型替代 |
