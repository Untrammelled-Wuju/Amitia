# Amitia 扩展系统重构第 58 步实施文档

## 第 58 步：实现开发模式与热重载

---

## 一、步骤目标

实现独立于正式安装链的 Extension Developer Mode，使开发者能够将本地工作区连接到 Amitia，实时构建、验证、启动、调试和热重载 Extension，而不破坏正式 Artifact、Installation、Permission、Scope 和用户数据。

本步骤目标：

> 建立 Development Workspace、Developer Trust、Dev Generation、File Watcher、Rebuild Pipeline、Runtime Reload、UI Reload、State Preservation 和错误回退体系。

---

## 二、开发模式与正式模式分离

开发模式对象：

```text
Development Workspace
```

不是正式：

```text
ExtensionInstallation
```

可以在领域层建模为：

```text
Development Installation / Development Revision
```

但必须明显区分：

-未签名正式包；
-本地路径；
-热重载；
-Source Map；
-DevTools；
-不自动更新；
-不能发布为 Trusted。

---

## 三、启用条件

开发模式必须：

-用户显式打开；
-本地桌面环境；
-开发者确认；
-显示风险；
-可随时关闭；
-不允许远程控制；
-不在普通用户默认启用。

---

## 四、Development Workspace

建议：

```go
type DevelopmentWorkspace struct {
    WorkspaceID       string
    PathReference     string
    ExtensionID       ExtensionID
    ManifestPath      string
    CurrentRevision   string
    Status            string
    WatchEnabled      bool
    AutoReload        bool
    CreatedAt         time.Time
}
```

实际路径只存宿主安全引用。

---

## 五、开发信任

Development Trust：

-仅绑定工作区；
-仅当前设备；
-仅开发模式；
-不等于 Publisher Trust；
-不允许 Service Runtime 默认放行；
-不允许关键包安全绕过；
-可撤销。

---

## 六、连接流程

```text
CLI dev
→ Developer Host Auth
→ Register Workspace
→ Validate Manifest
→ Build Dev Artifact/View
→ Create Dev Definition
→ Create Dev Generation
→ Start Runtime
→ Register Contributions
→ Enable Hot Reload
```

---

## 七、Developer Host

Amitia 提供本地受认证接口：

```text
DeveloperHost
```

负责：

-工作区注册；
-状态；
-构建结果；
-重载；
-日志；
-调试；
-调用；
-停止；
-清理。

不开放远程网络。

---

## 八、认证

CLI 与 Amitia 之间：

-本地 Pipe/Socket；
-一次性配对；
-短期 Token；
-用户确认；
-工作区绑定；
-进程绑定预留；
-过期；
-撤销。

---

## 九、文件监听

监听：

-Manifest；
-src；
-UI；
-Schema；
-resources；
-workflow；
-SKILL.md；
-WASM；
-配置。

排除：

-`node_modules`；
-构建输出；
-临时；
-`.git`；
-缓存；
-Secret；
-大目录。

---

## 十、变更分类

### 无需 Runtime Restart

-文案；
-部分 Schema UI；
-静态资源；
-本地化；
-部分 UI CSS/JS Bundle。

### 需要 Runtime Reload

-Main Runtime 代码；
-Handler；
-Host API 调用；
-Event/Hook；
-Provider。

### 需要 Definition Rebuild

-Manifest；
-Contribution；
-Permission；
-Scope；
-Dependency；
-Runtime 类型；
-Entry；
-Storage Schema。

### 需要完整重新安装计划

-Service Binary；
-不可逆 Migration；
-Extension ID；
-Publisher；
-核心包结构变化。

---

## 十一、Dev Generation

每次成功构建生成：

```text
development_revision
generation
definition_hash
build_hash
```

旧 Generation：

-停止新调用；
-Drain；
-销毁 UI Session；
-停止 Runtime；
-清理资源。

---

## 十二、原子重载

流程：

1.检测变更。
2.构建到临时目录。
3.校验。
4.创建新 Dev Definition。
5.准备新 Runtime。
6.健康检查。
7.原子切换 Contribution。
8.销毁旧 Generation。
9.失败则保留旧 Generation。

禁止先停止旧 Runtime 再尝试构建，除非类型不支持并行。

---

## 十三、状态保留

热重载默认保留：

-Storage Broker 数据；
-Secret Reference；
-Scope；
-Permission；
-用户 UI Preference。

不保留：

-内存状态；
-Pending Promise；
-UI Session；
-事件监听；
-Timer；
-Resource Handle。

---

## 十四、可选 State Transfer

第一版不支持插件任意序列化内存状态跨 Generation。

如未来支持，必须有 Schema、大小和安全限制。

---

## 十五、Tool 调用期间重载

运行中 Invocation 固定旧 Generation。

策略：

-允许完成；
-达到超时取消；
-新调用进入新 Generation；
-非幂等调用不迁移；
-旧 Runtime Drain 后停止。

---

## 十六、Event/Hook 重载

新 Generation 注册成功后原子切换。

避免：

-同一 Event 双订阅；
-同一 Hook 双执行；
-旧 Schedule 双触发。

---

## 十七、Schedule

开发模式 Schedule 默认 Disabled，除非开发者和用户显式启用。

