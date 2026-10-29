# QQ 个人号渠道

个人 QQ 通过 SnowLuma（OneBot 11 协议端）接入 Amitia。

插件 ID：`com.amitia/channel-qq-personal`

渠道 ID：`qq_personal`

## 运行方式

插件根据宿主平台提供 SnowLuma 运行方式：

| 模式 | 适用平台 | 说明 |
| --- | --- | --- |
| 原生运行 | Windows、Linux | 插件下载并托管 SnowLuma Lite，固定使用项目内置 Node |
| Docker | Windows、Linux、macOS | 连接已有 SnowLuma 容器，读取 WebUI 与 noVNC 凭据 |

Windows 可在 Docker 与 Windows 原生之间切换，Linux 可在 Docker 与 Linux 原生之间切换。macOS 只允许 Docker。运行模式切换后点击保存即可持久化；原生模式会启动插件托管进程，Docker 模式会停止原托管进程并转为容器连接。

## Windows 原生运行

1. 安装 Windows 桌面版 NTQQ 并保持 SnowLuma 与 QQ 使用相同用户、相同权限运行。
2. 在插件页面选择“Windows 原生”，点击“安装 SnowLuma”。
3. 插件从 SnowLuma 官方 GitHub Release 下载 Lite 发行包，校验 SHA-256 后解压到插件数据目录。
4. 安装完成后插件自动使用项目内置 Node 启动 SnowLuma，并生成只包含 Amitia 反向 WebSocket 客户端的首次配置。
5. 打开插件页面中的 SnowLuma WebUI，完成协议确认和首次改密。
6. 在桌面版 QQ 中扫码登录。Windows 原生模式直接使用 QQ 窗口，不展示 noVNC。

插件不会安装 QQ。SnowLuma 使用其源码可见非商业许可，下载和使用前应阅读官方 EULA。

## Linux Docker 运行

Linux 官方推荐 Docker。Amitia 默认监听 `127.0.0.1:6199`，容器必须使用 host 网络才能访问该回环地址：

```yaml
services:
  snowluma:
    image: motricseven7/snowluma:latest
    container_name: snowluma
    network_mode: host
    restart: unless-stopped
    shm_size: 1gb
    cap_add:
      - SYS_PTRACE
    security_opt:
      - seccomp=unconfined
    environment:
      SNOWLUMA_WEBUI_HOST: 127.0.0.1
      SNOWLUMA_WEBUI_PORT: 5099
      SNOWLUMA_ONEBOT_HTTP_PORT: 16100
      SNOWLUMA_ONEBOT_WS_PORT: 16101
      SNOWLUMA_HOOK_AUTOLOAD: "1"
    volumes:
      - qq-gateway-data:/app/data
      - qq-client-config:/app/.config
      - qq-client-data:/app/.local/share

volumes:
  qq-gateway-data:
  qq-client-config:
  qq-client-data:
```

启动：

```bash
docker compose up -d
```

打开 `http://127.0.0.1:6081/` 进入 noVNC 并扫码登录，打开 `http://127.0.0.1:5099/` 进入 SnowLuma WebUI。

不要使用 SnowLuma 默认的 `3000/3001` OneBot 服务。插件只需要反向 WebSocket 客户端，示例已将内部监听改到 `16100/16101`。

## Linux 原生运行

Linux 原生模式需要用户自行准备 Linux QQ、Xvfb、窗口管理器、VNC/noVNC、ptrace 能力和独立的 `DISPLAY`。SnowLuma 官方将这条路径列为进阶方案。

准备完成后，在插件中选择“Linux 原生”并点击“安装 SnowLuma”。插件负责下载、校验、安装、启动 SnowLuma 和生成首次 OneBot 配置，不负责安装 Linux QQ、Xvfb、VNC/noVNC 或其他系统依赖。

## 凭据读取

- 原生模式：插件读取 SnowLuma 数据目录中的 `config/webui.json`，并从托管进程首次启动输出中安全捕获临时密码。
- Docker 模式：插件通过 `docker exec` 读取 `/app/data/config/webui.json` 与 `/app/data/config/vnc-password`。

SnowLuma 只保存密码哈希，不能反推出已修改的密码。插件支持通过官方接口修改密码，或由用户录入当前密码。

## 配置项

| 字段 | 默认值 | 说明 |
| --- | --- | --- |
| `runtimeMode` | Windows 为 `native`，Linux/macOS 为 `docker` | `native` 或 `docker`；macOS 固定为 `docker` |
| `listenPort` | `6199` | 反向 WebSocket 监听端口 |
| `websocketPath` | `/ws` | 反向 WebSocket 路径 |
| `webuiPort` | `5099` | SnowLuma WebUI 端口 |
| `novncPort` | `6081` | Docker noVNC 端口 |
| `containerName` | `snowluma` | Docker 容器名 |
| `dockerBin` | 空 | docker 可执行文件路径 |

环境变量覆盖项包括 `QQ_PERSONAL_RUNTIME_MODE`、`QQ_PERSONAL_WEBUI_PORT`、`QQ_PERSONAL_NOVNC_PORT`、`QQ_PERSONAL_CONTAINER` 和 `QQ_PERSONAL_DOCKER_BIN`。

## 端口约束

插件固定使用项目内端口 `6199`，不会使用 `3000`。原生模式首次生成的 `config/onebot.json` 不创建 HTTP/WS 服务；如果已有配置启用 `3000`，插件会拒绝启动并提示修正。

## 打包

```bash
AMITIA_ZIG=/path/to/zig AmitiaData/node/bin/node plugins-code/channels/qq-personal/scripts/build-package.mjs
```

构建需要 Zig 0.15 或更高版本交叉编译 Linux ARM64 兼容层；也可将 Zig 加入 `PATH` 后省略 `AMITIA_ZIG`。

产物：仓库根目录 `plugin/com.amitia.channel-qq-personal-<版本>.amitiax`。
