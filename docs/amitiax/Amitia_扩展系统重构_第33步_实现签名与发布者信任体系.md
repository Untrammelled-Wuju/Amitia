# Amitia 扩展系统重构第 33 步实施文档

## 第 33 步：实现签名与发布者信任体系

---

## 一、步骤目标

在第 13 步包安全基础上，将签名验证、Publisher Identity、密钥轮换、撤销、信任级别、安装策略和更新连续性正式接入 Extension Kernel。

目标：

```text
Package Signature
→ Publisher Key
→ Signature Verification
→ Revocation/Expiry
→ Publisher Trust
→ Lifecycle Policy
→ User Confirmation
```

---

## 二、信任不等于权限

即使 Official Publisher：

-仍需声明 Permission；
-仍受 Scope；
-仍受 Runtime 安全；
-仍受包安全；
-仍受资源限制。

Trust 只影响：

-默认策略；
-警告级别；
-自动更新资格；
-高风险 Runtime 资格；
-发布者身份连续性。

---

## 三、签名算法

建议首选：

```text
Ed25519
```

内容摘要：

```text
SHA-256
```

必须使用成熟密码库。

不得自行设计签名算法。

---

## 四、签名载荷

签名至少绑定：

-Extension ID；
-Version；
-Manifest Version；
-Manifest Hash；
-Content Tree Hash；
-Package Hash；
-Publisher ID；
-Key ID；
-Created At；
-Compatibility 摘要；
-可选渠道。

---

## 五、签名文件

建议：

```json
{
  "format": "amitiax-signature-v1",
  "algorithm": "ed25519",
  "publisherId": "com.example",
  "keyId": "main-2026",
  "payloadHash": "sha256:...",
  "signature": "base64:...",
  "createdAt": "..."
}
```

---

## 六、Publisher Identity

```go
type PublisherIdentity struct {
    PublisherID  string
    DisplayName  string
    Keys         []PublisherKey
    TrustLevel   PublisherTrustLevel
    Source       string
    UpdatedAt    time.Time
}
```

---

## 七、Trust Level

统一：

```text
official
trusted
user_trusted
unknown
blocked
revoked
development
```

---

## 八、密钥状态

```text
active
rotated
expired
revoked
compromised
unknown
```

---

## 九、密钥轮换

新版本使用新 Key 时必须证明连续性：

-旧 Key 签名新 Key；
-官方信任库更新；
-用户手动确认；
-发布者账户签名链。

不能只因 Publisher ID 相同就自动接受新 Key。

---

## 十、撤销

撤销来源：

-官方撤销列表；
-本地阻止；
-用户撤销信任；
-密钥泄露报告；
-包 Hash 黑名单。

撤销后：

-阻止新安装；
-阻止自动更新；
-已安装 Extension 进入警告或 Quarantine，按严重度决定；
-不自动删除用户数据；
-写审计。

---

## 十一、离线验证

必须支持离线验证已缓存信任资料。

在线更新撤销列表是补充，不应成为每次本地启动的硬依赖。

---

## 十二、信任存储

建议：

```text
publisher_identities
publisher_keys
publisher_trust_decisions
publisher_revocations
package_signature_records
package_hash_blocks
```

用户信任决定必须可撤销。

---

## 十三、用户信任

用户可对：

-单个包 Hash；
-单个 Key；
-Publisher；
-仅当前版本；
-开发工作区；

建立信任。

默认优先级：

```text
blocked/revoked > package block > user trust > publisher trust
```

---

## 十四、Unknown Publisher

默认策略：

-允许预览；
-允许安装但默认禁用，或要求确认；
-禁止高风险 Service Runtime；
-禁止静默自动更新；
-显示权限和二进制；
-保存信任决定。

---

## 十五、Official Publisher

只能通过内置官方根或受保护更新渠道确认。

Manifest 自报 `official` 无效。

---

## 十六、Development Trust

开发模式：

-仅本地工作区；
-明显标识；
-不允许自动传播为正式信任；
-可热重载；
-仍受路径、权限和 Host API；
-不得绕过关键包安全。

---

## 十七、更新连续性

Update Plan 检查：

-Extension ID 相同；
-Publisher 相同；
-Key 连续；
-版本递增；
-签名有效；
-内容 Hash；
-渠道；
-权限变化。

Publisher 变化视为高风险所有权转移。

---

## 十八、所有权转移

需要：

-旧 Publisher 授权；
-新 Publisher 接受；
-用户确认；
-审计；
-更新策略重置；
-自动更新暂停；
-权限重新确认可选。

---

## 十九、签名时间

签名时间仅作记录，不能单独证明可信时间。

如使用时间戳服务，必须独立验证。

本地不应因系统时间轻微错误直接拒绝所有签名。

---

## 二十、策略引擎

Lifecycle Policy 使用：

-Trust；
-Runtime Type；
-Permission Risk；
-平台二进制；
-发布者连续性；
-撤销；
-用户设置。

输出：

```text
allow
allow_with_confirmation
install_disabled
deny
quarantine
```

---

## 二十一、前端

展示：

-发布者；
-Key；
-签名状态；
-Trust；
-撤销；
-首次安装；
-所有权变化；
-版本连续性；
-包 Hash；
-用户信任范围。

避免只显示“安全/不安全”二元标签。

---

## 二十二、自动更新资格

仅当：

-签名有效；
-Publisher 连续；
-Trust 策略允许；
-无权限扩大或用户策略允许；
-无高风险 Migration；
-无撤销；
-版本合法。

---

## 二十三、测试要求

覆盖：

-有效签名；
-错误签名；
-内容修改；
-Manifest 修改；
-未知 Key；
-轮换；
-撤销；
-过期；
-用户信任；
-Blocked；
-Development；
-所有权转移；
-离线；
-信任库损坏；
-同版本重发；
-跨平台；
-自动更新策略。

---

## 二十四、实施任务

1. 定义签名格式。
2. 实现载荷 Canonicalization。
3. 实现 Ed25519 验证。
4. 建立 Publisher Store。
5. 建立 Trust Decision。
6. 实现 Key Rotation。
7. 实现 Revocation。
8. 实现 Package Hash Block。
9. 接入 Lifecycle Policy。
10. 接入 Install/Update/Rollback。
11. 接入自动更新资格。
12. 实现前端信任页面。
13. 实现开发信任。
14. 完成安全测试。

---

## 二十五、验收标准

1. 签名绑定完整内容树。
2. Publisher 自报不构成信任。
3. Trust 与 Permission 分离。
4. Key Rotation 有连续性验证。
5. Revocation 可阻止安装。
6. 已安装包不被静默删除。
7. Unknown Publisher 有明确策略。
8. Development Trust 不外溢。
9. Update 检查发布者连续性。
10. 前端可解释。
11. 自动更新只针对合格包。
12. 可进入第 34 步更新、回滚与数据迁移。

---

## 二十六、执行约束

> 信任体系证明“谁发布了这份内容以及内容是否被修改”，不证明扩展拥有无限权限或绝对安全。

禁止：

-官方包绕过 Permission；
-Manifest 自报 Trusted；
-仅比较 Publisher ID；
-忽略撤销；
-未知 Key 自动继承；
-开发信任变正式信任；
-签名失败仍自动更新。
