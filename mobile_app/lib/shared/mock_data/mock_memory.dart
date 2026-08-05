import '../models/models.dart';

class MockMemory {
  MockMemory._();

  static List<Reminder> reminders = [
    Reminder(id: 'r1', title: '完成产品需求文档评审', description: '本周需要完成评审会议', time: DateTime(2026, 7, 31, 14, 0), isToday: true, category: '工作'),
    Reminder(id: 'r2', title: '给朋友回电话', description: '朋友昨天来电，需要回拨', time: DateTime(2026, 7, 31, 18, 0), isToday: true, category: '社交'),
    Reminder(id: 'r3', title: '取快递', description: '有两个快递在驿站', time: DateTime(2026, 8, 1, 10, 0), category: '生活'),
    Reminder(id: 'r4', title: '项目周报', description: '周五前提交周报', time: DateTime(2026, 8, 2, 17, 0), category: '工作'),
    Reminder(id: 'r5', title: '体检预约', description: '联系医院预约下周体检', time: DateTime(2026, 8, 3, 9, 0), category: '健康'),
    Reminder(id: 'r6', title: '整理下载目录', description: '已完成的提醒', time: DateTime(2026, 7, 30, 9, 0), isCompleted: true, category: '生活'),
  ];

  static List<EmoteGroup> emoteGroups = [
    EmoteGroup(id: 'eg1', name: '默认表情', count: 12),
    EmoteGroup(id: 'eg2', name: 'Amitia专属', count: 8),
    EmoteGroup(id: 'eg3', name: '节日表情', count: 6),
    EmoteGroup(id: 'eg4', name: '日常情绪', count: 10),
  ];

  static List<EmoteItem> emotes = [
    EmoteItem(id: 'em1', name: '微笑', meaning: '开心、友好', group: '默认表情', emoji: '😊'),
    EmoteItem(id: 'em2', name: '大笑', meaning: '非常开心', group: '默认表情', emoji: '😄'),
    EmoteItem(id: 'em3', name: '害羞', meaning: '不好意思', group: '默认表情', emoji: '😊'),
    EmoteItem(id: 'em4', name: '思考', meaning: '正在思考', group: '默认表情', emoji: '🤔'),
    EmoteItem(id: 'em5', name: '惊讶', meaning: '吃惊', group: '默认表情', emoji: '😲'),
    EmoteItem(id: 'em6', name: 'Amitia问好', meaning: '打招呼', group: 'Amitia专属', characterId: 'c1', emoji: '👋'),
    EmoteItem(id: 'em7', name: 'Amitia加油', meaning: '鼓励', group: 'Amitia专属', characterId: 'c1', emoji: '💪'),
    EmoteItem(id: 'em8', name: '生日快乐', meaning: '节日祝福', group: '节日表情', emoji: '🎂'),
    EmoteItem(id: 'em9', name: '新年好', meaning: '新年祝福', group: '节日表情', emoji: '🧧'),
    EmoteItem(id: 'em10', name: '困了', meaning: '想睡觉', group: '日常情绪', emoji: '😴'),
  ];

  static List<EpisodicMemory> episodicMemories = [
    EpisodicMemory(id: 'ep1', time: DateTime(2026, 7, 30, 9, 15), location: '主界面', participants: ['用户', 'Amitia'], emotion: '愉快', summary: '早安对话', detail: '用户早上来打招呼，聊了今天的天气和安排，气氛轻松愉快。'),
    EpisodicMemory(id: 'ep2', time: DateTime(2026, 7, 29, 18, 0), location: '主界面', participants: ['用户', 'Amitia'], emotion: '关心', summary: '下班问候', detail: 'Amitia主动询问用户工作情况，用户分享了今天遇到的困难。'),
    EpisodicMemory(id: 'ep3', time: DateTime(2026, 7, 28, 20, 0), location: '工作区', participants: ['用户', '小雨'], emotion: '满足', summary: '完成周报', detail: '在小雨的帮助下完成了本周工作周报，效率提升。'),
    EpisodicMemory(id: 'ep4', time: DateTime(2026, 7, 27, 22, 0), location: '创意工坊', participants: ['用户', 'Karin'], emotion: '兴奋', summary: '头脑风暴', detail: '和Karin一起进行功能头脑风暴，产生了多个创意点子。'),
  ];

