import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/backend_access/business_backend_unavailable.dart';
import '../../../core/models/conversation.dart';
import '../../../core/services/chat_service.dart';
import '../../../core/services/channel_service.dart';
import '../../../core/services/providers.dart';
import '../../../shared/models/models.dart';
import '../../conversation/rendering/stream/markdown_stream_scheduler.dart';
import 'conversation_message_ledger.dart';

/// UI-agnostic conversation runtime shared by the built-in UI and extension UI.
///
/// Mobile uses the same streaming web-chat contract as the desktop client:
/// send-stream SSE is the primary reply path, while persisted-message sync and
/// generation status are reconciliation fallbacks for queued/remote delivery.
class ConversationRuntimeController extends ChangeNotifier {
  ConversationRuntimeController(this._chatService, this._emoteService);

  final ChatService _chatService;
  final EmoteService _emoteService;
  final ConversationMessageLedger _messages = ConversationMessageLedger();
  String? _conversationId;
  String? _characterId;
  ConversationWorkspaceDto? _workspace;
  bool _sending = false;
  Object? _lastError;
  int _generationEpoch = 0;
  Timer? _liveSyncTimer;
  bool _syncingMessages = false;
  int _messageSyncEpoch = 0;
  bool _disposed = false;
  int _draftEpoch = 0;
  ChatStreamCancellation? _activeSendCancellation;
  ChatStreamCancellation? _messageEventsCancellation;
  String _activeReasoningContent = '';
  int _activeReasoningDurationMs = 0;
  String _activeResponseGroupId = '';
  int _modelConfigId = 0;
  String _reasoningEffort = 'medium';
  bool _liveReasoningAttached = false;
  final MarkdownStreamScheduler _streamScheduler = MarkdownStreamScheduler();

  static const Duration _liveSyncInterval = Duration(seconds: 15);
  static const Duration _businessReadyRetryWindow = Duration(seconds: 20);
  static const Duration _businessReadyRetryDelay = Duration(milliseconds: 250);

  List<ChatMessage> get messages => _messages.messages;
  String? get conversationId => _conversationId;
  ConversationWorkspaceDto? get workspace => _workspace;
  bool get sending => _sending;
  Object? get lastError => _lastError;
  String get state => _sending ? 'sending' : 'idle';
  int get draftEpoch => _draftEpoch;
  int get modelConfigId => _modelConfigId;
  String get reasoningEffort => _reasoningEffort;

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

  Future<void> updateModelSettings(
    int modelConfigId,
    String reasoningEffort,
  ) async {
    _modelConfigId = modelConfigId;
    _reasoningEffort = reasoningEffort.isEmpty ? 'medium' : reasoningEffort;
    notifyListeners();
    final conversationId = _conversationId?.trim() ?? '';
    if (conversationId.isEmpty) return;
    await _chatService.updateConversationModelSettings(
      conversationId,
      modelConfigId: _modelConfigId,
      reasoningEffort: _reasoningEffort,
    );
  }

