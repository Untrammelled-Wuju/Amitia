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
启动后端必须使用release文件夹中的编译后程序(server.exe)，禁止使用源码go run启动。

Git上传规则：
构建产物不上传git（desktop/release/、desktop/build/、desktop/dist-types/等编译输出目录）
依赖不上传git（node_modules/等）
release文件夹需要上传git，但其中的exe文件不上传，只上传对应的zip压缩包
surreal.exe有自动解压机制，上传surreal.zip即可
qdrant.exe有自动解压机制，上传qdrant.zip即可
server.exe上传server.zip即可
根目录的release文件夹整体结构需要保留并上传
