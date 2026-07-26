# Amitia 扩展系统重构第 42 步实施文档

## 第 42 步：升级 Schema UI 系统

---

## 一、步骤目标

将现有 Plugin Schema UI 从“插件管理表单”升级为正式的声明式 UI Runtime，使扩展能够通过受控 Schema 贡献设置页、状态卡片、操作面板、列表、详情和轻量交互界面。

本步骤目标：

> 建立 Host-native、可主题化、可无障碍、可版本化、可校验、可绑定 Host Action 的 Schema UI Renderer，作为 Amitia UI Extension 的默认低风险方案。

---

## 二、适用范围

Schema UI 适合：

-扩展设置；
-MCP 配置；
-Provider 配置；
-状态展示；
-表单；
-列表；
-详情；
-按钮组；
-权限说明；
-资源清单；
-运行诊断；
-简单向导；
-确认界面。

不适合：

-复杂图形编辑器；
-高级动画；
-富文本编辑器；
-复杂画布；
-实时视频；
-自定义大型交互应用。

复杂场景使用 Restricted Web UI。

---

## 三、核心原则

### 1. 宿主原生渲染

扩展只提供 Schema。

Vue Component 由 Amitia 提供。

### 2. 数据与 UI 分离

Schema 描述结构，数据通过 Data Source 提供。

### 3. Action 显式声明

按钮不能内嵌脚本。

### 4. 组件白名单

只允许宿主支持组件。

### 5. 主题统一

使用 Amitia Design Tokens。

---

## 四、Schema 文档

建议：

```json
{
  "schemaVersion": 1,
  "type": "page",
  "title": "天气扩展",
  "layout": {
    "type": "stack",
    "gap": "md"
  },
  "children": []
}
```

---

## 五、基础节点

建议首批支持：

```text
page
section
stack
row
grid
tabs
card
text
markdown
badge
divider
icon
image
field
select
switch
slider
button
button_group
list
table
empty_state
alert
progress
code
key_value
resource_link
permission_summary
runtime_status
```

---

## 六、禁止节点

第一版禁止：

-任意 HTML；
-script；
-style；
-iframe；
-webview；
-canvas 任意脚本；
-动态 Vue Component；
-自定义模板；
-表达式执行；
-宿主组件名称反射。

---

## 七、节点模型

建议：

```go
type SchemaUINode struct {
    ID         string
    Type       string
    Props      json.RawMessage
    Bindings   []SchemaUIBinding
    Actions    []SchemaUIActionBinding
    Visibility []UICondition
    Children   []SchemaUINode
}
```

---

## 八、布局

支持受控：

-Stack；
-Row；
-Grid；
-Tabs；
-Card；
-Section。

限制：

-最大嵌套；
-最大节点；
-Grid 列数；
-禁止绝对定位；
-禁止负间距；
-禁止覆盖宿主层级。

---

## 九、属性 Schema

每个组件有独立 Props Schema。

例如 Button：

```json
{
  "label": "保存",
  "variant": "primary",
  "size": "medium",
  "disabled": false
}
```

未知 Props 拒绝或警告。

---

## 十、数据绑定

绑定来源：

```text
input
state
query
runtime
host
form
```

示例：

```json
{
  "path": "form.apiEndpoint",
  "source": "state"
}
```

禁止任意 JavaScript 表达式。

---

## 十一、受限表达式

允许：

-字段访问；
-等值；
-布尔；
-空值；
-长度；
-枚举；
-简单格式化；
-国际化键。

禁止：

-函数调用；
-循环；
-网络；
-文件；
-动态路径构造；
-Prototype；
-正则灾难；
-宿主对象。

---

## 十二、表单

表单必须使用 JSON Schema：

-字段类型；
-必填；
-格式；
-范围；
-枚举；
-自定义宿主格式；
-错误文案；
-默认值。

Secret 字段：

-只绑定 Secret Reference；
-不回显明文；
-使用专用 Secret Input；
-保存走 Secret Broker。

---

## 十三、Data Source

建议：

```go
type SchemaUIDataSource struct {
    ID            string
    Type          string
    InputSchema   json.RawMessage
    OutputSchema  json.RawMessage
    RefreshPolicy string
    RuntimeEntry  string
}
```

类型：

```text
static
extension_runtime
host_query
storage
runtime_status
resource_list
```

---

## 十四、Action

按钮绑定声明 Action：

```json
{
  "actionId": "save_settings",
  "target": "extension_runtime",
  "input": {
    "source": "form"
  }
}
```

宿主执行：

-校验；
-确认；
-Permission；
-Scope；
-Runtime；
-结果；
-反馈。

---

## 十五、UI State

Schema UI 临时状态：

-表单；
-选中 Tab；
-展开；
-过滤；
-分页；
-加载。

由 Host 管理，不直接持久化。

需要持久化时显式 Action 写 Storage。

---

## 十六、页面状态

统一：

```text
idle
loading
ready
submitting
success
error
empty
offline
runtime_unavailable
```

扩展可提供文案，但不能隐藏关键安全错误。

