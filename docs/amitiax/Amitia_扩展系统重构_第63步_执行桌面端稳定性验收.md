# Amitia 扩展系统重构第 63 步实施文档

## 第 63 步：执行桌面端稳定性验收

---

## 一、步骤目标

针对 Amitia 当前核心运行环境 Electron + Vue + Go Backend，在 Windows、macOS、Linux 桌面平台上完成 Extension Kernel 的稳定性、资源、进程、窗口、休眠恢复、升级、崩溃和长时间运行验收。

本步骤目标：

> 证明新扩展系统不会造成 Electron 主进程、Renderer、Go Backend、Runtime 子进程、MCP 进程、Service Runtime、WASM Runtime 和扩展 UI 的异常残留、启动冲突、关闭卡死、内存持续增长、重复拉起、系统托盘异常或跨平台不可用。

---

## 二、平台矩阵

至少覆盖：

### Windows

- Windows 11 x64；
-高 DPI；
-标准用户；
-中文路径；
-长路径；
-杀毒软件常见拦截；
-系统关机；
-休眠/唤醒；
-多显示器。

### macOS

- Apple Silicon；
-Intel 如仍支持；
-应用签名；
-权限提示；
-Dock/Menu；
-休眠；
-应用退出；
-沙箱约束。

### Linux

-主流发行版；
-X11；
-Wayland；
-不同桌面环境；
-托盘可用与不可用；
-文件权限；
-通知服务；
-系统关机。

---

## 三、进程拓扑

验收必须明确：

```text
Electron Main
Electron Renderer
Go Backend
JavaScript Plugin Host
Task Runtime
MCP Process
Trusted Service Runtime
WASM Runtime Host
Updater
```

每个进程有：

-Owner；
-启动入口；
-PID/Handle；
-停止顺序；
-崩溃处理；
-日志；
-资源；
-重复检测。

---

## 四、单实例

验证：

-应用单实例；
-Backend 单实例；
-同一 Runtime Generation 单实例；
-MCP Server 防双进程；
-Service 防双进程；
-开发版/正式版互斥；
-旧系统进程不再拉起。

锁不能只依赖 PID。

---

## 五、启动验收

覆盖：

-正常启动；
-首次启动；
-大量扩展；
-无扩展；
-安全模式；
-修复模式；
-上次崩溃；
-更新后；
-回滚后；
-数据库迁移；
-包恢复；
-磁盘不足；
-目录只读；
-端口/Pipe 冲突；
-旧 Stale Lock。

---

## 六、启动时间预算

分段测量：

- Electron；
-Backend；
-数据库；
-Extension Definition；
-Registry；
-Runtime；
-MCP；
-UI；
-Ready。

扩展应尽量按需启动，不能所有 Runtime 阻塞主界面。

---

## 七、Ready 语义

只有以下完成后才 Ready：

-核心数据库；
-迁移；
-恢复；
-Registry；
-关键系统 Tool；
-必要 Runtime；
-旧双运行检查；
-安全检查。

可选 Extension 故障可进入 Degraded，不应拖死整个应用。

---

## 八、关闭验收

覆盖：

-用户退出；
-窗口关闭但托盘继续；
-托盘退出；
-系统关机；
-更新重启；
-开发重载；
-Backend 异常；
-Renderer Crash；
-Electron Main Crash；
-强制结束；
-超时强制停止。

---

## 九、关闭顺序

必须验证：

1.停止新扩展调用。
2.暂停 Schedule/Event。
3.Drain Tool/Workflow/Task。
4.关闭 UI Session。
5.停止 JavaScript Runtime。
6.停止 MCP/Service。
7.释放 Desktop Resource。
8.刷写审计。
9.关闭数据库。
10.退出 Backend。
11.退出 Electron。

---

## 十、进程残留

应用退出后检查：

-Plugin Host；
-MCP；
-Service；
-Task；
-Backend；
-临时 Pipe；
-Lock；
-临时目录；
-窗口；
-全局快捷键；
-托盘；
-端口。

任何残留必须有 Owner 和清理记录。

---

## 十一、崩溃恢复

分别注入：

-Renderer Crash；
-Electron Main Crash；
-Backend Crash；
-Plugin Host Crash；
-MCP Crash；
-Service Crash；
-Task Crash；
-WASM Trap；
-磁盘写失败；
-数据库断开。

验证：

-不重复副作用；
-不双启动；
-恢复 Journal；
-UI 可解释；
-Quarantine；
-资源清理。

---

## 十二、Renderer 重载

Renderer 刷新不应：

-重启 Backend；
-重启所有 Runtime；
-重复注册；
-重复连接 MCP；
-重复 Schedule；
-丢失 Extension 状态；
-泄漏 UI Session。

---

## 十三、窗口管理

验证：

-主窗口；
-扩展页面窗口；
-Dialog；
-Drawer；
-托盘；
-关闭；
-多显示器；
-DPI；
-最小化；
-全屏；
-主题；
-窗口恢复位置；
-卸载扩展时窗口关闭。

---

## 十四、托盘

验证：

-图标清晰；
-亮暗主题；
-菜单；
-扩展项；
-启停；
-退出；
-多次重建；
-Explorer 重启；
-Linux 无托盘环境；
-macOS 菜单栏。

---

## 十五、快捷键

验证：

-注册；
-冲突；
-退出注销；
-崩溃恢复；
-平台保留；
-布局；
-Wayland；
-开发重载；
-卸载。

---

## 十六、休眠/唤醒

