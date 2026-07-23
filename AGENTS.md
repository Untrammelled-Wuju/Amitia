禁止使用cmd和powershell工具进行批量替换。
电脑中已经安装powershell7,必须使用powershell时优先使用powershell7。
必须中文回复。
项目中不写注释。
项目每次修改后必须重启完整服务(前端、后端)。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。
禁止修改编译后产物，任何代码修改只允许在源码中进行修改。
用户提出的任何修改需求需要先结合项目进行详细的分析后再进行修改。
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

桌面端构建规则：
桌面端运行时使用AmitiaCore.exe作为核心后端服务，该文件由Go后端编译产物server.exe重命名而来。

桌面端安装程序构建前应当先更新其依赖的后端核心、侧车等

构建桌面端安装程序前必须停止完整项目并清理desktop/release，避免旧产物或运行数据混入。

必须重新编译Go后端server.exe，并将本次编译结果重命名同步为desktop/resources/core/AmitiaCore.exe，禁止使用旧核心。

必须重新构建微信和QQ侧车的TypeScript与bundle.mjs，并同步各自的bundle.mjs和launcher.mjs。

desktop/resources/core发布资源使用严格白名单，只允许包含AmitiaCore.exe、sidecar/bundle.mjs、sidecar/launcher.mjs、qq-sidecar/bundle.mjs、qq-sidecar/launcher.mjs。

禁止将侧车node_modules、源码、测试、日志、数据库、缓存、备份和其他运行数据带入安装包。侧车bundle必须在不携带node_modules的环境中通过启动和状态接口检查。

禁止将desktop/resources/core/qdrant/storage及任何Qdrant运行时存储带入安装包。Qdrant和SurrealDB只通过resources/qdrant/qdrant.zip与resources/surrealdb/surreal.zip发布。

桌面端正式构建统一从desktop目录执行pnpm dist:win，禁止绕过scripts/build-release.mjs直接执行electron-builder，以确保使用规定的压缩等级和--publish never。

electron-builder保持compression: normal，实际7z压缩等级由scripts/build-release.mjs设置为ELECTRON_BUILDER_COMPRESSION_LEVEL=5。

必须保持nsis.differentialPackage: true，禁止为了加速构建关闭差分更新。

完整发布必须同时生成AmitiaSetup-${version}-x64.exe、AmitiaSetup-${version}-x64.exe.blockmap和latest.yml，缺少任意一项都视为构建失败。

每个正式版本必须维护desktop/release-notes.md，并通过releaseInfo写入latest.yml。latest.yml中的版本、安装包文件名、文件大小和SHA-512必须与实际安装包一致。

构建完成后必须验证blockmap可解压解析、app-update.yml指向正确GitHub仓库、安装内容不含运行数据，并检查AmitiaCore.exe与本次Go构建产物哈希一致。

构建完成后必须执行桌面端测试和侧车测试，并按项目规则重启完整服务，确认前端、核心、Qdrant、SurrealDB、微信侧车和QQ侧车端口及健康接口正常。

未收到用户上传Git的明确指令时，桌面构建和发布过程不得上传GitHub Release或推送Git。

启动项目前必须先杀一遍项目占用（环境除外）

