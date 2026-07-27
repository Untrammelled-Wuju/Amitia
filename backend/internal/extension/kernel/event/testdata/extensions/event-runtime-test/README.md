# 事件系统运行时测试扩展

本扩展用于测试 U-Ai 扩展框架的事件系统运行时功能。

## 扩展信息

- 扩展 ID：`event-runtime-test`
- 版本：`1.0.0`
- 运行时类型：`javascript_main`

## 结构说明

```
event-runtime-test/
├── amitiax.json              扩展包清单，定义扩展元信息、模块与事件订阅贡献点
├── modules/
│   └── main/
│       ├── module.json       模块定义，声明 javascript_main 运行时与事件处理器入口
│       ├── index.js          JavaScript 入口，注册 on-test-event 事件处理器
│       └── events.json       事件订阅定义，订阅 test.event.triggered 事件
└── README.md
```

## 事件订阅

- 贡献点 ID：`test-event-subscription`
- 订阅事件类型：`test.event.triggered`
- 事件类型 ID（扩展命名空间）：`ext.event-runtime-test.event.triggered`
- 处理器入口：`on-test-event`

## 处理器行为

当收到 `test.event.triggered` 事件时，`on-test-event` 处理器会记录事件日志，包含事件 ID、事件类型、投递 ID、订阅 ID 和尝试次数，并返回 `{ received: true, eventId: <事件ID> }`。

## 用途

本扩展供 `backend/internal/extension/kernel/event` 包的测试使用，用于验证事件订阅注册、事件投递和 JavaScript 运行时事件处理器调用的完整链路。
