# UI Style Agent Contract

该契约定义桌面端、手机端和后端共用的样式规则格式。公共工具名保持 `uiagent.appearance`，动作保持 `inspect`、`apply`、`undo`、`reset`，`schemaVersion` 继续为 `1`。

## 核心概念

- `scope`：限定平台、页面和区域。
- `target`：稳定部件标识、DOM 选择器或 `dom:` 选择器。
- `states`：`default`、`hover`、`focus`、`focus-visible`、`active`、`pressed`、`disabled`、`dark`、`light`。
- `properties` 或 `set`：样式属性集合，支持颜色、尺寸、间距、圆角、边框、字体、布局、阴影、透明度和状态样式。
- `reset`：按作用域、部件、状态、属性或事务精确删除覆盖规则。

## 应用示例

```json
{
  "schemaVersion": 1,
  "targets": [
    {
      "source": "ai",
      "scope": {
        "platform": "desktop",
        "page": "chat"
      },
      "target": "chat.composer.input",
      "states": ["default", "focus"],
      "properties": {
        "backgroundColor": "#000000",
        "foregroundColor": "#FFFFFF",
        "placeholderColor": "#9CA3AF",
        "borderRadius": 16
      }
    }
  ]
}
```

## 精确重置示例

```json
{
  "schemaVersion": 1,
  "reset": {
    "scope": {
      "platform": "desktop",
      "page": "chat"
    },
    "target": "chat.composer.input",
    "states": ["default", "focus"],
    "properties": [
      "backgroundColor",
      "foregroundColor",
      "placeholderColor"
    ],
    "source": "ai"
  }
}
```

重置只删除匹配的覆盖规则，不修改默认 UI，不影响其他页面、部件、状态、属性或用户设置。

页面标识统一为小写点分形式：`/chat` 与 `chat` 等价，`/settings/appearance` 与 `settings.appearance` 等价，`/extension/page/emote` 规范为 `plugin.emote`。桌面端可以使用 DOM 选择器；手机端使用稳定部件 target。
