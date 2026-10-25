# Client Module 与宿主访问

扩展 UI 统一使用同页客户端模块和 Schema UI。旧的 iframe WebUI、独立 origin bridge、`web_restricted`、`web_isolated` 与 `sandbox_webui` 已删除。

## 客户端模块

- Provider 使用 `web_module` 入口，模块导出 Vue 组件。
- 客户端模块与宿主共享同一 Vue 运行时，构建时不得打包第二份 Vue；Vue 生命周期、响应式依赖和渲染上下文必须落在宿主运行时上。
- 宿主导入带 `contentHash` 和 `styleHash` 的不可变模块。
- 认证快照签发短时模块能力票据，资源端点校验票据、Provider 信任级别和内容哈希。
- 模块票据只允许访问本机 Core 的 Provider 资源，并从访问日志中移除。
- 模块只接收 `uiContext`、`provider`、`uiActions` 和 `uiNavigate`。
- Provider action 通过当前 contribution 的宿主 action 接口执行，导航通过宿主 Router 执行。
- 模块渲染异常由 `UIProviderHost` 捕获并进入 Provider fallback。

`web_module` 只允许 `system`、`official`、`trusted`、`user_trusted` 或 `verified` Provider。

## Workspace Tab

- 通用工作区通过 `workspace.tab` Provider 扩展 Tab，不绑定渠道或特定业务。
- Provider 的 `metadata.workspace` 选择工作区，`routePrefix` 与 `contextMatch` 选择生效范围，`tabId`、`title`、`order` 和 `keepMounted` 描述 Tab。
- `schema_renderer` 和 `web_module` 入口在宿主页面中渲染插件 UI。
- `external_web_ui` 入口只允许 Desktop，由主进程创建隔离的 Electron Guest 并加载原始 HTTP(S) WebUI。
- External Guest 使用独立 Session 分区，禁止 Node、预加载脚本、嵌套 WebView、权限、下载、弹窗和应用 Host 请求。
- Guest 主页面导航只允许 Provider 声明的 `allowedOrigins`，外链由用户通过宿主操作打开系统浏览器。
- `storage` 为 `persistent` 时使用按服务身份隔离的持久分区，为 `session` 时使用临时分区。
- Guest 在 Tab 隐藏时保留页面、表单和原生 history，切换回来不重新登录。
- Web 浏览器端不提供 Electron Guest；同一 Provider 应提供独立 UI，或由用户打开系统浏览器。

## Runtime Host Access

`runtime.hostAccess` 显式描述宿主能力：

```json
{
  "filesystem": {
    "mode": "read_write",
    "roots": ["package", "plugin_data"]
  },
  "network": {
    "mode": "host"
  },
  "process": {
    "mode": "direct",
    "maxProcesses": 1
  }
}
```

- `filesystem.mode`：`none`、`read`、`read_write`。
- `filesystem.roots`：`package`、`plugin_data`。
- `network.mode`：`none`、`host`。
- `process.mode`：`none`、`direct`。
- 非 `none` 宿主访问只允许 `official`、`trusted` 或 `user_trusted` 发布者。
- Node runtime 始终启用权限系统，只开放已解析的包目录和插件数据目录。
- `plugin_data` 通过 `AMITIA_PLUGIN_DATA_DIR` 和 `AMITIA_HOST_FILESYSTEM_ROOTS_JSON` 暴露。
- 直接子进程由平台进程树上限管理，插件退出时回收整棵进程树。
