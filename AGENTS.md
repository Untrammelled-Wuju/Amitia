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

启动项目前必须先杀一遍项目占用（环境除外）


