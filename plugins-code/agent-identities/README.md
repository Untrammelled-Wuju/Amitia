# Agent 身份插件

身份 Provider 全部位于插件源码目录，不进入核心后端。

插件：

- `email`：Gmail、Microsoft Graph、Agent 邮箱身份和对外署名
- `calendar`：Google Calendar、Microsoft Graph 和 Agent 日历身份
- `phone`：Twilio 兼容短信和语音通话
- `browser-profile`：CDP 或通用浏览器 Profile Provider
- `wallet`：外部钱包 Provider，默认关闭资产操作

构建全部插件时，在项目根目录使用项目 Node 运行：

```bash
for item in email calendar phone wallet browser-profile; do
  node plugins-code/agent-identities/scripts/build-package.mjs --package "plugins-code/agent-identities/$item"
done
```

打包产物写入 `plugin/agent-identities/`，每个 `.amitiax` 同时生成 SPDX 2.3 `.sbom.json`。上传到管理端市场：

```bash
AMITIA_PLUGIN_SIGNING_KEY=AmitiaData/security/plugin-publisher-private.key \
node plugins-code/agent-identities/scripts/sign-packages.mjs plugin/agent-identities

AMITIA_ADMIN_USERNAME=... \
AMITIA_ADMIN_PASSWORD=... \
AMITIA_MARKET_PUBLISH=1 \
node plugins-code/agent-identities/scripts/upload-to-market.mjs plugin/agent-identities
```

凭据只通过 `credentialRef` 和 `host.secret.get` 读取，不写入插件状态或源码。