---

## 十七、Markdown

Markdown 使用宿主安全渲染：

-禁止 HTML；
-链接协议白名单；
-图片包内或受控资源；
-代码长度限制；
-禁止脚本 URL；
-外链打开需确认策略。

---

## 十八、Image

图片来源：

-包内 Asset；
-Host Resource；
-受控网络资源引用。

禁止任意 Data URL 大图和远程追踪像素。

---

## 十九、Table/List

必须：

-分页；
-数量限制；
-稳定 Key；
-空状态；
-加载状态；
-无无限 DOM；
-Action 按行声明。

---

## 二十、主题 Token

Schema 使用语义 Token：

```text
surface
surfaceElevated
textPrimary
textSecondary
border
accent
danger
success
warning
spacing.*
radius.*
typography.*
```

禁止自定义十六进制颜色作为默认设计手段。

---

## 二十一、响应式

宿主根据 Slot 和窗口宽度处理：

-桌面宽屏；
-窄窗口；
-侧栏；
-弹窗；
-全页。

Schema 不使用固定像素布局。

---

## 二十二、无障碍

Renderer 统一提供：

-Label；
-Description；
-Error；
-Focus；
-Keyboard；
-ARIA；
-Contrast；
-Reduced Motion。

扩展必须提供必要文案。

---

## 二十三、Schema Version

```text
schema-ui/1
```

破坏性组件变化进入新版本。

Renderer 提供兼容层，但不允许长期隐式猜测。

---

## 二十四、校验阶段

安装时：

-JSON Schema；
-节点数量；
-深度；
-组件；
-Props；
-Binding；
-Action；
-Data Source；
-Secret；
-资源；
-本地化。

运行时：

-Data；
-Action Input；
-Action Output；
-Visibility；
-大小。

---

## 二十五、性能预算

限制：

-节点数量；
-嵌套；
-表格行；
-刷新频率；
-Data Source 并发；
-Markdown 大小；
-图片；
-Action 次数。

---

## 二十六、错误隔离

单节点错误：

-显示占位；
-记录；
-不中断整个 Extension 页面，除非根节点损坏。

Schema 结构无效：

-拒绝加载；
-显示诊断；
-不运行任意降级脚本。

---

## 二十七、缓存

可缓存：

-已验证 Schema；
-编译后的节点树；
-本地化；
-静态资源引用。

缓存绑定：

```text
definition_hash + schema_ui_version + theme_version
```

---

## 二十八、开发者工具

支持：

-Schema Preview；
-节点树；
-Props；
-Binding；
-模拟数据；
-Action Mock；
-主题；
-响应式；
-无障碍；
-性能；
-校验错误。

---

## 二十九、迁移现有 Plugin Schema UI

现有表单 Schema：

-盘点字段；
-映射标准组件；
-移除直接 API；
-Action 转 Host Action；
-Secret 转 Secret Reference；
-状态转 Data Source；
-前端专用逻辑移出 Schema。

---

## 三十、API 与文件

建议包内：

```text
modules/ui/<id>/schema/page.json
modules/ui/<id>/schema/data-sources.json
modules/ui/<id>/schema/actions.json
```

也可单文件，但必须有 Hash。

---

## 三十一、测试要求

覆盖：

-全部节点；
-Props；
-Binding；
-Visibility；
-Form；
-Secret；
-Data Source；
-Action；
-Markdown；
-Image；
-Table；
-主题；
-响应式；
-无障碍；
-错误节点；
-大 Schema；
-深度；
-恶意表达式；
-缓存；
-迁移旧 Schema。

---

## 三十二、实施任务

1. 定义 Schema UI v1。
2. 定义组件白名单。
3. 定义 Props Schema。
4. 实现 Node Parser。
5. 实现 Renderer。
6. 实现 Binding Engine。
7. 实现 Form Engine。
8. 实现 Data Source。
9. 实现 Action Dispatcher。
10. 实现 Secret Input。
11. 实现 Markdown/Resource 安全。
12. 接入 Theme/Locale。
13. 实现错误边界。
14. 实现开发者 Preview。
15. 迁移旧 Plugin Schema UI。
16. 完成无障碍与性能测试。

---

## 三十三、验收标准

1. Schema UI 成为正式 UI Runtime。
2. 只使用宿主组件。
3. 无任意 HTML/JS。
4. Data 与 Schema 分离。
5. Action 不包含脚本。
6. Secret 使用 Broker。
7. 主题和响应式统一。
8. 无障碍基础由宿主提供。
9. 大 Schema 有限制。
10. 旧 Schema UI 已有迁移报告。
11. 可进入第 43 步沙箱 Web UI。

---

## 三十四、执行约束

> Schema UI 是扩展界面的首选低风险方案，只允许声明结构、数据绑定和宿主动作，不允许成为嵌入任意脚本的变体。

禁止：

-HTML 节点；
-script；
-style；
-任意表达式；
-直接 API URL；
-Secret 明文；
-自定义 Vue；
-绝对定位；
-绕过 Action Dispatcher。
