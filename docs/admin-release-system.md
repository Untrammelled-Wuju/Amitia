# Amitia 更新管理端

## 服务组成

- 管理前端源码：`admin-system/`
- 管理服务源码：`backend/cmd/admin-server/`
- 发布业务源码：`backend/internal/adminrelease/`
- 管理数据：MySQL `amitia_admin`
- 本地受管 MySQL：`127.0.0.1:13306`
- 管理服务端口：`18880`
- 管理前端开发端口：`15179`

客户端更新地址保持不变：

- 桌面端稳定通道：`https://amitia.untrammelled.top/amitia/latest.yml`
- 桌面端测试通道：`https://amitia.untrammelled.top/amitia/beta.yml`
- 手机端：`https://amitia.untrammelled.top/amitia/android/<channel>.json`

## 首次启动

管理服务首次启动且数据库中没有管理员账号时，使用 `AMITIA_ADMIN_BOOTSTRAP_USERNAME` 指定的用户名和 `AMITIA_ADMIN_BOOTSTRAP_PASSWORD` 指定的密码创建管理员。未配置密码时，服务生成随机密码并在首次创建账号时写入启动日志。

## 环境变量

```text
AMITIA_ADMIN_ADDR=127.0.0.1:18880
AMITIA_ADMIN_MYSQL_HOST=127.0.0.1
AMITIA_ADMIN_MYSQL_PORT=3306
AMITIA_ADMIN_MYSQL_USER=amitia_admin
AMITIA_ADMIN_MYSQL_PASSWORD=
AMITIA_ADMIN_MYSQL_DATABASE=amitia_admin
AMITIA_ADMIN_MYSQL_CHARSET=utf8mb4
AMITIA_ADMIN_MYSQL_LOC=Local
AMITIA_ADMIN_MYSQL_MAX_OPEN_CONNS=50
AMITIA_ADMIN_MYSQL_MAX_IDLE_CONNS=10
AMITIA_ADMIN_MYSQL_CONN_MAX_LIFETIME_MINUTES=60
AMITIA_ADMIN_DATA_DIR=admin-data
AMITIA_ADMIN_PUBLISH_ROOT=admin-data/publish
AMITIA_ADMIN_DESKTOP_PUBLISH_DIR=admin-data/publish/amitia
AMITIA_ADMIN_ANDROID_PUBLISH_DIR=admin-data/publish/amitia/android
AMITIA_ADMIN_PUBLIC_BASE_URL=https://amitia.untrammelled.top/amitia
AMITIA_ADMIN_COOKIE_SECURE=false
AMITIA_ADMIN_SESSION_HOURS=12
AMITIA_ADMIN_ALLOWED_ORIGINS=http://127.0.0.1:15179,http://localhost:15179
AMITIA_ADMIN_MAX_UPLOAD_BYTES=4294967296
AMITIA_ADMIN_ANDROID_PACKAGE_NAME=com.amitia.amitia_app
AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PATH=
AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PASSPHRASE=
AMITIA_ADMIN_WEB_DIR=admin-system/dist
```

生产环境必须启用 HTTPS，并设置：

```text
AMITIA_ADMIN_COOKIE_SECURE=true
```

## 桌面端发布

1. 创建桌面端发布草稿，填写更新标题、最多 8 条本版亮点和 Markdown 完整更新内容。
2. 上传 `AmitiaSetup-<version>-x64.exe`。
3. 上传 `AmitiaSetup-<version>-x64.exe.blockmap`。
4. 执行制品校验。
5. 发布到稳定通道时写入 `latest.yml`，发布到测试通道时写入 `beta.yml`。
6. 稳定通道发布时同步刷新 `beta.yml`，测试通道始终选择稳定版和测试版中语义版本最高的版本。

桌面端最新指针只能帮助尚未升级的客户端。已经安装更高版本的客户端需要发布更高版本的修复包。

桌面端 beta 安装包在首次运行且没有历史偏好时默认开启测试更新。用户主动关闭后保持关闭，不会因版本升级强制重新开启。

## 手机端发布

