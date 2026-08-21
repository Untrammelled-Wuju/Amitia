import '../backend_transport/backend_service_api.dart';
import '../models/conversation.dart';

class ChatSubmitResult {
  final String conversationId;
  final String userMessageId;
  final String status;
  final int mergeWindowMs;

  const ChatSubmitResult({
    required this.conversationId,
    required this.userMessageId,
    required this.status,
    required this.mergeWindowMs,
  });

  factory ChatSubmitResult.fromJson(Map<String, dynamic> json) {
    return ChatSubmitResult(
      conversationId: (json['conversationId'] ?? '').toString(),
      userMessageId: (json['userMessageId'] ?? '').toString(),
      status: (json['status'] ?? '').toString(),
      mergeWindowMs: (json['mergeWindowMs'] as num?)?.toInt() ?? 0,
    );
  }
}

class ChatService {
  final BackendServiceApi _api;

  ChatService(this._api);

  Future<List<ConversationDto>> listConversations() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/conversations',
      queryParameters: const {'page': 1, 'pageSize': 200},
    );
    final rows = resp?['items'];
    if (rows is! List) return const [];
    return rows
        .whereType<Map>()
        .map((row) => ConversationDto.fromJson(Map<String, dynamic>.from(row)))
        .toList(growable: false);
  }

  Future<ConversationDto?> createConversation(String? characterId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/web-chat/conversations',
      data: {
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        'channel': 'web',
        'source': 'mobile',
      },
    );
    if (resp == null) return null;
    return ConversationDto.fromJson(resp);
  }

  Future<List<MessageDto>> getMessages(
    String conversationId, {
    int page = 1,
    int pageSize = 200,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/conversations/$conversationId/messages',
      queryParameters: {'page': page, 'pageSize': pageSize},
    );
    final rows = resp?['items'];
    if (rows is! List) return const [];
    return rows
        .whereType<Map>()
        .map((row) => MessageDto.fromJson(Map<String, dynamic>.from(row)))
        .toList(growable: false);
  }

  Future<bool> deleteConversation(String id) async {
    await _api.delete('/api/web-chat/conversations/$id');
    return true;
  }

  Future<void> deleteMessages(String conversationId) async {
    await _api.delete('/api/chats/conversations/$conversationId/messages');
  }

  Future<void> deleteMessage(String messageId) async {
    await _api.delete('/api/chats/messages/$messageId');
  }

  Future<void> deleteAllConversations() async {
    await _api.delete('/api/chats/all');
  }

  Future<ConversationDto?> changeCharacter(String conversationId, String characterId) async {
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/chats/conversations/$conversationId/character',
      data: {'characterId': characterId},
    );
    if (resp == null) return null;
    return ConversationDto.fromJson(resp);
  }

  Future<Map<String, dynamic>?> conversationSummary(String conversationId) async {
    return _api.get<Map<String, dynamic>>('/api/chats/conversations/$conversationId/summary');
  }

  Future<Map<String, dynamic>?> generateConversationSummary(String conversationId) async {
    return _api.post<Map<String, dynamic>>('/api/chats/conversations/$conversationId/summary/generate');
  }

  Future<void> deleteConversationSummary(String conversationId) async {
    await _api.delete('/api/chats/conversations/$conversationId/summary');
  }

  Future<List<MessageDto>> searchMessages(String keyword, {String? conversationId, int page = 1, int pageSize = 50}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/chats/search',
      queryParameters: {
        'keyword': keyword,
        'page': page,
        'pageSize': pageSize,
        if (conversationId != null && conversationId.isNotEmpty) 'conversationId': conversationId,
      },
    );
    final rows = resp?['items'];
    if (rows is! List) return const [];
    return rows
        .whereType<Map>()
        .map((row) => MessageDto.fromJson(Map<String, dynamic>.from(row)))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>?> messageStatus(String messageId) async {
    return _api.get<Map<String, dynamic>>('/api/web-chat/message-status/$messageId');
  }

  Future<ChatSubmitResult> submitMessage({
    required String message,
    String? conversationId,
    String? characterId,
    String? imageUrl,
    String? audioUrl,
    double audioDuration = 0,
    String? videoUrl,
    String? replyToMessageId,
  }) async {
    final now = DateTime.now().microsecondsSinceEpoch;
    final requestId = 'mobile-$now';
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/web-chat/messages',
      data: {
        'message': message,
        if (conversationId != null && conversationId.isNotEmpty)
          'conversationId': conversationId,
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
        if (imageUrl != null && imageUrl.isNotEmpty) 'imageUrl': imageUrl,
        if (audioUrl != null && audioUrl.isNotEmpty) ...{
          'audioUrl': audioUrl,
          'audioDuration': audioDuration,
          'voiceMessage': true,
        },
        if (videoUrl != null && videoUrl.isNotEmpty) 'videoUrl': videoUrl,
        if (replyToMessageId != null && replyToMessageId.isNotEmpty)
          'replyToMessageId': replyToMessageId,
        'source': 'mobile',
        'requestId': requestId,
        'clientMessageId': requestId,
        'deviceTimezone': DateTime.now().timeZoneName,
      },
    );
    if (resp == null) {
      throw StateError('消息提交未返回结果');
    }
    return ChatSubmitResult.fromJson(resp);
  }

  Future<String> generationStatus(String conversationId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/conversations/$conversationId/generations/current/status',
    );
    return (resp?['status'] ?? 'idle').toString();
  }

  Future<void> cancelGeneration(String conversationId) async {
    await _api.post<Map<String, dynamic>>(
      '/api/web-chat/conversations/$conversationId/generations/current/cancel',
    );
  }

  Future<Map<String, dynamic>?> regenerate(String conversationId) {
    return _api.post<Map<String, dynamic>>(
      '/api/web-chat/conversations/$conversationId/regenerate',
    );
  }

  /// Compatibility helper for callers that still need a single blocking reply.
  Future<Map<String, dynamic>?> chat(
    String message, {
    String? conversationId,
    String? characterId,
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/web-chat/send',
      data: {
        'message': message,
        if (conversationId != null) 'conversationId': conversationId,
        if (characterId != null) 'characterId': characterId,
        'source': 'mobile',
      },
    );
    return resp;
  }
}
