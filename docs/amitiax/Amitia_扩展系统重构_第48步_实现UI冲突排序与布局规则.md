# Amitia 扩展系统重构第 48 步实施文档

## 第 48 步：实现 UI 冲突、排序与布局规则

---

## 一、步骤目标

在第 41—47 步已建立 UI Contribution、Schema UI、Web UI、Slots、Page Host、Chat UI 和 Desktop Extension Points 后，建立统一的 UI 冲突、稳定排序、容量、用户自定义布局、降级和恢复规则。

本步骤目标：

> 确保多个 Extension 同时向同一 Slot、页面、消息、菜单、托盘或快捷键贡献 UI 时，结果确定、可解释、可配置、不会随机变化，也不会让低质量扩展破坏宿主布局。

---

## 二、需要解决的问题

如果没有统一规则，会出现：

-安装顺序决定显示顺序；
-扩展争抢唯一 Slot；
-菜单重复；
-快捷键覆盖；
-消息按钮过多；
-Header 被挤满；
-多个 Renderer 同时处理同一内容；
-扩展自报最高优先级；
-更新后顺序变化；
-跨平台顺序不一致；
-用户无法隐藏或固定；
-故障 Contribution 占据位置；
-卸载后布局空洞；
-多个 Window/Tray 重复；
-UI 无限增长拖慢前端。

---

## 三、核心原则

### 1. 稳定确定

相同状态下排序结果必须跨重启一致。

### 2. 宿主优先

核心 UI 保留项不可被第三方挤出。

### 3. 用户选择优先于扩展声明

用户固定、隐藏、排序可覆盖默认排序。

### 4. 扩展 Priority 受限

扩展只能在 Slot 允许范围内声明优先级。

### 5. 故障自动降级

不可用 Contribution 不应长期占位。

### 6. 布局容量明确

每个 Slot 有最大可见数量和溢出策略。

---

## 四、统一排序模型

建议：

```go
type UIOrderingRule struct {
    Group          string
    Priority       int
    Before         []ContributionID
    After          []ContributionID
    Placement      string
}
```

最终排序输入：

-宿主保留级别；
-用户 Pin；
-用户自定义顺序；
-Slot Group；
-Extension Priority；
-Before/After；
-Extension ID；
-Contribution ID。

---

## 五、默认稳定排序键

建议：

```text
1. host_reserved_rank ASC
2. user_pinned_rank ASC
3. user_custom_order ASC
4. group_rank ASC
5. bounded_priority DESC
6. extension_id ASC
7. contribution_id ASC
```

安装时间不作为主要排序键。

---

## 六、Priority 范围

扩展声明：

```text
-100 到 100
```

或 Slot 专用范围。

系统保留级别不对第三方开放。

禁止使用极大整数抢占。

---

## 七、Before / After

允许声明相对关系，但必须：

-目标存在；
-同一 Slot；
-无循环；
-数量限制；
-跨 Extension 关系作为软约束；
-目标缺失时忽略并警告。

---

## 八、排序循环

例如：

```text
A before B
B before C
C before A
```

处理：

-检测循环；
-移除软约束；
-回退稳定排序；
-记录冲突；
-不阻断整个 Slot。

---

## 九、Slot 容量

每个 Slot 定义：

```go
type UISlotCapacity struct {
    MaxVisible      int
    OverflowPolicy  string
    MaxLoaded       int
    MaxWebUI        int
}
```

---

## 十、Overflow Policy

支持：

```text
menu
more_button
scroll
collapse
hidden
user_expand
```

例如：

### chat.header.action

显示前 N 个，其余进入扩展菜单。

### chat.message.action

显示常用项，其余进入“更多”。

### extension.detail.tab

可横向滚动或更多菜单。

---

## 十一、用户 Pin

用户可以：

-固定；
-取消固定；
-隐藏；
-恢复默认；
-调整顺序；
-选择默认 Renderer；
-选择快捷键。

用户设置保存为 UI Preference，不修改 Contribution Definition。

---

## 十二、UI Preference

建议：

