import 'package:flutter/material.dart';
import '../../app/app_routes.dart';
import '../models/models.dart';

export 'mock_characters.dart';
export 'mock_memory.dart';
export 'mock_extensions.dart';
export 'mock_workshop.dart';
export 'mock_settings.dart';
export 'mock_channels.dart';
export 'mock_kernel.dart';

class MockData {
  MockData._();

  static List<Character> characters = [
    Character(
      id: 'c1',
      name: 'Amitia',
      avatarColor: '#7668EE',
      avatarInitial: '阿',
      status: '在线',
      mood: '心情很好',
      identity: '你的专属 AI 伙伴',
      description: '温柔、细心，喜欢帮助你解决问题',
      relationshipDays: 128,
      messageCount: 3421,
      personality: '温柔、细心、有耐心，善于倾听',
      speakingStyle: '语气温和，偶尔带着俏皮',
      userRelation: '亲密伙伴',
      prompt: '你是一个温柔细心的 AI 伙伴，名叫Amitia……',
      currentActivity: '整理今天的记忆',
      location: '主界面',
    ),
    Character(
      id: 'c2',
      name: '小雨',
      avatarColor: '#52B788',
      avatarInitial: '雨',
      status: '在线',
      mood: '专注中',
      identity: '效率助手',
      description: '理性、高效，擅长分析和规划',
      relationshipDays: 56,
      messageCount: 892,
      personality: '理性、高效、条理清晰',
      speakingStyle: '简洁直接，注重逻辑',
      userRelation: '工作搭档',
      prompt: '你是一个高效理性的助手……',
      currentActivity: '分析工作数据',
      location: '工作区',
    ),
    Character(
      id: 'c3',
      name: 'Epsilon',
      avatarColor: '#6C8FEA',
      avatarInitial: 'E',
      status: '离线',
      mood: '休眠',
      identity: '技术顾问',
      description: '冷静、专业，精通技术问题',
      relationshipDays: 30,
      messageCount: 456,
      personality: '冷静、专业、逻辑严密',
      speakingStyle: '技术性强，用词精准',
      userRelation: '技术顾问',
      prompt: '你是一个专业的技术顾问……',
      currentActivity: '待机中',
      location: '后台',
    ),
    Character(
      id: 'c4',
      name: 'Karin',
      avatarColor: '#E9A23B',
      avatarInitial: 'K',
      status: '在线',
      mood: '活力满满',
      identity: '创意伙伴',
      description: '活泼、充满创意，喜欢头脑风暴',
      relationshipDays: 15,
      messageCount: 234,
      personality: '活泼、创意、充满激情',
      speakingStyle: '生动有趣，喜欢用比喻',
      userRelation: '创意搭档',
      prompt: '你是一个充满创意的伙伴……',
      currentActivity: '构思新点子',
      location: '创意工坊',
    ),
  ];

