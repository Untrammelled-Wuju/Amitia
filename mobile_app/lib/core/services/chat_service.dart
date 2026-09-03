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

class ConversationWorkspaceDto {
  final String conversationId;
  final String workspaceId;
  final String deviceId;
  final String workspaceName;
  final String workspaceKind;
  final String rootUri;

  const ConversationWorkspaceDto({
    this.conversationId = '',
    required this.workspaceId,
    this.deviceId = '',
    this.workspaceName = '',
    this.workspaceKind = 'local',
    required this.rootUri,
  });

  factory ConversationWorkspaceDto.fromJson(Map<String, dynamic> json) {
    final workspaceId = (json['workspaceId'] ?? '').toString().trim();
    return ConversationWorkspaceDto(
      conversationId: (json['conversationId'] ?? '').toString().trim(),
      workspaceId: workspaceId,
      deviceId: (json['deviceId'] ?? '').toString().trim(),
      workspaceName: (json['workspaceName'] ?? '').toString().trim(),
      workspaceKind: (json['workspaceKind'] ?? 'local').toString().trim(),
      rootUri: (json['rootUri'] ?? 'amitia://workspace/@$workspaceId/').toString().trim(),
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
        'workspaceId': workspaceId,
        'deviceId': deviceId,
        'workspaceName': workspaceName,
        'workspaceKind': workspaceKind,
        'rootUri': rootUri,
      };
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

  Future<ConversationWorkspaceDto?> conversationWorkspace(String conversationId) async {
    final id = conversationId.trim();
    if (id.isEmpty) return null;
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/conversations/${Uri.encodeComponent(id)}/workspace',
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    if (resp == null || (resp['workspaceId'] ?? '').toString().trim().isEmpty) {
      return null;
    }
    return ConversationWorkspaceDto.fromJson(resp);
  }

  Future<ConversationWorkspaceDto> setConversationWorkspace(
    String conversationId,
    ConversationWorkspaceDto workspace,
  ) async {
    final id = conversationId.trim();
    if (id.isEmpty) throw ArgumentError('conversationId 不能为空');
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/web-chat/conversations/${Uri.encodeComponent(id)}/workspace',
      data: workspace.toJson(),
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    if (resp == null) throw StateError('保存工作目录失败：后端未返回结果');
    return ConversationWorkspaceDto.fromJson(resp);
  }

  Future<void> clearConversationWorkspace(String conversationId) async {
    final id = conversationId.trim();
    if (id.isEmpty) return;
    await _api.delete(
      '/api/web-chat/conversations/${Uri.encodeComponent(id)}/workspace',
    );
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

  Future<void> renameConversation(String id, String title) async {
    final trimmed = title.trim();
    if (trimmed.isEmpty) throw ArgumentError('会话标题不能为空');
    await _api.put<Map<String, dynamic>>(
      '/api/web-chat/conversations/$id',
      data: {'title': trimmed},
    );
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

  Future<Map<String, dynamic>?> updateConversationSummary(String conversationId, String summaryText) {
    return _api.put<Map<String, dynamic>>(
      '/api/chats/conversations/$conversationId/summary',
      data: {'summaryText': summaryText.trim()},
    );
  }

  Future<String> exportConversation(String conversationId, {String format = 'markdown'}) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chats/export',
      data: {
        'format': format == 'json' ? 'json' : 'markdown',
        'conversationIds': [conversationId],
      },
    );
    return (resp?['exportUrl'] ?? '').toString();
  }

  Future<List<Map<String, dynamic>>> messageFeedback(String messageId) async {
    final resp = await _api.get<dynamic>('/api/messages/$messageId/feedback');
    return _mapList(resp);
  }

  Future<List<Map<String, dynamic>>> recentFeedback({int limit = 100}) async {
    final resp = await _api.get<dynamic>('/api/messages/feedback/recent', queryParameters: {'limit': limit});
    return _mapList(resp, keys: const ['items']);
  }

  Future<Map<String, dynamic>?> contextPreview(String conversationId) {
    return _api.get<Map<String, dynamic>>(
      '/api/agent/context-preview',
      queryParameters: {'conversationId': conversationId},
    );
  }

  Future<Map<String, dynamic>?> messagePsyche(String messageId) {
    return _api.get<Map<String, dynamic>>('/api/psyche/messages/$messageId');
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
    ConversationWorkspaceDto? workspace,
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
        if (workspace != null) ...<String, dynamic>{
          'workspaceId': workspace.workspaceId,
          'workspaceDeviceId': workspace.deviceId,
          'workspaceName': workspace.workspaceName,
          'workspaceKind': workspace.workspaceKind,
          'workspaceRootUri': workspace.rootUri,
        },
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

  List<Map<String, dynamic>> _mapList(dynamic resp, {List<String> keys = const []}) {
    dynamic raw = resp;
    if (raw is Map) {
      for (final key in keys) {
        if (raw[key] is List) { raw = raw[key]; break; }
      }
    }
    if (raw is! List) return const [];
    return raw.whereType<Map>().map((row) => Map<String, dynamic>.from(row)).toList(growable: false);
  }
}
