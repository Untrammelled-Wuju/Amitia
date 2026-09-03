class EpisodicDto {
  final String id;
  final String userId;
  final String sceneType;
  final String title;
  final String content;
  final String contextBefore;
  final String contextAfter;
  final String triggerKeywords;
  final int sentimentScore;
  final String messageIdStart;
  final String messageIdEnd;
  final String messageTimeStart;
  final String messageTimeEnd;
  final String sourceConvId;
  final String createdAt;
  final String updatedAt;
  final int retentionLevel;
  final double memoryStrength;
  final String strengthUpdatedAt;
  final String lastReinforcedAt;
  final int reinforceCount;
  final String decayState;
  final String archivedAt;

  const EpisodicDto({
    required this.id,
    this.userId = '',
    this.sceneType = '',
    this.title = '',
    this.content = '',
    this.contextBefore = '',
    this.contextAfter = '',
    this.triggerKeywords = '',
    this.sentimentScore = 0,
    this.messageIdStart = '',
    this.messageIdEnd = '',
    this.messageTimeStart = '',
    this.messageTimeEnd = '',
    this.sourceConvId = '',
    this.createdAt = '',
    this.updatedAt = '',
    this.retentionLevel = 4,
    this.memoryStrength = 0.5,
    this.strengthUpdatedAt = '',
    this.lastReinforcedAt = '',
    this.reinforceCount = 0,
    this.decayState = 'active',
    this.archivedAt = '',
  });

  factory EpisodicDto.fromJson(Map<String, dynamic> json) => EpisodicDto(
        id: (json['id'] ?? '').toString(),
        userId: (json['userId'] ?? '').toString(),
        sceneType: (json['sceneType'] ?? '').toString(),
        title: (json['title'] ?? '').toString(),
        content: (json['content'] ?? '').toString(),
        contextBefore: (json['contextBefore'] ?? '').toString(),
        contextAfter: (json['contextAfter'] ?? '').toString(),
        triggerKeywords: (json['triggerKeywords'] ?? '').toString(),
        sentimentScore: (json['sentimentScore'] as num?)?.toInt() ?? 0,
        messageIdStart: (json['messageIdStart'] ?? '').toString(),
        messageIdEnd: (json['messageIdEnd'] ?? '').toString(),
        messageTimeStart: (json['messageTimeStart'] ?? '').toString(),
        messageTimeEnd: (json['messageTimeEnd'] ?? '').toString(),
        sourceConvId: (json['sourceConvId'] ?? '').toString(),
        createdAt: (json['createdAt'] ?? '').toString(),
        updatedAt: (json['updatedAt'] ?? '').toString(),
        retentionLevel: (json['retentionLevel'] as num?)?.toInt() ?? 4,
        memoryStrength: (json['memoryStrength'] as num?)?.toDouble() ?? 0.5,
        strengthUpdatedAt: (json['strengthUpdatedAt'] ?? '').toString(),
        lastReinforcedAt: (json['lastReinforcedAt'] ?? '').toString(),
        reinforceCount: (json['reinforceCount'] as num?)?.toInt() ?? 0,
        decayState: (json['decayState'] ?? 'active').toString(),
        archivedAt: (json['archivedAt'] ?? '').toString(),
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'userId': userId,
        'sceneType': sceneType,
        'title': title,
        'content': content,
        'contextBefore': contextBefore,
        'contextAfter': contextAfter,
        'triggerKeywords': triggerKeywords,
        'sentimentScore': sentimentScore,
        'messageIdStart': messageIdStart,
        'messageIdEnd': messageIdEnd,
        'messageTimeStart': messageTimeStart,
        'messageTimeEnd': messageTimeEnd,
        'sourceConvId': sourceConvId,
        'createdAt': createdAt,
        'updatedAt': updatedAt,
        'retentionLevel': retentionLevel,
        'memoryStrength': memoryStrength,
        'strengthUpdatedAt': strengthUpdatedAt,
        'lastReinforcedAt': lastReinforcedAt,
        'reinforceCount': reinforceCount,
        'decayState': decayState,
        'archivedAt': archivedAt,
      };
}
