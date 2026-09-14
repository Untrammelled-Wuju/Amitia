# 扩展上下文协调契约

扩展上下文不绑定具体插件。宿主只负责发现、调用、排序和聚合，插件负责声明自己能够提供哪些上下文槽位。

## 上下文提供者

插件的内部工具可以通过 `spec.metadata` 声明上下文能力：

```json
{
  "metadata": {
    "amitia.context.provides": [
      "chat.realtime.schedule"
    ],
    "amitia.context.priority": 230,
    "amitia.context.key": "lifestyle"
  }
}
```

- `amitia.context.provides`：该工具能够提供的上下文槽位，可以是字符串或字符串数组。
- `amitia.context.priority`：同一槽位内多个提供者的优先级，数值越大越先执行和聚合。
- `amitia.context.key`：同一扩展内区分多个提供者的稳定名称；省略时使用扩展 ID。

宿主调用提供者时传入统一的上下文请求字段：

- `userId`
- `characterId`
- `conversationId`
- `at`

每个提供者返回自己的业务数据。宿主生成统一快照：

```json
{
  "slot": "chat.realtime.schedule",
  "contributions": [
    {
      "source": "com.amitia/lifestyle:lifestyle",
      "extensionId": "com.amitia/lifestyle",
      "moduleId": "lifestyle-runtime",
      "toolId": "com.amitia/lifestyle/context",
      "priority": 230,
      "data": {}
    }
  ]
}
```

单个提供者失败会记录在该贡献项的 `error` 中，不阻断其他提供者，也不阻断记忆、情绪、主动消息等独立链路。

## 其他协同通道

- `event.subscription`：跨插件事件与状态变化通知。
- `schedule`：插件自有定时任务和周期性调度。
- `workflow`：多步骤任务、补偿、重试和跨插件工具调用。
- `before_prompt`：向统一提示词构建阶段贡献上下文。
- `after_reply`：回复后的观察、记忆候选或后续事件。
- `message.send`：主动消息通过宿主公共发送能力落库和投递。
- `dependencies`：显式声明扩展、模块、Provider、MCP 或宿主 API 依赖。

宿主不会为具体插件增加专用上下文接口。新插件需要参与聊天、实时语音、主动消息或其他运行态时，只需声明上下文槽位或使用上述公共通道。
