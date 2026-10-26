# QQ 个人号渠道

个人 QQ 通过 SnowLuma（OneBot 11 协议端）接入 Amitia。

插件 ID：`com.amitia/channel-qq-personal`

渠道 ID：`qq_personal`

## 运行 SnowLuma

```bash
docker run -d --name snowluma \
  --cap-add=SYS_PTRACE \
  --security-opt seccomp=unconfined \
  --shm-size=1g \
  -p 127.0.0.1:5099:5099 \
  -p 127.0.0.1:6081:6081 \
  -v snowluma-data:/app/data \
  motricseven7/snowluma:latest
```

- WebUI：`http://127.0.0.1:5099/`
- noVNC：`http://127.0.0.1:6081/`，用于扫码登录 QQ
- 账号密码：插件页面「运行环境」直接展示，并支持一键复制

## 连接地址

宿主直接运行时：

```text
ws://127.0.0.1:6199/ws
```

Docker 中的 SnowLuma 需要访问宿主，使用：

```text
ws://host.docker.internal:6199/ws
```

在 SnowLuma 的网络配置中新建 WebSocket 客户端（`networks.wsClients`），填写上述地址和插件页面中的 OneBot Token，`messageFormat` 建议使用 `array`。

## 工作区 Tab

插件向渠道工作区注册两个 Tab，追加在宿主内置的「插件页面」「消息统计」之后：

| Tab | 地址 |
| --- | --- |
| SnowLuma WebUI | `http://127.0.0.1:5099/` |
| noVNC 桌面 | `http://127.0.0.1:6081/` |

端口在插件页面「桥接配置」中可改。改动端口后需要同步修改容器的端口映射，Tab 地址固定指向官方默认端口。

## 凭据读取

插件通过 `docker exec` 读取容器内文件：

- `/app/data/config/webui.json`：WebUI 用户与密码状态
- `/app/data/config/vnc-password`：noVNC 明文密码

首次启动的临时密码只出现在容器日志中（`initial credentials: user=... password=...`），插件会自动取最新一条。WebUI 密码只能读到哈希，修改过密码后页面会提示「已由你修改，插件无法读取」。

容器名默认 `snowluma`；`docker` 不在 PATH 时，可在插件页面填写可执行文件的完整路径。

## WebUI 密码记录

SnowLuma 只保存密码的 scrypt 哈希，改密后无法反推，所以插件提供两个入口：

- `修改密码`：走 SnowLuma 官方 `POST /api/auth/change-password`，成功后把新密码加密记录到插件数据目录
- `记录当前密码`：你在 SnowLuma 里自己改过密码时，录入一次即可

记录使用 AES-256-GCM 加密，密钥来自宿主 `AMITIA_SECRET_LEASE`，没有时退化为本地
`webui-credential.key`（32 字节，0600）；密文在 `webui-credential.json`（0600），不含明文。

页面上的密码状态：

| 状态 | 含义 |
| --- | --- |
| 已记录 | 使用加密记录里的密码，可显示和复制 |
| 临时密码 | 未记录，使用容器日志里的初始密码 |
| 未记录 | 已改密但插件不知道，需要录入 |
| 未读取到 | 容器或 docker 不可用，见错误提示 |

## 配置项

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `listenPort` | `6199` | 反向 WebSocket 监听端口 |
| `websocketPath` | `/ws` | 反向 WebSocket 路径 |
| `webuiPort` | `5099` | SnowLuma WebUI 端口 |
| `novncPort` | `6081` | noVNC 端口 |
| `containerName` | `snowluma` | 容器名 |
| `dockerBin` | 空 | docker 可执行文件路径 |

也可通过环境变量 `QQ_PERSONAL_WEBUI_PORT`、`QQ_PERSONAL_NOVNC_PORT`、`QQ_PERSONAL_CONTAINER`、`QQ_PERSONAL_DOCKER_BIN` 覆盖。

## 打包

```bash
node plugins-code/channels/qq-personal/scripts/build-package.mjs
```

产物：仓库根目录 `plugin/com.amitia.channel-qq-personal-<版本>.amitiax`。
