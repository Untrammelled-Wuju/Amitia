import '../models/models.dart';

class MockKernel {
  MockKernel._();

  static List<WasmModule> wasmModules = [
    WasmModule(id: 'wm1', name: '图像处理器', status: '已加载', quota: 100, used: 23),
    WasmModule(id: 'wm2', name: '加密模块', status: '已加载', quota: 50, used: 5),
    WasmModule(id: 'wm3', name: 'JSON 解析器', status: '已卸载', quota: 100, used: 0),
  ];

  static List<HookEntry> hooks = [
    HookEntry(id: 'h1', point: 'message.receive', contributor: '消息处理插件', priority: 10),
    HookEntry(id: 'h2', point: 'message.send', contributor: '消息处理插件', priority: 10),
    HookEntry(id: 'h3', point: 'memory.store', contributor: '记忆管理插件', priority: 20),
    HookEntry(id: 'h4', point: 'memory.retrieve', contributor: '记忆管理插件', priority: 20),
    HookEntry(id: 'h5', point: 'emotion.update', contributor: '情感引擎插件', priority: 15, status: HookStatus.circuitOpen),
  ];

  static List<TrustedService> trustedServices = [
    TrustedService(id: 'ts1', name: '文件系统服务', runStatus: '已启动'),
    TrustedService(id: 'ts2', name: '网络请求服务', runStatus: '已启动'),
    TrustedService(id: 'ts3', name: '数据库服务', runStatus: '已启动'),
    TrustedService(id: 'ts4', name: '加密服务', runStatus: '已停止', isolated: true),
  ];

  static List<KernelTask> kernelTasks = [
    KernelTask(id: 'kt1', name: '文件整理任务', status: '运行中', output: '处理中...'),
    KernelTask(id: 'kt2', name: '记忆合并任务', status: '已完成', output: '合并了 12 条记忆', hasCheckpoint: true),
    KernelTask(id: 'kt3', name: '数据备份任务', status: '已暂停'),
    KernelTask(id: 'kt4', name: '索引重建任务', status: '失败', error: '磁盘空间不足'),
  ];

  static List<KernelEvent> kernelEvents = [
    KernelEvent(id: 'ke1', type: 'message.receive', status: '已处理', time: DateTime(2026, 7, 30, 9, 15)),
    KernelEvent(id: 'ke2', type: 'memory.created', status: '已处理', time: DateTime(2026, 7, 30, 9, 16)),
    KernelEvent(id: 'ke3', type: 'hook.error', status: '死信', time: DateTime(2026, 7, 30, 9, 20), detail: '情感引擎插件超时'),
    KernelEvent(id: 'ke4', type: 'task.completed', status: '已处理', time: DateTime(2026, 7, 30, 9, 25)),
  ];

  static List<ScheduleEntry> schedules = [
    ScheduleEntry(id: 'se1', name: '每日记忆合并', nextRun: DateTime(2026, 8, 1, 3, 0), lastRun: DateTime(2026, 7, 31, 3, 0)),
    ScheduleEntry(id: 'se2', name: '每小时心跳检查', nextRun: DateTime(2026, 7, 31, 11, 0), lastRun: DateTime(2026, 7, 31, 10, 0)),
    ScheduleEntry(id: 'se3', name: '每周数据备份', nextRun: DateTime(2026, 8, 4, 2, 0), lastRun: DateTime(2026, 7, 28, 2, 0), isEnabled: false),
  ];

  static List<DesktopContribution> desktopContributions = [
    DesktopContribution(id: 'dc1', type: '快捷键', label: '打开主界面', value: 'Ctrl+Shift+A'),
    DesktopContribution(id: 'dc2', type: '菜单', label: '托盘菜单', value: '右键托盘图标'),
    DesktopContribution(id: 'dc3', type: '窗口', label: '悬浮窗', value: '已启用'),
  ];

  static List<UpdateInfo> updateHistory = [
    UpdateInfo(version: '1.2.0', status: '已安装', date: DateTime(2026, 7, 25)),
    UpdateInfo(version: '1.1.5', status: '已回滚', date: DateTime(2026, 7, 20)),
    UpdateInfo(version: '1.1.0', status: '已安装', date: DateTime(2026, 7, 15)),
  ];

  static UpdateInfo availableUpdate = UpdateInfo(version: '1.3.0', status: '可更新', date: DateTime(2026, 7, 30), isAvailable: true);

  static List<DevConsoleLog> devConsoleLogs = [
    DevConsoleLog(level: 'INFO', module: 'backend', message: '服务启动完成', time: DateTime(2026, 7, 30, 9, 0)),
    DevConsoleLog(level: 'INFO', module: 'chat', message: '消息服务已就绪', time: DateTime(2026, 7, 30, 9, 0, 5)),
    DevConsoleLog(level: 'WARN', module: 'mcp', message: 'MCP Runtime 未启动', time: DateTime(2026, 7, 30, 9, 0, 10)),
    DevConsoleLog(level: 'ERROR', module: 'qq', message: 'QQ Bot 连接失败: Token 验证失败', time: DateTime(2026, 7, 30, 8, 1)),
    DevConsoleLog(level: 'INFO', module: 'memory', message: '记忆合并任务完成', time: DateTime(2026, 7, 30, 3, 0)),
  ];

  static List<MigrationPlan> migrationPlans = [
    MigrationPlan(id: 'mp1', name: 'v1.2.0 数据迁移', status: '已完成', progress: 100),
    MigrationPlan(id: 'mp2', name: 'v1.3.0 灰度发布', status: '灰度中', progress: 35),
    MigrationPlan(id: 'mp3', name: '记忆系统重构', status: '已回滚', progress: 0, rollbackReason: '性能回退'),
  ];

  static List<DevWorkspace> devWorkspaces = [
    DevWorkspace(id: 'dw1', name: '文件分类助手', version: '1.0.0', status: '已注册'),
    DevWorkspace(id: 'dw2', name: '邮件摘要', version: '0.9.0', status: '开发中'),
  ];
}
