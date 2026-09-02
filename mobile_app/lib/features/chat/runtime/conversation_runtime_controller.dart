import 'dart:async';
import 'dart:collection';

import 'package:flutter/foundation.dart';

import '../../../core/models/conversation.dart';
import '../../../core/services/chat_service.dart';
import '../../../core/services/channel_service.dart';
import '../../../shared/models/models.dart';

/// UI-agnostic conversation runtime shared by the built-in UI and extension UI.
///
/// Mobile uses the same asynchronous web-chat contract as the desktop client:
/// submit -> generation status -> persisted messages. This keeps cancellation,
/// retries and media messages backed by real server state instead of UI mocks.
class ConversationRuntimeController extends ChangeNotifier {
  ConversationRuntimeController(this._chatService, this._emoteService);

  final ChatService _chatService;
  final EmoteService _emoteService;
  final List<ChatMessage> _messages = <ChatMessage>[];
  String? _conversationId;
  String? _characterId;
  bool _sending = false;
  Object? _lastError;
  int _generationEpoch = 0;

  UnmodifiableListView<ChatMessage> get messages =>
      UnmodifiableListView<ChatMessage>(_messages);
  String? get conversationId => _conversationId;
  bool get sending => _sending;
  Object? get lastError => _lastError;
  String get state => _sending ? 'sending' : 'idle';

  void setCharacterId(String? characterId) {
    _characterId = characterId?.trim().isEmpty == true ? null : characterId;
  }

  ChatMessage _copy(
    ChatMessage message, {
    String? id,
    MessageStatus? status,
  }) {
    return ChatMessage(
      id: id ?? message.id,
      role: message.role,
      type: message.type,
      content: message.content,
      time: message.time,
      status: status ?? message.status,
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

  Future<void> sendText(String text, {String? replyToMessageId, String? replyToExcerpt}) async {
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
    if (conv == null || conv.isEmpty || character == null || character.isEmpty) {
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
  }) async {
    if (_sending) return;
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.image,
        content: '[图片]',
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
      ),
      message: '[图片]',
      imageUrl: resourceUri,
    );
  }