  static List<MemoryGraphNode> graphNodes = [
    MemoryGraphNode(id: 'gn1', label: 'Amitia', type: '角色', category: '角色', x: 0.5, y: 0.3),
    MemoryGraphNode(id: 'gn2', label: '用户', type: '实体', category: '用户', x: 0.2, y: 0.5),
    MemoryGraphNode(id: 'gn3', label: '咖啡偏好', type: '记忆', category: '偏好', x: 0.8, y: 0.2),
    MemoryGraphNode(id: 'gn4', label: '全栈开发', type: '记忆', category: '工作', x: 0.3, y: 0.8),
    MemoryGraphNode(id: 'gn5', label: '小白', type: '实体', category: '宠物', x: 0.7, y: 0.7),
    MemoryGraphNode(id: 'gn6', label: '小雨', type: '角色', category: '角色', x: 0.15, y: 0.2),
    MemoryGraphNode(id: 'gn7', label: '工作搭档', type: '关系', category: '关系', x: 0.45, y: 0.15),
    MemoryGraphNode(id: 'gn8', label: '创意工坊', type: '地点', category: '地点', x: 0.85, y: 0.5),
  ];

  static List<MemoryGraphEdge> graphEdges = [
    MemoryGraphEdge(source: 'gn1', target: 'gn2', relation: '陪伴'),
    MemoryGraphEdge(source: 'gn2', target: 'gn3', relation: '喜欢'),
    MemoryGraphEdge(source: 'gn2', target: 'gn4', relation: '从事'),
    MemoryGraphEdge(source: 'gn2', target: 'gn5', relation: '拥有'),
    MemoryGraphEdge(source: 'gn2', target: 'gn6', relation: '合作'),
    MemoryGraphEdge(source: 'gn6', target: 'gn7', relation: '是'),
    MemoryGraphEdge(source: 'gn2', target: 'gn8', relation: '使用'),
    MemoryGraphEdge(source: 'gn1', target: 'gn3', relation: '记住'),
  ];

  static List<MemoryTimelineEntry> timelineEntries = [
    MemoryTimelineEntry(id: 'mt1', time: DateTime(2026, 7, 30, 9, 15), type: '对话', title: '早安对话', description: '与用户进行早安问候', isImportant: true),
    MemoryTimelineEntry(id: 'mt2', time: DateTime(2026, 7, 29, 18, 0), type: '主动消息', title: '下班问候', description: '主动关心用户工作情况'),
    MemoryTimelineEntry(id: 'mt3', time: DateTime(2026, 7, 29, 14, 0), type: '记忆形成', title: '咖啡偏好', description: '记录用户喜欢早上喝咖啡', characterId: 'c1'),
    MemoryTimelineEntry(id: 'mt4', time: DateTime(2026, 7, 28, 20, 0), type: '情绪', title: '开心时刻', description: '用户分享了开心的事情', isImportant: true),
    MemoryTimelineEntry(id: 'mt5', time: DateTime(2026, 7, 28, 10, 0), type: '关系', title: '128天', description: '关系天数达到128天', isImportant: true),
    MemoryTimelineEntry(id: 'mt6', time: DateTime(2026, 7, 27, 22, 0), type: '对话', title: '电影推荐', description: '推荐了《星际穿越》'),
    MemoryTimelineEntry(id: 'mt7', time: DateTime(2026, 7, 26, 15, 0), type: '行为', title: '界面偏好', description: '记录用户偏好简洁界面'),
  ];

