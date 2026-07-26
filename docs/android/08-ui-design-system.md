# Amitia Android 设计系统（Design System）

> 文档编号：08
> 适用范围：Amitia Android 客户端（com.amitia.android）及其衍生多端 UI
> 维护团队：Amitia Android 工程组
> 关联代码：`android/core/src/main/java/com/amitia/core/designsystem/`
> 版本：v0.1.0 · 状态：Phase 1 工程骨架基线

---

## 1. 设计哲学

Amitia 是一款面向长时段陪伴的本地化 AI 运行时客户端，UI 不应抢夺用户对内容本身的注意力。Design System 的核心命题是「让运行时与对话成为主角，让外壳退到背景里」。

### 1.1 设计关键词

| 关键词 | 含义 | 落地方式 |
|--------|------|----------|
| 克制 | 视觉元素不喧哗，颜色、形状、动效都为内容服务 | 禁止高饱和强对比的装饰性元素，主色仅用于必要焦点 |
| 安静 | 界面默认处于「静音」状态，不通过闪烁/脉冲制造焦虑 | 默认无自动动画，状态变化使用最短必要的过渡 |
| 沉浸 | 用户进入对话或运行时状态时，UI 框架尽量消失 | 全屏沉浸式布局，edge-to-edge，毛玻璃遮罩而非实色块 |
| 陪伴感 | 长时间使用不疲劳，色彩与排版稳定可预期 | 单一主色 + 单一辅色，状态色低饱和，文字层级清晰 |
| 低饱和 | 整体色温偏冷、灰阶为主，避免任何荧光感 | 全部颜色 L 值控制在低饱和区间，禁用纯饱和原色 |
| 深色优先 | 以深色为默认主题，亮色为辅助主题 | Material3 darkColorScheme 为基线，亮色仅作降级 |
| 轻微毛玻璃 | 关键浮层使用毛玻璃而非实色，营造层次 | 半透明 + 模糊 + 1px 高光边，仅用于浮层/导航/弹窗 |
| 清晰层级 | z 轴层级靠 elevation 与 surfaceVariant 表达 | 严格遵循 Material3 surface 层级，不滥用阴影 |
| 移动端原生 | 遵循 Android Material 3 规范，不引入 iOS 控件 | 使用 Material3 组件库，禁用 Cupertino 风格 |
| 少量高质量动效 | 动效用于状态过渡与因果解释，不做装饰 | 单次过渡 200-300ms，使用 FastOutSlowInEasing |

### 1.2 设计禁止项

| 禁止项 | 原因 |
|--------|------|
| 大面积蓝紫渐变 | 与 AI 同质化，违背低饱和原则 |
| 默认 AI 紫主色 | 行业审美疲劳，且与运行时无语义关联 |
| 大量功能卡片堆叠首页 | 制造信息过载，违背克制与陪伴感 |
| 大量发光边框 | 视觉噪声，长时间使用疲劳 |
| 大量阴影 | Material3 已用 surface 层级替代阴影，过度阴影显廉价 |
| 全胶囊按钮 | 破坏排版的水平基线，仅用于次要 chip |
| 首页工具宫格 | 桌面端范式，移动端应通过导航+对话触达能力 |
| 复制桌面 Web 排版 | 移动端信息密度与交互范式不同，单列优先 |
| 无意义粒子/光效背景 | 装饰性元素抢夺注意力，违背安静原则 |
| 复杂动画掩盖加载 | 应明确告知加载进度，不用动画欺骗用户 |

---

## 2. 颜色 Token 表

颜色 token 全部声明于 `core/designsystem/Color.kt` 的 `AmitiaColors` object。所有 hex 已在生产深色面板上验证可读性，禁止在 UI 中直接使用裸 hex 值，必须通过 token 引用。

### 2.1 背景与表面层

