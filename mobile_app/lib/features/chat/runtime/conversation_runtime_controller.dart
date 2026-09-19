import 'dart:async';
import 'dart:collection';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/models/conversation.dart';
import '../../../core/services/chat_service.dart';
import '../../../core/services/channel_service.dart';
import '../../../core/services/providers.dart';
import '../../../shared/models/models.dart';

/// UI-agnostic conversation runtime shared by the built-in UI and extension UI.
///
/// Mobile uses the same streaming web-chat contract as the desktop client:
/// send-stream SSE is the primary reply path, while persisted-message sync and
/// generation status are reconciliation fallbacks for queued/remote delivery.
class ConversationRuntimeController extends ChangeNotifier {
  ConversationRuntimeController(this._chatService, this._emoteService);

  final ChatService _chatService;
  final EmoteService _emoteService;
  final List<ChatMessage> _messages = <ChatMessage>[];
  String? _conversationId;
  String? _characterId;
  ConversationWorkspaceDto? _workspace;
  bool _sending = false;
  Object? _lastError;
  int _generationEpoch = 0;
  Timer? _liveSyncTimer;
  bool _syncingMessages = false;
  bool _disposed = false;
  ChatStreamCancellation? _activeSendCancellation;
  ChatStreamCancellation? _messageEventsCancellation;

  static const Duration _liveSyncInterval = Duration(seconds: 15);

  UnmodifiableListView<ChatMessage> get messages =>
      UnmodifiableListView<ChatMessage>(_messages);
  String? get conversationId => _conversationId;
  ConversationWorkspaceDto? get workspace => _workspace;
  bool get sending => _sending;
  Object? get lastError => _lastError;
  String get state => _sending ? 'sending' : 'idle';

  void setCharacterId(String? characterId) {
    _characterId = characterId?.trim().isEmpty == true ? null : characterId;
  }

  void setWorkspace(ConversationWorkspaceDto? workspace) {
    final previous = _workspace;
    if (previous?.workspaceId == workspace?.workspaceId &&
        previous?.deviceId == workspace?.deviceId &&
        previous?.workspaceName == workspace?.workspaceName &&
        previous?.rootUri == workspace?.rootUri) {
      return;
    }
    _workspace = workspace;
    notifyListeners();
  }

  ChatMessage _copy(ChatMessage message, {String? id, MessageStatus? status}) {
    return ChatMessage(
      id: id ?? message.id,
      renderId: message.renderId,
      role: message.role,
      type: message.type,
      content: message.content,
      reasoningContent: message.reasoningContent,
      time: message.time,
      status: status ?? message.status,
      agentTaskId: message.agentTaskId,
      agentTaskTitle: message.agentTaskTitle,
      agentTaskSteps: message.agentTaskSteps,
      agentTaskProgress: message.agentTaskProgress,
      agentTaskElapsed: message.agentTaskElapsed,
      fileName: message.fileName,
      fileSizeKB: message.fileSizeKB,
      resourceUri: message.resourceUri,
      mediaUrl: message.mediaUrl,
      mimeType: message.mimeType,
      durationMs: message.durationMs,
      toolName: message.toolName,
      toolResult: message.toolResult,
      replyToMessageId: message.replyToMessageId,
      replyToExcerpt: message.replyToExcerpt,
    );
  }

