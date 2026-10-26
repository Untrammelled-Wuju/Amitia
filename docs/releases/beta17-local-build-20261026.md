# beta.17 本地构建记录

记录时间：2026-10-26T18:11:52+08:00

版本：26.2.0-beta.17；Android versionCode：23。
源码基线：0c38b70fd8d32fdf539b4eb1b8b08179671e782c。
提交日志：chore(release): refresh beta.17 renderer smoke source manifest。
白屏修复功能提交：fef49ff608b17fe0e776991fb42c2a61c3c73835。
上一已发布基线：7f110257bb0acb9d5065f89cc534c03060b64a90。

Windows NSIS：AmitiaSetup-26.2.0-beta.17-x64.exe，274772827 字节，SHA-256 72bb4a7736a60ac560ac2145b63360d00ebfed8e8984e0742407757dfe8b8507。
核心 SHA-256：31b37ead8d232c1fb4319e37865866165184bf599a6d4265805075b333235b7e。
APK：amitia-26.2.0-beta.17-release-arm64-v8a.apk，136379183 字节，SHA-256 03716a22ec76b73a2ca9efc4edd310f9c384a49707f10fa802706ece941ad611。
Runtime Package SHA-256：eb50a1ea624c28d341bc3aff22f3373ba5f67e415fe36504efaf20c63d8c3bd8。
Source Gate：59e1ee7afba5cf9129c03292130d0686bc3d70d56f6b2803953455dc23af7a1b；11192 个冻结文件通过校验。

EXE 三件套、SHA-512、blockmap 解压与累计大小、AMD64 PE 核心、generic 更新地址、资源白名单、包内无运行数据校验通过。
APK 正式证书、v2 签名、16 KB 对齐、单 arm64-v8a、内嵌 Runtime 与最终源码编号校验通过。
后端 11 个改动包校验，前端原 47 项、启动恢复 4 项、可信模块 2 项、桌面诊断 2 项、手机端 84 项、QQ 40 项、微信 22 项定向测试通过。
初次 Node 环境覆盖引发配置测试失败，隔离环境后通过；一次剪贴板时序用例偶发失败，单项 10 次及所在包复跑通过。
类型检查、生产构建、脚本语法、Electron 挂载冒烟、Android Release Vital 与无障碍 lint 通过。
5 个插件全部重打包并校验，插件合集与独立包在交付文件目录。
完整项目已重启，前端及 Core、Qdrant、SurrealDB、已安装渠道插件健康接口正常；Core beta.17，readyz 为 6 ready、0 degraded、0 blocking、0 failed。

当前为 macOS，未实测 Windows NSIS 安装。最终 APK 构建完成时设备断开，未安装最终包；早期 e4d54dbd 构建曾覆盖安装并冷启动成功，不作为最终包真机验证结果。
初次本地交付按用户要求未发布；随后收到“推送云端更新”的明确指令，正式发布结果如下。未发布 GitHub Release，未推送 Git。
详细记录：artifacts/release-26.2.0-beta.17/release-build-record.json。
本记录提交不是下一次差异比较的源码目标提交。


## 云端正式通道发布

记录时间：2026-10-26T19:07:16+08:00。
通过现有管理端 HTTPS 接口发布，先完成两个平台的草稿上传及服务端校验，再切换 stable 通道。
桌面端发布编号 45；Android 发布编号 46，versionCode 23。
线上 latest.yml 版本 beta.17，安装包大小、SHA-512、更新说明与本次制品一致，blockmap SHA-256 与本地一致。
线上 android/stable.json 版本 beta.17 / 23，APK 大小和 SHA-256 一致；stable.json.sig 使用客户端内置公钥通过 RSA SHA-256 验证。
EXE 与 APK 首段、中段、尾段共 6 个 Range 内容逐字节与本地一致。
公开清单：https://amitia.untrammelled.top/amitia/latest.yml 及 https://amitia.untrammelled.top/amitia/android/stable.json 。
完整发布结果与线上校验保存在 artifacts/release-26.2.0-beta.17/admin-publish-result.json 和 cloud-verification.json。
未推送 Git；本记录提交不是下一次发布差异的源码目标提交。