  static List<ChatMessage> chatMessages = [
    ChatMessage(
      id: 'm1',
      role: MessageRole.system,
      type: MessageType.systemNotice,
      content: '今天 09:15',
      time: DateTime(2026, 7, 30, 9, 15),
    ),
    ChatMessage(
      id: 'm2',
      role: MessageRole.assistant,
      type: MessageType.text,
      content: '早上好！今天天气不错，你感觉怎么样？需要我帮你规划一下今天的安排吗？',
      time: DateTime(2026, 7, 30, 9, 15),
    ),
    ChatMessage(
      id: 'm3',
      role: MessageRole.user,
      type: MessageType.text,
      content: '帮我整理一下下载目录，太乱了',
      time: DateTime(2026, 7, 30, 9, 18),
    ),
    ChatMessage(
      id: 'm4',
      role: MessageRole.assistant,
      type: MessageType.agentTask,
      content: '',
      time: DateTime(2026, 7, 30, 9, 18),
      agentTaskTitle: '整理下载目录',
      agentTaskSteps: ['扫描下载目录', '识别重复文件', '整理文件分类', '生成结果报告'],
      agentTaskProgress: 32,
      agentTaskElapsed: '00:18',
    ),
    ChatMessage(
      id: 'm5',
      role: MessageRole.assistant,
      type: MessageType.toolCall,
      content: '',
      time: DateTime(2026, 7, 30, 9, 19),
      toolName: '文件系统',
      toolResult: '已扫描 1,247 个文件，识别 23 个重复文件',
    ),
    ChatMessage(
      id: 'm6',
      role: MessageRole.user,
      type: MessageType.file,
      content: '产品需求文档.pdf',
      time: DateTime(2026, 7, 30, 9, 25),
      fileName: '产品需求文档.pdf',
      fileSizeKB: 2048,
    ),
    ChatMessage(
      id: 'm7',
      role: MessageRole.assistant,
      type: MessageType.text,
      content: '收到你的文档，我已经帮你分析了一下。这个需求文档结构清晰，主要分为三个模块。需要我帮你提取关键信息或者生成摘要吗？',
      time: DateTime(2026, 7, 30, 9, 26),
    ),
    ChatMessage(
      id: 'm8',
      role: MessageRole.user,
      type: MessageType.text,
      content: '生成一份摘要吧',
      time: DateTime(2026, 7, 30, 9, 27),
    ),
    ChatMessage(
      id: 'm9',
      role: MessageRole.assistant,
      type: MessageType.text,
      content: '好的，我正在生成摘要。文档核心内容是关于用户管理系统的重构方案，包含权限模型升级、数据迁移策略和灰度发布计划三个部分。',
      time: DateTime(2026, 7, 30, 9, 28),
    ),
  ];

  static List<Conversation> conversations = [
    Conversation(
      id: 'conv1',
      title: '和Amitia的日常对话',
      lastMessage: '好的，我正在生成摘要……',
      lastTime: DateTime(2026, 7, 30, 9, 28),
      characterId: 'c1',
      isPinned: true,
    ),
    Conversation(
      id: 'conv2',
      title: '帮我整理下载目录',
      lastMessage: '已扫描 1,247 个文件',
      lastTime: DateTime(2026, 7, 30, 9, 18),
      characterId: 'c1',
      isPinned: true,
    ),
    Conversation(
      id: 'conv3',
      title: '分析产品需求文档',
      lastMessage: '文档核心内容是关于用户管理系统……',
      lastTime: DateTime(2026, 7, 29, 16, 30),
      characterId: 'c2',
    ),
    Conversation(
      id: 'conv4',
      title: '生成一份周报模板',
      lastMessage: '已生成周报模板，请查看',
      lastTime: DateTime(2026, 7, 29, 14, 0),
      characterId: 'c2',
    ),
    Conversation(
      id: 'conv5',
      title: '关于 Amitia 的功能规划',
      lastMessage: '建议从三个方向推进',
      lastTime: DateTime(2026, 7, 28, 20, 15),
      characterId: 'c3',
    ),
    Conversation(
      id: 'conv6',
      title: '推荐几部科幻电影',
      lastMessage: '推荐你看看《星际穿越》',
      lastTime: DateTime(2026, 7, 27, 22, 0),
      characterId: 'c4',
    ),
    Conversation(
      id: 'conv7',
      title: '调试 API 接口问题',
      lastMessage: '问题已定位到请求头格式',
      lastTime: DateTime(2026, 7, 26, 11, 30),
      characterId: 'c3',
    ),
  ];

  static List<AgentTask> agentTasksRunning = [
    AgentTask(
      id: 't1',
      title: '整理下载目录',
      currentStep: '正在识别重复文件',
      progress: 32,
      status: AgentTaskStatus.running,
      elapsed: '00:18',
      category: '文件处理',
      requiredPermissions: ['访问下载目录', '读取文件信息'],
      createdAt: DateTime(2026, 7, 30, 9, 18),
    ),
    AgentTask(
      id: 't2',
      title: '生成周报摘要',
      currentStep: '分析本周工作记录',
      progress: 65,
      status: AgentTaskStatus.running,
      elapsed: '00:42',
      category: '内容生成',
      createdAt: DateTime(2026, 7, 30, 9, 0),
    ),
  ];

