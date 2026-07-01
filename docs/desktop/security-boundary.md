# Amitia 桌面端安全边界

## 预加载层

- 只暴露部署配置、环境信息、运行时状态、日志目录打开能力
- 不暴露任意文件系统访问
- 不暴露任意命令执行

## BrowserWindow

- `contextIsolation` 启用
- `sandbox` 启用
- `nodeIntegration` 关闭
- 外部链接统一走系统浏览器打开

## 前端调用原则

- 渲染进程只通过 `window.amitiaDesktop` 访问桌面能力
- 业务模块不直接引用 Electron API
- 服务地址切换统一收口到 runtime adapter，避免散落硬编码
