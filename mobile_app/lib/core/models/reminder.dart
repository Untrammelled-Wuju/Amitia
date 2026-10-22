class ReminderDto {
  final int generation;
  final String ownerId;
  final String executionMode;
  final String source;
  final String state;
  final String localRemindAt;
  final String timezone;
  final String localAnchorAt;
  final String endAt;
  final int maxOccurrences;
  final bool paused;
  final bool systemNotification;
  final Map<String, dynamic> recurrence;
  final Map<String, dynamic> policy;
  final List<dynamic> channelTargets;
  final String id;
  final String title;
  final String content;
  final String channel;
  final String conversationId;
  final String characterId;
  final String remindAt;
  final String repeatRule;
  final int enabled;
  final String lastTriggeredAt;
  final String createdAt;
  final String updatedAt;
  final String conversationTitle;
  final String characterName;

  const ReminderDto({
    required this.id,
    this.generation = 0,
    this.ownerId = '',
    this.executionMode = '',
    this.source = '',
    this.state = '',
    this.localRemindAt = '',
    this.timezone = 'Asia/Shanghai',
    this.localAnchorAt = '',
    this.endAt = '',
    this.maxOccurrences = 0,
    this.paused = false,
    this.systemNotification = true,
    this.recurrence = const {},
    this.policy = const {},
    this.channelTargets = const [],
    this.title = '',
    this.content = '',
    this.channel = 'web',
    this.conversationId = '',
    this.characterId = '',
    this.remindAt = '',
    this.repeatRule = 'none',
    this.enabled = 1,
    this.lastTriggeredAt = '',
    this.createdAt = '',
    this.updatedAt = '',
    this.conversationTitle = '',
    this.characterName = '',
  });

  bool get isEnabled => enabled == 1;

  factory ReminderDto.fromJson(Map<String, dynamic> json) => ReminderDto(
    id: (json['id'] ?? '').toString(),
    generation: (json['generation'] as num?)?.toInt() ?? 0,
    ownerId: (json['ownerId'] ?? '').toString(),
    executionMode: (json['executionMode'] ?? '').toString(),
    source: (json['source'] ?? '').toString(),
    state: (json['state'] ?? '').toString(),
    localRemindAt: (json['localRemindAt'] ?? '').toString(),
    timezone: (json['timezone'] ?? '').toString(),
    localAnchorAt: (json['localAnchorAt'] ?? '').toString(),
    endAt: (json['endAt'] ?? '').toString(),
    maxOccurrences: (json['maxOccurrences'] as num?)?.toInt() ?? 0,
    paused: json['paused'] == true,
    systemNotification: json['systemNotification'] == true,
    recurrence: Map<String, dynamic>.from(json['recurrence'] as Map? ?? {}),
    policy: Map<String, dynamic>.from(json['policy'] as Map? ?? {}),
    channelTargets: json['channelTargets'] as List? ?? const [],
    title: (json['title'] ?? '').toString(),
    content: (json['content'] ?? '').toString(),
    channel: (json['channel'] ?? 'web').toString(),
    conversationId: (json['conversationId'] ?? '').toString(),
    characterId: (json['characterId'] ?? '').toString(),
    remindAt: (json['remindAt'] ?? '').toString(),
    repeatRule: (json['repeatRule'] ?? 'none').toString(),
    enabled: json['enabled'] is bool
        ? (json['enabled'] == true ? 1 : 0)
        : (json['enabled'] as num?)?.toInt() ?? 1,
    lastTriggeredAt: (json['lastTriggeredAt'] ?? '').toString(),
    createdAt: (json['createdAt'] ?? '').toString(),
    updatedAt: (json['updatedAt'] ?? '').toString(),
    conversationTitle: (json['conversationTitle'] ?? '').toString(),
    characterName: (json['characterName'] ?? '').toString(),
  );

  Map<String, dynamic> toJson() => {
    'id': id,
    'generation': generation,
    'ownerId': ownerId,
    'executionMode': executionMode,
    'source': source,
    'state': state,
    'timezone': timezone,
    'localAnchorAt': localAnchorAt,
    'localRemindAt': localRemindAt,
    'endAt': endAt,
    'maxOccurrences': maxOccurrences,
    'paused': paused,
    'systemNotification': systemNotification,
    'recurrence': recurrence,
    'policy': policy,
    'channelTargets': channelTargets,
    'title': title,
    'content': content,
    'channel': channel,
    'conversationId': conversationId,
    'characterId': characterId,
    'remindAt': remindAt,
    'repeatRule': repeatRule,
    'enabled': enabled,
    'lastTriggeredAt': lastTriggeredAt,
    'createdAt': createdAt,
    'updatedAt': updatedAt,
    'conversationTitle': conversationTitle,
    'characterName': characterName,
  };
}
