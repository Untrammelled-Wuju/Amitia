import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';

import '../backend_transport/backend_service_api.dart';
import '../models/conversation.dart';
import '../models/project.dart';
import '../native_bridge/device_timezone_cache.dart';

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

class ChatStreamCancellation {
  final CancelToken _token = CancelToken();

  CancelToken get token => _token;
  bool get isCancelled => _token.isCancelled;

  void cancel([String reason = 'cancelled']) {
    if (!_token.isCancelled) _token.cancel(reason);
  }
}

class ChatStreamEvent {
  final String type;
  final Map<String, dynamic> data;

  const ChatStreamEvent(this.type, this.data);
}

class ConversationWorkspaceDto {
  final String conversationId;
  final String projectId;
  final String workspaceId;
  final String deviceId;
  final String workspaceName;
  final String workspaceKind;
  final String rootUri;

  const ConversationWorkspaceDto({
    this.conversationId = '',
    this.projectId = '',
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
      projectId: (json['projectId'] ?? '').toString().trim(),
      workspaceId: workspaceId,
      deviceId: (json['deviceId'] ?? '').toString().trim(),
      workspaceName: (json['workspaceName'] ?? '').toString().trim(),
      workspaceKind: (json['workspaceKind'] ?? 'local').toString().trim(),
      rootUri: (json['rootUri'] ?? 'amitia://workspace/@$workspaceId/')
          .toString()
          .trim(),
    );
  }

  Map<String, dynamic> toJson() => <String, dynamic>{
    'workspaceId': workspaceId,
    'projectId': projectId,
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

  Future<List<ConversationDto>> archivedConversations({
    String projectId = '',
    String keyword = '',
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/conversations',
      queryParameters: {
        'page': 1,
        'pageSize': 200,
        'archivedOnly': true,
        if (projectId.trim().isNotEmpty) 'projectId': projectId.trim(),
        if (keyword.trim().isNotEmpty) 'keyword': keyword.trim(),
      },
    );
    final rows = resp?['items'];
    if (rows is! List) return const [];
    return rows
        .whereType<Map>()
        .map((row) => ConversationDto.fromJson(Map<String, dynamic>.from(row)))
        .where((conversation) => conversation.channel == 'web')
        .toList(growable: false);
  }

  Future<ConversationDto?> createConversation({String? projectId}) async {
    final id = projectId?.trim() ?? '';
    final path = id.isEmpty
        ? '/api/web-chat/conversations'
        : '/api/web-chat/projects/${Uri.encodeComponent(id)}/conversations';
    final resp = await _api.post<Map<String, dynamic>>(
      path,
      data: {'projectId': id, 'channel': 'web', 'source': 'mobile'},
    );
    if (resp == null) return null;
    return ConversationDto.fromJson(resp);
  }

  Future<ConversationDto?> getConversation(String conversationId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/conversations/$conversationId',
    );
    return resp == null ? null : ConversationDto.fromJson(resp);
  }

  Future<void> updateConversationModelSettings(
    String conversationId, {
    required int modelConfigId,
    required String reasoningEffort,
  }) async {
    await _api.put<Map<String, dynamic>>(
      '/api/web-chat/conversations/$conversationId',
      data: <String, dynamic>{
        'modelConfigId': modelConfigId,
        'reasoningEffort': reasoningEffort,
      },
    );
  }

