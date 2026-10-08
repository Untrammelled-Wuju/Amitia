import 'dart:async';
import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

class PendingNotificationCallAction {
  const PendingNotificationCallAction({
    required this.operation,
    required this.callId,
    required this.conversationId,
    required this.reason,
    required this.createdAtMillis,
  });

  final String operation;
  final String callId;
  final String conversationId;
  final String reason;
  final int createdAtMillis;

  Map<String, dynamic> toJson() => <String, dynamic>{
    'operation': operation,
    'callId': callId,
    'conversationId': conversationId,
    'reason': reason,
    'createdAtMillis': createdAtMillis,
  };

  static PendingNotificationCallAction? fromJson(Map<String, dynamic> json) {
    final rawOperation = (json['operation'] ?? 'end')
        .toString()
        .trim()
        .toLowerCase();
    final operation = rawOperation == 'answer' ? 'answer' : 'end';
    final callId = (json['callId'] ?? '').toString().trim();
    final conversationId = (json['conversationId'] ?? '').toString().trim();
    final reason = (json['reason'] ?? 'user_declined').toString().trim();
    final createdAtMillis = switch (json['createdAtMillis']) {
      final num value => value.toInt(),
      final String value => int.tryParse(value) ?? 0,
      _ => 0,
    };
    if (callId.isEmpty || createdAtMillis <= 0) return null;
    return PendingNotificationCallAction(
      operation: operation,
      callId: callId,
      conversationId: conversationId,
      reason: reason.isEmpty
          ? (operation == 'answer' ? 'answered' : 'user_declined')
          : reason,
      createdAtMillis: createdAtMillis,
    );
  }
}

class NotificationCallActionOutbox {
  static const _storageKey = 'amitia.notification.call-action-outbox.v1';
  static const _maxEntries = 20;
  static const _maxAge = Duration(minutes: 2);

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

  Future<List<PendingNotificationCallAction>> load() =>
      _serialized(_loadUnlocked);

  Future<void> put(PendingNotificationCallAction action) =>
      _serialized(() async {
        final entries = await _loadUnlocked();
        // A later lifecycle decision for the same call supersedes an earlier
        // one. In particular, a local hangup replaces a queued answer ack.
        entries.removeWhere((entry) => entry.callId == action.callId);
        entries.add(action);
        entries.sort(
          (left, right) =>
              left.createdAtMillis.compareTo(right.createdAtMillis),
        );
        final trimmed = entries.length > _maxEntries
            ? entries.sublist(entries.length - _maxEntries)
            : entries;
        await _saveUnlocked(trimmed);
      });

  Future<void> remove(String callId) => _serialized(() async {
    final entries = await _loadUnlocked();
    entries.removeWhere((entry) => entry.callId == callId);
    await _saveUnlocked(entries);
  });

  Future<List<PendingNotificationCallAction>> _loadUnlocked() async {
    final preferences = await SharedPreferences.getInstance();
    final raw = preferences.getString(_storageKey);
    if (raw == null || raw.trim().isEmpty) {
      return <PendingNotificationCallAction>[];
    }
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! List) return <PendingNotificationCallAction>[];
      final cutoff = DateTime.now().subtract(_maxAge).millisecondsSinceEpoch;
      final entries = decoded
          .whereType<Map>()
          .map(
            (value) => PendingNotificationCallAction.fromJson(
              Map<String, dynamic>.from(value),
            ),
          )
          .whereType<PendingNotificationCallAction>()
          .where((entry) => entry.createdAtMillis >= cutoff)
          .toList(growable: true);
      entries.sort(
        (left, right) => left.createdAtMillis.compareTo(right.createdAtMillis),
      );
      return entries;
    } catch (_) {
      return <PendingNotificationCallAction>[];
    }
  }

  Future<void> _saveUnlocked(
    List<PendingNotificationCallAction> entries,
  ) async {
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
