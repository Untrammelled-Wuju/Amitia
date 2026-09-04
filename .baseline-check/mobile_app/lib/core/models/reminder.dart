class ReminderDto {
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
