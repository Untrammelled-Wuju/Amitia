# Amitia 插件系统重写验收

日期：2026-11-09

## 结论

最终 12 项源码级验收全部通过。桌面端已完成从当前源码重建、打包、签名、完整重启和运行态健康检查。

> 2026-11-10 更新：本文件记录旧架构基线，不能作为当前目标的完成证明。用户随后明确裁定统筹模式、角色、记忆、主动任务、个人空间、设置、归档、渠道中心、游戏中心、聊天导入、运行时洞察、工作区、存储维护和创意工坊必须保留为核心模块。当前验收范围应按 `docs/architecture/plugin-migration-matrix.md` 的核心例外重新审计；上述模块不得重新转为插件。

## 验收矩阵

| 验收项 | 结果 | 证据 |
| --- | --- | --- |
| 1. 单一 Manifest 格式 | 通过 | 后端只使用 `plugincontract.Manifest`；旧 `manifest_v1` 目录已删除；`TestAllSourceManifestsUseCanonicalContract`、`TestParseRejectsLegacyManifestFields` 通过 |
| 2. 无旧路径和隐式 Adapter 映射 | 通过 | 源码搜索不存在旧 Manifest、`ContributionAdapterForKind` 或旧插件专用安装接口；Contribution 必须显式声明 Adapter |
| 3. 新增非核心功能零内核业务修改 | 通过 | 新脚手架工具项目只通过 Manifest、SDK 和 Host API 完成构建、安装、启用和调用，未增加宿主业务分支 |
| 4. 第三方替换矩阵 | 通过 | `TestToolRegistryThirdPartyReplacementAfterOfficialUnregister`、`TestThirdPartyServiceProviderReplacesOfficialProviderByPriority`、前后端第三方替换矩阵测试通过 |
| 5. 服务依赖图组合 | 通过 | 拓扑顺序、循环依赖、版本冲突、可选依赖、多提供者、独占提供者和反向停用测试通过 |
| 6. 动态注册同步 | 通过 | Plugin Broker 动态工具、UI Provider、Route、Service 注册、上限、冲突、注销和快照测试通过 |
| 7. 生命周期可重复执行 | 通过 | 真实 CLI 完成 `init -> build -> validate --strict -> pack -> install -> enable -> disable -> enable -> uninstall` |
| 8. 零残留清理 | 通过 | 卸载后测试插件进程退出，路由、Provider、工具、服务、监听器和安装记录清理；包事务恢复测试通过 |
| 9. 可选插件损坏不阻断 Core | 通过 | 旧格式随包插件被隔离告警；包恢复失败不再终止 Core；`/readyz` 返回 `blockingCount=0`、`failedCount=0` |
| 10. 无官方特权通道 | 通过 | 固定业务插件与普通插件目录分离；内置包走同一 Manifest、安装、权限和生命周期；Provider 路径无官方插件 ID 白名单 |
| 11. 安全边界 | 通过 | 权限声明、Scope 隔离、核心命名空间保护、Adapter 信任、包签名、路径穿越、存储配额、密钥引用和无敏感日志测试通过 |
| 12. 跨端一致性 | 通过 | Web、桌面和移动端共享 Provider 路由契约；平台不可用原因、核心保护路由和插件路由批次由三端契约测试校验 |

## 运行态证据

- Core 由当前源码重新构建，健康接口返回 HTTP 200、`ready=true`、`runtime=ready`。
- `/readyz` 返回 `blockingCount=0`、`failedCount=0`；仅本机缺少 Chromium 的非阻断浏览器组件降级。
- 旧 `com.amitia.builtin.active-task` 安装已退役，只保留 `com.amitia/active-task` 1.2.4。
- 主动任务停用后 Provider `enabled=false`，再次启用后恢复；历史提醒数据继续由核心 Reminder Host API 承载。

## 验证命令

- `go test ./...`
- `go vet ./...`
- 前端 `vitest run`、`vue-tsc --noEmit`、`vite build`
- 移动端 `flutter test`
- 桌面 `vitest run`、`tsc --noEmit`、`vite build`
- SDK 与 CLI 的 TypeScript 检查、单元测试和真实 Host 生命周期
- `scripts/build-marketplace-plugins.mjs --build`
- `scripts/build-freeze-scope.mjs --verify`
