import 'dart:collection';

import '../../../shared/models/models.dart';

class ConversationMessageLedger {
  final LinkedHashMap<String, ChatMessage> _items =
      LinkedHashMap<String, ChatMessage>();

  List<ChatMessage> get messages {
    final items = _items.values.toList(growable: false);
    items.sort(_compare);
    return List<ChatMessage>.unmodifiable(items);
  }

  int get length => _items.length;
  bool get isEmpty => _items.isEmpty;

  ChatMessage? findById(String id) {
    final normalized = id.trim();
    if (normalized.isEmpty) return null;
    for (final message in _items.values) {
      if (message.id == normalized) return message;
    }
    return null;
  }

  ChatMessage? findByRenderId(String renderId, {MessageRole? role}) {
    final normalized = renderId.trim();
    if (normalized.isEmpty) return null;
    for (final message in _items.values) {
      if (role != null && message.role != role) continue;
      if (message.renderId == normalized) return message;
    }
    return null;
  }

  bool upsert(ChatMessage message) {
    final key = _key(message);
    final existingKey = _findKey(message, key);
    final current = existingKey == null ? null : _items[existingKey];
    if (existingKey != null && existingKey != key) {
      _items.remove(existingKey);
    }
    _items[key] = message;
    return current == null || !_same(current, message);
  }

  bool remove(ChatMessage message) {
    final key = _findKey(message, _key(message));
    if (key == null) return false;
    _items.remove(key);
    return true;
  }

  bool removeById(String id) {
    final message = findById(id);
    return message != null && remove(message);
  }

  void clear() {
    _items.clear();
  }

  String? _findKey(ChatMessage message, String preferredKey) {
    if (_items.containsKey(preferredKey)) return preferredKey;
    final id = message.id.trim();
    if (id.isNotEmpty) {
      final byId = findById(id);
      if (byId != null) return _key(byId);
    }
    final renderId = message.renderId.trim();
    if (renderId.isNotEmpty) {
      final byRenderId = findByRenderId(renderId, role: message.role);
      if (byRenderId != null) return _key(byRenderId);
    }
    return null;
  }

  String _key(ChatMessage message) {
    final renderId = message.renderId.trim();
    final id = message.id.trim();
    return '${message.role.name}:${renderId.isNotEmpty ? renderId : id}';
  }

  int _compare(ChatMessage a, ChatMessage b) {
    final aSequence = a.sequence;
    final bSequence = b.sequence;
    if (aSequence != null && bSequence != null && aSequence != bSequence) {
      return aSequence.compareTo(bSequence);
    }
    final timeOrder = a.time.compareTo(b.time);
    if (timeOrder != 0) return timeOrder;
    if (aSequence != null && bSequence == null) return -1;
    if (aSequence == null && bSequence != null) return 1;
    return _key(a).compareTo(_key(b));
  }

  bool _same(ChatMessage a, ChatMessage b) {
    return a.id == b.id &&
        a.renderId == b.renderId &&
        a.role == b.role &&
        a.type == b.type &&
        a.content == b.content &&
        a.reasoningContent == b.reasoningContent &&
        a.time == b.time &&
        a.sequence == b.sequence &&
        a.status == b.status &&
        a.agentTaskId == b.agentTaskId &&
        a.agentTaskTitle == b.agentTaskTitle &&
        _sameStringList(a.agentTaskSteps, b.agentTaskSteps) &&
        a.agentTaskProgress == b.agentTaskProgress &&
        a.agentTaskElapsed == b.agentTaskElapsed &&
        a.fileName == b.fileName &&
        a.fileSizeKB == b.fileSizeKB &&
        a.resourceUri == b.resourceUri &&
        a.mediaUrl == b.mediaUrl &&
        a.mimeType == b.mimeType &&
        a.durationMs == b.durationMs &&
        a.toolName == b.toolName &&
        a.toolResult == b.toolResult &&
        a.replyToMessageId == b.replyToMessageId &&
        a.replyToExcerpt == b.replyToExcerpt;
  }

  bool _sameStringList(List<String>? a, List<String>? b) {
    if (identical(a, b)) return true;
    if (a == null || b == null || a.length != b.length) return false;
    for (var index = 0; index < a.length; index += 1) {
      if (a[index] != b[index]) return false;
    }
    return true;
  }
}
