# API 分类

> 范围：`backend/internal/extension/router.go` 中所有注册的路由，以及 MCP 路由

---

## 一、保留并改名

无纯保留的 API。所有当前 API 均绑定旧模型。

## 二、兼容迁移（过渡期保留原路径）

| API 路径 | 方法 | 处理方式 | 新路径 |
|---|---|---|---|
| `/extensions/openapi.json` | GET | 保留并改名 | `/extensions/openapi.json`（不变） |
| `/extensions/capabilities` | GET | 保留并改名 | `/extensions/capability-definitions` |
| `/extensions/runs` | GET | 改造后复用 | `/extensions/run-records` |
| `/extensions/runs/:runId` | GET | 改造后复用 | `/extensions/run-records/:runId` |

---

## 三、替换（新路径替代旧路径）

### Skill → Capability/Contribution
| 旧 API | 新 API | 备注 |
|---|---|---|
| `GET /extensions/skills` | `GET /extensions/capabilities` | 返回统一 Capability 列表 |
| `GET /extensions/skills/:id` | `GET /extensions/capabilities/:id` | 统一详情 |
| `POST /extensions/skills/:id/enable` | `POST /extensions/capabilities/:id/enable` | 统一生命周期 |
| `POST /extensions/skills/:id/disable` | `POST /extensions/capabilities/:id/disable` | 同上 |
| `GET /extensions/skills/:id/permissions` | `GET /extensions/capabilities/:id/permissions` | 统一 Permission Broker |
| `PUT /extensions/skills/:id/permissions` | `PUT /extensions/capabilities/:id/permissions` | 同上 |
| `GET /extensions/skills/:id/config` | `GET /extensions/capabilities/:id/config` | 统一 Config Store |
| `PUT /extensions/skills/:id/config` | `PUT /extensions/capabilities/:id/config` | 同上 |
| `POST /extensions/skills/:id/config/reset` | `POST /extensions/capabilities/:id/config/reset` | 同上 |
| `POST /extensions/skills/:id/execute` | `POST /extensions/capabilities/:id/execute` | 统一执行 |
| `POST /extensions/skills/:id/workshop/fork` | `POST /extensions/capabilities/:id/fork` | 统一 Fork |
| `POST /extensions/skills/:id/versions/:version/rollback` | `POST /extensions/capabilities/:id/versions/:version/rollback` | 统一 Rollback |

### Agent Skill
| 旧 API | 新 API |
|---|---|
| `POST /extensions/agent-skills/import/preview` | `POST /extensions/agent-skills/preview` |
| `POST /extensions/agent-skills/import/install` | `POST /extensions/agent-skills/install` |
| `GET /extensions/agent-skills` | `GET /extensions/agent-skills`（不变，改名） |
| `GET /extensions/agent-skills/:id` | 合并到 Capability 详情 |
| `POST /extensions/agent-skills/:id/enable` | 合并到统一生命周期 |
| `POST /extensions/agent-skills/:id/disable` | 合并到统一生命周期 |
| `DELETE /extensions/agent-skills/:id` | 合并到统一卸载 |
| `GET /extensions/agent-skills/:id/resources` | `GET /extensions/agent-skills/:id/resources`（不变） |
| `GET /extensions/agent-skills/:id/resources/content` | 不变 |
| `GET /extensions/agent-skills/:id/assets/content` | 不变 |
| `GET /extensions/agent-skills/:id/activations` | 合并到审计记录 |

### Plugin
| 旧 API | 新 API |
|---|---|
| `GET /extensions/plugins` | `GET /extensions/contributions?type=plugin` |
| `GET /extensions/plugins/:id` | `GET /extensions/contributions/:id` |
| `POST /extensions/plugins/:id/enable` | 统一生命周期 |
| `POST /extensions/plugins/:id/disable` | 统一生命周期 |
| `POST /extensions/plugins/:id/reload` | 统一重载 |
| 其余 Plugin 专属 API | 合并到统一 Extension API |

### Package
| 旧 API | 新 API |
|---|---|
| `POST /extensions/packages/import/preview` | `POST /extensions/packages/preview` |
| `POST /extensions/packages/import/install` | `POST /extensions/packages/install` |
| `GET /extensions/packages/metrics` | 不变 |
| 其余 Package API | 保留路径，内部重写 |

### Workshop
| 旧 API | 新 API |
|---|---|
| `GET /extensions/workshop/sessions` | `GET /extensions/dev/sessions` |
| `POST /extensions/workshop/sessions` | `POST /extensions/dev/sessions` |
| 其余 Workshop API | 迁移到 `/extensions/dev/` 路径 |

---

## 四、删除（无调用或直接删除）

| API 路径 | 原因 |
|---|---|
| 无明确的零调用 API | 所有当前路由均被前端调用 |

---

## 五、最终删除（新 API 替代后）

所有旧路径的兼容层（alias），在迁移完成后删除：

| 旧路径 | 删除条件 |
|---|---|
| `/extensions/skills/*` | Capability API 就绪且前端切换 |
| `/extensions/plugins/*` | Contribution API 就绪且前端切换 |
| `/extensions/workshop/*` | Developer Tooling API 就绪 |
| 所有旧 Skill 概念 URL | 全部前端切换完成 |