  static List<AgentTask> agentTasksPending = [
    AgentTask(
      id: 't3',
      title: '安装开发工具包',
      currentStep: '等待权限审批',
      progress: 0,
      status: AgentTaskStatus.pending,
      category: '系统操作',
      requiredPermissions: ['访问下载目录', '运行 Shell 命令'],
      createdAt: DateTime(2026, 7, 30, 8, 30),
    ),
    AgentTask(
      id: 't4',
      title: '备份工作文档',
      currentStep: '等待确认备份范围',
      progress: 0,
      status: AgentTaskStatus.pending,
      category: '文件处理',
      requiredPermissions: ['读取文档目录', '写入备份位置'],
      createdAt: DateTime(2026, 7, 30, 8, 45),
    ),
  ];

  static List<AgentTask> agentTasksCompleted = [
    AgentTask(
      id: 't5',
      title: '整理桌面文件',
      currentStep: '已完成',
      progress: 100,
      status: AgentTaskStatus.completed,
      elapsed: '01:23',
      category: '文件处理',
      result: '已整理 156 个文件，分类为 8 个文件夹',
      createdAt: DateTime(2026, 7, 29, 15, 0),
    ),
    AgentTask(
      id: 't6',
      title: '生成周报摘要',
      currentStep: '已完成',
      progress: 100,
      status: AgentTaskStatus.completed,
      elapsed: '02:15',
      category: '内容生成',
      result: '已生成本周工作摘要，包含 5 个主要进展',
      createdAt: DateTime(2026, 7, 29, 14, 0),
    ),
    AgentTask(
      id: 't7',
      title: '备份工作文档',
      currentStep: '已完成',
      progress: 100,
      status: AgentTaskStatus.completed,
      elapsed: '03:48',
      category: '文件处理',
      result: '已备份 89 个文档到指定目录',
      createdAt: DateTime(2026, 7, 28, 18, 0),
    ),
  ];

  static List<AgentTaskStep> agentTaskDetailSteps = [
    AgentTaskStep(name: '读取下载目录', status: '已完成', duration: '0.8 秒'),
    AgentTaskStep(name: '分析文件类型', status: '已完成', duration: '1.2 秒'),
    AgentTaskStep(name: '识别重复文件', status: '执行中'),
    AgentTaskStep(name: '整理文件分类', status: '等待中'),
    AgentTaskStep(name: '生成结果报告', status: '等待中'),
  ];

  static List<ToolCallRecord> toolCallRecords = [
    ToolCallRecord(
      toolName: '文件系统 · 读取目录',
      input: '路径: ~/Downloads',
      output: '返回 1,247 个文件',
      duration: '0.8 秒',
      status: '成功',
    ),
    ToolCallRecord(
      toolName: '文件分析 · 类型识别',
      input: '1,247 个文件',
      output: '识别 12 种文件类型',
      duration: '1.2 秒',
      status: '成功',
    ),
    ToolCallRecord(
      toolName: '去重分析 · 哈希比对',
      input: '1,247 个文件',
      output: '发现 23 个重复文件',
      duration: '执行中',
      status: '运行中',
    ),
  ];

  static List<QuickTask> quickTasks = [
    QuickTask(title: '控制手机', icon: Icons.phone_android, category: '设备'),
    QuickTask(title: '处理文件', icon: Icons.folder_outlined, category: '文件'),
    QuickTask(title: '打开工作区', icon: Icons.work_outline, category: '工作'),
    QuickTask(title: '新建工作流', icon: Icons.account_tree_outlined, category: '自动化'),
    QuickTask(title: '数据分析', icon: Icons.analytics_outlined, category: '分析'),
    QuickTask(title: '信息搜索', icon: Icons.search, category: '搜索'),
  ];

