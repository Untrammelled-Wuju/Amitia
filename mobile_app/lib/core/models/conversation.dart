class ConversationDto {
  final String id;
  final String characterId;
  final String title;
  final String channel;
  final String source;
  final int messageCount;
  final String createdAt;
  final String updatedAt;

  ConversationDto({
    required this.id,
    required this.characterId,
    this.title = '',
    this.channel = '',
    this.source = '',
    this.messageCount = 0,
    this.createdAt = '',
    this.updatedAt = '',
  });

  factory ConversationDto.fromJson(Map<String, dynamic> json) {
    return ConversationDto(
      id: (json['id'] ?? '').toString(),
      characterId: (json['characterId'] ?? '').toString(),
      title: json['title'] as String? ?? '',
      channel: json['channel'] as String? ?? '',
      source: json['source'] as String? ?? '',
      messageCount: (json['messageCount'] as num?)?.toInt() ?? 0,
      createdAt: json['createdAt'] as String? ?? '',
      updatedAt: json['updatedAt'] as String? ?? '',
    );
  }
}

class MessageDto {
  final String id;
  final String conversationId;
  final String role;
  final String content;
  final String status;
  final String msgType;
  final String createdAt;
  final String imageUrl;
  final String audioUrl;
  final double audioDuration;
  final String videoUrl;
  final String? emoteId;
  final String? altText;
  final int? tokens;

  MessageDto({
    required this.id,
    required this.conversationId,
    required this.role,
    required this.content,
    required this.createdAt,
    this.status = 'sent',
    this.msgType = 'text',
    this.imageUrl = '',
    this.audioUrl = '',
    this.audioDuration = 0,
    this.videoUrl = '',
    this.emoteId,
    this.altText,
    this.tokens,
  });

  factory MessageDto.fromJson(Map<String, dynamic> json) {
    return MessageDto(
      id: (json['id'] ?? '').toString(),
      conversationId: (json['conversationId'] ?? '').toString(),
      role: json['role'] as String? ?? '',
      content: json['content'] as String? ?? '',
      status: json['status'] as String? ?? 'sent',
      msgType: json['msgType'] as String? ?? 'text',
      createdAt: json['createdAt'] as String? ?? '',
      imageUrl: json['imageUrl'] as String? ?? '',
      audioUrl: json['audioUrl'] as String? ?? '',
      audioDuration: (json['audioDuration'] as num?)?.toDouble() ?? 0,
      videoUrl: json['videoUrl'] as String? ?? '',
      emoteId: json['emoteId'] as String?,
      altText: json['altText'] as String?,
      tokens: (json['tokens'] as num?)?.toInt(),
    );
  }
}
