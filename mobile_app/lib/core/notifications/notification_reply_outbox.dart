import 'dart:async';
import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

class PendingNotificationReply {
  const PendingNotificationReply({
    required this.id,
    required this.conversationId,
    required this.message,
    required this.createdAtMillis,
  });

  final String id;
  final String conversationId;
  final String message;
  final int createdAtMillis;

  Map<String, dynamic> toJson() => <String, dynamic>{
    'id': id,
    'conversationId': conversationId,
    'message': message,
    'createdAtMillis': createdAtMillis,
  };

  static PendingNotificationReply? fromJson(Map<String, dynamic> json) {
    final id = (json['id'] ?? '').toString().trim();
    final conversationId = (json['conversationId'] ?? '').toString().trim();
    final message = (json['message'] ?? '').toString().trim();
    final createdAtMillis = switch (json['createdAtMillis']) {
      final num value => value.toInt(),
      final String value => int.tryParse(value) ?? 0,
      _ => 0,
    };
    if (id.isEmpty || conversationId.isEmpty || message.isEmpty) return null;
    return PendingNotificationReply(
      id: id,
      conversationId: conversationId,
      message: message,
      createdAtMillis: createdAtMillis,
    );
  }
}

class NotificationReplyOutbox {
  static const _storageKey = 'amitia.notification.reply-outbox.v1';
  static const _maxEntries = 50;
  static const _maxAge = Duration(hours: 1);

  Future<void> _tail = Future<void>.value();

  Future<T> _serialized<T>(Future<T> Function() action) {
    final completer = Completer<T>();
    final previous = _tail;
    _tail = () async {
      try {
        await previous;
        completer.complete(await action());
      } catch (error, stackTrace) {
        completer.completeError(error, stackTrace);
      }
    }();
    return completer.future;
  }

  Future<List<PendingNotificationReply>> load() => _serialized(_loadUnlocked);

  Future<void> put(PendingNotificationReply reply) => _serialized(() async {
    final entries = await _loadUnlocked();
    entries.removeWhere((entry) => entry.id == reply.id);
    entries.add(reply);
    entries.sort(
      (left, right) => left.createdAtMillis.compareTo(right.createdAtMillis),
    );
    final trimmed = entries.length > _maxEntries
        ? entries.sublist(entries.length - _maxEntries)
        : entries;
    await _saveUnlocked(trimmed);
  });

  Future<void> remove(String id) => _serialized(() async {
    final entries = await _loadUnlocked();
    entries.removeWhere((entry) => entry.id == id);
    await _saveUnlocked(entries);
  });

  Future<List<PendingNotificationReply>> _loadUnlocked() async {
    final preferences = await SharedPreferences.getInstance();
    final raw = preferences.getString(_storageKey);
    if (raw == null || raw.trim().isEmpty) return <PendingNotificationReply>[];
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! List) return <PendingNotificationReply>[];
      final cutoff = DateTime.now().subtract(_maxAge).millisecondsSinceEpoch;
      final entries = decoded
          .whereType<Map>()
          .map(
            (value) => PendingNotificationReply.fromJson(
              Map<String, dynamic>.from(value),
            ),
          )
          .whereType<PendingNotificationReply>()
          .where((entry) => entry.createdAtMillis >= cutoff)
          .toList(growable: true);
      entries.sort(
        (left, right) => left.createdAtMillis.compareTo(right.createdAtMillis),
      );
      return entries;
    } catch (_) {
      return <PendingNotificationReply>[];
    }
  }

  Future<void> _saveUnlocked(List<PendingNotificationReply> entries) async {
    final preferences = await SharedPreferences.getInstance();
    if (entries.isEmpty) {
      await preferences.remove(_storageKey);
      return;
    }
    await preferences.setString(
      _storageKey,
      jsonEncode(
        entries.map((entry) => entry.toJson()).toList(growable: false),
      ),
    );
  }
}
