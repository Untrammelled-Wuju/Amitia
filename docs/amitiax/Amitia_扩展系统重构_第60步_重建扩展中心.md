# Amitia 扩展系统重构第 60 步实施文档

## 第 60 步：重建扩展中心

---

## 一、步骤目标

在 Extension Kernel、`.amitiax`、Runtime、UI Contribution、正式迁移、SDK、CLI 和 Developer Console 已完成后，重建 Amitia Extension Center。

本步骤目标是：

> 将原有分散的 Skill、Plugin、Agent Skill、MCP、Workflow、Package、扩展工坊和管理入口，统一为一个以 Extension 为核心、Contribution 为能力展示、Lifecycle Plan 为操作入口、Trust/Permission/Scope 为安全信息的扩展中心。

扩展中心是用户侧统一管理界面，不再保留多套安装与启停中心。

---

## 二、核心信息架构

建议主导航：

```text
已安装
发现
本地导入
开发中
更新
需要处理
设置
```

其中“发现”可在无在线市场时先展示：

-官方内置；
-本地推荐；
-已下载包；
-未来市场接口占位。

---

## 三、扩展中心对象

首页卡片对象必须是：

```text
Extension
```

不是：

-Tool；
-Skill；
-MCP Tool；
-Workflow；
-Plugin Handler。

卡片可汇总 Contribution 类型。

---

## 四、卡片信息

建议显示：

-Icon；
-名称；
-说明；
-版本；
-Publisher；
-Trust；
-安装状态；
-Enabled；
-更新；
-主要 Contribution 标签；
-平台；
-故障；
-权限风险；
-开发模式；
-用户修改；
-来源。

---

## 五、Contribution 标签

标签可包括：

```text
Tools
Agent Skills
Workflows
MCP
Providers
UI
Desktop
Tasks
Hooks
Events
```

用于理解扩展能力，不成为独立安装对象。

---

## 六、状态模型

卡片必须区分：

```text
未安装
安装中
已安装且启用
已安装但禁用
部分可用
运行故障
需要权限
依赖缺失
不兼容
已隔离
更新中
回滚中
开发中
迁移待处理
```

不得将所有异常都显示成“未启用”。

---

## 七、已安装页

支持筛选：

-全部；
-启用；
-禁用；
-故障；
-需要权限；
-不兼容；
-有更新；
-开发模式；
-系统；
-用户；
-未知发布者；
-有 UI；
-有 MCP；
-有 Tool；
-有 Workflow。

---

## 八、搜索

搜索范围：

-名称；
-描述；
-Publisher；
-Extension ID；
-关键词；
-Contribution；
-Tool 名；
-Agent Skill；
-Workflow；
-MCP；
-权限；
-类别。

使用本地索引，不在每次输入时扫描全部 Artifact。

---

## 九、排序

支持：

-最近使用；
-最近安装；
-名称；
-更新；
-故障；
-风险；
-类型；
-Publisher。

默认排序应稳定。

---

## 十、分类

用户可见分类：

```text
AI 能力
效率工具
工作流
连接与 MCP
角色与聊天
媒体
桌面增强
界面扩展
开发工具
系统组件
```

分类是展示标签，后端仍使用统一 Extension 模型。

---

## 十一、本地导入

流程：

```text
选择 .amitiax
→ Package Security
→ Parse
→ Extension Preview
→ Trust
→ Permission
→ Scope
→ Dependency
→ Lifecycle Plan
→ Confirmation
→ Install
```

前端不得直接上传后显示成功。

---

## 十二、拖拽导入

支持拖入 `.amitiax`，但：

-文件类型校验；
-不自动安装；
-只进入 Preview；
-显示 Hash；
-显示来源；
-禁止拖入目录直接运行；
-禁止远程 URL 自动下载。

---

## 十三、安装预览

必须展示：

-Extension；
-Version；
-Publisher；
-Signature；
-Trust；
-Modules；
-Contributions；
-Runtimes；
-Permissions；
-Scope；
-Dependencies；
-Native Service；
-MCP Command；
-UI；
-Background Task；
-Storage；
-数据迁移；
-平台；
-风险。