| Token | Hex | 用途 |
|-------|-----|------|
| `Background` | `#0F1115` | 应用最底层背景，对话/列表容器底色 |
| `BackgroundElevated` | `#13161C` | 略抬升的背景，用于全屏沉浸页 |
| `Surface` | `#1A1D24` | 卡片、对话框、底部表表面 |
| `SurfaceVariant` | `#222631` | 次级表面，输入框、二级卡片 |
| `SurfaceContainer` | `#1F232B` | Material3 surfaceContainer，列表项分组容器 |
| `SurfaceContainerHigh` | `#262B35` | 高层级表面，弹出菜单、下拉 |
| `OnBackground` | `#E4E6EB` | 背景上的主文字色 |
| `OnSurface` | `#D5D8DF` | 表面上的主文字色 |
| `OnSurfaceVariant` | `#9CA1AB` | 表面次级文字、辅助说明 |
| `OnSurfaceMuted` | `#6B717C` | 占位符、禁用态、时间戳 |

### 2.2 主色与辅色

| Token | Hex | 用途 |
|-------|-----|------|
| `Primary` | `#8A9BB0` | 灰蓝主色，焦点按钮、激活态、选中态 |
| `OnPrimary` | `#11141A` | 主色上的文字 |
| `PrimaryContainer` | `#2B3340` | 主色容器，激活 chip 背景 |
| `OnPrimaryContainer` | `#B6C4D6` | 主色容器上的文字 |
| `Secondary` | `#B0A995` | 暖灰辅色，次要按钮、标签 |
| `OnSecondary` | `#1A1812` | 辅色上的文字 |
| `SecondaryContainer` | `#332F25` | 辅色容器 |
| `OnSecondaryContainer` | `#D6CFBA` | 辅色容器上的文字 |
| `Tertiary` | `#8FA8A0` | 青灰第三色，记忆/运行时相关 |
| `OnTertiary` | `#0F1714` | 第三色上的文字 |
| `TertiaryContainer` | `#1F2C27` | 第三色容器 |
| `OnTertiaryContainer` | `#B5CCC3` | 第三色容器上的文字 |

### 2.3 状态色

| Token | Hex | 用途 |
|-------|-----|------|
| `StateRunning` | `#8FA8A0` | 运行时 Running 态指示 |
| `StateDegraded` | `#B0A995` | 运行时 Degraded 态指示 |
| `StateFailed` | `#B08585` | 运行时 Failed 态指示 |
| `StateInstalling` | `#8A9BB0` | 运行时 Installing/Updating 态指示 |
| `StateIdle` | `#6B717C` | 运行时 Idle/Stopped 态指示 |

### 2.4 错误与边界

| Token | Hex | 用途 |
|-------|-----|------|
| `Error` | `#B08585` | 错误文字、危险动作按钮 |
| `OnError` | `#1A0E0E` | 错误色上的文字 |
| `ErrorContainer` | `#321818` | 错误容器背景 |
| `OnErrorContainer` | `#D7A9A9` | 错误容器上的文字 |
| `Outline` | `#3A3F4A` | 输入框边框、分割强调 |
| `OutlineVariant` | `#262A33` | 弱化边框、卡片描边 |
| `Divider` | `#222631` | 列表分割线 |
| `Border` | `#2A2E38` | 通用边框 |

### 2.5 浮层与毛玻璃

| Token | Hex | 用途 |
|-------|-----|------|
| `Scrim` | `#000000CC` | 模态遮罩，80% 黑 |
| `Overlay` | `#0F111599` | 浮层叠加，60% 背景 |
| `GlassTint` | `#1A1D24` | 毛玻璃组件底色（配合 blur） |
| `GlassBorder` | `#FFFFFF33` | 毛玻璃 1px 高光边，20% 白 |

### 2.6 颜色使用约束

1. 主色 `Primary` 仅用于：当前焦点按钮、激活态导航、选中态指示、进度条。不可用于大面积背景。
2. 辅色 `Secondary` 仅用于：次要按钮、角色/记忆标签、可识别的次要分组。
3. 状态色仅用于：运行时状态指示点、状态徽标。禁止用于按钮或文字主体。
4. 毛玻璃 `GlassTint`/`GlassBorder` 仅用于：底部导航、顶部 AppBar 浮动状态、模态弹窗。普通卡片禁用。
5. 任何颜色不得与默认 Material3 紫（`#6750A4`）混用。

---

## 3. 字体阶梯

字体定义于 `core/designsystem/Type.kt` 的 `AmitiaTypography`，使用系统默认字体（`FontFamily.Default`），不内置自定义字体文件以减小包体并保持各 OEM 一致性。