  Future<ConversationSidebarDto> conversationSidebar() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/sidebar',
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    return ConversationSidebarDto.fromJson(resp ?? const <String, dynamic>{});
  }

  Future<List<ConversationDto>> channelConversations() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/channels',
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    final rows = resp?['items'];
    if (rows is! List) return const [];
    return rows
        .whereType<Map>()
        .map((row) => ConversationDto.fromJson(Map<String, dynamic>.from(row)))
        .toList(growable: false);
  }

  Future<ProjectDto> createProject({
    required String name,
    required String workspaceId,
    String deviceId = '',
    String rootUri = '',
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/web-chat/projects',
      data: <String, dynamic>{
        'name': name.trim(),
        'workspaceId': workspaceId.trim(),
        'deviceId': deviceId.trim(),
        'rootUri': rootUri.trim(),
      },
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    if (resp == null) throw StateError('创建项目失败：后端未返回结果');
    return ProjectDto.fromJson(resp);
  }

  Future<void> deleteProject(String projectId) async {
    await _api.delete(
      '/api/web-chat/projects/${Uri.encodeComponent(projectId)}',
    );
  }

  Future<void> updateProject(
    String projectId, {
    String? name,
    String? workspaceId,
    String? deviceId,
    String? rootUri,
    bool? pinned,
  }) async {
    await _api.patch<Map<String, dynamic>>(
      '/api/web-chat/projects/${Uri.encodeComponent(projectId)}',
      data: <String, dynamic>{
        if (name != null) 'name': name.trim(),
        if (workspaceId != null) 'workspaceId': workspaceId.trim(),
        if (deviceId != null) 'deviceId': deviceId.trim(),
        if (rootUri != null) 'rootUri': rootUri.trim(),
        if (pinned != null) 'pinned': pinned,
      },
    );
  }

  Future<Map<String, dynamic>> projectLocation(String projectId) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/projects/${Uri.encodeComponent(projectId)}/location',
      fromJson: (e) => Map<String, dynamic>.from(e as Map),
    );
    return resp ?? <String, dynamic>{};
  }

  Future<void> moveConversationToProject(
    String conversationId,
    String projectId,
  ) async {
    await _api.put<Map<String, dynamic>>(
      '/api/web-chat/conversations/${Uri.encodeComponent(conversationId)}',
      data: <String, dynamic>{'projectId': projectId.trim()},
    );
  }

  Future<List<MessageDto>> getMessages(
    String conversationId, {
    int page = 1,
    int pageSize = 200,
    bool latest = false,
  }) async {
    final first = await _getMessagesPage(conversationId, page, pageSize);
    if (!latest || page != 1) return first.items;
    final totalPages = first.totalPages > 1 ? first.totalPages : 1;
    if (totalPages <= 1) return first.items;
    final last = await _getMessagesPage(conversationId, totalPages, pageSize);
    return last.items;
  }

  Future<({List<MessageDto> items, int totalPages})> _getMessagesPage(
    String conversationId,
    int page,
    int pageSize,
  ) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/web-chat/conversations/$conversationId/messages',
      queryParameters: {'page': page, 'pageSize': pageSize},
    );
    final rows = resp?['items'];
    final items = rows is! List
        ? const <MessageDto>[]
        : rows
              .whereType<Map>()
              .map((row) => MessageDto.fromJson(Map<String, dynamic>.from(row)))
              .toList(growable: false);
    return (
      items: items,
      totalPages: (resp?['totalPages'] as num?)?.toInt() ?? 0,
    );
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

  Future<void> setConversationPinned(String id, bool pinned) async {
    await _api.put<Map<String, dynamic>>(
      '/api/web-chat/conversations/$id',
      data: {'pinned': pinned},
    );
  }

  Future<void> archiveConversation(String id) async {
    await _api.put<Map<String, dynamic>>(
      '/api/web-chat/conversations/$id',
      data: {'archived': true},
    );
  }

  Future<void> restoreArchivedConversation(String id) async {
    await _api.put<Map<String, dynamic>>(
      '/api/web-chat/conversations/$id',
      data: {'archived': false},
    );
  }

  Future<void> deleteMessages(String conversationId) async {
    await _api.delete('/api/chats/conversations/$conversationId/messages');
  }

  Future<void> deleteMessage(String messageId) async {
    await _api.delete('/api/chats/messages/$messageId');
  }

  Future<void> updateMessage(String messageId, String content) async {
    await _api.put<Map<String, dynamic>>(
      '/api/web-chat/messages/$messageId',
      data: {'content': content.trim()},
    );
  }

  Future<void> deleteAllConversations() async {
    await _api.delete('/api/chats/all');
  }

  Future<Map<String, dynamic>?> conversationSummary(
    String conversationId,
  ) async {
    return _api.get<Map<String, dynamic>>(
      '/api/chats/conversations/$conversationId/summary',
    );
  }

  Future<Map<String, dynamic>?> generateConversationSummary(
    String conversationId,
  ) async {
    return _api.post<Map<String, dynamic>>(
      '/api/chats/conversations/$conversationId/summary/generate',
    );
  }

  Future<void> deleteConversationSummary(String conversationId) async {
    await _api.delete('/api/chats/conversations/$conversationId/summary');
  }

  Future<Map<String, dynamic>?> updateConversationSummary(
    String conversationId,
    String summaryText,
  ) {
    return _api.put<Map<String, dynamic>>(
      '/api/chats/conversations/$conversationId/summary',
      data: {'summaryText': summaryText.trim()},
    );
  }

  Future<String> exportConversation(
    String conversationId, {
    String format = 'markdown',
  }) async {
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
    final resp = await _api.get<dynamic>(
      '/api/messages/feedback/recent',
      queryParameters: {'limit': limit},
    );
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

  Future<List<MessageDto>> searchMessages(
    String keyword, {
    String? conversationId,
    int page = 1,
    int pageSize = 50,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/chats/search',
      queryParameters: {
        'keyword': keyword,
        'page': page,
        'pageSize': pageSize,
        if (conversationId != null && conversationId.isNotEmpty)
          'conversationId': conversationId,
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
    return _api.get<Map<String, dynamic>>(
      '/api/web-chat/message-status/$messageId',
    );
  }

  ChatStreamCancellation createStreamCancellation() => ChatStreamCancellation();

  Stream<ChatStreamEvent> submitMessageStream({
    required String message,
    String? clientMessageId,
    String? conversationId,
    String? characterId,
    String? imageUrl,
    String? audioUrl,
    double audioDuration = 0,
    String? videoUrl,
    String? replyToMessageId,
    int? modelConfigId,
    String? reasoningEffort,
    ConversationWorkspaceDto? workspace,
    required ChatStreamCancellation cancellation,
  }) async* {
    final now = DateTime.now().microsecondsSinceEpoch;
    final normalizedClientMessageId = clientMessageId?.trim() ?? '';
    final requestId = normalizedClientMessageId.isEmpty
        ? 'mobile-$now'
        : normalizedClientMessageId;
    final stream = await _api.postStream(
      '/api/web-chat/send-stream',
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
        if (modelConfigId != null && modelConfigId > 0)
          'modelConfigId': modelConfigId,
        if (reasoningEffort != null && reasoningEffort.isNotEmpty)
          'reasoningEffort': reasoningEffort,
        if (workspace != null) ...<String, dynamic>{
          if (workspace.projectId.isNotEmpty) 'projectId': workspace.projectId,
          'workspaceId': workspace.workspaceId,
          'workspaceDeviceId': workspace.deviceId,
          'workspaceName': workspace.workspaceName,
          'workspaceKind': workspace.workspaceKind,
          'workspaceRootUri': workspace.rootUri,
        },
        'source': 'mobile',
        'requestId': requestId,
        'clientMessageId': requestId,
        if (DeviceTimezoneCache.hasValue)
          'deviceTimezone': DeviceTimezoneCache.ianaTimezone,
      },
      headers: const {'Accept': 'text/event-stream'},
      cancelToken: cancellation.token,
    );
    yield* _decodeEventStream(stream);
  }

  Stream<ChatStreamEvent> messageEvents({
    required ChatStreamCancellation cancellation,
  }) async* {
    final stream = await _api.getStream(
      '/api/messages/events',
      queryParameters: const {'channel': 'web'},
      headers: const {'Accept': 'text/event-stream'},
      cancelToken: cancellation.token,
    );
    yield* _decodeEventStream(stream);
  }

  Stream<ChatStreamEvent> _decodeEventStream(Stream<List<int>> source) async* {
    final text = source.transform(utf8.decoder);
    var buffer = '';
    await for (final chunk in text) {
      buffer += chunk;
      while (true) {
        final boundary = _eventBoundary(buffer);
        if (boundary == null) break;
        final block = buffer.substring(0, boundary.$1);
        buffer = buffer.substring(boundary.$1 + boundary.$2);
        final event = _parseEventBlock(block);
        if (event != null) yield event;
      }
    }
    final tail = buffer.trim();
    if (tail.isEmpty) return;
    final event = _parseEventBlock(tail);
    if (event != null) {
      yield event;
      return;
    }
    final raw = jsonDecode(tail);
    if (raw is! Map) throw StateError('聊天流返回格式无效');
    final map = Map<String, dynamic>.from(raw);
    final payload = map['data'];
    if (payload is Map && (payload['status'] ?? '').toString() == 'queued') {
      yield ChatStreamEvent('queued', Map<String, dynamic>.from(payload));
      return;
    }
    final message = (map['message'] ?? map['msg'] ?? '聊天请求失败').toString();
    throw StateError(message);
  }

  (int, int)? _eventBoundary(String value) {
    int? bestIndex;
    var bestLength = 0;
    for (final separator in const <String>['\r\n\r\n', '\n\n', '\r\r']) {
      final index = value.indexOf(separator);
      if (index >= 0 && (bestIndex == null || index < bestIndex)) {
        bestIndex = index;
        bestLength = separator.length;
      }
    }
    return bestIndex == null ? null : (bestIndex, bestLength);
  }

  ChatStreamEvent? _parseEventBlock(String block) {
    String? type;
    final dataLines = <String>[];
    final normalized = block.replaceAll('\r\n', '\n').replaceAll('\r', '\n');
    for (final line in normalized.split('\n')) {
      if (line.startsWith('event:')) {
        type = line.substring(6).trim();
      } else if (line.startsWith('data:')) {
        dataLines.add(line.substring(5).trimLeft());
      }
    }
    if (type == null || type.isEmpty || dataLines.isEmpty) return null;
    final decoded = jsonDecode(dataLines.join('\n'));
    if (decoded is! Map) return null;
    return ChatStreamEvent(type, Map<String, dynamic>.from(decoded));
  }

  Future<ChatSubmitResult> submitMessage({
    required String message,
    String? clientMessageId,
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
    final normalizedClientMessageId = clientMessageId?.trim() ?? '';
    final requestId = normalizedClientMessageId.isEmpty
        ? 'mobile-$now'
        : normalizedClientMessageId;
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
          if (workspace.projectId.isNotEmpty) 'projectId': workspace.projectId,
          'workspaceId': workspace.workspaceId,
          'workspaceDeviceId': workspace.deviceId,
          'workspaceName': workspace.workspaceName,
          'workspaceKind': workspace.workspaceKind,
          'workspaceRootUri': workspace.rootUri,
        },
        'source': 'mobile',
        'requestId': requestId,
        'clientMessageId': requestId,
        if (DeviceTimezoneCache.hasValue)
          'deviceTimezone': DeviceTimezoneCache.ianaTimezone,
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

  List<Map<String, dynamic>> _mapList(
    dynamic resp, {
    List<String> keys = const [],
  }) {
    dynamic raw = resp;
    if (raw is Map) {
      for (final key in keys) {
        if (raw[key] is List) {
          raw = raw[key];
          break;
        }
      }
    }
    if (raw is! List) return const [];
    return raw
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .toList(growable: false);
  }
}
