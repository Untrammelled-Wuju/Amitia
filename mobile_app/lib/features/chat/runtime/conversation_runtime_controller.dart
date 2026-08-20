import 'dart:collection';

import 'package:flutter/foundation.dart';

import '../../../core/services/chat_service.dart';
import '../../../shared/models/models.dart';

/// UI-agnostic conversation state. Native, schema and sandbox-web providers all
/// consume the same runtime through a serialized context/action contract.
class ConversationRuntimeController extends ChangeNotifier {
  ConversationRuntimeController(this._chatService);

  final ChatService _chatService;
  final List<ChatMessage> _messages = <ChatMessage>[];
  final Map<String, String> _agentTaskStatus = <String, String>{};
  String? _conversationId;
  bool _sending = false;
  Object? _lastError;

  UnmodifiableListView<ChatMessage> get messages =>
      UnmodifiableListView<ChatMessage>(_messages);
  Map<String, String> get agentTaskStatus =>
      UnmodifiableMapView<String, String>(_agentTaskStatus);
  String? get conversationId => _conversationId;
  bool get sending => _sending;
  Object? get lastError => _lastError;
  String get state => _sending ? 'sending' : 'idle';

  ChatMessage _cloneWithStatus(ChatMessage message, MessageStatus status) {
    return ChatMessage(
      id: message.id,
      role: message.role,
      type: message.type,
      content: message.content,
      time: message.time,
      status: status,
      agentTaskTitle: message.agentTaskTitle,
      agentTaskSteps: message.agentTaskSteps,
      agentTaskProgress: message.agentTaskProgress,
      agentTaskElapsed: message.agentTaskElapsed,
      fileName: message.fileName,
      fileSizeKB: message.fileSizeKB,
      toolName: message.toolName,
      toolResult: message.toolResult,
    );
  }

