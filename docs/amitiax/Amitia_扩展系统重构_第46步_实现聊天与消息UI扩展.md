# Amitia 扩展系统重构第 46 步实施文档

## 第 46 步：实现聊天与消息 UI 扩展

---

## 一、步骤目标

为 Amitia 最核心的聊天界面建立受控 UI Contribution 扩展点，使 Extension 可以贡献消息操作、输入区动作、侧栏面板、消息附件渲染器、状态卡片和受限自定义消息类型，但不能破坏消息主链、修改历史真值或绕过消息发送安全流程。

本步骤目标：

> 在保持聊天链路稳定、单用户多角色隔离、消息数据最小暴露和模型执行安全的前提下，建立 Chat UI Extension Contract。

---

## 二、允许的 Chat Slot

建议第一阶段开放：

```text
chat.header.action
chat.sidebar.panel
chat.message.action
chat.message.badge
chat.message.attachment_renderer
chat.message.custom_renderer
chat.composer.action
chat.composer.attachment
chat.composer.hint
chat.empty_state.card
chat.status.item
```

暂不开放：

-替换整个聊天列表；
-替换整个输入框；
-替换消息排序；
-接管消息发送；
-接管流式渲染主链；
-覆盖角色选择器；
-直接修改消息数据库。

---

## 三、核心原则

### 1. 消息真值由宿主管理

Extension UI 不能修改已持久化消息内容。

需要更正、标记或派生信息时使用：

-Annotation；
-Badge；
-Extension Metadata；
-新消息；
-宿主提供的编辑命令。

### 2. 消息发送必须走宿主

Composer Extension 只能提交：

```text
Composer Action Request
```

最终发送仍经过 Amitia Message Pipeline。

### 3. 数据最小化

默认只提供 Message Summary。

正文、附件和上下文按 Permission 与 Action 读取。

### 4. 多角色隔离

所有 Chat Contribution 绑定 Character/Conversation Scope。

---

## 四、Chat UI Context

建议基础：

```ts
interface ChatUIContext {
  characterId: string;
  conversationId: string;
  channel: string;
  platform: string;
  conversationState: "idle" | "generating" | "offline";
  capabilities: string[];
}
```

---

## 五、Message Summary

```ts
interface MessageSummary {
  messageId: string;
  type: string;
  direction: "incoming" | "outgoing" | "system";
  senderType: "user" | "character" | "system" | "extension";
  createdAt: string;
  status: string;
  hasText: boolean;
  attachmentTypes: string[];
  extensionType?: string;
}
```

默认不含正文。

---

## 六、读取正文

需要：

```text
chat.message.content.read
```

并通过 Data Request：

```text
UI → Host Bridge → Message Query Service
```

返回：

-必要字段；
-脱敏；
-大小限制；
-当前 Message；
-当前 Scope。

---

## 七、Message Action

示例：

-复制结构化内容；
-添加标签；
-创建任务；
-发送到 Workflow；
-翻译；
-生成摘要；
-导出；
-打开相关资源。

定义：

```go
type ChatMessageActionSpec struct {
    ActionID       string
    SupportedTypes []string
    Position       string
    RequiresContent bool
    Confirmation   string
    Target         UIActionTarget
}
```

---

## 八、Action 执行

```text
Click
→ Validate Message still exists
→ Scope
→ Permission
→ Load allowed content
→ Execute Tool/Workflow/Runtime
→ Show Result
```

Extension 不获得长期消息对象引用。

---

## 九、Message Badge

只读展示：

-分类；
-处理状态；
-来源；
-风险；
-扩展标记；
-Workflow 结果。

Badge 不改变消息状态。

---

## 十、Attachment Renderer

允许为声明的 MIME/Attachment Type 提供渲染器。

流程：

```text
Attachment Metadata
→ Renderer Selection
→ Permission
→ Resource Handle
→ Schema/Web Renderer
```

不得直接获得真实文件路径。

---

## 十一、Custom Message Type

扩展可声明：

```text
extension.<extension-id>.<message-type>
```

Custom Message 必须有：

-Payload Schema；
-Version；
-文本回退；
-导出回退；
-无扩展时的安全占位；
-最大大小；
-资源引用；
-渲染器。

---

## 十二、自定义消息持久化

数据库保存：

-Message 基础字段；
-Extension Message Type；
-Payload Version；
-结构化 Payload；
-资源引用；
-文本回退；
-Owner Extension；
-Definition Version。

不能保存 UI Component 或 HTML。

---

## 十三、Extension 卸载后的消息

历史 Custom Message 必须仍可显示：

-文本回退；
-通用 JSON 摘要；
-缺少扩展提示；
-附件仍按 Owner Policy 处理。

不得删除聊天历史。

---

## 十四、消息渲染冲突

同一 Custom Type 只能有 Owner Extension 的正式 Renderer。

普通 Attachment Renderer 可多个候选，按 MIME 精确度和优先级选择。

不得让其他扩展覆盖他人自定义消息类型。

---

## 十五、Composer Action

示例：

-选择扩展资源；
-插入模板；
-创建投票；
-生成附件；
-调用 Workflow；
-添加元数据；
-语音/图片处理。

Composer Action 不得直接发送消息。

---

## 十六、Composer Draft

扩展可请求：

```text
insert_text
replace_selection
attach_resource
set_extension_metadata
open_dialog
```

宿主执行并显示结果。

---

## 十七、Draft Permission

读取当前 Draft：

```text
chat.composer.draft.read
```

修改：

```text
chat.composer.draft.write
```

发送：

```text
chat.message.send
```

必须分离。

---

## 十八、发送主链

Extension 请求发送时：

