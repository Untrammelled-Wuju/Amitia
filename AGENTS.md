禁止使用cmd和powershell工具进行批量替换。
构建手机端时必须从后端源码重新构建 Runtime Package，并使用同一份后端源码重新构建手机端所需后端文件，禁止使用旧 Runtime Package 或旧后端产物。
任何修改必须先给出行业标准方案；如果现有设计不符合行业标准方案，必须先给出行业标准方案，等待用户确认无误后，再按照确认后的行业标准方案进行修改或重构。
项目中不使用系统Node环境，必须使用nodeexe或Linux版本可直接运行的Node二进制文件。
电脑中已经安装powershell7,必须使用powershell时优先使用powershell7。
Go 安装路径：C:\Code\Go\bin\go.exe
Git 安装路径：C:\Code\Git\Git\bin\git.exe
必须中文回复。
项目中不写注释。
项目每次修改后必须重启完整服务(前端、后端)。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。

禁止在有xx系统的情况下，重复造第二套相同的系统，优先对原系统进行修改，如果修改难度超过重构难度那就进行局部重构，遇到已有两套系统的情况优先进行询问，或者遇到无法抉择的情况也优先进行询问。

开发一个功能时，如果了解到的信息不够支持进行开发，优先询问用户，当拿到充足的信息后，有100%把握一次性完成开发后再进行开发。

用户提出的任何修改需求需要先结合项目进行详细的分析后再进行修改。
除非用户明确指定本次修改属于管理端，否则默认禁止改动管理端任何代码、配置、端口、数据库和发布流程；当需求不可避免地涉及管理端改动时，必须先向用户说明原因并询问，获得明确授权后才能修改。
任何的修改不要使用3000端口(CCX)，项目中使用的端口应避开3000。
禁止拉取git。
项目启动端口严格使用项目内端口，禁止乱改端口。
启动=启动完整项目。
重启=重启完整项目。

Git上传规则：
构建产物不上传git（desktop/release/、desktop/build/、desktop/dist-types/等编译输出目录）
依赖不上传git（node_modules/等）
surreal.exe有自动解压机制，上传surreal.zip即可
qdrant.exe有自动解压机制，上传qdrant.zip即可
server.exe上传server.zip即可

Git冲突处理规则：
git提交或合并出现任何冲突时，直接以本地代码强制覆盖冲突内容并继续提交，不需要询问用户。

插件目录规则：
插件完整源码统一放在项目根目录的plugins文件夹内。
插件打包产物统一输出到项目根目录的plugin文件夹内。
禁止将插件完整源码放入plugin文件夹。

桌面端构建规则：
桌面端运行时使用AmitiaCore.exe作为核心后端服务，该文件由Go后端编译产物server.exe重命名而来。

桌面端安装程序构建前应当先更新其依赖的后端核心与公共运行时

构建桌面端安装程序前必须停止完整项目并清理desktop/release，避免旧产物或运行数据混入。

必须重新编译Go后端server.exe，并将本次编译结果重命名同步为desktop/resources/core/AmitiaCore.exe，禁止使用旧核心。

desktop/resources/core发布资源使用严格白名单，只允许包含AmitiaCore.exe以及公共Node运行时压缩包。

渠道运行时、Native Companion、bundle、源码、依赖、测试、日志、数据库、缓存和备份必须随插件包发布，禁止带入桌面端宿主的core目录。

禁止将desktop/resources/core/qdrant/storage及任何Qdrant运行时存储带入安装包。Qdrant和SurrealDB只通过resources/qdrant/qdrant.zip与resources/surrealdb/surreal.zip发布。

桌面端正式构建统一从desktop目录执行pnpm dist:win，禁止绕过scripts/build-release.mjs直接执行electron-builder，以确保使用规定的压缩等级和--publish never。

electron-builder保持compression: normal，实际7z压缩等级由scripts/build-release.mjs设置为ELECTRON_BUILDER_COMPRESSION_LEVEL=5。

