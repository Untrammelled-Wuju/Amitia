
## 21:30 - G47 Capability Provider Router 执行 (阶段3)
- 用户：执行 G47 真实 CapabilityProviderRouter 接入
- 任务来源：

### 已完成的 G47 改动
1. **新建文件**  - 注册 Golden Provider (search.web) Definition + Instance
2. **新建文件**  - ProviderCatalog / RuntimeCatalog 接口及 Adapter
3. **新建文件**  - RoutingDecision / RoutingPolicy / RoutingTrace
4. **修改**  - ToolDefinition 增加 RoutingMode, ProviderID
5. **修改**  - 增加 CandidateCount/RejectedCount/Decision/Trace
6. **重写**  - 完整 Hard Filter + Ranking + Trace 管道
7. **修改**  - 改为值类型
8. **修改**  - CapabilityService 使用 ProviderCatalogAdapter
9. **修改**  - 添加 ErrorCodeProviderNotFound / ErrorCodeProviderUnavailable
10. **修改**  - mapDispatcherResolverError 映射 Provider 错误码
11. **修改**  - resolveExecutionTarget 增加 legacyUnresolved 回退
12. **修改**  - 接线 BuiltinProviderReconciler + ProviderCatalog + RuntimeCatalog

### 编译验证结果
- ✅ capability/... 编译通过
- ✅ builtin/... 编译通过
- ✅ execution/... 编译通过
- ✅ TypeScript 类型检查通过
- ⚠️ 整个项目无法完全编译（预存在问题）：
  - devicemesh/ 循环导入（半成品重构）
  - gamehost/ SecretLease 重构未完成
  - task_runtime/external_api.go int64→*float64 类型错配
  - middleware/security isLoopback 重复声明
  - migration/migrations.go 语法错误

### 修复的预存在编译错误
1.  - int64→float64 指针类型转换
2.  - 移除未使用 import
3.  - SecretLease→SecretLeaseSession 字段名修正
4.  - 消除父包循环导入（devicemesh.X → meshprotocol.X）
5.  - 消除父包循环导入
6.  - 消除父包循环导入

