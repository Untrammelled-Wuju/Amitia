# Amitia 扩展系统重构：`.amitiax` 包与 Runtime 阶段索引

本阶段范围：第 29—40 步。

1. 第 29 步：定义 `.amitiax` Manifest v2
2. 第 30 步：定义 `.amitiax` 多模块包结构
3. 第 31 步：重写 `.amitiax` 解析器与校验器
4. 第 32 步：重写 `.amitiax` 安装事务
5. 第 33 步：实现签名与发布者信任体系
6. 第 34 步：实现更新、回滚与数据迁移
7. 第 35 步：确定桌面插件 Runtime 技术方案
8. 第 36 步：实现 JavaScript Main Runtime
9. 第 37 步：实现隔离 Task Runtime
10. 第 38 步：实现内部 JSON-RPC 协议
11. 第 39 步：实现 Trusted Service Runtime
12. 第 40 步：实现 WASM Runtime

## 本阶段核心决策

主插件 Runtime：

```text
独立 Node.js 子进程
+ TypeScript SDK
+ 自定义受控模块加载器
+ 内部 JSON-RPC
+ Host API Gateway
+ 每 Module 独立 Runtime Instance
```

其他 Runtime：

```text
Task Runtime：一次性任务、迁移、批处理
Trusted Service Runtime：可信原生服务
WASM Runtime：受限、可移植计算模块
```

## 完成后的主链

```text
. amitiax Package
→ Package Security
→ Manifest v2 Parser
→ ExtensionDefinition
→ Lifecycle Install/Update
→ Contribution Registry
→ Runtime Supervisor
→ JavaScript / Task / Service / WASM Runtime
→ Host API Gateway
```

下一阶段从第 41 步开始，进入 UI Contribution 协议、Schema UI、沙箱 Web UI、前端扩展槽、扩展页面宿主、聊天界面扩展与桌面扩展点。