  static List<Memory> memories = [
    Memory(
      id: 'mem1',
      content: '记得你喜欢在早上喝咖啡',
      source: '对话',
      importance: '较高',
      time: DateTime(2026, 7, 30, 9, 15),
      category: '情景记忆',
      isPinned: true,
    ),
    Memory(
      id: 'mem2',
      content: '你的工作方向是全栈开发，主要使用 Go 和 TypeScript',
      source: '对话',
      importance: '高',
      time: DateTime(2026, 7, 29, 14, 0),
      category: '长期记忆',
    ),
    Memory(
      id: 'mem3',
      content: '你有一个叫小白的宠物猫',
      source: '对话',
      importance: '中',
      time: DateTime(2026, 7, 28, 20, 0),
      category: '关系记忆',
    ),
    Memory(
      id: 'mem4',
      content: '本周需要完成产品需求文档的评审',
      source: '日程',
      importance: '高',
      time: DateTime(2026, 7, 27, 9, 0),
      category: '情景记忆',
    ),
    Memory(
      id: 'mem5',
      content: '你偏好简洁的界面风格，不喜欢过多装饰',
      source: '行为分析',
      importance: '中',
      time: DateTime(2026, 7, 26, 15, 30),
      category: '长期记忆',
    ),
    Memory(
      id: 'mem6',
      content: 'Amitia的世界设定：来自一个科技与魔法并存的世界',
      source: '设定',
      importance: '高',
      time: DateTime(2026, 7, 25, 10, 0),
      category: '世界设定',
    ),
    Memory(
      id: 'mem7',
      content: '上次见面时你提到了一个关于 AI 陪伴的想法',
      source: '对话',
      importance: '较高',
      time: DateTime(2026, 7, 24, 18, 0),
      category: '情景记忆',
    ),
  ];

  static List<Extension> installedExtensions = [
    Extension(
      id: 'e1',
      name: '文件系统',
      description: '提供文件读写、目录管理等能力',
      type: ExtensionType.mcp,
      icon: Icons.folder_outlined,
      isInstalled: true,
      isEnabled: true,
    ),
    Extension(
      id: 'e2',
      name: 'Web 搜索',
      description: '搜索互联网获取最新信息',
      type: ExtensionType.mcp,
      icon: Icons.language,
      isInstalled: true,
      isEnabled: true,
    ),
    Extension(
      id: 'e3',
      name: '代码执行',
      description: '在沙箱环境中执行代码',
      type: ExtensionType.skill,
      icon: Icons.code,
      isInstalled: true,
      isEnabled: true,
    ),
    Extension(
      id: 'e4',
      name: '数据库',
      description: '查询和管理本地数据库',
      type: ExtensionType.mcp,
      icon: Icons.storage,
      isInstalled: true,
      isEnabled: false,
    ),
    Extension(
      id: 'e5',
      name: 'PDF 分析器',
      description: '解析和分析 PDF 文档内容',
      type: ExtensionType.skill,
      icon: Icons.picture_as_pdf_outlined,
      isInstalled: true,
      isEnabled: true,
    ),
  ];

  static List<Extension> recommendedExtensions = [
    Extension(
      id: 'e6',
      name: '思维导图生成',
      description: '根据文本自动生成思维导图',
      type: ExtensionType.skill,
      icon: Icons.account_tree_outlined,
      isInstalled: false,
      isEnabled: false,
      isRecommended: true,
    ),
    Extension(
      id: 'e7',
      name: '图像理解',
      description: '识别和分析图片内容',
      type: ExtensionType.mcp,
      icon: Icons.image_outlined,
      isInstalled: false,
      isEnabled: false,
      isRecommended: true,
    ),
    Extension(
      id: 'e8',
      name: '网页摘要',
      description: '自动提取网页核心内容',
      type: ExtensionType.plugin,
      icon: Icons.article_outlined,
      isInstalled: false,
      isEnabled: false,
      isRecommended: true,
    ),
    Extension(
      id: 'e9',
      name: 'GitHub 工具',
      description: '管理仓库、Issue 和 PR',
      type: ExtensionType.mcp,
      icon: Icons.code,
      isInstalled: false,
      isEnabled: false,
      isRecommended: true,
    ),
  ];

  static RuntimeInfo runtimeInfo = RuntimeInfo(
    status: '已安装',
    version: 'Ubuntu ARM64',
    backendStatus: '已连接',
    storageUsage: '2.43 GB',
    components: [
      RuntimeComponent(name: 'Amitia Backend', status: '运行中'),
      RuntimeComponent(name: 'SurrealDB', status: '运行中'),
      RuntimeComponent(name: 'Qdrant', status: '运行中'),
      RuntimeComponent(name: 'MCP Runtime', status: '已停止'),
    ],
  );