1. 创建手机端发布草稿，填写 `versionCode`、灰度比例、最低支持版本、更新标题、最多 8 条本版亮点和 Markdown 完整更新内容。
2. 上传正式签名的 arm64-v8a APK。
3. 执行制品校验。
4. 发布后服务端生成 `<channel>.json`，使用 RSA SHA-256 生成 `<channel>.json.sig`。

上传顺序固定为 APK、签名文件、通道清单。手机端客户端不会接受低于已安装 `versionCode` 的清单。

## 更新展示字段

- `updateTitle`：弹窗“本次更新”标题下方的用户可见标题。
- `highlights`：本版亮点数组，最多 8 条；桌面端两列展示，手机端单列展示。
- `releaseNotes`：完整更新内容，按 Markdown 渲染。
- 字段会同时写入桌面端 `latest.yml` / `beta.yml` 和 Android 签名通道清单。
- 历史发布记录没有新增字段时，客户端继续使用兼容回退内容。

## 并发上传与下载

管理端大文件上传使用 8 MiB 分片和 4 路并发：

- `POST /api/v1/admin/releases/:id/upload-sessions` 创建上传会话。
- `GET /api/v1/admin/releases/:id/upload-sessions/:uploadId` 查询已上传分片。
- `PUT /api/v1/admin/releases/:id/upload-sessions/:uploadId/parts/:index` 上传单个分片，`X-Chunk-SHA256` 为分片摘要。
- `POST /api/v1/admin/releases/:id/upload-sessions/:uploadId/complete` 校验全部分片、合并并计算最终 SHA-256 和 SHA-512。
- `DELETE /api/v1/admin/releases/:id/upload-sessions/:uploadId` 取消会话。

上传会话临时文件和元数据保存在管理数据目录的 `artifact-uploads`，默认 24 小时清理。前端会记录未完成会话，页面刷新后只补传缺失分片。

桌面端继续优先使用 electron-updater 差分下载，只有完整安装包下载才切换为 8 路 Range 下载。Android 使用前台 `dataSync` 服务进行自适应并发下载，Wi-Fi 默认 4 路、蜂窝网络默认 2 路；服务器不支持 Range 时自动回退单线程。两端都会校验清单中的文件大小和哈希，失败时保留可恢复分片或在完整校验失败后清理。

## 服务器部署

服务器端管理库使用 `deploy/mysql/amitia-admin-seed.sql` 初始化，可在管理服务首次启动前后执行：

```bash
mysql --host=<host> --port=<port> --user=root --password=<password> < deploy/mysql/amitia-admin-seed.sql
```

脚本执行结果：

- 创建 `amitia_admin` 库（utf8mb4 / utf8mb4_unicode_ci）以及 `admin_users`、`admin_sessions`、`update_releases`、`update_release_artifacts`、`admin_audit_logs` 五张表，表结构与迁移 `202609240001` 完全一致

账号由管理服务初始化。重复执行 SQL 保留现有账号和会话，库中已存在管理员账号时启动流程跳过创建。迁移版本与校验值由管理服务启动时登记。

MySQL 业务账号不写入该脚本，凭据仅通过环境变量注入：

```text
AMITIA_ADMIN_MYSQL_USER=amitia_admin
AMITIA_ADMIN_MYSQL_PASSWORD=<secret>
AMITIA_ADMIN_MYSQL_DATABASE=amitia_admin
```

## 本地验证

```powershell
$env:PATH = "D:\桌面\跟进项目\U-Ai\desktop\resources\core\node;$env:PATH"
pnpm --dir admin-system typecheck
pnpm --dir admin-system build

cd backend
& "C:\Code\Go\bin\go.exe" test .\internal\adminrelease .\cmd\admin-server
& "C:\Code\Go\bin\go.exe" build -o .\admin-server.exe .\cmd\admin-server
```

完整项目启动脚本会自动构建缺失的管理服务二进制并启动管理 API 与管理前端开发服务。

本地开发默认读取已忽略的 `config/admin-mysql.local.json`，启动脚本会在 `C:/Code/AmitiaMySQLAdmin/data` 管理独立 MySQL 实例。生产环境应删除本地配置文件并通过环境变量或密钥管理系统注入外部 MySQL 凭据。
