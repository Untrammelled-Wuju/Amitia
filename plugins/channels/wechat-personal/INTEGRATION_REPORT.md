# 个人微信渠道改造报告

## 结论

个人微信渠道已从本机微信/Native Companion 方案切换为腾讯 iLink 协议方案。连接、账号状态和消息收发均由插件自身完成，不再依赖安装设备上的微信客户端。

## 已移除依赖

- 删除 `runtime.nativeCompanions` 声明
- 删除本机微信安装路径探测
- 删除本机微信进程启动、隐藏和窗口控制
- 删除 `child_process` 调用
- 删除 Native Agent、Hook Driver 和 Preload 运行链路
- 删除无鉴权本地回调入口

## 当前链路

1. `/api/connect` 调用 iLink `get_bot_qrcode`
2. 插件将二维码转换为 PNG Data URL 返回页面
3. 后台轮询 `get_qrcode_status`
4. `confirmed` 后保存 `bot_token`、`ilink_bot_id` 和 `baseurl`
5. 启动 `getupdates` 长轮询
6. 文本消息转发到 `/api/channels/inbound`
7. `/api/send` 使用入站消息携带的 `context_token` 调用 `sendmessage`

## 账号隔离

插件账号文件与其他微信插件、本机微信、OpenClaw 目录完全分离。断开只清除个人微信插件自己的账号令牌，不影响电脑微信或其他渠道插件。

## 验证

- `service-e2e.mjs`：二维码、扫码确认、令牌保存、入站消息、文本回复、幂等和断开
- `service-security-e2e.mjs`：Bearer 鉴权、无 wildcard CORS、旧回调入口不可用

两个测试均使用模拟 iLink 服务，不要求真机扫码，也不启动本机微信。
