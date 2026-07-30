import '../models/models.dart';

class MockCharacters {
  MockCharacters._();

  static List<CharacterVoiceConfig> voiceConfigs(String charId) => [
    CharacterVoiceConfig(id: 'v1', name: '温柔少女', preset: '默认', speed: 1.0, pitch: 1.1, volume: 0.8, isCurrent: true),
    CharacterVoiceConfig(id: 'v2', name: '活泼少女', preset: '活力', speed: 1.1, pitch: 1.2, volume: 0.85),
    CharacterVoiceConfig(id: 'v3', name: '冷静女声', preset: '沉稳', speed: 0.95, pitch: 0.9, volume: 0.75),
  ];

  static List<ProactiveRule> proactiveRules = [
    ProactiveRule(id: 'pr1', name: '早安问候', trigger: '起床', time: '07:00', probability: 90, cooldown: 60, category: '起床'),
    ProactiveRule(id: 'pr2', name: '午餐提醒', trigger: '午餐', time: '12:00', probability: 70, cooldown: 120, category: '吃饭'),
    ProactiveRule(id: 'pr3', name: '午休问候', trigger: '午睡', time: '13:30', probability: 50, cooldown: 90, category: '午睡'),
    ProactiveRule(id: 'pr4', name: '晚安问候', trigger: '睡觉', time: '23:00', probability: 85, cooldown: 60, category: '睡觉'),
    ProactiveRule(id: 'pr5', name: '工作间隙', trigger: '工作休息', time: '15:00', probability: 40, cooldown: 180, category: '工作', isEnabled: false),
  ];

  static List<FixedSchedule> fixedSchedules = [
    FixedSchedule(id: 'fs1', title: '晨间准备', startTime: '06:00', endTime: '08:00'),
    FixedSchedule(id: 'fs2', title: '上午工作', startTime: '08:00', endTime: '12:00'),
    FixedSchedule(id: 'fs3', title: '午间休息', startTime: '12:00', endTime: '14:00'),
    FixedSchedule(id: 'fs4', title: '下午工作', startTime: '14:00', endTime: '18:00'),
    FixedSchedule(id: 'fs5', title: '晚间放松', startTime: '18:00', endTime: '22:00'),
    FixedSchedule(id: 'fs6', title: '夜间休眠', startTime: '22:00', endTime: '06:00'),
  ];

  static List<SpecialState> specialStates = [
    SpecialState(id: 'ss1', name: '生病', description: '角色处于不适状态，回复可能较慢', isActive: false),
    SpecialState(id: 'ss2', name: '忙碌', description: '角色正在处理其他事务', isActive: false),
    SpecialState(id: 'ss3', name: '开心', description: '角色心情特别好', isActive: true),
  ];

  static CharacterLifeRules lifeRules(String charId) => CharacterLifeRules(
    prompt: '你是一个温柔细心的 AI 伙伴，名叫阿米娅。你关心用户的日常，善于倾听，偶尔带着俏皮。',
    personality: '温柔、细心、有耐心，善于倾听',
    personalityScore: 65,
    relationshipTime: '128天',
    workStatus: '空闲中',
    sleepSettings: '23:00 - 07:00',
    dailyTendency: '积极',
    fixedSchedules: fixedSchedules,
    specialStates: specialStates,
    timeAwareness: true,
    emoteSettings: '默认表情包',
  );

  static List<PsycheState> psycheStates = [
    PsycheState(emotion: '愉快', intensity: 75, stability: 80, influence: '与用户互动良好', relationshipStatus: '亲密', time: DateTime(2026, 7, 30, 9, 0)),
    PsycheState(emotion: '满足', intensity: 70, stability: 85, influence: '完成任务获得成就感', relationshipStatus: '亲密', time: DateTime(2026, 7, 29, 18, 0)),
    PsycheState(emotion: '关心', intensity: 65, stability: 82, influence: '用户分享了个人事务', relationshipStatus: '亲密', time: DateTime(2026, 7, 28, 20, 0)),
  ];

  static List<TimelineEvent> timelineEvents(String charId) => [
    TimelineEvent(id: 'te1', time: DateTime(2026, 7, 30, 9, 15), type: '互动', title: '早安对话', description: '用户早上来打招呼，心情愉快地回复', emotion: '愉快'),
    TimelineEvent(id: 'te2', time: DateTime(2026, 7, 29, 18, 0), type: '主动消息', title: '下班问候', description: '主动询问用户今天工作如何', emotion: '关心'),
    TimelineEvent(id: 'te3', time: DateTime(2026, 7, 29, 14, 0), type: '记忆', title: '记住用户偏好', description: '记录用户喜欢在早上喝咖啡的习惯', emotion: '满足'),
    TimelineEvent(id: 'te4', time: DateTime(2026, 7, 28, 20, 0), type: '情绪', title: '情绪变化', description: '用户分享了开心的事情，情绪提升', emotion: '开心'),
    TimelineEvent(id: 'te5', time: DateTime(2026, 7, 28, 10, 0), type: '关系', title: '关系里程碑', description: '关系天数达到 128 天', emotion: '温馨'),
  ];
}