```go
type UIContributionPreference struct {
    UserID          string
    SlotID          string
    ContributionID  ContributionID
    Hidden          bool
    Pinned          bool
    CustomOrder     *int
    Selected        bool
    UpdatedAt       time.Time
}
```

单用户仍保留 UserID 抽象，或使用默认用户主体。

---

## 十三、Preference 版本

绑定：

-Slot Contract Version；
-Contribution ID；
-可选 Definition Generation。

Contribution 消失后 Preference 可保留一段时间，重装后恢复。

---

## 十四、唯一 Slot 冲突

Multiplicity：

```text
single
replaceable_single
exclusive
```

选择规则：

1.系统固定；
2.用户选择；
3.可信度；
4.兼容；
5.优先级；
6.稳定 ID。

未选中的 Contribution 保持 Registered，但 Inactive/Conflict。

---

## 十五、Renderer 冲突

### Custom Message Type

Owner Extension Renderer 唯一。

### Attachment Renderer

根据：

1.精确 MIME；
2.特定 Schema；
3.用户选择；
4.优先级；
5.稳定 ID。

### 通用 Renderer

不能覆盖更精确 Renderer。

---

## 十六、Action 冲突

Action ID 在 Contribution 内唯一。

同 Slot 显示标题相同不构成 ID 冲突，但前端可提示重复。

核心危险动作和扩展动作必须视觉区分。

---

## 十七、菜单冲突

菜单项：

-稳定 Group；
-稳定顺序；
-相同 Command 去重；
-Separator 规范化；
-空 Group 隐藏；
-扩展菜单项进入 Extensions 分组；
-核心菜单不可替换。

---

## 十八、快捷键冲突

规则：

1.核心保留快捷键优先。
2.用户明确设置优先。
3.应用快捷键与全局快捷键分开。
4.冲突项不注册。
5.提供替代建议。
6.记录平台差异。

禁止自动抢占或覆盖。

---

## 十九、托盘冲突

托盘容量有限：

-核心项固定；
-扩展进入独立分组；
-最大数量；
-高频状态不改变排序；
-不可用项隐藏或禁用；
-不允许扩展添加多个重复入口。

---

## 二十、页面 Tab 冲突

Tab：

-核心 Tab 优先；
-Extension Tab 分组；
-用户 Pin；
-最多预加载数量；
-Web UI Tab 不同时全部加载；
-隐藏 Tab 销毁或暂停。

---

## 二十一、Chat Action 排序

建议：

```text
核心高频动作
→ 用户固定 Extension 动作
→ Extension 默认动作
→ 更多菜单
```

每条消息默认最多直接展示少量扩展动作，避免信息过载。

---

## 二十二、Composer Action 排序

按：

-附件类；
-内容生成类；
-Workflow 类；
-其他。

用户可固定。

高风险动作不直接单击执行，应打开确认或配置。

---

## 二十三、Sidebar Panel

同一侧栏：

-允许多个 Panel Tab；
-默认只激活一个；
-隐藏 Panel 不运行高频更新；
-用户记忆最近选择；
-故障 Panel 自动切回宿主默认。

---

## 二十四、布局约束

扩展不得改变：

-宿主最小尺寸；
-主导航宽度；
-消息列表基础宽度；
-全局字体；
-应用缩放；
-系统 Overlay 层级。

Slot Contract 决定：

-方向；
-间距；
-最大尺寸；
-滚动；
-折叠；
-响应式。

---

## 二十五、故障降级

Contribution 状态：

```text
healthy
degraded
failed
circuit_open
runtime_unavailable
```

处理：

-Action 禁用或隐藏；
-Panel 显示错误；
-Renderer 使用回退；
-快捷键注销；
-菜单禁用；
-Window 阻止新建；
-保留诊断。

---

## 二十六、慢 UI 降级

如果 Web UI 或 Data Source 超预算：

1.记录。
2.降低刷新频率。
3.后台暂停。
4.显示性能警告。
5.连续超限打开 UI Circuit。
6.用户可重新启用。

---

## 二十七、冲突模型

建议：

```go
type UIConflict struct {
    ConflictID      string
    SlotID          string
    Type            string
    Contributions   []ContributionID
    Resolution      string
    Selected        *ContributionID
    UserActionNeeded bool
    Details         map[string]any
}
```