必须保持nsis.differentialPackage: true，禁止为了加速构建关闭差分更新。

完整发布必须同时生成AmitiaSetup-${version}-x64.exe、AmitiaSetup-${version}-x64.exe.blockmap和latest.yml，缺少任意一项都视为构建失败。

每个正式版本必须维护desktop/release-notes.md，并通过releaseInfo写入latest.yml。latest.yml中的版本、安装包文件名、文件大小和SHA-512必须与实际安装包一致。

构建完成后必须验证blockmap可解压解析、app-update.yml指向正确GitHub仓库、安装内容不含运行数据，并检查AmitiaCore.exe与本次Go构建产物哈希一致。

构建完成后只执行本次源码修改直接涉及的桌面端、后端和插件测试，禁止默认运行全量测试；未改动模块不得执行测试。并按项目规则重启完整服务，确认前端、核心、Qdrant、SurrealDB及已安装渠道插件端口和健康接口正常。

未收到用户上传Git的明确指令时，桌面构建和发布过程不得上传GitHub Release或推送Git。

桌面端版本发布规则：
桌面端版本更新托管在自有服务器（amitia.untrammelled.top），使用electron-updater的generic provider，通过FTP上传构建产物到服务器静态目录。

发布基线与更新日志规则：

当用户明确要求构建桌面端 NSIS 安装包或 Android Release APK 时，必须先创建本次发布的源码基线提交，且不得包含构建产物、依赖、缓存、日志或运行数据。

构建版本号必须保持项目当前版本；只有用户明确指定新的版本号时才允许修改，禁止自行递增、降级或调整版本号格式。

构建前必须读取本文件中的最近一次对应平台源码基线提交，与本次源码基线提交进行差异对比。对比范围仅限发布相关源码、配置、迁移和文档；忽略 AGENTS.md 发布记录、构建产物、依赖、缓存、日志与运行数据。

构建前的测试范围必须基于本次源码差异确定，只运行修改文件所属模块的定向测试；禁止默认运行 `go test ./...`、前端全量测试或其他无关模块测试。

构建门禁（release:gate）执行规则：

- 后端门禁只做编译级校验加改动包定向测试：`go mod verify`、`go vet ./...`、`go build ./cmd/server`，以及由源码差异推导出的改动包 `go test`；构建流程中禁止默认执行 `go test ./...`。
- 差异推导默认以 HEAD 为基准并包含工作区改动；工作区干净时用 `AMITIA_RELEASE_BASE_REF` 指定基线提交，推导为空则不执行后端测试。
- 前端门禁只执行 `typecheck` 与 `build`，默认不执行前端测试；需要时用 `AMITIA_RELEASE_FRONT_TEST_FILES` 显式指定要跑的测试文件，禁止默认执行前端全量测试。
- 编译级校验通过即可执行 `pnpm dist:win` 产出发布产物；不得为了"保险"执行未改动模块的测试。
- 门禁环境变量：`AMITIA_RELEASE_BASE_REF`（差异基准提交）、`AMITIA_RELEASE_GO_TEST_PACKAGES`（显式指定后端测试包）、`AMITIA_RELEASE_FRONT_TEST_FILES`（显式指定前端测试文件）、`AMITIA_RELEASE_FULL_GO_TESTS=1`、`AMITIA_RELEASE_FULL_FRONT_TESTS=1`、`AMITIA_RELEASE_RACE_CHECKS=1`、`AMITIA_RELEASE_SKIP_FRONT_TESTS=1`、`AMITIA_RELEASE_PLAN_ONLY=1`（只打印本次门禁范围，不执行任何命令）。
- 发布输出目录默认为 `desktop/release`，可用 `AMITIA_RELEASE_DIR` 指定相对 desktop 的目录名；当 `release` 被系统进程锁住（例如杀软仍持有 `win-unpacked/resources/app.asar`）导致 `EPERM` 时，用它输出到新目录后重试，且该目录仍需落在 `desktop/release-*` 忽略规则内。
- GitHub Actions 的 Backend Full Test Gate 仍按工作流执行全量后端测试，与本地发布门禁相互独立。
- 桌面端构建前先跑一次范围预检确认门禁内容（不执行任何命令）：在 desktop 目录用项目内 node 执行 `scripts/verify-release-prerequisites.mjs`，并设置 `AMITIA_RELEASE_PLAN_ONLY=1`；随后按预检结果执行 `pnpm dist:win`。
- 已知与本轮改动无关的陈旧用例：`internal/search` 的超时断言停留在 `timeoutpolicy` 之前、`internal/system` 仍在断言已删除的 legacy web-chat 路由、`internal/psyche_testdata` 未注入统一入口执行器。它们不在定向门禁范围内，重建对应模块时一并修复。

