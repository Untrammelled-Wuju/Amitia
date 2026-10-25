# QQ 个人号渠道

个人 QQ 通过 OneBot 11 反向 WebSocket 接入 Amitia。

插件 ID：`com.amitia/channel-qq-personal`

渠道 ID：`qq_personal`

NapCat 连接地址默认为：

```text
ws://127.0.0.1:6199/ws
```

Docker Desktop 中的 NapCat 使用：

```text
ws://host.docker.internal:6199/ws
```

NapCat 网络配置需要新建“WebSockets 客户端”，填写上述地址和插件页面中的 OneBot Token。

插件页面同时展示 NapCat WebUI 登录 Token。该值从宿主机挂载的
`webui.json` 读取，与 OneBot Token 不同，不能混用。默认自动查找
`~/napcat-docker/data/config/webui.json`、`~/napcat/config/webui.json`
等常见路径，也可以通过 `QQ_PERSONAL_NAPCAT_WEBUI_CONFIG` 指定完整路径。

登录后的 NapCat 使用账号级配置：

```text
data/config/onebot11_<QQ号>.json
```

仓库根目录的 `plugin/com.amitia.channel-qq-personal-<版本>.amitiax` 是安装包。

一键运行包仍放置在 `runtime/llbot`，该目录不进入 Git。
