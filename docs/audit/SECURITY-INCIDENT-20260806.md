# Security Incident 2026-08-06-001：后端运行包凭据泄露

状态：已按阶段 0 处理完毕（凭据已轮换为模板值，运行数据已从工作区与 git 索引清除，Source Archive Guard 已建立）。

## 事件摘要

审计发现后端运行包归档含真实凭据与运行数据，全部按已泄露处理。相关凭据一律视为泄露并完成轮换，禁止复用。

## 泄露凭据类型（值已轮换，不在此记录原文）

- Local Root Token：backend/config/config.yml security.localToken
- JWT Secret：backend/config/config.yml、config/config.yml、desktop/resources/config-template/config.yaml
- MCP Secrets Key：backend/data/mcp-secrets.key、desktop/resources/data/mcp-secrets.key（均已从 git 索引移除并删除）
- QQ 机器人 Token：backend/qq-sidecar/data/qqbot-config.json
- FTP 发布凭据：desktop/scripts/.publish-config.json（未跟踪但本地存在）
- QQ 设备指纹：backend/device.json、backend/qq-sidecar/data/device.json（均已从 git 索引移除并删除）

## 已执行动作（2026-08-06）

1. 停止归档传播：backend-source.zip 已从 git 索引移除并加入 .gitignore。
2. 凭据轮换：上述配置文件凭据已替换为随机生成的新值。config.yml 保留为模板（localToken / JWT secret 使用新随机值）；qqbot-config.json 的 token 置空；.publish-config.json 使用占位符。
3. 撤销会话：本地运行数据全部删除，Desktop Session、Runtime Bootstrap Ticket 与全部旧 JWT 随数据库移除而失效。
4. MCP Secret Store：mcp-secrets.key 已删除，Fresh Install 时重新生成密钥并重加密。
5. 运行数据删除：AmitiaData/、data/、qdrant/、surrealdb/、storage/、backend/qdrant/、backend/data/、backend/cmd/data/、logs/、*.db、*.log、迁移备份、已编译产物已全部清除。
6. 防再提交：.gitignore 新增 backend/data/、backend/cmd/data/、**/migration_backups/、**/migration_backups_prev/、**/mcp-secrets.key、**/local-token、device.json、qrcode.png、backend-source.zip 等规则。
7. Source Archive Guard：新增 scripts/verify-source-archive.ps1 与 scripts/verify-source-archive.sh，黑名单包含全部已知泄露值；新增 .gitattributes 统一行尾与二进制标记。

## 待执行（用户侧）

- 在外部系统轮换：QQ 机器人 AppSecret / Token、FTP 账号密码（amitia.untrammelled.top）、任何写入其他部署环境的 JWT Secret。
- 重新填写 desktop/scripts/.publish-config.json 的 FTP 凭据后恢复发布能力。
- QQ 侧车重新配置 backend/qq-sidecar/data/qqbot-config.json（appId / token）后启用。

## 验证结果

- scripts/verify-source-archive.ps1 当前通过（无运行数据、无泄露凭据）。
- git 索引中已无任何凭据文件、数据库、日志或编译产物。