必须根据差异生成面向用户的更新日志。桌面端更新 desktop/release-notes.md；Android Release 更新对应发布说明。生成内容作为本次发布版本的更新说明。

更新日志截图的样式参考：

- 使用纯白背景，不添加卡片、渐变、纹理、插画或装饰性背景。
- 页面顶部左侧显示简洁的 Amitia 品牌字样，右侧显示 BETA 标识。
- 主标题使用粗体“更新日志”，下方紧接版本号，版本号下放置一条短蓝色横线。
- 正文使用左对齐的黑色或深灰色中文小字号，二级标题用于“更新内容”“适用范围”等分组，三级标题用于“更新通道”等具体分类。
- 更新项目使用圆点列表，行距紧凑，不使用编号徽标、彩色卡片或大面积强调块。
- 截图采用纵向长页面排版，内容连续排列，底部不保留大面积空白。
- 只参考并复用上述排版样式，不在文档或项目中嵌入参考图片。

构建、安装和验证全部通过后，必须更新本文件中的发布基线记录，包含平台、版本、源码基线提交编号、提交日志和记录时间。该记录变更需单独提交，且不得作为下一次发布差异对比的目标提交。

首次执行对应平台构建且不存在已记录源码基线时，以本次源码基线提交的父提交作为差异对比起点。

若工作区存在无关变更、提交冲突或无法判断变更归属，必须先询问用户，禁止混入发布基线提交。

桌面端发布文件必须先上传至服务器临时目录，验证完整性后向用户提供明确的服务器替换命令。用户确认替换成功后，必须将可复用的上传、验证与替换流程补充至本文件。

最近发布源码基线：

- Desktop NSIS：版本 26.2.0-beta.8.1，源码基线提交 a33980cec，提交日志 chore(release): bump to 26.2.0-beta.8.1，安装包 AmitiaSetup-26.2.0-beta.8.1-x64.exe，大小 275810656 字节，SHA-256 2c244f8e25949e8eadd68c752810e5c771a8efae7756194a5922a1014436d1a1，记录时间 2026-10-19 17:41:47 +08:00，构建输出目录 desktop/release，通过管理端接口发布到 stable 通道，latest.yml 与安装包已通过 HTTPS 返回 200；已在 E:\asdadsadsadasd 覆盖安装，插件页面 iframe、bridge.js、styles.css、app.js 均返回 200 并完成渲染；管理端现有部署版本仅支持 desktop stable，beta.yml 仍停留在 26.2.0-beta.6，待替换该文件或重新部署管理端后测试版客户端才能收到本次更新
- Android Release：版本 26.2.0-beta.8.1，versionCode 14，源码基线提交 a33980cec，提交日志 chore(release): bump to 26.2.0-beta.8.1，安装包 amitia-26.2.0-beta.8.1-release-arm64-v8a-20261019-175800.apk，大小 135341610 字节，SHA-256 7e022b0f4854999b4673daa56bb85e09189c8f592c1db2adb93607fc687aa6e1，记录时间 2026-10-19 17:58:00 +08:00，beta 与 stable 两个通道均已发布，stable.json、beta.json 与 APK 已通过 HTTPS 返回 200；本次构建时无已连接真机，未执行覆盖安装验证