  static List<PermissionItem> permissions = [
    PermissionItem(
      name: '无障碍服务',
      icon: Icons.accessibility_new,
      status: '已授权',
      description: '允许 Amitia 读取屏幕内容并提供辅助',
    ),
    PermissionItem(
      name: '通知读取',
      icon: Icons.notifications_outlined,
      status: '已授权',
      description: '读取系统通知以提供智能提醒',
    ),
    PermissionItem(
      name: '悬浮窗',
      icon: Icons.picture_in_picture,
      status: '已授权',
      description: '在其他应用上方显示悬浮窗',
    ),
    PermissionItem(
      name: '文件访问',
      icon: Icons.folder_outlined,
      status: '需要设置',
      description: '访问设备存储中的文件',
    ),
    PermissionItem(
      name: '麦克风',
      icon: Icons.mic_outlined,
      status: '已授权',
      description: '语音输入和通话',
    ),
    PermissionItem(
      name: '相机',
      icon: Icons.camera_alt_outlined,
      status: '未授权',
      description: '拍照和扫描功能',
    ),
    PermissionItem(
      name: '位置',
      icon: Icons.location_on_outlined,
      status: '未授权',
      description: '获取设备位置信息',
    ),
    PermissionItem(
      name: '电池优化',
      icon: Icons.battery_std,
      status: '已授权',
      description: '忽略电池优化以保持后台运行',
    ),
    PermissionItem(
      name: 'Shizuku',
      icon: Icons.security,
      status: '不可用',
      description: '提供高级系统操作能力',
    ),
  ];

  static List<SettingGroup> mainSettings = [
    SettingGroup(title: 'AI 与个性化', items: [
      SettingItem(title: '模型设置', icon: Icons.psychology_outlined, value: 'GPT-4', route: AppRoutes.settingsModels),
      SettingItem(title: 'AI 配置', icon: Icons.smart_toy_outlined, route: AppRoutes.settingsAi),
      SettingItem(title: '外观设置', icon: Icons.palette_outlined, value: '亮色', route: AppRoutes.settingsAppearance),
      SettingItem(title: '主题设置', icon: Icons.color_lens_outlined, route: AppRoutes.settingsTheme),
      SettingItem(title: '用户设置', icon: Icons.person_outline, route: AppRoutes.settingsUser),
      SettingItem(title: '时间感知', icon: Icons.schedule_outlined, route: AppRoutes.settingsTemporal),
    ]),
    SettingGroup(title: '系统与维护', items: [
      SettingItem(title: 'Runtime', icon: Icons.terminal, value: '运行中', route: AppRoutes.settingsRuntime),
      SettingItem(title: '系统权限', icon: Icons.lock_outlined, route: AppRoutes.settingsPermissions),
      SettingItem(title: '存储管理', icon: Icons.storage_outlined, route: AppRoutes.settingsStorage),
      SettingItem(title: '安全设置', icon: Icons.security_outlined, route: AppRoutes.settingsSafety),
      SettingItem(title: '维护工具', icon: Icons.build_circle_outlined, route: AppRoutes.settingsMaintenance),
      SettingItem(title: '工具箱', icon: Icons.handyman_outlined, subtitle: '运行日志、状态诊断与开发辅助工具', value: '诊断工具', route: AppRoutes.settingsToolbox),
    ]),
    SettingGroup(title: '部署与隐私', items: [
      SettingItem(title: '部署配置', icon: Icons.cloud_upload_outlined, route: AppRoutes.settingsDeployment),
      SettingItem(title: '隐私扫描', icon: Icons.privacy_tip_outlined, route: AppRoutes.settingsPrivacyScan),
      SettingItem(title: '系统设置', icon: Icons.settings_applications_outlined, route: AppRoutes.settingsSystem),
    ]),
    SettingGroup(title: '关于', items: [
      SettingItem(title: '备份与恢复', icon: Icons.backup_outlined, route: AppRoutes.settingsBackup),
      SettingItem(title: '关于 Amitia', icon: Icons.info_outline, value: 'v1.0.0', route: AppRoutes.settingsAbout),
    ]),
  ];

  static List<String> gamePlugins = ['Minecraft 控制器', '星露谷助手', '原神计时器'];
  static List<String> gameTasks = ['每日签到提醒', '体力恢复提醒', '活动倒计时'];

  static List<String> desktopPetPlugins = ['Amitia桌宠', '小雨桌宠', '自定义桌宠'];
}