防止保存文件时重复触发实际副作用。

---

## 十八、Permission 变化

Manifest 增加 Permission：

-热重载暂停；
-要求用户确认；
-旧 Generation 保持；
-未确认不切换；
-不自动 Grant。

---

## 十九、Scope 变化

Scope 扩大：

-同样要求确认；
-不自动应用；
-旧 Scope 保持。

---

## 二十、依赖变化

依赖新增/版本变化：

-重新 Resolve；
-缺失则构建成功但运行阻塞；
-需要安装依赖时生成开发子计划；
-不自动下载未确认依赖。

---

## 二十一、UI 热重载

### Schema UI

重新校验并替换 Schema。

### Web UI

创建新 UI Generation，销毁旧 Session，保留宿主页面位置。

### Extension Page

路由保持，内容重新加载。

### Chat Renderer

新消息使用新 Renderer；当前可见消息按安全策略重建。

---

## 二十二、Source Map

仅开发模式：

-路径映射；
-错误栈；
-源码定位；
-不上传；
-不打入生产包；
-不显示 Secret。

---

## 二十三、DevTools

Restricted Web UI 可打开 DevTools。

Plugin Main Runtime 可通过受控 Debug Protocol 或日志调试。

不开放 Electron Main DevTools 给扩展。

---

## 二十四、开发模式数据隔离

可选择：

```text
shared_dev_data
isolated_dev_data
```

默认推荐：

```text
isolated_dev_data
```

避免开发代码破坏正式 Extension 数据。

---

## 二十五、Mock Scope

开发者可模拟角色/会话，但：

-只能使用用户已有数据，需权限；
-或使用测试数据；
-不能伪造越权；
-明确标记。

---

## 二十六、错误回退

构建或校验失败：

-保留最后成功 Generation；
-显示错误；
-不更新 Registry；
-不停止旧 Runtime；
-日志关联 Build ID。

Runtime 启动失败：

-回切旧 Generation；
-清理新资源；
-显示诊断。

---

## 二十七、开发模式关闭

流程：

1.停止 Watch。
2.停止 Dev Runtime。
3.注销 Dev Contribution。
4.关闭 UI Session。
5.清理临时 Artifact。
6.保留或删除 Dev Storage 按用户选择。
7.撤销 Developer Session。
8.恢复正式 Extension。

---

## 二十八、与正式安装冲突

同 Extension ID 同时存在正式版和开发版：

-用户明确选择 Active Source；
-默认开发版覆盖运行，但正式版保留；
-状态和 UI 显示明显；
-关闭开发模式自动恢复正式版；
-不能同时注册。

---

## 二十九、前端开发状态

Extension 详情显示：

-Development；
-Workspace；
-Revision；
-构建；
-热重载；
-最后成功；
-当前 Generation；
-权限变化；
-错误；
-日志；
-关闭开发模式。

---

## 三十、持久化

建议：

```text
extension_development_workspaces
extension_development_sessions
extension_development_builds
extension_development_revisions
extension_development_reload_records
extension_development_errors
```

---

## 三十一、测试要求

覆盖：

-Workspace 注册；
-认证；
-文件监听；
-构建；
-代码重载；
-Manifest 变化；
-Permission 变化；
-Scope 变化；
-依赖变化；
-Tool 运行中；
-Event/Hook 双执行；
-Schedule；
-Schema UI；
-Web UI；
-Page；
-状态隔离；
-构建失败；
-Runtime 失败；
-关闭；
-正式版恢复；
-跨平台路径；
-大量变更；
-编辑器临时文件。

---

## 三十二、实施任务

1. 建立 Developer Host。
2.实现 CLI 配对。
3.定义 Development Workspace。
4.实现 File Watcher。
5.实现 Build Pipeline。
6.实现变更分类。
7.实现 Dev Generation。
8.实现原子 Runtime Reload。
9.实现 Contribution Switch。
10.实现 UI Reload。
11.实现权限/Scope 变化拦截。
12.实现数据隔离。
13.实现错误回退。
14.实现正式版恢复。
15.实现 Source Map/DevTools。
16.改造前端。
17.完成跨平台和副作用测试。

---

## 三十三、验收标准

1. 开发模式与正式安装分离。
2.只支持本地受认证连接。
3.每次成功构建生成 Dev Generation。
4.构建失败保留旧版本。
5.热重载不双注册 Tool/Event/Hook/Schedule。
6.运行中调用固定旧 Generation。
7.Permission/Scope 扩大需确认。
8.状态默认隔离。
9.关闭开发模式可恢复正式版。
10.开发信任不等于 Publisher Trust。
11.关键跨平台测试通过。
12.可进入第 59 步开发者控制台。

---

## 三十四、执行约束

> 开发模式提升的是迭代效率，不是安全权限；工作区代码仍必须经过 Manifest、Schema、Permission、Scope、Runtime 和 Host API 边界。

禁止：

-远程 Developer Host；
-热重载绕过校验；
-权限自动扩大；
-开发版和正式版双运行；
-构建失败停止旧版本；
-Dev Storage 默认污染正式数据；
-开发信任变正式信任；
-直接加载源码到 Electron Main。