---

## 十四、安装操作

所有操作调用 Lifecycle Manager：

```text
Plan
→ Confirmation
→ Execute
→ Progress
→ Result
```

扩展中心不得直接：

-复制文件；
-启动 Runtime；
-连接 MCP；
-写 Enabled；
-注册 Tool。

---

## 十五、更新页

展示：

-当前/目标版本；
-发布者连续性；
-签名；
-Diff；
-权限变化；
-Scope；
-依赖；
-数据迁移；
-回滚能力；
-用户资产；
-自动更新资格。

---

## 十六、需要处理页

集中展示：

-权限待确认；
-Scope 待绑定；
-依赖缺失；
-发布者变化；
-签名问题；
-迁移冲突；
-用户资产冲突；
-Runtime Quarantine；
-资源泄漏；
-旧系统残留；
-更新失败；
-恢复操作。

---

## 十七、开发中页

展示 Development Workspace：

-路径摘要；
-Revision；
-构建；
-热重载；
-错误；
-Runtime；
-权限变化；
-关闭开发模式；
-打开 Developer Console。

---

## 十八、系统扩展

系统 Extension：

-标记系统；
-可能不可卸载；
-仍可查看 Contribution、权限和运行状态；
-部分 Module 或 Contribution 可禁用；
-关键 Tool 有保护策略。

---

## 十九、用户 Synthetic Extension

用户导入 Agent Skill、Workflow、MCP 显示为正式 Extension。

可使用更简化卡片，但不回到独立管理中心。

---

## 二十、扩展工坊定位

原“扩展工坊”如果保留，应成为：

```text
创建与编辑 Extension 的用户界面
```

而不是另一套扩展管理中心。

工坊输出：

-Manifest；
-Agent Skill；
-Workflow；
-MCP 配置；
-资源；
-`.amitiax`。

---

## 二十一、权限摘要

卡片只显示风险摘要：

```text
无特殊权限
网络访问
消息读取
文件写入
桌面快捷键
原生服务
高风险控制
```

详情页展示完整 Requirement 和 Grant。

---

## 二十二、Scope 摘要

显示：

```text
全局
指定角色
指定会话
尚未绑定
```

不得将 Scope 当 Enabled 开关。

---

## 二十三、依赖摘要

显示：

-已满足；
-缺失；
-冲突；
-共享；
-需要安装；
-需要更新；
-受影响扩展。

---

## 二十四、Runtime 摘要

显示：

-无代码；
-JavaScript；
-Task；
-MCP；
-WASM；
-Trusted Service；
-Legacy Go；
-运行状态；
-Health；
-Circuit。

---

## 二十五、故障反馈

卡片故障动作：

-查看原因；
-重试；
-Repair Plan；
-打开日志；
-打开 Developer Console；
-禁用；
-回滚。

不提供“清空所有状态再试”这种破坏性默认操作。

---

## 二十六、批量操作

允许低风险批量：

-检查更新；
-启用/禁用选中扩展；
-导出诊断；
-清理无引用 Cache。

高风险批量安装、更新、卸载必须生成联合 Plan，并限制并发。

---

## 二十七、离线设计

Extension Center 首先保证本地可用：

-已安装；
-导入；
-更新包；
-信任；
-诊断；
-开发模式。

未来在线市场不可用不影响本地管理。

---

## 二十八、数据来源

扩展中心使用统一 Read Model：

```text
ExtensionCenterReadModel
```

聚合：

-Definition；
-Installation；
-Effective State；
-Runtime；
-Contribution；
-Trust；
-Permission；
-Scope；
-Dependency；
-Update；
-Migration。

不得前端分别调用十几个旧 API 拼接状态。

---

## 二十九、实时更新

生命周期、Runtime 和 Permission 状态通过统一事件增量更新。

需要：

-Generation；
-去重；
-断线恢复；
-页面不可见暂停；
-避免全量刷新。

