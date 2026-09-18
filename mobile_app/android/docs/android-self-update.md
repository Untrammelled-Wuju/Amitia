# Android 自建更新发布

## 通道

- `stable.json`
- `beta.json`
- `alpha.json`

客户端默认读取：

```text
https://amitia.untrammelled.top/amitia/android/stable.json
```

版本清单签名文件与清单同目录，命名为：

```text
stable.json.sig
```

APK 使用不可变版本目录：

```text
/amitia/android/releases/<versionCode>/<apk-name>
```

## 签名密钥

私钥和口令文件只保存在本机，并已被 `.gitignore` 排除：

```text
mobile_app/.update-signing/manifest-private-key.pem
mobile_app/.update-signing/passphrase.txt
mobile_app/.update-signing/manifest-public-key.pem
```

客户端内置公钥：

```text
mobile_app/android/app/src/main/assets/app-update/manifest-public-key.pem
```

私钥必须离线备份。丢失后无法继续签发兼容现有安装包使用的更新清单。

## 构建

修改 `mobile_app/pubspec.yaml` 的版本，`+` 后必须是单调递增的整数：

```yaml
version: 1.0.2+3
```

构建正式 APK 并生成更新清单：

```powershell
pwsh -NoProfile -File scripts/build-android-update.ps1 -Channel stable
```

测试通道和灰度：

```powershell
pwsh -NoProfile -File scripts/build-android-update.ps1 -Channel beta -RolloutPercentage 10
```

强制更新：

```powershell
pwsh -NoProfile -File scripts/build-android-update.ps1 -Channel stable -Mandatory -MinVersionCode 3
```

本地校验：

```powershell
pwsh -NoProfile -File scripts/publish-android-update.ps1 -Channel stable
```

## 上传

未明确要求上传时，不执行带 `-Upload` 的命令。

```powershell
pwsh -NoProfile -File scripts/publish-android-update.ps1 -Channel stable -Upload
```

上传顺序固定为：

1. APK
2. 清单签名
3. 更新清单

清单最后上传，避免客户端拿到指向不存在 APK 的版本信息。

## 服务器

Nginx 必须支持：

- `.apk` 的 MIME 类型为 `application/vnd.android.package-archive`
- `.json` 的 MIME 类型为 `application/json; charset=utf-8`
- APK 支持 `Range` 请求
- APK 使用长期缓存
- 通道清单使用 `Cache-Control: no-cache`