  ChatMessage _copy(
    ChatMessage message, {
    String? id,
    int? sequence,
    MessageStatus? status,
  }) {
    return ChatMessage(
      id: id ?? message.id,
      renderId: message.renderId,
      characterId: message.characterId,
      role: message.role,
      type: message.type,
      content: message.content,
      reasoningContent: message.reasoningContent,
      reasoningDurationMs: message.reasoningDurationMs,
      time: message.time,
      sequence: sequence ?? message.sequence,
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
      responseGroupId: message.responseGroupId,
      deliverySequence: message.deliverySequence,
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
    final pendingMessage = _copy(localMessage, sequence: _nextLocalSequence());
    _messages.upsert(pendingMessage);
    debugPrint(
      'Chat local message added id=${pendingMessage.id} total=${_messages.length}',
    );
    _lastError = null;
    _sending = true;
    _activeReasoningContent = '';
    _activeReasoningDurationMs = 0;
    _activeResponseGroupId = pendingMessage.renderId;
    _liveReasoningAttached = false;
    final epoch = ++_generationEpoch;
    final cancellation = _chatService.createStreamCancellation();
    _activeSendCancellation?.cancel('superseded');
    _activeSendCancellation = cancellation;
    notifyListeners();

    var queued = false;
    try {
      await _ensureConversationForSend();
      await for (final event in _submitStreamWithReadinessRetry(
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
            await _handleMessageStart(pendingMessage, event.data, epoch);
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
              await _bindConversationWorkspace(conversationId);
              notifyListeners();
            }
            break;
          case 'message_end':
          case 'done':
            _streamScheduler.flush();
            break;
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
      var localMessage = _messages.findById(pendingMessage.id);
      if (localMessage == null) {
        for (final message in _messages.messages) {
          if (message.role == MessageRole.user &&
              message.status == MessageStatus.sending) {
            localMessage = message;
            break;
          }
        }
      }
      if (localMessage != null) {
        _messages.upsert(_copy(localMessage, status: MessageStatus.error));
      }
    } finally {
      if (identical(_activeSendCancellation, cancellation)) {
        _activeSendCancellation = null;
      }
      if (epoch == _generationEpoch) {
        _activeResponseGroupId = '';
        _sending = false;
        _streamScheduler.schedule(notifyListeners);
      }
    }
  }

  Future<void> _ensureConversationForSend() async {
    final existing = _conversationId?.trim() ?? '';
    if (existing.isNotEmpty) return;
    final deadline = DateTime.now().add(_businessReadyRetryWindow);
    while (true) {
      try {
        final conversation = await _chatService.createConversation(
          projectId: _workspace?.projectId ?? '',
        );
        if (conversation == null) {
          throw StateError('创建会话未返回结果');
        }
        _conversationId = conversation.id;
        _restartLiveSync();
        notifyListeners();
        return;
      } on BusinessBackendUnavailable {
        if (DateTime.now().isAfter(deadline)) rethrow;
        await Future<void>.delayed(_businessReadyRetryDelay);
      }
    }
  }

  Stream<ChatStreamEvent> _submitStreamWithReadinessRetry({
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
    required ChatStreamCancellation cancellation,
  }) async* {
    final deadline = DateTime.now().add(_businessReadyRetryWindow);
    while (true) {
      var started = false;
      try {
        await for (final event in _chatService.submitMessageStream(
          message: message,
          clientMessageId: clientMessageId,
          conversationId: conversationId,
          characterId: characterId,
          imageUrl: imageUrl,
          audioUrl: audioUrl,
          audioDuration: audioDuration,
          videoUrl: videoUrl,
          replyToMessageId: replyToMessageId,
          modelConfigId: _modelConfigId,
          reasoningEffort: _reasoningEffort,
          workspace: workspace,
          cancellation: cancellation,
        )) {
          started = true;
          yield event;
        }
        return;
      } on BusinessBackendUnavailable {
        if (started ||
            cancellation.isCancelled ||
            DateTime.now().isAfter(deadline)) {
          rethrow;
        }
        await Future<void>.delayed(_businessReadyRetryDelay);
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
    _activeReasoningContent = (data['reasoningContent'] ?? '').toString();
    _activeReasoningDurationMs =
        (data['reasoningDurationMs'] as num?)?.toInt() ?? 0;
    final current = _messages.findById(localMessage.id);
    if (current != null) {
      _messages.upsert(
        _copy(
          current,
          id: userMessageId.isEmpty ? null : userMessageId,
          status: MessageStatus.sent,
        ),
      );
    }
    await _bindConversationWorkspace(conversationId);
    if (current != null) notifyListeners();
  }

  Future<void> _bindConversationWorkspace(String conversationId) async {
    final workspace = _workspace;
    if (workspace == null || conversationId.isEmpty) return;
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
    final reasoningContent = (data['reasoningContent'] ?? '').toString();
    final reasoningDurationMs =
        (data['reasoningDurationMs'] as num?)?.toInt() ?? 0;
    final responseGroupId =
        (data['responseGroupId'] ??
                data['response_group_id'] ??
                _activeResponseGroupId)
            .toString()
            .trim();
    final deliverySequence =
        (data['deliverySequence'] as num?)?.toInt() ??
        (data['delivery_sequence'] as num?)?.toInt() ??
        0;
    final existing =
        _messages.findById(id) ??
        _messages.findByRenderId(id, role: MessageRole.assistant);
    final characterId =
        (data['characterId'] ??
                data['character_id'] ??
                existing?.characterId ??
                _characterId)
            .toString()
            .trim();
    final attachReasoning =
        !_liveReasoningAttached &&
        (reasoningContent.isNotEmpty || _activeReasoningContent.isNotEmpty);
    final effectiveReasoningContent = reasoningContent.isNotEmpty
        ? reasoningContent
        : _activeReasoningContent;
    final effectiveReasoningDurationMs = reasoningDurationMs > 0
        ? reasoningDurationMs
        : _activeReasoningDurationMs;
    if (attachReasoning) _liveReasoningAttached = true;
    final next = ChatMessage(
      id: id,
      renderId: existing?.renderId ?? id,
      characterId: characterId,
      role: MessageRole.assistant,
      type: audioUrl.isNotEmpty ? MessageType.audio : type,
      content: content.isNotEmpty ? content : existing?.content ?? '',
      reasoningContent: attachReasoning
          ? effectiveReasoningContent
          : existing?.reasoningContent ?? '',
      reasoningDurationMs: attachReasoning
          ? effectiveReasoningDurationMs
          : existing?.reasoningDurationMs ?? 0,
      time: existing?.time ?? createdAt,
      sequence: existing?.sequence ?? _nextLocalSequence(),
      status: MessageStatus.sending,
      resourceUri: audioUrl.isNotEmpty ? audioUrl : existing?.resourceUri,
      mediaUrl: audioUrl.isNotEmpty ? audioUrl : existing?.mediaUrl,
      mimeType: audioUrl.isNotEmpty ? 'audio/*' : existing?.mimeType,
      durationMs: duration > 0
          ? (duration * 1000).round()
          : existing?.durationMs,
      replyToMessageId: existing?.replyToMessageId,
      replyToExcerpt: existing?.replyToExcerpt,
      responseGroupId: responseGroupId,
      deliverySequence: deliverySequence,
    );
    _messages.upsert(next);
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
    final syncEpoch = ++_messageSyncEpoch;
    final persisted = await _chatService.getMessages(conv, latest: true);
    debugPrint(
      'Chat sync persisted conversation=$conv count=${persisted.length}',
    );
    if (_disposed ||
        syncEpoch != _messageSyncEpoch ||
        _conversationId != conv) {
      return;
    }
    if (persisted.isEmpty) {
      debugPrint('Chat sync skipped empty persisted conversation=$conv');
      return;
    }

    var changed = false;
    for (final dto in persisted) {
      final persistedRequestId = dto.requestId.trim();
      final role = _roleFor(dto.role);
      var existing = _messages.findById(dto.id);
      if (existing != null && existing.role != role) existing = null;
      if (existing == null && persistedRequestId.isNotEmpty) {
        existing = _messages.findByRenderId(persistedRequestId, role: role);
      }
      final type = _typeForDto(dto, existing);
      final agentTask = type == MessageType.agentTask
          ? _agentTaskPayload(dto.content)
          : const <String, dynamic>{};
      final messageTime =
          DateTime.tryParse(dto.createdAt) ?? existing?.time ?? DateTime.now();
      final reasoningContent = dto.reasoningContent.isNotEmpty
          ? dto.reasoningContent
          : existing?.reasoningContent ?? '';
      final reasoningDurationMs = dto.reasoningDurationMs > 0
          ? dto.reasoningDurationMs
          : existing?.reasoningDurationMs ?? 0;
      final responseGroupId = dto.responseGroupId.trim().isNotEmpty
          ? dto.responseGroupId.trim()
          : role == MessageRole.assistant && persistedRequestId.isNotEmpty
          ? persistedRequestId
          : existing?.responseGroupId ?? '';
      final next = ChatMessage(
        id: dto.id,
        renderId:
            existing?.renderId ??
            (role == MessageRole.user && persistedRequestId.isNotEmpty
                ? persistedRequestId
                : dto.id),
        characterId: dto.characterId.isNotEmpty
            ? dto.characterId
            : existing?.characterId ?? '',
        role: role,
        type: type,
        content: dto.content.trim().isNotEmpty || existing == null
            ? dto.content
            : existing.content,
        reasoningContent: reasoningContent,
        reasoningDurationMs: reasoningDurationMs,
        time: messageTime,
        sequence: dto.sequence > 0 ? dto.sequence : existing?.sequence,
        status: _statusForDto(dto.status),
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
            _firstString(agentTask, const <String>['elapsed', 'elapsedTime']) ??
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
        replyToMessageId: dto.replyToMessageId ?? existing?.replyToMessageId,
        replyToExcerpt: dto.replyToExcerpt ?? existing?.replyToExcerpt,
        responseGroupId: responseGroupId,
        deliverySequence: dto.deliverySequence > 0
            ? dto.deliverySequence
            : existing?.deliverySequence ?? 0,
      );
      changed = _messages.upsert(next) || changed;
    }
    debugPrint(
      'Chat sync reconciled conversation=$conv total=${_messages.length}',
    );
    if (changed) notifyListeners();
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
    final responseGroupId =
        (metadata['responseGroupId'] ??
                metadata['response_group_id'] ??
                metadata['requestId'] ??
                '')
            .toString()
            .trim();
    final message = ChatMessage(
      id: id,
      characterId: _characterId ?? '',
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
      sequence: _nextLocalSequence(),
      status: MessageStatus.error,
      responseGroupId: responseGroupId,
    );
    _messages.upsert(message);
    if (messageType == 'text_error') {
      _lastError = StateError(message.content);
    }
    notifyListeners();
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

  MessageStatus _statusForDto(String value) {
    final status = value.trim().toLowerCase();
    return switch (status) {
      'queued' || 'pending' => MessageStatus.queued,
      'sending' => MessageStatus.sending,
      'streaming' ||
      'generating' ||
      'collecting' ||
      'processing' => MessageStatus.streaming,
      'interrupted' || 'paused' => MessageStatus.interrupted,
      'cancelled' || 'canceled' || 'stopped' => MessageStatus.cancelled,
      'failed' || 'error' => MessageStatus.error,
      _ => MessageStatus.delivered,
    };
  }

  bool canRetryMessage(int index) {
    final messages = _messages.messages;
    if (_sending || index < 0 || index >= messages.length) return false;
    final message = messages[index];
    return message.role == MessageRole.user &&
        message.status == MessageStatus.error;
  }

  Future<void> retryMessage(int index) async {
    if (!canRetryMessage(index)) return;
    final message = _messages.messages[index];
    _messages.remove(message);
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

  Future<void> deleteMessage(String messageId) async {
    final id = messageId.trim();
    if (id.isEmpty) return;
    await _chatService.deleteMessage(id);
    if (_messages.removeById(id)) {
      notifyListeners();
    }
  }

  Future<void> editMessage(String messageId, String content) async {
    final id = messageId.trim();
    final value = content.trim();
    if (id.isEmpty || value.isEmpty) return;
    await _chatService.updateMessage(id, value);
    final existing = _messages.findById(id);
    if (existing == null) return;
    _messages.upsert(
      ChatMessage(
        id: existing.id,
        renderId: existing.renderId,
        characterId: existing.characterId,
        role: existing.role,
        type: existing.type,
        content: value,
        reasoningContent: existing.reasoningContent,
        reasoningDurationMs: existing.reasoningDurationMs,
        time: existing.time,
        sequence: existing.sequence,
        status: existing.status,
        agentTaskId: existing.agentTaskId,
        agentTaskTitle: existing.agentTaskTitle,
        agentTaskSteps: existing.agentTaskSteps,
        agentTaskProgress: existing.agentTaskProgress,
        agentTaskElapsed: existing.agentTaskElapsed,
        fileName: existing.fileName,
        fileSizeKB: existing.fileSizeKB,
        resourceUri: existing.resourceUri,
        mediaUrl: existing.mediaUrl,
        mimeType: existing.mimeType,
        durationMs: existing.durationMs,
        toolName: existing.toolName,
        toolResult: existing.toolResult,
        replyToMessageId: existing.replyToMessageId,
        replyToExcerpt: existing.replyToExcerpt,
        responseGroupId: existing.responseGroupId,
        deliverySequence: existing.deliverySequence,
      ),
    );
    notifyListeners();
  }

  Future<void> stop() async {
    if (!_sending) return;
    final conv = _conversationId;
    _activeSendCancellation?.cancel('user stopped');
    _activeSendCancellation = null;
    _streamScheduler.flush();
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
    _messageSyncEpoch++;
    _conversationId = id;
    _characterId = characterId?.trim().isEmpty == true ? null : characterId;
    _messages.clear();
    _lastError = null;
    _sending = false;
    try {
      final conversation = await _chatService.getConversation(id);
      _modelConfigId = conversation?.modelConfigId ?? 0;
      _reasoningEffort = conversation?.reasoningEffort.isNotEmpty == true
          ? conversation!.reasoningEffort
          : 'medium';
    } catch (_) {
      _modelConfigId = 0;
      _reasoningEffort = 'medium';
    }
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
      _messages.upsert(
        ChatMessage(
          id: _localId('sys'),
          role: MessageRole.system,
          type: MessageType.systemNotice,
          content: '聊天记录已从当前界面清空',
          time: DateTime.now(),
          sequence: _nextLocalSequence(),
        ),
      );
    }
    notifyListeners();
  }

  void startDraft({ConversationWorkspaceDto? workspace}) {
    _activeSendCancellation?.cancel('new conversation');
    _activeSendCancellation = null;
    _draftEpoch++;
    ++_generationEpoch;
    _messageSyncEpoch++;
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
    _streamScheduler.dispose();
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

  int _nextLocalSequence() {
    var sequence = 0;
    for (final message in _messages.messages) {
      final current = message.sequence ?? 0;
      if (current > sequence) sequence = current;
    }
    return sequence + 1;
  }

  Map<String, dynamic> serializeMessage(ChatMessage message) =>
      <String, dynamic>{
        'id': message.id,
        'renderId': message.renderId,
        'characterId': message.characterId,
        'role': message.role.name,
        'type': message.type.name,
        'content': message.content,
        if (message.reasoningDurationMs > 0)
          'reasoningDurationMs': message.reasoningDurationMs,
        'time': message.time.toIso8601String(),
        if (message.sequence != null) 'sequence': message.sequence,
        'status': message.status.name,
        'responseGroupId': message.responseGroupId,
        'deliverySequence': message.deliverySequence,
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
