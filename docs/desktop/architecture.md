# Amitia Electron 桌面端架构

当前桌面端采用 `desktop/` 目录承载 Electron 主进程、预加载层和桌面运行时骨架，前端 UI 继续复用 `front/` 的 Vue 应用。

## 目录划分

- `desktop/src/main`：主进程入口、窗口、托盘、IPC 注册、配置存储
- `desktop/src/preload`：向渲染进程暴露受限桌面能力
- `desktop/src/runtime`：运行时管理与本地进程监管骨架
- `desktop/src/shared`：桌面端与前端共享的类型、部署模式、IPC 通道常量
- `front/src/runtime`：浏览器模式与桌面模式的统一连接适配层

## 运行链路

1. Electron 主进程启动后读取部署模式配置。
2. 主进程创建 `DesktopRuntimeManager` 并注册 IPC。
3. 渲染进程启动时调用 `getRuntimeConnection()`。
4. 浏览器模式继续使用当前页面源或 Vite 代理。
5. 桌面模式根据部署模式切换到本地、云端或自建服务器地址。

## 当前边界

- 本阶段本地模式只保留骨架状态，未接入真实本地运行时安装与守护逻辑。
- 云端与自建模式已经具备前端请求、SSE、WebSocket 地址切换能力。
- 前端业务模块不直接感知 Electron，只通过 runtime adapter 获取连接信息。