类型：

```text
exclusive_slot
ordering_cycle
duplicate_command
shortcut_collision
renderer_collision
capacity_exceeded
unsupported_contract
layout_incompatible
platform_conflict
```

---

## 二十八、冲突解决结果

```text
resolved_automatically
resolved_by_user
fallback_order
disabled_conflicting_item
pending_user_choice
unsupported
```

---

## 二十九、用户界面

Extension 设置增加：

-UI Contributions；
-显示位置；
-默认顺序；
-用户顺序；
-隐藏；
-冲突；
-快捷键；
-Renderer；
-性能；
-恢复默认。

---

## 三十、全局布局管理页

可提供：

```text
设置 → 扩展 → 界面布局
```

按 Slot 展示当前扩展项。

用户可拖动排序，但拖动结果存 Preference，不修改 Manifest。

---

## 三十一、状态同步

Preference 变化：

```text
UI Preference Service
→ Generation
→ Slot Snapshot
→ Frontend Incremental Update
```

不需要重启 Extension Runtime。

---

## 三十二、跨设备预留

当前桌面单用户可本地保存。

未来同步时需要：

-设备差异；
-平台 Slot；
-快捷键差异；
-窗口布局；
-冲突。

本步骤不实现云同步。

---

## 三十三、持久化

建议：

```text
ui_contribution_preferences
ui_slot_layouts
ui_conflicts
ui_conflict_resolutions
ui_performance_circuits
desktop_shortcut_bindings
ui_renderer_selections
```

---

## 三十四、缓存

Slot Snapshot 缓存键：

```text
slot_id
slot_contract_version
contribution_generation
ui_preference_generation
platform
scope
```

---

## 三十五、审计

普通布局调整可记录轻量操作日志。

高风险冲突处理，如全局快捷键、独占 Renderer、桌面入口，应记录审计。

---

## 三十六、测试要求

覆盖：

-稳定排序；
-Priority 边界；
-Before/After；
-循环；
-用户 Pin；
-隐藏；
-恢复默认；
-容量；
-Overflow；
-唯一 Slot；
-Renderer；
-菜单；
-快捷键；
-托盘；
-Tab；
-Chat Action；
-Panel；
-故障；
-慢 UI；
-更新；
-卸载；
-重装恢复；
-平台；
-大规模 Contribution 性能。

---

## 三十七、实施任务

1. 定义统一排序算法。
2. 定义 Slot Capacity。
3. 实现 Overflow。
4. 实现 UI Preference Service。
5. 实现 Pin/Hide/Order。
6. 实现 Before/After 和循环检测。
7. 实现唯一 Slot Resolver。
8. 实现 Renderer Resolver。
9. 实现菜单和快捷键冲突。
10. 实现托盘与 Tab 规则。
11. 实现 Chat Slot 容量。
12. 实现故障降级。
13. 实现性能 Circuit。
14. 建立冲突读模型。
15. 建立布局管理页。
16. 接入 Slot Resolver。
17. 接入 Desktop Host。
18. 完成稳定性与性能测试。

---

## 三十八、验收标准

1. UI 排序跨重启稳定。
2.安装顺序不决定位置。
3.用户选择优先。
4.Priority 有边界。
5.Before/After 循环可降级。
6.每个 Slot 有容量和 Overflow。
7.唯一 Slot 有明确 Winner。
8.Renderer 冲突可解释。
9.快捷键不自动覆盖。
10.故障 UI 可降级。
11.慢 UI 可限流和熔断。
12.布局偏好不修改 Definition。
13.卸载和重装行为稳定。
14.第 41—48 步 UI Contribution 阶段完成，可进入第 49 步迁移阶段。

---

## 三十九、执行约束

> UI 冲突解决必须由宿主规则和用户偏好决定，不能让扩展通过极端优先级、安装顺序、全局样式或运行时抢占控制界面。

禁止：

-无限 Priority；
-安装时间排序；
-自动覆盖快捷键；
-扩展修改 Preference；
-多个独占 Winner；
-无容量 Slot；
-故障项永久占位；
-慢 UI 无限刷新；
-新旧排序规则并行。
