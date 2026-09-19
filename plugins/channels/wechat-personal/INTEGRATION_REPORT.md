# 个人微信渠道改造报告

## 结论

个人微信渠道已切换为 Wechaty Web 协议方案。连接、账号状态和消息收发均由插件自身完成，不再依赖安装设备上的微信客户端。

## 已移除依赖

- 删除 iLink Bot 二维码、令牌和消息轮询链路
- 删除 `bot_token`、`ilink_bot_id` 和 `context_token` 依赖
- 保留宿主 Trusted Service 鉴权与统一渠道消息入口

## 当前链路

1. `/api/connect` 启动 Wechaty 与 `wechaty-puppet-wechat4u`
2. Wechaty `scan` 事件将二维码转换为 PNG Data URL 返回页面
3. Wechaty `login` 事件保存独立账号标识与会话状态
4. Wechaty `message` 事件将文本消息转发到 `/api/channels/inbound`
5. `/api/send` 在当前联系人或群会话上调用文本发送

## 账号隔离

插件账号文件与其他微信插件、本机微信、OpenClaw 目录完全分离。断开只清除个人微信插件自己的账号令牌，不影响电脑微信或其他渠道插件。

## 验证

- `service-e2e.mjs`：二维码、扫码确认、令牌保存、入站消息、文本回复、幂等和断开
- `service-security-e2e.mjs`：Bearer 鉴权、无 wildcard CORS、旧回调入口不可用

两个测试均使用模拟 Wechaty 运行时，不要求真机扫码，也不启动本机微信。