---

## 三十、路由

建议：

```text
/extensions
/extensions/installed
/extensions/discover
/extensions/import
/extensions/development
/extensions/updates
/extensions/action-required
/extensions/:extensionId
```

旧 Skill、MCP、Workflow 管理路由逐步跳转到新中心对应筛选或详情。

---

## 三十一、UI 风格

要求：

-与 Amitia 亮/暗主题一致；
-轻毛玻璃仅用于必要层次；
-不滥用 Box Shadow；
-信息密度可控；
-风险状态明确；
-系统/第三方/开发区分；
-不使用过多彩色 Badge；
-支持键盘和缩放。

---

## 三十二、空状态

分别设计：

-无扩展；
-无更新；
-无故障；
-无开发工作区；
-搜索无结果；
-离线无发现内容；
-迁移完成。

---

## 三十三、迁移旧页面

旧：

-Plugin Center；
-Skill 页面；
-Agent Skill 页面；
-MCP 页面；
-Workflow 页面；
-Package 页面；
-扩展工坊安装列表。

迁移为：

-Extension Center 筛选；
-Extension Detail Tab；
-独立业务编辑页；
-兼容跳转。

---

## 三十四、兼容跳转

例如：

```text
/skills/:id
→ /extensions/:extensionId?tab=agent-skills&focus=<id>
```

保留一段版本周期，并统计访问。

---

## 三十五、性能

-虚拟列表；
-分页；
-本地索引；
-图片懒加载；
-状态批量；
-避免卡片加载完整 Definition；
-更新检查批量；
-搜索防抖；
-大规模 1000+ Contribution 测试。

---

## 三十六、安全

扩展中心展示的数据必须脱敏。

不得显示：

-Secret；
-真实敏感路径；
-OAuth Token；
-完整消息；
-系统 Prompt；
-内部 SQL。

导入文件预览不能执行任何代码。

---

## 三十七、测试要求

覆盖：

-已安装；
-筛选；
-搜索；
-排序；
-状态；
-本地导入；
-拖拽；
-安装 Plan；
-更新；
-需要处理；
-开发中；
-系统 Extension；
-Synthetic；
-权限；
-Scope；
-依赖；
-Runtime；
-故障；
-批量；
-离线；
-实时更新；
-旧路由；
-主题；
-无障碍；
-大量数据；
-恶意包预览。

---

## 三十八、实施任务

1. 定义 ExtensionCenterReadModel。
2.建立统一后端查询接口。
3.实现主页和导航。
4.实现已安装筛选。
5.实现搜索和索引。
6.实现本地导入。
7.实现安装预览。
8.接入 Lifecycle Plan。
9.实现更新页。
10.实现需要处理页。
11.实现开发中页。
12.实现系统/Synthetic 展示。
13.实现批量操作。
14.实现实时状态。
15.迁移旧页面和路由。
16.完成主题、无障碍和性能测试。
17.输出旧入口删除清单。

---

## 三十九、验收标准

1. 扩展管理只有一个中心。
2.卡片对象是 Extension。
3.Tool/Skill/MCP/Workflow 作为 Contribution 展示。
4.导入只进入 Preview。
5.安装更新卸载均走 Lifecycle Manager。
6.权限、Scope、依赖和 Runtime 可解释。
7.故障与 Enabled 分离。
8.系统和开发扩展有明确标识。
9.旧管理页可跳转并停止新增功能。
10.离线可管理本地扩展。
11.大量扩展性能通过。
12.可进入第 61 步扩展详情页。

---

## 四十、执行约束

> Extension Center 是统一管理和发现入口，不得重新为 Skill、MCP、Workflow、Plugin 建立彼此独立的安装、启停和更新中心。

禁止：

-前端拼多套状态；
-导入即执行；
-卡片直接连接 MCP；
-卡片直接写 Enabled；
-隐藏权限变化；
-在线市场成为本地管理硬依赖；
-旧页面继续新增管理逻辑；
-Secret 进入 UI。