  Future<void> sendText(String text) {
    final now = DateTime.now();
    return addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.text,
        content: text,
        time: now,
      ),
    );
  }

  Future<void> sendFile(String fileName, int sizeKB) {
    final now = DateTime.now();
    return addUserMessage(
      ChatMessage(
        id: 'u${now.millisecondsSinceEpoch}',
        role: MessageRole.user,
        type: MessageType.file,
        content: fileName,
        time: now,
        fileName: fileName,
        fileSizeKB: sizeKB,
      ),
    );
  }

  static const _mockPrefix = '[mock]';
  static const _image = 'image';
  static const _video = 'video';
  static const _audio = 'audio';
  static const _emote = 'emote';
  static const _code = 'code';

  static String mockImagePayload(String name) => '$_mockPrefix$_image|$name';
  static String mockVideoPayload(String title) => '$_mockPrefix$_video|$title';
  static String mockAudioPayload(String duration) => '$_mockPrefix$_audio|$duration';
  static String mockEmotePayload(String emoji, String name) => '$_mockPrefix$_emote|$emoji|$name';
  static String mockCodePayload(String lang, String body) => '$_mockPrefix$_code|$lang|$body';

  Future<void> sendImage(String name) => sendText(mockImagePayload(name));

  Future<void> sendCode(String language, String code) =>
      sendText(mockCodePayload(language, code));

  Future<void> sendVoice(String duration) =>
      sendText(mockAudioPayload(duration));

  Future<void> sendEmote(String emoji, String name) =>
      sendText(mockEmotePayload(emoji, name));

  Future<void> addUserMessage(ChatMessage message) async {
    _messages.add(message);
    _lastError = null;
    notifyListeners();
    await _replyAfter(message);
  }

  Future<void> _replyAfter(ChatMessage userMessage) async {
    var content = userMessage.content;
    if (content.startsWith('__mock:audio|')) {
      content = '[语音消息]';
    } else if (content.startsWith('__mock:emote|')) {
      content = '[表情]';
    } else if (content.startsWith('__mock:code|')) {
      content = content.substring('__mock:code|'.length);
    }

    _sending = true;
    notifyListeners();
    try {
      final response = await _chatService.chat(
        content,
        conversationId: _conversationId,
      );
      final replyTime = DateTime.now();
      final reply = response?['reply'] as String? ??
          response?['content'] as String? ??
          response?['message'] as String? ??
          '已收到你的消息';
      _messages.add(
        ChatMessage(
          id: 'a${replyTime.millisecondsSinceEpoch}',
          role: MessageRole.assistant,
          type: MessageType.text,
          content: reply,
          time: replyTime,
        ),
      );
      _lastError = null;
    } catch (error) {
      _lastError = error;
      final replyTime = DateTime.now();
      _messages.add(
        ChatMessage(
          id: 'a${replyTime.millisecondsSinceEpoch}',
          role: MessageRole.assistant,
          type: MessageType.text,
          content: '连接失败: ${error.toString().replaceFirst('Exception: ', '')}',
          time: replyTime,
          status: MessageStatus.error,
        ),
      );
    } finally {
      _sending = false;
      notifyListeners();
    }
  }

  Future<void> retryMessage(int index) async {
    if (index < 0 || index >= _messages.length) return;
    final message = _messages[index];
    _messages[index] = _cloneWithStatus(message, MessageStatus.sending);
    notifyListeners();
    await Future<void>.delayed(const Duration(milliseconds: 600));
    if (index < _messages.length && _messages[index].id == message.id) {
      _messages[index] = _cloneWithStatus(
        _messages[index],
        MessageStatus.sent,
      );
      notifyListeners();
    }
  }

  void pauseAgentTask(int index) {
    if (index < 0 || index >= _messages.length) return;
    _agentTaskStatus[_messages[index].id] = '已暂停';
    notifyListeners();
  }

  void resumeAgentTask(int index) {
    if (index < 0 || index >= _messages.length) return;
    _agentTaskStatus[_messages[index].id] = '运行中';
    notifyListeners();
  }


  Future<void> regenerate({String? messageId}) async {
    ChatMessage? source;
    if (messageId != null && messageId.isNotEmpty) {
      final index = _messages.indexWhere((message) => message.id == messageId);
      if (index >= 0) {
        for (var i = index; i >= 0; i--) {
          if (_messages[i].role == MessageRole.user) {
            source = _messages[i];
            break;
          }
        }
      }
    }
    if (source == null) {
      for (var i = _messages.length - 1; i >= 0; i--) {
        if (_messages[i].role == MessageRole.user) {
          source = _messages[i];
          break;
        }
      }
    }
    if (source != null) await _replyAfter(source);
  }

  void deleteMessage(String messageId) {
    final index = _messages.indexWhere((message) => message.id == messageId);
    if (index < 0) return;
    _messages.removeAt(index);
    _agentTaskStatus.remove(messageId);
    notifyListeners();
  }

  void stop() {
    // The current mobile ChatService is request/response based and does not yet
    // expose a cancellation token. Keep the runtime contract stable and mark
    // the visual state idle; streaming transports can bind cancellation here.
    if (!_sending) return;
    _sending = false;
    notifyListeners();
  }

  Future<bool> createConversation(String? characterId) async {
    final conversation = await _chatService.createConversation(characterId);
    if (conversation == null) return false;
    _conversationId = conversation.id;
    _messages.clear();
    _agentTaskStatus.clear();
    _lastError = null;
    notifyListeners();
    return true;
  }

  void clear({bool addSystemNotice = false}) {
    _messages.clear();
    _agentTaskStatus.clear();
    if (addSystemNotice) {
      _messages.add(
        ChatMessage(
          id: 'sys${DateTime.now().millisecondsSinceEpoch}',
          role: MessageRole.system,
          type: MessageType.systemNotice,
          content: '聊天记录已清空',
          time: DateTime.now(),
        ),
      );
    }
    notifyListeners();
  }

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
        if (message.toolName != null) 'toolName': message.toolName,
        if (message.toolResult != null) 'toolResult': message.toolResult,
      };
}
