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
      };
}
