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
  final int reasoningEnabled;
  final String permissionMode;

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
    this.reasoningEnabled = -1,
    this.permissionMode = 'request_approval',
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
      reasoningEnabled: (json['reasoningEnabled'] as num?)?.toInt() ?? -1,
      permissionMode: (json['permissionMode'] ?? 'request_approval').toString(),
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

class MessagePageDto {
  final List<MessageDto> items;
  final int page;
  final int pageSize;
  final int total;
  final int totalPages;

  const MessagePageDto({
    required this.items,
    required this.page,
    required this.pageSize,
    required this.total,
    required this.totalPages,
  });

  bool get hasPreviousPage => page > 1;
}

class AssistantTurnItemDto {
  final String id;
  final String turnId;
  final String conversationId;
  final int sequence;
  final String type;
  final String status;
  final String callId;
  final String toolName;
  final String content;
  final String argumentsJson;
  final String resultJson;
  final String errorCode;
  final int durationMs;
  final bool isFinal;
  final String legacyMessageId;
  final String createdAt;
  final String updatedAt;

  const AssistantTurnItemDto({
    required this.id,
    this.turnId = '',
    this.conversationId = '',
    this.sequence = 0,
    this.type = '',
    this.status = '',
    this.callId = '',
    this.toolName = '',
    this.content = '',
    this.argumentsJson = '',
    this.resultJson = '',
    this.errorCode = '',
    this.durationMs = 0,
    this.isFinal = false,
    this.legacyMessageId = '',
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory AssistantTurnItemDto.fromJson(Map<String, dynamic> json) {
    return AssistantTurnItemDto(
      id: (json['id'] ?? '').toString(),
      turnId: (json['turnId'] ?? '').toString(),
      conversationId: (json['conversationId'] ?? '').toString(),
      sequence: (json['sequence'] as num?)?.toInt() ?? 0,
      type: (json['type'] ?? '').toString(),
      status: (json['status'] ?? '').toString(),
      callId: (json['callId'] ?? '').toString(),
      toolName: (json['toolName'] ?? '').toString(),
      content: (json['content'] ?? '').toString(),
      argumentsJson: (json['argumentsJson'] ?? '').toString(),
      resultJson: (json['resultJson'] ?? '').toString(),
      errorCode: (json['errorCode'] ?? '').toString(),
      durationMs: (json['durationMs'] as num?)?.toInt() ?? 0,
      isFinal: (json['isFinal'] as num?)?.toInt() == 1,
      legacyMessageId: (json['legacyMessageId'] ?? '').toString(),
      createdAt: (json['createdAt'] ?? '').toString(),
      updatedAt: (json['updatedAt'] ?? '').toString(),
    );
  }
}

class AssistantTurnDto {
  final String id;
  final String conversationId;
  final String characterId;
  final String userMessageId;
  final String requestId;
  final String responseGroupId;
  final int sequence;
  final String status;
  final String createdAt;
  final String updatedAt;
  final String completedAt;
  final List<AssistantTurnItemDto> items;

  const AssistantTurnDto({
    required this.id,
    this.conversationId = '',
    this.characterId = '',
    this.userMessageId = '',
    this.requestId = '',
    this.responseGroupId = '',
    this.sequence = 0,
    this.status = '',
    this.createdAt = '',
    this.updatedAt = '',
    this.completedAt = '',
    this.items = const <AssistantTurnItemDto>[],
  });

  factory AssistantTurnDto.fromJson(Map<String, dynamic> json) {
    final rawItems = json['items'];
    final items = rawItems is List
        ? rawItems
              .whereType<Map>()
              .map(
                (item) => AssistantTurnItemDto.fromJson(
                  Map<String, dynamic>.from(item),
                ),
              )
              .toList()
        : const <AssistantTurnItemDto>[];
    if (items.length > 1) {
      items.sort((left, right) => left.sequence.compareTo(right.sequence));
    }
    return AssistantTurnDto(
      id: (json['id'] ?? '').toString(),
      conversationId: (json['conversationId'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      userMessageId: (json['userMessageId'] ?? '').toString(),
      requestId: (json['requestId'] ?? '').toString(),
      responseGroupId: (json['responseGroupId'] ?? '').toString(),
      sequence: (json['sequence'] as num?)?.toInt() ?? 0,
      status: (json['status'] ?? '').toString(),
      createdAt: (json['createdAt'] ?? '').toString(),
      updatedAt: (json['updatedAt'] ?? '').toString(),
      completedAt: (json['completedAt'] ?? '').toString(),
      items: items,
    );
  }
}