assisted 安装器的更新行为（已实现并验证）：

- 首次安装保持向导（选择用户与安装目录）；后续更新静默覆盖安装到原目录并自动重启，由 `desktop/nsis/installer.nsh` 提供：
  - `customCheckAppRunning`：只精确匹配 `$INSTDIR\` 下的进程并静默结束。electron-builder 默认用 `Path.StartsWith($INSTDIR)`，会把数据目录 `%LOCALAPPDATA%\Programs\AmitiaData` 也判为运行中，导致升级永远提示"Amitia 无法关闭"。
  - `customUnInstallCheck` / `customUnInstallCheckCurrentUser`：旧卸载器返回非 0 时不再中止安装，由新 payload 原地覆盖。
- 验证方式：`AmitiaSetup-<version>-x64.exe /S --updated --force-run`（electron-updater 的原始参数）覆盖已安装版本，安装器约 60 秒完成、`app.asar` 更新、应用自动重启且 `/readyz` 为 ready。

更新通道说明：

- 手机端默认通道是 `stable`（`AMITIA_ANDROID_UPDATE_CHANNEL` 默认值），发布必须同时写 `stable`，否则正式通道客户端会显示"更新服务暂未配置可用版本"。
- 本机同时存在 `C:\WINDOWS\system32\adb.exe`（1.0.40）与 SDK `platform-tools\adb.exe`（1.0.41），两者会互相 kill server 并导致设备掉线、`input tap` 失效；所有 adb 操作必须使用 SDK 的完整路径。

管理端发布接口（当前实际使用的发布通道）：

- `desktop/scripts/.publish-config.json` 中的 FTP 账号仍是模板占位符 `REPLACE_WITH_FTP_USER`，FTP 上传会返回 `530 Access denied`；发布前必须先把真实 FTP 账号写入该文件，否则改用管理端接口发布。
- 管理端接口发布流程（同源 HTTPS，登录 cookie + `X-CSRF-Token`）：
  1. `POST /api/v1/admin/auth/login`，body 为 `{username,password}`，从响应 `data.csrfToken` 取 CSRF
  2. `POST /api/v1/admin/releases` 创建草稿，桌面端 `product=desktop channel=stable`，手机端 `product=android channel=<stable|beta|alpha> versionCode=<n> abi=arm64-v8a`
  3. `POST /api/v1/admin/releases/<id>/artifacts` 上传制品，multipart 字段 `kind`（桌面端 `installer`/`blockmap`，手机端 `apk`）与 `file`
  4. `POST /api/v1/admin/releases/<id>/validate` 校验，再 `POST /api/v1/admin/releases/<id>/publish` 发布
- 发布结果：桌面端写入 `latest.yml` 与安装包；手机端写入 `releases/<versionCode>/<apk>` 并刷新 `stable.json`/`beta.json`/`alpha.json` 与 `.sig`，通道为累进式（stable 只服务 stable，beta 服务 stable+beta，alpha 服务全部）

发布配置：

- 更新服务器：https://amitia.untrammelled.top/amitia
- FTP配置文件：desktop/scripts/.publish-config.json（含密码，已gitignore，禁止上传git）
- 配置模板：desktop/scripts/.publish-config.example.json
- electron-builder.yml中publish.provider为generic，url为https://amitia.untrammelled.top/amitia
- update-manager.ts中RELEASES_URL指向同一地址

发布命令：

- 构建完成后，在desktop目录执行 pnpm upload 自动上传发布
- 脚本会自动上传 latest.yml、AmitiaSetup-${version}-x64.exe、AmitiaSetup-${version}-x64.exe.blockmap 三个文件
- 上传完成后自动验证 https://amitia.untrammelled.top/amitia/latest.yml 是否可访问

SSH发布流程：

- 服务器地址：8.156.72.48
- 部署用户：codexdeploy
- 专用私钥：C:\Users\Untrammelled\.ssh\codex_ashareking_ed25519_20260912
- Windows OpenSSH 可能存在兼容问题，上传使用 Paramiko，连接时禁用 agent 和自动查找密钥
- 生产目录：/www/wwwroot/amitia.untrammelled.top/amitia
- codexdeploy 无生产目录写权限，先上传到 /tmp/amitia-<version>-<timestamp>
- 上传后使用 sha256sum 核对远端文件与本地文件哈希，再提供宝塔 root 终端替换命令
- 桌面端替换 latest.yml、AmitiaSetup-<version>-x64.exe 和 blockmap
- Android 端替换 android/beta.json、android/beta.json.sig，并将 APK 放入 android/releases/<versionCode>/
- 替换完成后删除旧安装包、旧 Android APK 和失效频道清单
- 发布验收必须通过 HTTPS 检查 latest.yml、beta.json、beta.json.sig、EXE 和 APK 均返回 200

发布前检查项：

- 构建产物必须完整（exe + blockmap + latest.yml 三件套）
- 服务器Nginx配置中 /amitia/ 路径必须配置正确的MIME类型和charset utf-8
- 宝塔安全组必须放行FTP 21端口和被动模式端口范围（39000-40000）

管理端（Amitia 更新管理）构建与部署流程：

上线位置：

- 站点根：/www/wwwroot/amitia.untrammelled.top（www:www 755），访问入口 https://amitia.untrammelled.top/
- 更新通道仍在同目录 amitia/ 子目录，桌面端 latest.yml、Android beta.json 由管理服务写入该目录
- 后端安装目录：/www/server/amitia-admin（admin-server 与 amitia-admin.env，env 必须 chmod 600）
- 服务：systemd 单元 amitia-admin.service，User=www，Restart=always，enabled
- 数据库：服务器 MySQL 5.7，库名与用户名均为 amitia_admin，初始化使用 deploy/mysql/amitia-admin-seed.sql
- 管理服务默认监听 127.0.0.1:18880（AMITIA_ADMIN_ADDR），开发代理 target 在 admin-system/vite.config.ts
- 管理前端请求走同源 /api/v1/admin，服务器上必须由 nginx 将 /api/ 反代到 127.0.0.1:18880，否则页面能开但无法登录

本地构建（全部使用项目内 Node 与固定 Go 路径）：

1. 前端：在 admin-system 目录用项目内 node 执行 vite build，产物为 admin-system/dist
2. 后端：GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o admin-server-linux-amd64 ./cmd/admin-server
3. 打包内容：dist/、admin-server、deploy/mysql/amitia-admin-seed.sql，并用 PowerShell Get-FileHash 生成 SHA256SUMS

上传与替换：

1. 用 codexdeploy 账号与专用私钥执行 ssh，在 /tmp 下创建 amitia-admin-<yyyyMMdd-HHmmss> 暂存目录（生产目录 codexdeploy 无写权限）
2. 用 scp -r 把 dist、admin-server、SQL、SHA256SUMS 上传到该暂存目录
3. 上传后必须在该目录执行 sha256sum -c SHA256SUMS，逐文件确认 OK 后再交付替换命令
4. 由用户在宝塔 root 终端执行替换：静态文件落到站点根、后端 install 到 /www/server/amitia-admin、写入 env 与 systemd 单元并 enable --now
5. 站点根目录放静态文件时禁止使用 cp -a <dir>/. ./（会把源目录权限 700 覆盖到站点根，导致 codexdeploy 无法读取且面板操作异常），用 cp -a <dir>/* ./ 或在复制后 chmod 755
6. 站点根存在宝塔受保护的 .user.ini（chattr +i），chown -R 会报 Operation not permitted，授权时只对本次新增文件与目录执行

nginx 要求：

- server 块内需要 location ^~ /api/ 反代到 127.0.0.1:18880（含 proxy_read_timeout 与 client_max_body_size 4096m，桌面安装包为 GB 级）
- 需要 location / 配 try_files $uri $uri/ /index.html，否则 /login、/settings 等前端直链返回 404
- 宝塔「伪静态」文件 /www/server/panel/vhost/rewrite/<域名>.conf 由 vhost 以 server 级 include 引入，其中的 rewrite 会先于 location 匹配执行，必须保持为空
- 修改 vhost 或伪静态后必须 nginx -t 校验并 nginx -s reload，再从服务器本机复测

发布验收：

- 在服务器本机执行：curl -sk -o /dev/null -w '%{http_code}' -H 'Host: <域名>' https://127.0.0.1<path>
- 必须覆盖 /、/login、/settings、/assets/ 下的入口 JS、/api/v1/admin/auth/me（未登录应为 401）、/amitia/latest.yml
- 必须实际提交一次 POST /api/v1/admin/auth/login，确认返回 200「登录成功」且 Set-Cookie 带 HttpOnly、Secure、SameSite=Strict
- 服务必须 systemctl is-active 与 is-enabled 同时为 active/enabled，并确认监听 127.0.0.1:18880

可复用部署流程（2026-10-16 管理端上线时已完整执行并验证）：

本地构建与打包：

1. 在 admin-system 目录用项目内 Node 执行 vite build，产物为 admin-system/dist
2. 在 backend 目录执行：GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o admin-server-linux-amd64 ./cmd/admin-server
3. 在 artifacts/admin-deploy-<yyyyMMdd-HHmmss> 下组装 dist/、admin-server（由 admin-server-linux-amd64 改名）、amitia-admin-seed.sql
4. 用 PowerShell Get-FileHash -Algorithm SHA256 生成 SHA256SUMS，每行格式为「小写哈希 + 两个空格 + 相对路径」；哈希必须转小写，大写哈希会导致远端 sha256sum -c 校验失败

上传（Paramiko，禁用 agent 与自动查找密钥）：

1. sftp 在 /tmp/admin-deploy-<yyyyMMdd-HHmmss> 建目录并递归上传全部文件，admin-server 上传后 chmod 755
2. 远端执行 cd <暂存目录> && sha256sum -c SHA256SUMS，逐文件 OK 后才能向用户交付替换命令
3. Windows 下禁止把含中文的本地路径写入 ascii 编码的临时文件再读取（会被替换成 ? 导致上传空跑），改用环境变量传参

宝塔 root 终端替换命令：

1. cp -a dist/index.html 覆盖站点根 index.html；rm -rf 站点根/assets 后 cp -a dist/assets 落到站点根
2. chown -R www:www 站点根/index.html 与 assets；chmod 755 站点根与 assets；find assets -type f -exec chmod 644 {} +
3. systemctl stop amitia-admin；install -m 0755 -o www -g www admin-server /www/server/amitia-admin/admin-server；chown www:www /www/server/amitia-admin；systemctl start amitia-admin
4. 数据库无需手工操作：新二进制启动时会自动执行 adminMigrations，创建 user_feedbacks 等新表
5. 禁止重新执行 deploy/mysql/amitia-admin-seed.sql，它会重置管理员密码并删除全部管理会话，仅用于首次初始化

替换后必须验收：

1. ls 站点根/assets | grep <本次新增前端分块名>，确认新页面分块已就位
2. 服务器本机 curl 必须覆盖 /、/login、/settings、/feedback、/api/v1/admin/auth/me（未登录 401）
3. 读取 /www/server/amitia-admin/amitia-admin.env 后执行 SHOW TABLES LIKE '<新表名>'，确认增量迁移已生效
4. curl -F 'payload={...}' https://127.0.0.1/api/v1/feedbacks 必须返回 {"code":200,"msg":"反馈已接收"}；返回 404「接口不存在」说明新二进制没有生效
5. 公网复测必须加 --noproxy '*'，本机 7890 代理会让请求超时并误判为服务异常

启动项目前必须先杀一遍项目占用（环境除外）

Electron 桌面端启动时会自动通过 CoreManager 拉起后端服务（AmitiaCore.exe），无需手动单独启动后端。

数据库迁移规则（三库统一版本注册）：

架构概述：

- SQLite 主库：baseline.sql（go:embed 嵌入）为声明式基线，包含全部 CREATE TABLE IF NOT EXISTS 语句
- 增量迁移：backend/internal/migration/migrations.go 中 DefaultMigrations() 返回有序迁移列表
- 版本追踪：统一注册到 schema_migrations 表，Qdrant 前缀 qdrant:NNN，SurrealDB 前缀 surreal:NNN
- Checksum 校验：每个迁移执行后计算 SHA-256 checksum 写入 schema_migrations，防止迁移被篡改

每次数据库结构变更时必须执行以下两步：

1. 追加增量迁移：在 migrations.go 的 DefaultMigrations() 末尾追加 Migration，Version 命名为 YYYYMMDDNNN（日期+三位序号）
2. 同步更新基线：在 baseline.sql 中追加对应的 CREATE TABLE IF NOT EXISTS 语句，保证新装用户一次建全

迁移编写约束：

- 迁移只增不改：已发布的迁移禁止修改，否则 Checksum 校验失败导致启动拒绝
- 如需兼容历史 checksum 变更：在 Migration 的 AcceptedChecksums 字段中声明旧 checksum
- CREATE TABLE 用 IF NOT EXISTS，ADD COLUMN 用 Step.AddColumn（内部自动判重）
- 数据迁移类操作（UPDATE/DELETE）放在 Up 函数中用 Step.Execute 执行
- 禁止在迁移中使用 DROP COLUMN 或 DROP TABLE（SQLite 限制），需要时用建新表+迁移数据+删旧表方式
- AutoMigrate 产生的表必须收编为正式版本化迁移（参照 ConsolidationAutoMigrateMigration），禁止在代码中直接 AutoMigrate

新库与老库自动处理：

- 新数据库：IsNewDatabase 检测空库 → ApplyBaseline 一次性建全部表 → MarkAllMigrationsApplied 标记所有迁移已应用 → 跳过历史迁移执行
- 已有数据库：CreatePreMigrationBackup 预迁移备份 → ApplyBaseline 幂等补全 → Apply 依次执行未应用的版本化迁移
- 内核 SQLite（extension/kernel/persistence/sqlite）有独立迁移系统，不纳入统一注册

Android 构建与真机安装规则：

- 手机端 versionName 必须与桌面端保持一致，versionCode 只能单调递增；两端版本号不一致时禁止发布。
- 仅允许构建、安装和验证 Release 版本 APK，禁止执行、安装或保留任何 Debug 版本 APK。
- 真机安装前必须确认包名为 com.amitia.amitia_app；发现 com.amitia.amitia_app.debug 时必须先卸载，并在安装后复核该 Debug 包不存在。
- 每次修复完成后，只要修改涉及手机端或手机内置 Runtime，必须自动构建 Release APK 并安装到已连接真机；只有用户明确说明仅修改源码或暂不构建时才可以跳过。

Android Release 构建方法：

- 项目构建不设置固定内存预算，不再限制为 1 GiB。默认由 JVM 和构建工具自动配置内存；需要时可根据机器可用内存调整，不再限制堆、Metaspace 和 CodeCache 的合计上限。
- APK 构建使用单个 Gradle worker，关闭 Gradle 并行构建；Kotlin 在 Gradle 同一进程中编译。
- 构建前检查并停止遗留的 Gradle/Kotlin 构建进程，避免重复构建；核验新进程使用的实际内存参数。
- 禁止为 APK 构建启用自动 JVM 堆转储，避免内存不足时生成大体积转储文件。
1. 禁止使用 scripts/build-apk.ps1 的临时源码复制流程。正式构建必须使用项目根目录中的同一份源码。
2. 项目位于中文路径时，统一从 R: 盘符映射构建。R: 必须映射到 D:\桌面\跟进项目\U-Ai，先执行 subst.exe 检查；缺少映射时执行 subst.exe R: "D:\桌面\跟进项目\U-Ai"。R: 只是同一源码目录的盘符映射，不是源码副本，禁止将源码复制到其他目录构建。
3. 当前进程必须配置正式签名环境变量 AMITIA_KEYSTORE_PATH、AMITIA_KEYSTORE_PASSWORD、AMITIA_KEY_ALIAS、AMITIA_KEY_PASSWORD。密钥和密码只能通过环境变量注入，禁止写入 AGENTS.md、源码、配置、日志或构建产物。
4. 构建前设置：
   - $env:PUB_HOSTED_URL = "https://pub.dev"
   - 若 $env:JAVA_TOOL_OPTIONS 不含 jdk.net.unixdomain.tmpdir=，追加 -Djdk.net.unixdomain.tmpdir=C:\Temp，并确保 C:\Temp 存在。
5. 在 R:\mobile_app 执行：
   - flutter clean
   - flutter pub get
   - flutter build apk --release --target-platform android-arm64 --no-tree-shake-icons --config-only
6. 在 R:\mobile_app\android 执行：
   - 先执行 `.\gradlew.bat :app:lintVitalRelease --no-daemon --no-parallel --max-workers=1 '-Pkotlin.compiler.execution.strategy=in-process' '-Dorg.gradle.jvmargs=-Dfile.encoding=UTF-8 -XX:-HeapDumpOnOutOfMemoryError'`
   - 上一步成功退出后，再执行 `.\gradlew.bat assembleRelease --no-daemon --no-parallel --max-workers=1 '-Pkotlin.compiler.execution.strategy=in-process' '-Dorg.gradle.jvmargs=-Dfile.encoding=UTF-8 -XX:-HeapDumpOnOutOfMemoryError'`
   - 两个阶段必须串行执行，使用同一份源码与新构建的 Runtime Package；禁止跳过 Release lint。内存由 JVM 自动配置，遇到内存不足时可根据机器资源调整。
7. 必须使用 Gradle 返回码判断构建结果。由于中文路径下 Flutter 包装命令可能在产物查找阶段误报失败，禁止仅凭 flutter build apk 的产物查找结果判定构建失败或成功。
8. APK 输出路径固定为 R:\mobile_app\android\app\build\outputs\flutter-apk\app-release.apk，等价实际路径为 D:\桌面\跟进项目\U-Ai\mobile_app\android\app\build\outputs\flutter-apk\app-release.apk。
9. 安装前必须验证：
   - applicationId 为 com.amitia.amitia_app
   - versionName、versionCode 与项目当前版本一致
   - apksigner 验证为正式 Release 签名
   - zipalign 验证通过
   - APK 仅包含 arm64-v8a
   - APK 内 assets/runtime-package/amitia-runtime-1.0.0.zip 的 SHA-256 与构建输入一致
10. 安装流程：
    - adb devices -l
    - adb shell pm list packages 检查 com.amitia.amitia_app.debug；存在时先执行 adb uninstall com.amitia.amitia_app.debug
    - adb install -r <APK路径>
    - adb shell dumpsys package com.amitia.amitia_app 复核版本，并再次确认 debug 包不存在
    - adb shell am start -W -n com.amitia.amitia_app/.MainActivity 启动应用
    - adb shell ps -A 与 adb shell dumpsys activity activities 复核应用进程和前台页面
11. 正式 APK 必须复制到 artifacts/apk/，文件名使用 amitia-<version>-release-arm64-v8a-<yyyyMMdd-HHmmss>.apk，并记录文件大小和 SHA-256。

源码副本规则：

- 禁止使用临时复制的源码目录构建、打包、安装或验证正式版本。
- 如因排查临时复制源码到项目根目录以外的位置，使用完成后必须立即删除该复制目录，并在删除后确认项目根目录仍为唯一构建来源。



在制作插件、修复插件功能、开发插件的时候，如果需要给插件系统新增功能，必须将新增的扩展能力抽象为公共扩展能力，让后续其他插件在制作的时候也可以使用。
