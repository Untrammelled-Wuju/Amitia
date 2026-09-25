# Amitia 更新管理端

## 服务组成

- 管理前端源码：`admin-system/`
- 管理服务源码：`backend/cmd/admin-server/`
- 发布业务源码：`backend/internal/adminrelease/`
- 管理数据：MySQL `amitia_admin`
- 本地受管 MySQL：`127.0.0.1:13306`
- 管理服务端口：`18998`
- 管理前端开发端口：`15179`

客户端更新地址保持不变：

- 桌面端：`https://amitia.untrammelled.top/amitia/latest.yml`
- 手机端：`https://amitia.untrammelled.top/amitia/android/<channel>.json`

## 首次启动

管理服务首次启动且数据库中没有管理员账号时，会创建以下默认账号：

```text
用户名：untrammelled
密码：925011Mc
```

可通过 `AMITIA_ADMIN_BOOTSTRAP_USERNAME` 和 `AMITIA_ADMIN_BOOTSTRAP_PASSWORD` 覆盖默认值。首次登录后应立即修改密码。

## 环境变量

```text
AMITIA_ADMIN_ADDR=127.0.0.1:18998
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

1. 创建桌面端发布草稿。
2. 上传 `AmitiaSetup-<version>-x64.exe`。
3. 上传 `AmitiaSetup-<version>-x64.exe.blockmap`。
4. 执行制品校验。
5. 发布后服务端生成 `latest.yml`，并最后写入发布目录。

桌面端最新指针只能帮助尚未升级的客户端。已经安装更高版本的客户端需要发布更高版本的修复包。

## 手机端发布

1. 创建手机端发布草稿，填写 `versionCode`、灰度比例和最低支持版本。
2. 上传正式签名的 arm64-v8a APK。
3. 执行制品校验。
4. 发布后服务端生成 `<channel>.json`，使用 RSA SHA-256 生成 `<channel>.json.sig`。

上传顺序固定为 APK、签名文件、通道清单。手机端客户端不会接受低于已安装 `versionCode` 的清单。

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