```text
UI Action
→ Composer Command
→ User Confirmation where needed
→ Message Validation
→ Channel Rules
→ Attachment Validation
→ Message Persist
→ Delivery
→ Audit
```

不能绕过连续消息合并、生成中限制、渠道能力和消息状态规则。

---

## 十九、Sidebar Panel

可展示：

-对话相关工具；
-角色相关数据；
-扩展状态；
-资源；
-Workflow；
-摘要；
-诊断。

Panel 必须：

-按 Conversation Scope；
-切换会话清理；
-隐藏时暂停；
-宽度由宿主控制；
-不覆盖聊天主区。

---

## 二十、Header Action

适合：

-打开扩展页面；
-运行当前会话 Workflow；
-切换扩展模式；
-查看状态。

数量受限，溢出进入扩展菜单。

---

## 二十一、Chat Status Item

可显示：

-MCP 连接；
-扩展处理；
-后台任务；
-上传；
-翻译状态。

不得伪造核心消息发送状态。

---

## 二十二、流式消息

Extension 不得直接修改模型流式 Token。

可订阅受限事件：

```text
message.generation.started
message.generation.completed
```

是否开放增量 Token 需要单独高频协议，第一阶段不开放给第三方 UI。

---

## 二十三、消息 Hook 与 UI 分离

消息发送前后逻辑使用 Hook Pipeline。

消息按钮和渲染使用 UI Contribution。

禁止用 UI 组件隐式修改发送数据。

---

## 二十四、Channel 差异

微信、QQ、Web 等渠道支持能力不同。

Chat Context 提供：

```text
capabilities
```

例如：

-文本；
-图片；
-文件；
-语音；
-回复；
-撤回；
-富消息。

Extension 必须按能力显示。

---

## 二十五、Extension Metadata

消息可附带受控扩展元数据：

```text
extensions.<extension-id>.<key>
```

大小、Schema、Owner 和可见性受限。

不得覆盖核心 Message 字段。

---

## 二十六、隐私与敏感内容

读取消息正文、附件、角色信息和记忆属于敏感能力。

要求：

-明确 Permission；
-Scope；
-最小读取；
-审计；
-不跨会话；
-不发送到网络，除非另有网络 Permission；
-UI 不缓存全文。

---

## 二十七、前端实现

建议组件：

```text
ChatExtensionSlotHost
MessageActionHost
MessageBadgeHost
AttachmentRendererHost
CustomMessageRendererHost
ComposerActionHost
ChatSidebarExtensionHost
ChatExtensionStatusHost
```

---

## 二十八、渲染性能

聊天列表是高频区域，必须限制：

-每条消息 Contribution 查询；
-渲染器数量；
-Web UI 数量；
-同步计算；
-Data Request；
-观察器；
-内存。

建议后端或前端预计算：

```text
message UI contribution snapshot
```

---

## 二十九、虚拟列表

Extension Renderer 必须兼容消息虚拟列表：

-可测量高度；
-Resize 请求受限；
-销毁重建；
-无全局定位；
-不假设 DOM 永久存在。

---

## 三十、错误处理

单个消息 Renderer 失败：

-显示文本回退；
-记录；
-不影响其他消息；
-可禁用 Renderer；
-不循环重试。

Composer Action 失败：

-保留 Draft；
-显示错误；
-不发送部分消息。

---

## 三十一、卸载与更新

更新：

-旧 Renderer Session 销毁；
-消息历史不变；
-新 Generation 重载；
-Payload Version 兼容检查。

卸载：

-移除 Action/Panel；
-历史 Custom Message 使用回退；
-Extension Metadata 保留或按策略；
-资源不误删。

---

## 三十二、测试要求

覆盖：

-全部 Chat Slot；
-Message Summary；
-正文权限；
-Action；
-Badge；
-Attachment；
-Custom Type；
-卸载回退；
-Composer；
-Draft；
-发送请求；
-Channel Capability；
-角色/会话切换；
-隐私；
-虚拟列表；
-大量消息；
-Web Renderer；
-错误；
-Runtime Crash；
-更新；
-性能。

---

## 三十三、实施任务

1. 定义 Chat Slot Contract。
2. 定义 Message Summary。
3. 实现 Message Data Request。
4. 实现 Message Action Host。
5. 实现 Badge Host。
6. 实现 Attachment Renderer。
7. 定义 Custom Message Type。
8. 实现文本回退。
9. 实现 Composer Action。
10. 实现 Draft Command。
11. 接入消息发送主链。
12. 实现 Sidebar/Header/Status。
13. 接入 Channel Capability。
14. 接入 Scope/Permission。
15. 实现更新/卸载兼容。
16. 优化虚拟列表与批量解析。
17. 迁移现有硬编码消息扩展。
18. 完成隐私和性能测试。

---

## 三十四、验收标准

1. Chat 扩展使用正式 Slot。
2. 扩展不能修改消息真值。
3.正文按权限读取。
4. Composer Action 不直接发送。
5.发送走宿主主链。
6. Custom Message 有 Schema 和文本回退。
7.卸载不破坏历史。
8.附件使用 Resource Handle。
9.多角色和会话隔离。
10.兼容虚拟列表。
11.流式 Token 不被第三方接管。
12.关键隐私和性能测试通过。
13.可进入第 47 步桌面扩展点。

---

## 三十五、执行约束

> 聊天 UI 扩展只能增强展示和发起受控动作，不能接管消息存储、发送、排序、流式生成或渠道投递主链。

禁止：

-直接改 Message；
-直接发送；
-直接访问全文；
-直接文件路径；
-覆盖核心消息类型；
-无文本回退；
-跨角色缓存；
-每条消息启动独立 Main Runtime；
-隐藏扩展网络上传。
