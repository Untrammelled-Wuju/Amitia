class ConversationDto {
  final String id;
  final String projectId;
  final String title;
  final String channel;
  final String source;
  final int messageCount;
  final String pinnedAt;
  final String archivedAt;
  final String createdAt;
  final String updatedAt;
  final int modelConfigId;
  final String reasoningEffort;

  ConversationDto({
    required this.id,
    this.projectId = '',
    this.title = '',
    this.channel = '',
    this.source = '',
    this.messageCount = 0,
    this.pinnedAt = '',
    this.archivedAt = '',
    this.createdAt = '',
    this.updatedAt = '',
    this.modelConfigId = 0,
    this.reasoningEffort = '',
  });

  factory ConversationDto.fromJson(Map<String, dynamic> json) {
    return ConversationDto(
      id: (json['id'] ?? '').toString(),
      projectId: (json['projectId'] ?? '').toString(),
      title: json['title'] as String? ?? '',
      channel: json['channel'] as String? ?? '',
      source: json['source'] as String? ?? '',
      messageCount: (json['messageCount'] as num?)?.toInt() ?? 0,
      pinnedAt: (json['pinnedAt'] ?? '').toString(),
      archivedAt: (json['archivedAt'] ?? '').toString(),
      createdAt: json['createdAt'] as String? ?? '',
      updatedAt: json['updatedAt'] as String? ?? '',
      modelConfigId: (json['modelConfigId'] as num?)?.toInt() ?? 0,
      reasoningEffort: (json['reasoningEffort'] ?? '').toString(),
    );
  }
}

class MessageDto {
  final String id;
  final String conversationId;
  final String characterId;
  final String role;
  final String content;
  final String reasoningContent;
  final int reasoningDurationMs;
  final String status;
  final String msgType;
  final String extensionType;
  final String requestId;
  final String responseGroupId;
  final int deliverySequence;
  final int sequence;
  final String createdAt;
  final String imageUrl;
  final String audioUrl;
  final double audioDuration;
  final String videoUrl;
  final String? emoteId;
  final String? altText;
  final int? tokens;
  final String? replyToMessageId;
  final String? replyToExcerpt;

  MessageDto({
    required this.id,
    required this.conversationId,
    this.characterId = '',
    required this.role,
    required this.content,
    this.reasoningContent = '',
    this.reasoningDurationMs = 0,
    required this.createdAt,
    this.status = 'sent',
    this.msgType = 'text',
    this.extensionType = '',
    this.requestId = '',
    this.responseGroupId = '',
    this.deliverySequence = 0,
    this.sequence = 0,
    this.imageUrl = '',
    this.audioUrl = '',
    this.audioDuration = 0,
    this.videoUrl = '',
    this.emoteId,
    this.altText,
    this.tokens,
    this.replyToMessageId,
    this.replyToExcerpt,
  });

  factory MessageDto.fromJson(Map<String, dynamic> json) {
    return MessageDto(
      id: (json['id'] ?? '').toString(),
      conversationId: (json['conversationId'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      role: json['role'] as String? ?? '',
      content: json['content'] as String? ?? '',
      reasoningContent: json['reasoningContent'] as String? ?? '',
      reasoningDurationMs: (json['reasoningDurationMs'] as num?)?.toInt() ?? 0,
      status: json['status'] as String? ?? 'sent',
      msgType: json['msgType'] as String? ?? 'text',
      extensionType: json['extensionType'] as String? ?? '',
      requestId: (json['requestId'] ?? '').toString(),
      responseGroupId: (json['responseGroupId'] ?? '').toString(),
      deliverySequence: (json['deliverySequence'] as num?)?.toInt() ?? 0,
      sequence: (json['sequence'] as num?)?.toInt() ?? 0,
      createdAt: json['createdAt'] as String? ?? '',
      imageUrl: json['imageUrl'] as String? ?? '',
      audioUrl: json['audioUrl'] as String? ?? '',
      audioDuration: (json['audioDuration'] as num?)?.toDouble() ?? 0,
      videoUrl: json['videoUrl'] as String? ?? '',
      emoteId: json['emoteId'] as String?,
      altText: json['altText'] as String?,
      tokens: (json['tokens'] as num?)?.toInt(),
      replyToMessageId: json['replyToMessageId'] as String?,
      replyToExcerpt: json['replyToExcerpt'] as String?,
    );
  }
}