| 角色 | Token | 字号 / 行高 / 字重 | 用途 |
|------|-------|---------------------|------|
| 显示 | `displayLarge` | 57 / 64 / Light | 启动页品牌名（仅一处） |
| 显示 | `displayMedium` | 45 / 52 / Light | 大数字展示（如运行时长） |
| 显示 | `displaySmall` | 36 / 44 / Normal | 空状态主标题 |
| 标题 | `headlineLarge` | 32 / 40 / Medium | 错误页主标题 |
| 标题 | `headlineMedium` | 28 / 36 / Medium | 设置组标题 |
| 标题 | `headlineSmall` | 24 / 32 / Medium | 对话页顶部标题 |
| 标题 | `titleLarge` | 22 / 28 / Medium | 详情页标题 |
| 标题 | `titleMedium` | 16 / 24 / Medium | 卡片标题、列表项主文字 |
| 标题 | `titleSmall` | 14 / 20 / Medium | 段落小标题 |
| 正文 | `bodyLarge` | 16 / 24 / Normal | 对话气泡正文、段落 |
| 正文 | `bodyMedium` | 14 / 20 / Normal | 列表项描述、卡片正文 |
| 正文 | `bodySmall` | 12 / 16 / Normal | 辅助说明、时间戳 |
| 标签 | `labelLarge` | 14 / 20 / Medium | 按钮文字 |
| 标签 | `labelMedium` | 12 / 16 / Medium | Chip、Tab 文字 |
| 标签 | `labelSmall` | 11 / 16 / Medium | 状态徽标、超小标签 |

### 字体使用约束

- 同一界面同时出现的字号不超过 4 种。
- 主标题统一使用 `titleLarge` 或 `headlineSmall`，禁止在普通页面使用 `display*`。
- 数字与单位之间使用窄空格（`\u202F`）而非普通空格。
- 中文不使用斜体；英文仅在引用文本时使用斜体。

---

## 4. 间距栅格

采用 4dp 基础栅格，所有间距必须是 4 的整数倍。

| Token | 值 | 用途 |
|-------|----|------|
| `spacingNone` | 0dp | 无间距 |
| `spacingXs` | 4dp | 紧凑内间距，icon 与文字之间 |
| `spacingSm` | 8dp | 列表项内部、Chip 内部 |
| `spacingMd` | 12dp | 卡片内边距、表单项之间 |
| `spacingLg` | 16dp | 标准内边距、段落之间 |
| `spacingXl` | 24dp | 区块之间、卡片之间 |
| `spacingXxl` | 32dp | 大区块分隔 |
| `spacingHuge` | 48dp | 页面顶部/底部留白 |
| `spacingMassive` | 64dp | 启动页/空状态中心留白 |

### 屏幕边距

- 横屏：左右 24dp
- 竖屏：左右 16dp（< 600dp 屏宽）/ 24dp（≥ 600dp 屏宽）
- 顶部状态栏下方统一 0dp，由 edge-to-edge 处理
- 底部导航栏下方 0dp，由 navigationBarsPadding 处理

### 圆角

| Token | 值 | 用途 |
|-------|----|------|
| `radiusXs` | 4dp | Chip、小标签 |
| `radiusSm` | 8dp | 输入框、小按钮 |
| `radiusMd` | 12dp | 卡片、对话气泡 |
| `radiusLg` | 16dp | 大卡片、底部表顶部 |
| `radiusXl` | 24dp | 模态弹窗 |
| `radiusFull` | 9999dp | 仅用于 FAB 与头像 |

按钮圆角统一使用 `radiusSm`（8dp），不全胶囊化。

---

## 5. 五大导航结构

Amitia Android 采用底部导航 + 二级页面的两级结构，共 5 个一级入口。底部导航栏使用毛玻璃材质，固定 5 个 Tab。

### 5.1 导航树