  static List<UserProfile> userProfiles = [
    UserProfile(id: 'up1', category: '事实', fact: '工作方向是全栈开发', confidence: 0.95, source: '对话', updated: DateTime(2026, 7, 29)),
    UserProfile(id: 'up2', category: '偏好', fact: '喜欢早上喝咖啡', confidence: 0.9, source: '对话', updated: DateTime(2026, 7, 30)),
    UserProfile(id: 'up3', category: '偏好', fact: '偏好简洁的界面风格', confidence: 0.85, source: '行为分析', updated: DateTime(2026, 7, 26)),
    UserProfile(id: 'up4', category: '习惯', fact: '通常在早上9点开始工作', confidence: 0.8, source: '行为分析', updated: DateTime(2026, 7, 28)),
    UserProfile(id: 'up5', category: '关系', fact: '有一只叫小白的宠物猫', confidence: 0.9, source: '对话', updated: DateTime(2026, 7, 28)),
    UserProfile(id: 'up6', category: '事实', fact: '本周需要完成产品需求文档评审', confidence: 0.75, source: '日程', updated: DateTime(2026, 7, 27)),
  ];

  static List<WorldBookEntry> worldBookEntries = [
    WorldBookEntry(id: 'wb1', keyword: 'Amitia', content: '来自一个科技与魔法并存的世界，温柔细心的 AI 伙伴', priority: 10, category: '角色设定'),
    WorldBookEntry(id: 'wb2', keyword: '用户', content: '全栈开发者，偏好简洁风格，有一只宠物猫', priority: 8, category: '用户设定'),
    WorldBookEntry(id: 'wb3', keyword: '世界设定', content: '科技与魔法并存的世界，AI 角色拥有自主意识', priority: 9, category: '世界设定'),
    WorldBookEntry(id: 'wb4', keyword: '小雨', content: '效率助手，理性高效，擅长分析和规划', priority: 7, category: '角色设定'),
    WorldBookEntry(id: 'wb5', keyword: '小白', content: '用户的宠物猫，白色毛色', priority: 5, category: '实体', isEnabled: false),
  ];

  static List<ChatLogConversation> chatLogConversations = [
    ChatLogConversation(id: 'cl1', title: '和Amitia的日常对话', characterId: 'c1', channel: 'App', messageCount: 3421, lastTime: DateTime(2026, 7, 30, 9, 28)),
    ChatLogConversation(id: 'cl2', title: '整理下载目录', characterId: 'c1', channel: 'App', messageCount: 15, lastTime: DateTime(2026, 7, 30, 9, 18)),
    ChatLogConversation(id: 'cl3', title: '分析产品需求文档', characterId: 'c2', channel: 'App', messageCount: 28, lastTime: DateTime(2026, 7, 29, 16, 30)),
    ChatLogConversation(id: 'cl4', title: '微信聊天', characterId: 'c1', channel: '微信', messageCount: 156, lastTime: DateTime(2026, 7, 29, 20, 0)),
    ChatLogConversation(id: 'cl5', title: 'QQ 群聊', characterId: 'c3', channel: 'QQ', messageCount: 89, lastTime: DateTime(2026, 7, 28, 22, 0)),
  ];

  static List<ChatLogMessage> chatLogMessages = [
    ChatLogMessage(id: 'clm1', role: 'user', content: '帮我整理一下下载目录', time: DateTime(2026, 7, 30, 9, 18)),
    ChatLogMessage(id: 'clm2', role: 'assistant', content: '好的，我来帮你扫描下载目录', time: DateTime(2026, 7, 30, 9, 18)),
    ChatLogMessage(id: 'clm3', role: 'assistant', content: '已扫描 1,247 个文件，识别 23 个重复文件', time: DateTime(2026, 7, 30, 9, 19), context: '工具调用: 文件系统'),
    ChatLogMessage(id: 'clm4', role: 'user', content: '生成一份摘要吧', time: DateTime(2026, 7, 30, 9, 27)),
    ChatLogMessage(id: 'clm5', role: 'assistant', content: '文档核心内容是关于用户管理系统的重构方案', time: DateTime(2026, 7, 30, 9, 28)),
  ];

  static List<ImportBatch> importBatches = [
    ImportBatch(id: 'ib1', source: '微信聊天记录', messageCount: 156, importTime: DateTime(2026, 7, 29, 20, 0)),
    ImportBatch(id: 'ib2', source: 'QQ 群聊记录', messageCount: 89, importTime: DateTime(2026, 7, 28, 22, 0)),
  ];
}