  Future<void> sendVideo({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'video/*',
    int durationMs = 0,
  }) async {
    if (_sending) return;
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.video,
        content: '[视频]',
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
        durationMs: durationMs,
      ),
      message: '[视频]',
      videoUrl: resourceUri,
    );
  }

  Future<void> sendVoice({
    required String resourceUri,
    required String displayUrl,
    required String fileName,
    String mimeType = 'audio/*',
    int durationMs = 0,
  }) async {
    if (_sending) return;
    await _send(
      ChatMessage(
        id: _localId('u'),
        role: MessageRole.user,
        type: MessageType.audio,
        content: '[语音]',
        time: DateTime.now(),
        status: MessageStatus.sending,
        fileName: fileName,
        resourceUri: resourceUri,
        mediaUrl: displayUrl,
        mimeType: mimeType,
        durationMs: durationMs,
      ),
      message: '[语音]',
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
    _lastError = null;
    _sending = true;
    final epoch = ++_generationEpoch;
    notifyListeners();

    try {
      final result = await _chatService.submitMessage(
        message: message,
        conversationId: _conversationId,
        characterId: _characterId,
        imageUrl: imageUrl,
        audioUrl: audioUrl,
        audioDuration: audioDuration,
        videoUrl: videoUrl,
        replyToMessageId: replyToMessageId,
      );
      if (epoch != _generationEpoch) return;
      if (result.conversationId.isNotEmpty) {
        _conversationId = result.conversationId;
      }
      final localIndex = _messages.indexWhere((m) => m.id == localMessage.id);
      if (localIndex >= 0) {
        _messages[localIndex] = _copy(
          _messages[localIndex],
          id: result.userMessageId.isEmpty ? null : result.userMessageId,
          status: MessageStatus.sent,
        );
      }
      notifyListeners();
      await _awaitGeneration(epoch);
      _lastError = null;
    } catch (error) {
      if (epoch != _generationEpoch) return;
      _lastError = error;
      final localIndex = _messages.indexWhere((m) => m.id == localMessage.id);
      if (localIndex >= 0) {
        _messages[localIndex] = _copy(
          _messages[localIndex],
          status: MessageStatus.error,
        );
      }
    } finally {
      if (epoch == _generationEpoch) {
        _sending = false;
        notifyListeners();
      }
    }
  }

  Future<void> _awaitGeneration(int epoch) async {
    final conv = _conversationId;
    if (conv == null || conv.isEmpty) return;
    final deadline = DateTime.now().add(const Duration(minutes: 2));
    var sawActiveState = false;
    while (epoch == _generationEpoch && DateTime.now().isBefore(deadline)) {
      final status = await _chatService.generationStatus(conv);
      if (status == 'collecting' || status == 'processing') {
        sawActiveState = true;
        await Future<void>.delayed(const Duration(milliseconds: 250));
        continue;
      }
      if (status == 'failed') {
        throw StateError('AI 生成失败');
      }
      if (status == 'cancelled') {
        await _syncMessages();
        return;
      }
      if (status == 'completed' || (status == 'idle' && sawActiveState)) {
        await _syncMessages();
        return;
      }
      // The submit goroutine may not have entered collecting/processing yet.
      await Future<void>.delayed(const Duration(milliseconds: 150));
    }
    if (epoch != _generationEpoch) return;
    await _syncMessages();
  }

  Future<void> _syncMessages() async {
    final conv = _conversationId;
    if (conv == null || conv.isEmpty) return;
    final persisted = await _chatService.getMessages(conv);
    if (persisted.isEmpty) return;

    final localById = <String, ChatMessage>{
      for (final message in _messages) message.id: message,
    };
    final mapped = persisted.map((dto) {
      final existing = localById[dto.id];
      final type = _typeForDto(dto, existing);
      return ChatMessage(
        id: dto.id,
        role: _roleFor(dto.role),
        type: type,
        content: dto.content,
        time: DateTime.tryParse(dto.createdAt) ?? existing?.time ?? DateTime.now(),
        status: dto.status == 'failed'
            ? MessageStatus.error
            : MessageStatus.delivered,
        fileName: existing?.fileName,
        fileSizeKB: existing?.fileSizeKB,
        resourceUri: _resourceForDto(dto, existing),
        mediaUrl: existing?.mediaUrl,
        mimeType: existing?.mimeType,
        durationMs: dto.audioDuration > 0
            ? (dto.audioDuration * 1000).round()
            : existing?.durationMs,
        replyToMessageId: dto.replyToMessageId ?? existing?.replyToMessageId,
        replyToExcerpt: dto.replyToExcerpt ?? existing?.replyToExcerpt,
      );
    }).toList(growable: false);
    _messages
      ..clear()
      ..addAll(mapped);
    notifyListeners();
  }

  MessageType _typeForDto(MessageDto dto, ChatMessage? existing) {
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

  Future<void> retryMessage(int index) async {
    if (_sending || index < 0 || index >= _messages.length) return;
    final message = _messages[index];
    if (message.role != MessageRole.user) return;
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

  Future<void> openConversation(String conversationId, {String? characterId}) async {
    final id = conversationId.trim();
    if (id.isEmpty) return;
    ++_generationEpoch;
    _conversationId = id;
    _characterId = characterId?.trim().isEmpty == true ? null : characterId;
    _messages.clear();
    _lastError = null;
    _sending = false;
    notifyListeners();
    try {
      await _syncMessages();
    } catch (error) {
      _lastError = error;
      notifyListeners();
    }
  }

  Future<bool> createConversation(String? characterId) async {
    final conversation = await _chatService.createConversation(characterId);
    if (conversation == null) return false;
    _conversationId = conversation.id;
    _characterId = characterId;
    _messages.clear();
    _lastError = null;
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

  String _localId(String prefix) =>
      '$prefix${DateTime.now().microsecondsSinceEpoch}';

  Map<String, dynamic> serializeMessage(ChatMessage message) =>
      <String, dynamic>{
        'id': message.id,
        'role': message.role.name,
        'type': message.type.name,
        'content': message.content,
        'time': message.time.toIso8601String(),
        'status': message.status.name,
        if (message.fileName != null) 'fileName': message.fileName,
        if (message.fileSizeKB != null) 'fileSizeKB': message.fileSizeKB,
        if (message.resourceUri != null) 'resourceUri': message.resourceUri,
        if (message.mediaUrl != null) 'mediaUrl': message.mediaUrl,
        if (message.mimeType != null) 'mimeType': message.mimeType,
        if (message.durationMs != null) 'durationMs': message.durationMs,
        if (message.toolName != null) 'toolName': message.toolName,
        if (message.toolResult != null) 'toolResult': message.toolResult,
        if ((message.replyToMessageId ?? '').isNotEmpty) 'replyToMessageId': message.replyToMessageId,
        if ((message.replyToExcerpt ?? '').isNotEmpty) 'replyToExcerpt': message.replyToExcerpt,
      };
}