```
Amitia
├── 首页（Home）
│   ├── 最近对话
│   ├── 运行时状态卡（折叠态）
│   └── 快速入口（→ 对话/角色）
├── 对话（Chat）
│   ├── 会话列表（侧滑抽屉）
│   ├── 当前会话
│   └── 输入区（含语音/附件）
├── 角色（Character）
│   ├── 角色列表
│   ├── 角色详情
│   └── 角色编辑
├── 能力（Capability）
│   ├── 模型管理（Models）
│   ├── 记忆库（Memory）
│   └── 渠道绑定（Channels）
└── 设置（Settings）
    ├── 通用（主题/语言）
    ├── 运行时（Runtime 详情/日志）
    ├── 通知
    └── 关于
```

### 5.2 各 Tab 定位

| Tab | 名称 | 图标 | 主色 | 定位 |
|-----|------|------|------|------|
| 1 | 首页 | `Icons.Outlined.Home` | Primary | 信息聚合入口，快速触达最近活动 |
| 2 | 对话 | `Icons.AutoMirrored.Outlined.Chat` | Primary | 核心交互场景，长会话沉浸 |
| 3 | 角色 | `Icons.Outlined.PersonOutline` | Secondary | 角色配置与切换 |
| 4 | 能力 | `Icons.Outlined.Tune` | Tertiary | 模型/记忆/渠道的能力中枢 |
| 5 | 设置 | `Icons.Outlined.Settings` | OnSurfaceVariant | 系统配置与运行时管理 |

### 5.3 导航交互约束

- 底部导航固定 5 项，不支持滚动。
- 选中态使用 `Primary` 着色图标与文字，未选中态使用 `OnSurfaceVariant`。
- Tab 切换不使用过渡动画，立即响应。
- 双击当前 Tab 触发「回到顶部」行为。
- 长按当前 Tab 触发快速切换菜单（Phase 3 实现）。

---

## 6. 组件原则

### 6.1 通用原则

1. **优先使用 Material3 原生组件**：不重复造轮子，自定义组件必须能在 Material3 体系内找到对应语义。
2. **状态优先于样式**：每个组件必须显式声明 `enabled`、`loading`、`error` 三态。
3. **可访问性**：所有可点击区域不小于 48dp × 48dp，所有文字与背景对比度 ≥ 4.5:1。
4. **Compose 优先**：禁止混用 View 体系，所有组件用 Compose 实现。
5. **无注释**：源码中不写注释，组件语义通过命名表达。

### 6.2 关键组件规范

| 组件 | 规范 |
|------|------|
| 按钮 | 主按钮填充 `Primary`，次按钮描边 `Outline`，文字按钮仅文字。圆角 8dp，高度 44dp |
| 卡片 | `Card` 使用 `SurfaceContainer` 填充，1px `OutlineVariant` 描边，圆角 12dp，无阴影 |
| 列表项 | 高度 56dp（单行）/ 72dp（双行），左侧 40dp 图标位，右侧 24dp trailing |
| 输入框 | `OutlinedTextField`，圆角 8dp，聚焦时边框变 `Primary` |
| 对话气泡 | 用户气泡 `PrimaryContainer`，AI 气泡 `SurfaceContainer`，圆角 12dp，最大宽度 80% |
| 状态指示 | 圆点 8dp + `labelSmall` 文字，圆点颜色取自 `State*` token |
| 底部表 | 顶部圆角 16dp，顶部 4dp 拖拽条，背景 `SurfaceContainerHigh` |
| 毛玻璃容器 | `GlassTint` 70% 透明 + 12dp 模糊 + 1px `GlassBorder` 高光 |

### 6.3 禁止组件

- 禁止 iOS 风格 segmented control
- 禁止自定义下拉刷新指示器（使用 Material3 `PullToRefresh`）
- 禁止 WebView 内嵌的伪原生组件
- 禁止任何带「发光」效果的边框
- 禁止全屏 modal bottom sheet（保留 16dp 顶部留白）

---

## 7. 动效原则

### 7.1 时长与曲线

| 类型 | 时长 | 曲线 |
|------|------|------|
| 微交互（按下/聚焦） | 100ms | `LinearEasing` |
| 状态过渡（颜色/透明度） | 150ms | `FastOutSlowInEasing` |
| 元素进出（列表项） | 200ms | `FastOutSlowInEasing` |
| 页面切换 | 300ms | `FastOutSlowInEasing` |
| 大型布局变化 | 400ms | `FastOutSlowInEasing` |

