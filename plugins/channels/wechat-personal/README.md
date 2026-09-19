# Amitia Personal WeChat Channel

独立个人微信渠道插件。插件使用 Wechaty Web 协议完成二维码登录、独立会话保存和文本回复，不依赖安装设备上的微信客户端，也不读取电脑微信的登录状态。

## 连接流程

1. 用户点击“获取登录二维码”。
2. 插件通过 Wechaty 生成二维码，并在插件页面内显示。
3. 用户使用手机微信扫码并确认。
4. Wechaty 返回登录事件后，插件保存独立会话并显示已连接。
5. 插件接收文本消息并将其转发到 Amitia 统一渠道入口。

断开操作只删除插件自己的账号文件，不会退出、关闭或修改电脑微信。

## 运行边界

- 渠道 ID：`wechat_personal`
- Provider 端口：`19878`
- 协议：Wechaty Web
- 文本接收：支持
- 文本发送：支持，首次回复需要联系人或群会话已在当前登录会话中出现
- 图片、视频、文件和语音发送：当前未声明
- 群聊：当前未声明
- 本机微信依赖：无
- Native Companion 依赖：无

## 账号存储

桌面端默认保存在：

```text
%APPDATA%\Amitia\extensions\wechat-personal\account.json
```

Linux 默认保存在：

```text
$XDG_CONFIG_HOME/amitia/extensions/wechat-personal/account.json
```

文件包含个人微信会话与账号标识，不应上传到 Git、日志或远端。

## Trusted Service 鉴权

`19878` 上所有接口都要求宿主注入的进程级 Bearer Token。插件通过 `AMITIA_CORE_URL` 提交入站消息，并携带 `X-Amitia-Extension-ID` 和 `X-Amitia-Module-ID`。插件不提供无鉴权回调入口，也不返回 wildcard CORS。

渠道状态由插件 Service 维护。插件页面只在打开时和用户显式操作后读取状态，不启动定时轮询。
