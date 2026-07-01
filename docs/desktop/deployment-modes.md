# Amitia 桌面端部署模式

## 本地模式

- 面向后续内置本地运行时
- 当前状态会展示为未安装
- 前端仍可识别为 `desktop-local`

## 云端模式

- 默认服务地址为 `https://api.amitia.cn`
- 请求地址自动切换到云端 API
- WebSocket 自动切换为 `wss://api.amitia.cn`

## 自建模式

- 允许输入 `http` 或 `https` 地址
- 自动去除 query、hash 和尾部斜杠
- 拒绝 `0.0.0.0`、`::` 等不可路由地址

## 前端接入原则

- Axios 请求通过 runtime adapter 注入 `baseURL`
- 原生 `fetch` 通过 `resolveApiUrl()` 与 `createAuthorizedRequestInit()`
- `EventSource` 与 `WebSocket` 必须先走 URL 解析再建立连接