休眠前：

-暂停必要 Timer；
-记录连接；
-避免无意义重试。

唤醒后：

-时钟校正；
-MCP 重连；
-Service Health；
-Schedule Missed Policy；
-UI 恢复；
-网络恢复；
-不重复运行。

---

## 十七、网络切换

覆盖：

-断网；
-代理变化；
-VPN；
-DNS；
-网络恢复；
-HTTP MCP；
-OAuth；
-Provider；
-更新检查。

避免无限快速重连。

---

## 十八、文件系统

覆盖：

-中文路径；
-空格；
-长路径；
-只读；
-文件被占用；
-杀毒隔离；
-符号链接；
-大小写；
-外接盘；
-跨盘安装；
-磁盘不足。

---

## 十九、自动更新

验证：

-应用更新期间 Extension 停止；
-新版本 Runtime 兼容；
-回滚；
-包恢复；
-安装目录不被扩展占用；
-旧进程清理；
-当前版本指针；
-数据库 Schema。

---

## 二十、内存稳定性

测试：

-8 小时；
-24 小时；
-72 小时建议；
-频繁 Tool；
-MCP 重连；
-UI 开关；
-角色切换；
-Workflow；
-Task；
-开发热重载。

记录：

-主进程；
-Renderer；
-Backend；
-各 Runtime；
-总内存；
-趋势；
-GC；
-Handle；
-线程；
-文件描述符。

---

## 二十一、CPU 稳定性

空闲时：

-无高频轮询；
-无无限重连；
-无隐藏 UI 动画；
-无未暂停 Data Subscription；
-无 Timer 泄漏；
-无 Watcher 泄漏。

---

## 二十二、磁盘稳定性

检查：

-日志轮转；
-Task 临时；
-Artifact；
-旧版本；
-Cache；
-Dead Letter；
-运行历史；
-Snapshot；
-开发构建；
-Source Map；
-崩溃转储。

必须有配额和清理策略。

---

## 二十三、资源泄漏

检测：

-进程；
-线程；
-Goroutine；
-Timer；
-File Watcher；
-Socket；
-Pipe；
-Window；
-Shortcut；
-Tray；
-Resource Handle；
-DB Connection；
-Event Subscription；
-UI Session。

---

## 二十四、扩展压力

场景：

-100 个 Extension；
-1000 个 Contribution；
-200 个 Tool；
-50 个 MCP；
-100 个 Workflow；
-大量 UI Slot；
-并发 Task；
-大量历史。

验证启动、搜索、详情、Registry、状态更新和关闭。

---

## 二十五、平台权限

### macOS

-通知；
-辅助功能；
-屏幕；
-文件；
-签名。

### Windows

-UAC；
-防火墙；
-Defender；
-全局快捷键；
-文件锁。

### Linux

-桌面门户；
-Wayland；
-通知；
-权限；
-AppImage/包形式。

Extension Permission UI 必须与系统权限差异协调。

---

## 二十六、安装器和数据目录

验证：

-安装路径；
-用户数据目录；
-`amitiaData`；
-升级；
-卸载保留数据；
-扩展 Artifact；
-Cache；
-Secret；
-权限；
-多用户系统预留。

---

## 二十七、日志和崩溃报告

每个进程：

-独立来源；
-统一 Trace；
-轮转；
-脱敏；
-崩溃摘要；
-版本；
-Extension；
-Generation；
-平台；
-诊断包。

---

## 二十八、自动化

建立：

-跨平台 CI；
-E2E；
-进程残留检测；
-内存趋势；
-文件句柄；
-反复启停；
-崩溃注入；
-休眠模拟可行部分；
-升级/回滚。

---

## 二十九、P0 阻塞

-应用无法退出；
-进程残留造成重复；
-数据损坏；
-扩展双执行；
-Renderer 重载重复连接；
-休眠后 Schedule 重复；
-更新失败无法恢复；
-内存持续无界增长；
-关键平台无法启动；
-托盘退出不完整；
-快捷键残留；
-Secret 日志泄漏。

---

## 三十、实施任务

1. 建立平台矩阵。
2.建立进程拓扑验收。
3.测试启动/Ready。
4.测试关闭/强制停止。
5.测试崩溃恢复。
6.测试 Renderer 重载。
7.测试窗口/托盘/快捷键。
8.测试休眠/唤醒。
9.测试网络切换。
10.测试文件系统异常。
11.测试应用更新/回滚。
12.进行 24/72 小时稳定性。
13.检测资源泄漏。
14.进行扩展压力测试。
15.完成跨平台权限测试。
16.输出稳定性报告。

---

## 三十一、验收标准

1.三平台核心链路通过。
2.无重复 Backend/Runtime/MCP。
3.正常关闭无残留。
4.强制关闭可恢复。
5.Renderer 重载不重复注册。
6.休眠恢复不重复 Schedule。
7.更新回滚可用。
8.空闲 CPU 合理。
9.内存无无界增长。
10.磁盘有清理策略。
11.资源泄漏 P0 清零。
12.可进入第 64 步安全验收。

---

## 三十二、执行约束

> 桌面稳定性验收必须以真实进程、真实文件系统、真实 Electron 生命周期和跨平台行为为准，不能只通过单元测试证明。

禁止：

-仅 Windows 验收；
-忽略系统关机；
-忽略 Renderer 重载；
-忽略托盘模式；
-忽略残留进程；
-用固定等待掩盖关闭竞态；
-内存测试时间过短；
-P0 未清零进入切换。