  Future<void> sendText(
    String text, {
    String? replyToMessageId,
    String? replyToExcerpt,
  }) async {
    final value = text.trim();
    if (value.isEmpty || _sending) return;
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.text,
        content: value,
        time: DateTime.now(),
        status: MessageStatus.sending,
        replyToMessageId: replyToMessageId,
        replyToExcerpt: replyToExcerpt,
      ),
      message: value,
      replyToMessageId: replyToMessageId,
    );
  }

  Future<void> sendCode(String language, String code) async {
    final body = code.trim();
    if (body.isEmpty || _sending) return;
    final lang = language.trim().isEmpty ? 'text' : language.trim();
    final content = '```$lang\n$body\n```';
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.code,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
      ),
      message: content,
    );
  }

  Future<void> sendEmote(String emoteId, String displayText) async {
    if (emoteId.trim().isEmpty || _sending) return;
    final conv = _conversationId;
    final character = _characterId;
    if (conv == null ||
        conv.isEmpty ||
        character == null ||
        character.isEmpty) {
      _lastError = StateError('发送表情前需要有效会话和角色');
      notifyListeners();
      return;
    }
    _sending = true;
    _lastError = null;
    notifyListeners();
    try {
      await _emoteService.sendEmote(conv, character, emoteId);
      await _syncMessages();
    } catch (error) {
      _lastError = error;
    } finally {
      _sending = false;
      notifyListeners();
    }
  }

  Future<void> sendImage({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'image/*',
    String text = '',
  }) async {
    if (_sending) return;
    final content = text.trim().isEmpty ? '[图片]' : text.trim();
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.image,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
      ),
      message: content,
      imageUrl: resourceUri,
    );
  }

  Future<void> sendVideo({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'video/*',
    int durationMs = 0,
    String text = '',
  }) async {
    if (_sending) return;
    final content = text.trim().isEmpty ? '[视频]' : text.trim();
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.video,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
        durationMs: durationMs,
      ),
      message: content,
      videoUrl: resourceUri,
    );
  }

  Future<void> sendVoice({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'audio/*',
    int durationMs = 0,
    String text = '',
  }) async {
    if (_sending) return;
    final content = text.trim().isEmpty ? '[语音]' : text.trim();
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.audio,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
        durationMs: durationMs,
      ),
      message: content,
      audioUrl: resourceUri,
      audioDuration: durationMs / 1000.0,
    );
  }

  Future<void> sendFile({
    required String resourceUri,
    required String fileName,
    required int sizeBytes,
    String mimeType = 'application/octet-stream',
  }) async {
    if (_sending) return;
    final content = '[文件] $fileName\n$resourceUri';
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.file,
        content: content,
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        fileSizeKB: (sizeBytes / 1024).ceil(),
        resourceUri: resourceUri,
        mimeType: mimeType,
      ),
      message: content,
    );
  }

  Future<void> addUserMessage(ChatMessage message) async {
    if (message.content.trim().isEmpty || _sending) return;
    await _send(message, message: message.content);
  }

  Future<void> _send(
    ChatMessage localMessage, {
    required String message,
    String? imageUrl,
    String? audioUrl,
    double audioDuration = 0,
    String? videoUrl,
    String? replyToMessageId,
  }) async {
    _messages.add(localMessage);
    debugPrint(
      'Chat local message added id=${localMessage.id} total=${_messages.length}',
    );
    _lastError = null;
    _sending = true;
    final epoch = ++_generationEpoch;
    final cancellation = _chatService.createStreamCancellation();
    _activeSendCancellation?.cancel('superseded');
    _activeSendCancellation = cancellation;
    notifyListeners();

    var queued = false;
    try {
      await for (final event in _chatService.submitMessageStream(
        message: message,
        clientMessageId: localMessage.renderId,
        conversationId: _conversationId,
        characterId: _characterId,
        imageUrl: imageUrl,
        audioUrl: audioUrl,
        audioDuration: audioDuration,
        videoUrl: videoUrl,
        replyToMessageId: replyToMessageId,
        workspace: _workspace,
        cancellation: cancellation,
      )) {
        if (epoch != _generationEpoch || cancellation.isCancelled) return;
        switch (event.type) {
          case 'message_start':
            await _handleMessageStart(localMessage, event.data, epoch);
            break;
          case 'token':
            _upsertStreamAssistant(event.data, MessageType.text);
            break;
          case 'voice_audio':
            _upsertStreamAssistant(event.data, MessageType.audio);
            break;
          case 'queued':
            queued = true;
            final conversationId = (event.data['conversationId'] ?? '')
                .toString()
                .trim();
            if (conversationId.isNotEmpty) {
              _conversationId = conversationId;
              _restartLiveSync();
            }
            break;
          case 'message_end':
          case 'done':
          case 'connected':
            break;
        }
      }
      if (epoch != _generationEpoch) return;
      if (queued) {
        await _awaitQueuedGeneration(epoch);
      } else {
        await _syncMessages();
      }
      _lastError = null;
    } catch (error, stackTrace) {
      if (epoch != _generationEpoch || cancellation.isCancelled) return;
      debugPrint('Chat send failed [${error.runtimeType}]: $error');
      for (final line in stackTrace.toString().split('\n').take(24)) {
        debugPrint(line);
      }
      _lastError = error;
      final localIndex = _messages.indexWhere(
        (m) =>
            m.id == localMessage.id ||
            (m.role == MessageRole.user && m.status == MessageStatus.sending),
      );
      if (localIndex >= 0) {
        _messages[localIndex] = _copy(
          _messages[localIndex],
          status: MessageStatus.error,
        );
      }
    } finally {
      if (identical(_activeSendCancellation, cancellation)) {
        _activeSendCancellation = null;
      }
      if (epoch == _generationEpoch) {
        _sending = false;
        notifyListeners();
      }
    }
  }

  Future<void> _handleMessageStart(
    ChatMessage localMessage,
    Map<String, dynamic> data,
    int epoch,
  ) async {
    final conversationId = (data['conversationId'] ?? '').toString().trim();
    if (conversationId.isNotEmpty && conversationId != _conversationId) {
      _conversationId = conversationId;
      _restartLiveSync();
    }
    final userMessageId = (data['userMessageId'] ?? '').toString().trim();
    final localIndex = _messages.indexWhere((m) => m.id == localMessage.id);
    if (localIndex >= 0) {
      _messages[localIndex] = _copy(
        _messages[localIndex],
        id: userMessageId.isEmpty ? null : userMessageId,
        status: MessageStatus.sent,
      );
      notifyListeners();
    }
    final workspace = _workspace;
    if (epoch != _generationEpoch ||
        workspace == null ||
        conversationId.isEmpty) {
      return;
    }
    try {
      if (workspace.projectId.isNotEmpty) {
        await _chatService.moveConversationToProject(
          conversationId,
          workspace.projectId,
        );
      }
    } catch (_) {}
  }

  void _upsertStreamAssistant(Map<String, dynamic> data, MessageType type) {
    final id = (data['id'] ?? data['messageId'] ?? '').toString().trim();
    if (id.isEmpty) return;
    final content = (data['content'] ?? '').toString();
    final createdAt =
        DateTime.tryParse((data['createdAt'] ?? '').toString()) ??
        DateTime.now();
    final audioUrl = (data['audioUrl'] ?? '').toString().trim();
    final duration = (data['duration'] as num?)?.toDouble() ?? 0;
    final index = _messages.indexWhere((message) => message.id == id);
    final existing = index >= 0 ? _messages[index] : null;
    final next = ChatMessage(
      id: id,
      renderId: existing?.renderId ?? id,
      role: MessageRole.assistant,
      type: audioUrl.isNotEmpty ? MessageType.audio : type,
      content: content.isNotEmpty ? content : existing?.content ?? '',
      time: existing?.time ?? createdAt,
      status: MessageStatus.sending,
      resourceUri: audioUrl.isNotEmpty ? audioUrl : existing?.resourceUri,
      mediaUrl: audioUrl.isNotEmpty ? audioUrl : existing?.mediaUrl,
      mimeType: audioUrl.isNotEmpty ? 'audio/*' : existing?.mimeType,
      durationMs: duration > 0
          ? (duration * 1000).round()
          : existing?.durationMs,
      replyToMessageId: existing?.replyToMessageId,
      replyToExcerpt: existing?.replyToExcerpt,
    );
    if (index >= 0) {
      _messages[index] = next;
    } else {
      _messages.add(next);
    }
    notifyListeners();
  }

  Future<void> _awaitQueuedGeneration(int epoch) async {
    final conv = _conversationId;
    if (conv == null || conv.isEmpty) return;
    final deadline = DateTime.now().add(const Duration(minutes: 2));
    var sawActiveState = false;
    while (epoch == _generationEpoch && DateTime.now().isBefore(deadline)) {
      final status = await _chatService.generationStatus(conv);
      if (status == 'collecting' || status == 'processing') {
        sawActiveState = true;
        await Future<void>.delayed(const Duration(milliseconds: 1000));
        continue;
      }
      if (status == 'failed') throw StateError('AI 生成失败');
      if (status == 'cancelled') {
        await _syncMessages();
        return;
      }
      if (status == 'completed' || (status == 'idle' && sawActiveState)) {
        await _syncMessages();
        return;
      }
      await Future<void>.delayed(const Duration(milliseconds: 750));
    }
    if (epoch == _generationEpoch) await _syncMessages();
  }

  Future<void> _syncMessages({bool background = false}) async {
    final conv = _conversationId;
    if (conv == null || conv.isEmpty) return;
    final persisted = await _chatService.getMessages(conv);
    debugPrint(
      'Chat sync persisted conversation=$conv count=${persisted.length}',
    );
    if (_disposed || _conversationId != conv || (background && _sending)) {
      return;
    }
    if (persisted.isEmpty) {
      debugPrint('Chat sync skipped empty persisted conversation=$conv');
      return;
    }

    final localById = <String, ChatMessage>{
      for (final message in _messages) message.id: message,
    };
    final localByRenderId = <String, ChatMessage>{
      for (final message in _messages)
        if (message.renderId.trim().isNotEmpty) message.renderId: message,
    };
    final transientErrors = _messages
        .where(
          (message) =>
              message.role == MessageRole.assistant &&
              message.status == MessageStatus.error &&
              message.id.contains('-error-'),
        )
        .toList(growable: false);
    final mapped = persisted
        .map((dto) {
          final persistedRequestId = dto.requestId.trim();
          final existing =
              localById[dto.id] ??
              (persistedRequestId.isEmpty
                  ? null
                  : localByRenderId[persistedRequestId]);
          final type = _typeForDto(dto, existing);
          final agentTask = type == MessageType.agentTask
              ? _agentTaskPayload(dto.content)
              : const <String, dynamic>{};
          return ChatMessage(
            id: dto.id,
            renderId:
                existing?.renderId ??
                (persistedRequestId.isEmpty ? dto.id : persistedRequestId),
            role: _roleFor(dto.role),
            type: type,
            content: dto.content.trim().isNotEmpty || existing == null
                ? dto.content
                : existing.content,
            reasoningContent: dto.reasoningContent.isNotEmpty
                ? dto.reasoningContent
                : existing?.reasoningContent ?? '',
            time:
                DateTime.tryParse(dto.createdAt) ??
                existing?.time ??
                DateTime.now(),
            status: dto.status == 'failed'
                ? MessageStatus.error
                : MessageStatus.delivered,
            agentTaskId:
                _firstString(agentTask, const <String>[
                  'taskRunId',
                  'task_run_id',
                  'runId',
                ]) ??
                existing?.agentTaskId,
            agentTaskTitle:
                _firstString(agentTask, const <String>[
                  'title',
                  'taskTitle',
                  'taskDefinitionId',
                ]) ??
                existing?.agentTaskTitle,
            agentTaskSteps:
                _stringList(agentTask['steps']) ?? existing?.agentTaskSteps,
            agentTaskProgress:
                _progressInt(agentTask) ?? existing?.agentTaskProgress,
            agentTaskElapsed:
                _firstString(agentTask, const <String>[
                  'elapsed',
                  'elapsedTime',
                ]) ??
                existing?.agentTaskElapsed,
            fileName: existing?.fileName,
            fileSizeKB: existing?.fileSizeKB,
            resourceUri: _resourceForDto(dto, existing),
            mediaUrl: existing?.mediaUrl,
            mimeType: existing?.mimeType,
            durationMs: dto.audioDuration > 0
                ? (dto.audioDuration * 1000).round()
                : existing?.durationMs,
            toolName: existing?.toolName,
            toolResult: existing?.toolResult,
            replyToMessageId:
                dto.replyToMessageId ?? existing?.replyToMessageId,
            replyToExcerpt: dto.replyToExcerpt ?? existing?.replyToExcerpt,
          );
        })
        .toList(growable: true);
    for (final transient in transientErrors) {
      if (!mapped.any((message) => message.id == transient.id)) {
        mapped.add(transient);
      }
    }
    for (final local in _messages) {
      if (local.status != MessageStatus.sending &&
          local.status != MessageStatus.sent) {
        continue;
      }
      if (mapped.any((message) => _samePersistedMessage(local, message))) {
        continue;
      }
      mapped.add(local);
    }
    mapped.sort((a, b) => a.time.compareTo(b.time));
    if (_sameMessageList(_messages, mapped)) return;
    _messages
      ..clear()
      ..addAll(mapped);
    debugPrint(
      'Chat sync replaced conversation=$conv total=${_messages.length} ids=${mapped.map((item) => item.id).join(',')}',
    );
    notifyListeners();
  }

  bool _samePersistedMessage(ChatMessage local, ChatMessage persisted) {
    if (local.id == persisted.id) return true;
    if (local.renderId.trim().isNotEmpty &&
        local.renderId == persisted.renderId) {
      return true;
    }
    return local.role == persisted.role && local.content == persisted.content;
  }

  void _restartLiveSync() {
    _liveSyncTimer?.cancel();
    _messageEventsCancellation?.cancel('conversation changed');
    _messageEventsCancellation = null;
    final conv = _conversationId?.trim() ?? '';
    if (_disposed || conv.isEmpty) return;
    _liveSyncTimer = Timer.periodic(_liveSyncInterval, (_) {
      if (_disposed || _syncingMessages) return;
      unawaited(_pollPersistedMessages());
    });
    final cancellation = _chatService.createStreamCancellation();
    _messageEventsCancellation = cancellation;
    unawaited(_runMessageEvents(conv, cancellation));
  }

  Future<void> _pollPersistedMessages() async {
    if (_syncingMessages) return;
    _syncingMessages = true;
    try {
      await _syncMessages(background: true);
    } catch (_) {
      // Proactive/reminder reconciliation is a background fallback. A transient
      // transport failure must not replace the current conversation with an
      // error state; explicit user actions still surface their own failures.
    } finally {
      _syncingMessages = false;
    }
  }

  Future<void> _runMessageEvents(
    String conversationId,
    ChatStreamCancellation cancellation,
  ) async {
    while (!_disposed &&
        !cancellation.isCancelled &&
        _conversationId == conversationId) {
      try {
        await for (final event in _chatService.messageEvents(
          cancellation: cancellation,
        )) {
          if (_disposed ||
              cancellation.isCancelled ||
              _conversationId != conversationId) {
            return;
          }
          if (event.type != 'message_created' &&
              event.type != 'message_updated') {
            continue;
          }
          final eventConversation = (event.data['conversationId'] ?? '')
              .toString();
          if (eventConversation != conversationId) continue;
          if (_handleTransientModelError(event.data)) continue;
          unawaited(_pollPersistedMessages());
        }
      } catch (_) {
        if (_disposed ||
            cancellation.isCancelled ||
            _conversationId != conversationId) {
          return;
        }
      }
      await Future<void>.delayed(const Duration(seconds: 3));
    }
  }

  bool _handleTransientModelError(Map<String, dynamic> event) {
    final rawMetadata = event['data'];
    if (rawMetadata is! Map) return false;
    final metadata = Map<String, dynamic>.from(rawMetadata);
    final messageType = (metadata['messageType'] ?? '').toString();
    if (!const <String>{
      'vision_error',
      'text_error',
      'voice_error',
      'vector_error',
    }.contains(messageType)) {
      return false;
    }
    final id = (event['messageId'] ?? '').toString().trim();
    if (id.isEmpty) return true;
    final rawError = (metadata['rawError'] ?? '').toString().trim();
    final content = (event['content'] ?? '').toString().trim();
    final message = ChatMessage(
      id: id,
      role: MessageRole.assistant,
      type: MessageType.systemNotice,
      content: rawError.isNotEmpty
          ? rawError
          : content.isNotEmpty
          ? content
          : '模型响应失败',
      time:
          DateTime.tryParse((event['createdAt'] ?? '').toString()) ??
          DateTime.now(),
      status: MessageStatus.error,
    );
    final index = _messages.indexWhere((item) => item.id == id);
    if (index >= 0) {
      _messages[index] = message;
    } else {
      _messages.add(message);
    }
    if (messageType == 'text_error') {
      _lastError = StateError(message.content);
    }
    notifyListeners();
    return true;
  }

  bool _sameMessageList(List<ChatMessage> current, List<ChatMessage> next) {
    if (current.length != next.length) return false;
    for (var i = 0; i < current.length; i++) {
      final a = current[i];
      final b = next[i];
      if (a.id != b.id ||
          a.role != b.role ||
          a.type != b.type ||
          a.content != b.content ||
          a.status != b.status ||
          a.time != b.time ||
          a.agentTaskId != b.agentTaskId ||
          a.agentTaskTitle != b.agentTaskTitle ||
          !_sameStringList(a.agentTaskSteps, b.agentTaskSteps) ||
          a.agentTaskProgress != b.agentTaskProgress ||
          a.agentTaskElapsed != b.agentTaskElapsed ||
          a.fileName != b.fileName ||
          a.fileSizeKB != b.fileSizeKB ||
          a.resourceUri != b.resourceUri ||
          a.mediaUrl != b.mediaUrl ||
          a.mimeType != b.mimeType ||
          a.durationMs != b.durationMs ||
          a.toolName != b.toolName ||
          a.toolResult != b.toolResult ||
          a.replyToMessageId != b.replyToMessageId ||
          a.replyToExcerpt != b.replyToExcerpt) {
        return false;
      }
    }
    return true;
  }

  bool _sameStringList(List<String>? a, List<String>? b) {
    if (identical(a, b)) return true;
    if (a == null || b == null || a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }

  Map<String, dynamic> _agentTaskPayload(String content) {
    final value = content.trim();
    if (value.isEmpty || (!value.startsWith('{') && !value.startsWith('['))) {
      return const <String, dynamic>{};
    }
    try {
      final decoded = jsonDecode(value);
      if (decoded is Map) return Map<String, dynamic>.from(decoded);
      if (decoded is List && decoded.isNotEmpty && decoded.first is Map) {
        return Map<String, dynamic>.from(decoded.first as Map);
      }
    } catch (_) {}
    return const <String, dynamic>{};
  }

  String? _firstString(Map<String, dynamic> source, List<String> keys) {
    for (final key in keys) {
      final value = (source[key] ?? '').toString().trim();
      if (value.isNotEmpty) return value;
    }
    return null;
  }

  List<String>? _stringList(dynamic value) {
    if (value is! List) return null;
    final items = value
        .map((item) {
          if (item is Map) {
            return (item['title'] ?? item['name'] ?? item['label'] ?? '')
                .toString()
                .trim();
          }
          return item.toString().trim();
        })
        .where((item) => item.isNotEmpty)
        .toList(growable: false);
    return items.isEmpty ? null : items;
  }

  int? _progressInt(Map<String, dynamic> source) {
    final direct = source['percentage'] ?? source['progress'];
    if (direct is num) return direct.round().clamp(0, 100);
    if (direct is Map) {
      final percentage = direct['percentage'];
      if (percentage is num) return percentage.round().clamp(0, 100);
      final current = direct['current'];
      final total = direct['total'];
      if (current is num && total is num && total > 0) {
        return (current.toDouble() / total.toDouble() * 100).round().clamp(
          0,
          100,
        );
      }
    }
    return null;
  }

  MessageType _typeForDto(MessageDto dto, ChatMessage? existing) {
    final rawType = dto.msgType.trim().toLowerCase().replaceAll('-', '_');
    switch (rawType) {
      case 'image':
        return MessageType.image;
      case 'video':
        return MessageType.video;
      case 'audio':
      case 'voice':
        return MessageType.audio;
      case 'emote':
      case 'emoji':
        return MessageType.emote;
      case 'code':
        return MessageType.code;
      case 'file':
        return MessageType.file;
      case 'agent_task':
      case 'agenttask':
        return MessageType.agentTask;
      case 'tool_call':
      case 'toolcall':
      case 'tool':
        return MessageType.toolCall;
      case 'system_notice':
      case 'systemnotice':
        return MessageType.systemNotice;
    }
    if (existing != null && existing.type != MessageType.text) {
      return existing.type;
    }
    if (dto.imageUrl.isNotEmpty) return MessageType.image;
    if (dto.videoUrl.isNotEmpty) return MessageType.video;
    if (dto.audioUrl.isNotEmpty) return MessageType.audio;
    if ((dto.emoteId ?? '').isNotEmpty) return MessageType.emote;
    if (dto.content.startsWith('```') && dto.content.endsWith('```')) {
      return MessageType.code;
    }
    return MessageType.text;
  }

  String? _resourceForDto(MessageDto dto, ChatMessage? existing) {
    if (dto.imageUrl.isNotEmpty) return dto.imageUrl;
    if (dto.videoUrl.isNotEmpty) return dto.videoUrl;
    if (dto.audioUrl.isNotEmpty) return dto.audioUrl;
    return existing?.resourceUri;
  }

  MessageRole _roleFor(String role) {
    switch (role) {
      case 'user':
        return MessageRole.user;
      case 'system':
        return MessageRole.system;
      default:
        return MessageRole.assistant;
    }
  }

  bool canRetryMessage(int index) {
    if (_sending || index < 0 || index >= _messages.length) return false;
    final message = _messages[index];
    if (message.status != MessageStatus.error) return false;
    if (message.role == MessageRole.user) return true;
    if (message.role != MessageRole.assistant) return false;
    final latestUserIndex = _messages.lastIndexWhere(
      (item) => item.role == MessageRole.user,
    );
    return latestUserIndex >= 0 && index > latestUserIndex;
  }

  Future<void> retryMessage(int index) async {
    if (!canRetryMessage(index)) return;
    final message = _messages[index];
    if (message.role == MessageRole.assistant) {
      await regenerate();
      return;
    }
    _messages.removeAt(index);
    notifyListeners();
    switch (message.type) {
      case MessageType.image:
        if (message.resourceUri == null || message.mediaUrl == null) {
          return sendText(message.content);
        }
        return sendImage(
          resourceUri: message.resourceUri!,
          displayUrl: message.mediaUrl!,
          fileName: message.fileName ?? 'image',
          mimeType: message.mimeType ?? 'image/*',
          text: message.content == '[图片]' ? '' : message.content,
        );
      case MessageType.video:
        if (message.resourceUri == null || message.mediaUrl == null) {
          return sendText(message.content);
        }
        return sendVideo(
          resourceUri: message.resourceUri!,
          displayUrl: message.mediaUrl!,
          fileName: message.fileName ?? 'video',
          mimeType: message.mimeType ?? 'video/*',
          durationMs: message.durationMs ?? 0,
          text: message.content == '[视频]' ? '' : message.content,
        );
      case MessageType.audio:
        if (message.resourceUri == null || message.mediaUrl == null) {
          return sendText(message.content);
        }
        return sendVoice(
          resourceUri: message.resourceUri!,
          displayUrl: message.mediaUrl!,
          fileName: message.fileName ?? 'audio',
          mimeType: message.mimeType ?? 'audio/*',
          durationMs: message.durationMs ?? 0,
          text: message.content == '[语音]' ? '' : message.content,
        );
      case MessageType.file:
        if (message.resourceUri == null) return sendText(message.content);
        return sendFile(
          resourceUri: message.resourceUri!,
          fileName: message.fileName ?? 'file',
          sizeBytes: (message.fileSizeKB ?? 0) * 1024,
          mimeType: message.mimeType ?? 'application/octet-stream',
        );
      case MessageType.code:
      case MessageType.emote:
      case MessageType.text:
      case MessageType.agentTask:
      case MessageType.toolCall:
      case MessageType.systemNotice:
        return sendText(message.content);
    }
  }

  Future<void> regenerate({String? messageId}) async {
    final conv = _conversationId;
    if (_sending || conv == null || conv.isEmpty) return;
    _sending = true;
    final epoch = ++_generationEpoch;
    _lastError = null;
    notifyListeners();
    try {
      await _chatService.regenerate(conv);
      if (epoch == _generationEpoch) await _syncMessages();
    } catch (error) {
      if (epoch == _generationEpoch) _lastError = error;
    } finally {
      if (epoch == _generationEpoch) {
        _sending = false;
        notifyListeners();
      }
    }
  }

  Future<void> deleteMessage(String messageId) async {
    final id = messageId.trim();
    if (id.isEmpty) return;
    await _chatService.deleteMessage(id);
    final index = _messages.indexWhere((message) => message.id == id);
    if (index >= 0) {
      _messages.removeAt(index);
      notifyListeners();
    }
  }

  Future<void> stop() async {
    if (!_sending) return;
    final conv = _conversationId;
    _activeSendCancellation?.cancel('user stopped');
    _activeSendCancellation = null;
    ++_generationEpoch;
    _sending = false;
    notifyListeners();
    if (conv != null && conv.isNotEmpty) {
      try {
        await _chatService.cancelGeneration(conv);
        await _syncMessages();
      } catch (error) {
        _lastError = error;
        notifyListeners();
      }
    }
  }

  Future<void> openConversation(
    String conversationId, {
    String? characterId,
  }) async {
    final id = conversationId.trim();
    if (id.isEmpty) return;
    if (_conversationId == id) {
      _characterId = characterId?.trim().isEmpty == true ? null : characterId;
      await _syncMessages();
      return;
    }
    debugPrint('Chat open conversation id=$id');
    _activeSendCancellation?.cancel('conversation changed');
    _activeSendCancellation = null;
    ++_generationEpoch;
    _conversationId = id;
    _characterId = characterId?.trim().isEmpty == true ? null : characterId;
    _messages.clear();
    _lastError = null;
    _sending = false;
    _restartLiveSync();
    notifyListeners();
    try {
      await _syncMessages();
    } catch (error) {
      _lastError = error;
      notifyListeners();
    }
  }

  Future<bool> createConversation(
    String? characterId, {
    String projectId = '',
  }) async {
    final conversation = await _chatService.createConversation(
      projectId: projectId,
    );
    if (conversation == null) return false;
    _conversationId = conversation.id;
    _characterId = characterId;
    _messages.clear();
    _lastError = null;
    _restartLiveSync();
    notifyListeners();
    return true;
  }

  void clear({bool addSystemNotice = false}) {
    _messages.clear();
    if (addSystemNotice) {
      _messages.add(
        ChatMessage(
          id: _localId('sys'),
          role: MessageRole.system,
          type: MessageType.systemNotice,
          content: '聊天记录已从当前界面清空',
          time: DateTime.now(),
        ),
      );
    }
    notifyListeners();
  }

  void startDraft({ConversationWorkspaceDto? workspace}) {
    _activeSendCancellation?.cancel('new conversation');
    _activeSendCancellation = null;
    ++_generationEpoch;
    _liveSyncTimer?.cancel();
    _liveSyncTimer = null;
    _conversationId = null;
    _messages.clear();
    _lastError = null;
    _sending = false;
    setWorkspace(workspace);
    notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    _activeSendCancellation?.cancel('disposed');
    _activeSendCancellation = null;
    _messageEventsCancellation?.cancel('disposed');
    _messageEventsCancellation = null;
    _liveSyncTimer?.cancel();
    _liveSyncTimer = null;
    super.dispose();
  }

  String _localId(String prefix) =>
      '$prefix${DateTime.now().microsecondsSinceEpoch}';

  Map<String, dynamic> serializeMessage(ChatMessage message) =>
      <String, dynamic>{
        'id': message.id,
        'renderId': message.renderId,
        'role': message.role.name,
        'type': message.type.name,
        'content': message.content,
        'time': message.time.toIso8601String(),
        'status': message.status.name,
        if ((message.agentTaskId ?? '').isNotEmpty)
          'agentTaskId': message.agentTaskId,
        if (message.fileName != null) 'fileName': message.fileName,
        if (message.fileSizeKB != null) 'fileSizeKB': message.fileSizeKB,
        if (message.resourceUri != null) 'resourceUri': message.resourceUri,
        if (message.mediaUrl != null) 'mediaUrl': message.mediaUrl,
        if (message.mimeType != null) 'mimeType': message.mimeType,
        if (message.durationMs != null) 'durationMs': message.durationMs,
        if (message.toolName != null) 'toolName': message.toolName,
        if (message.toolResult != null) 'toolResult': message.toolResult,
        if ((message.replyToMessageId ?? '').isNotEmpty)
          'replyToMessageId': message.replyToMessageId,
        if ((message.replyToExcerpt ?? '').isNotEmpty)
          'replyToExcerpt': message.replyToExcerpt,
      };
}

final conversationRuntimeControllerProvider =
    ChangeNotifierProvider<ConversationRuntimeController>((ref) {
      final controller = ConversationRuntimeController(
        ref.read(chatServiceProvider),
        ref.read(emoteServiceProvider),
      );
      ref.onDispose(controller.dispose);
      return controller;
    });
