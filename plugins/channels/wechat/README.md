# OpenClaw 微信插件

插件清单位于 `amitia-extension.json`，桌面页面源码位于 `ui/src`。构建脚本通过现有插件 CLI 生成完整性清单，输出到项目根目录 `plugin`。

在项目根目录使用项目 Node 执行：

```text
desktop/resources/core/node/node.exe --test plugins/channels/wechat/ui/tests/session.test.mjs
desktop/resources/core/node/node.exe plugins/channels/wechat/scripts/build-package.mjs
```

桌面页面复用宿主的 `window.amitiaUI` 和已有渠道命令。微信服务返回的登录链接在页面内编码成二维码，不直接请求远程图片，不调整宿主沙箱策略。

渠道服务从 `runtime/src` 编译，桌面页面从 `ui/src` 编译。手机页面沿用 `package-sources/1.0.14` 中的打包输入。归档目录中的旧桌面文件和旧服务 bundle 不会作为最终输出。

服务使用宿主通过环境变量注入的扩展身份、模块身份和服务凭据调用统一入站接口，凭据不写入源码或配置。页面状态查询会合并重复请求，扫码期间每 5 秒刷新、普通状态每 15 秒刷新，页面隐藏时暂停；遇到宿主限流会等待后再恢复。