### 7.2 动效原则

1. **服务于理解**：动效必须帮助用户理解状态变化或因果关系，禁止纯装饰。
2. **单次优先**：默认不使用循环动画。仅运行时 Installing/Starting 等待态允许脉冲。
3. **可关闭**：所有非必要动效必须尊重系统「减少动态效果」设置（`Settings.Global.ANIMATOR_DURATION_SCALE`）。
4. **不超过 400ms**：任何单次过渡不超过 400ms，超过即视为性能问题。
5. **不打断输入**：用户正在输入或滚动时，不触发覆盖性动画。
6. **加载诚实**：加载时间 > 300ms 才显示 loading 指示，< 300ms 直接显示结果，不用动画掩盖。

### 7.3 禁止动效

- 禁止粒子背景、流光边框
- 禁止 logo 旋转/呼吸
- 禁止列表项瀑布式入场
- 禁止按钮按下时的缩放反弹
- 禁止页面切换时的 3D 翻转

---

## 8. 深色与亮色策略

### 8.1 深色优先（默认）

- 应用默认跟随系统暗色模式。
- 在系统未指定时，强制使用深色主题。
- 深色主题是 Design System 的「第一公民」，所有视觉决策先在深色下验证。

### 8.2 亮色降级

- 亮色主题作为可选项，色板已在 `Theme.kt` 中定义。
- 亮色主题不强制与深色 1:1 对应，允许在亮色下使用更亮的 Primary 与更高的对比度。
- 任何 UI 元素必须在两种主题下都通过对比度检查。

### 8.3 毛玻璃处理

- 深色主题：`GlassTint` 70% 透明 + 12dp 模糊 + 1px `GlassBorder`。
- 亮色主题：`GlassTint` 替换为亮色对应值（80% 白），模糊半径降低到 8dp。
- Android 12 以上使用 `RenderEffect.createBlurEffect`，Android 11 及以下 fallback 到半透明纯色。

---

## 9. 资源命名约定

| 资源类型 | 前缀 | 示例 |
|----------|------|------|
| 颜色 | `amitia_` | `amitia_primary`、`amitia_surface` |
| 字符串 | `amitia_` | `amitia_app_name`、`amitia_chat_send` |
| 图片 | `ic_` / `img_` | `ic_home`、`img_empty_state` |
| 布局（如使用） | `view_` | `view_runtime_status` |
| Compose 组件 | `Amitia` 前缀 | `AmitiaButton`、`AmitiaCard` |

---

## 10. 与桌面端的差异

Amitia 桌面端基于 Web 排版，Android 端不复制其布局，遵循以下差异：

| 维度 | 桌面端 | Android 端 |
|------|--------|------------|
| 信息密度 | 高（鼠标精确） | 低（手指粗大） |
| 主导航 | 侧边栏 | 底部 Tab |
| 主交互 | 多窗口 | 单窗口 + 抽屉 |
| 字号 | 12-14px 为主 | 14-16sp 为主 |
| 卡片宽度 | 多列网格 | 单列优先 |
| 主题 | 跟随系统 | 深色优先 |
| 动效 | 少（避免分心） | 适度（触觉反馈） |

---

## 11. 验收清单

- [ ] 所有颜色通过 token 引用，无裸 hex
- [ ] 所有字号通过 `AmitiaTypography` 引用
- [ ] 所有间距为 4dp 整数倍
- [ ] 所有可点击区域 ≥ 48dp
- [ ] 文字对比度 ≥ 4.5:1（深色）/ ≥ 4.5:1（亮色）
- [ ] 深色与亮色主题均通过视觉走查
- [ ] 无任何「AI 紫」「蓝紫渐变」「发光边框」
- [ ] 单页面同时出现的字号不超过 4 种
- [ ] 单次动效不超过 400ms
- [ ] 启用「减少动态效果」后应用仍可用

---

## 12. 变更记录

| 版本 | 日期 | 变更 |
|------|------|------|
| v0.1.0 | 2026-07-26 | Phase 1 工程骨架基线，建立颜色/字体/栅格/导航/组件/动效原则 |
