import 'dart:async';

import 'package:flutter/services.dart';

class NotificationPlatformState {
  final String platform;
  final String? provider;
  final String? appearance;
  final String? pushToken;
  final String? voipToken;
  final List<String> invalidatedProviders;
  final Map<String, String> vendorTokens;
  final List<String> nativeDataProviders;
  final bool pushConfigured;
  final bool notificationsEnabled;
  final bool liveActivitySupported;
  final bool dynamicIslandSupported;
  final String? islandProvider;
  final bool xiaomiFocusPermissionGranted;
  final bool progressStyleSupported;
  final bool communicationNotificationSupported;
  final String? liveActivityPushToStartToken;

  const NotificationPlatformState({
    required this.platform,
    this.provider,
    this.appearance,
    this.pushToken,
    this.voipToken,
    this.invalidatedProviders = const <String>[],
    this.vendorTokens = const <String, String>{},
    this.nativeDataProviders = const <String>[],
    this.pushConfigured = false,
    this.notificationsEnabled = false,
    this.liveActivitySupported = false,
    this.dynamicIslandSupported = false,
    this.islandProvider,
    this.xiaomiFocusPermissionGranted = false,
    this.progressStyleSupported = false,
    this.communicationNotificationSupported = false,
    this.liveActivityPushToStartToken,
  });

  factory NotificationPlatformState.fromMap(Map<Object?, Object?> raw) {
    String? text(String key) {
      final value = raw[key]?.toString().trim();
      return value == null || value.isEmpty ? null : value;
    }

    return NotificationPlatformState(
      platform: text('platform') ?? '',
      provider: text('provider'),
      appearance: text('appearance'),
      pushToken: text('pushToken'),
      voipToken: text('voipToken'),
      invalidatedProviders:
          (raw['invalidatedProviders'] is List
                  ? raw['invalidatedProviders'] as List
                  : const <Object?>[])
              .map((value) => value.toString().trim())
              .where((value) => value.isNotEmpty)
              .toList(growable: false),
      vendorTokens: raw['vendorTokens'] is Map
          ? Map<Object?, Object?>.from(raw['vendorTokens'] as Map)
                .map(
                  (key, value) =>
                      MapEntry(key.toString().trim(), value.toString().trim()),
                )
                .cast<String, String>()
          : const <String, String>{},
      nativeDataProviders:
          (raw['nativeDataProviders'] is List
                  ? raw['nativeDataProviders'] as List
                  : const <Object?>[])
              .map((value) => value.toString().trim())
              .where((value) => value.isNotEmpty)
              .toList(growable: false),
      pushConfigured: raw['pushConfigured'] == true,
      notificationsEnabled: raw['notificationsEnabled'] == true,
      liveActivitySupported: raw['liveActivitySupported'] == true,
      dynamicIslandSupported: raw['dynamicIslandSupported'] == true,
      islandProvider: text('islandProvider'),
      xiaomiFocusPermissionGranted: raw['xiaomiFocusPermissionGranted'] == true,
      progressStyleSupported: raw['progressStyleSupported'] == true,
      communicationNotificationSupported:
          raw['communicationNotificationSupported'] == true,
      liveActivityPushToStartToken: text('liveActivityPushToStartToken'),
    );
  }
}

class NotificationPlatformInteraction {
  final String? deepLink;
  final String? reply;
  final String? conversationId;
  final String? callId;
  final String? reason;
  final String source;

  const NotificationPlatformInteraction({
    this.deepLink,
    this.reply,
    this.conversationId,
    this.callId,
    this.reason,
    this.source = '',
  });

  factory NotificationPlatformInteraction.fromMap(Map<Object?, Object?> raw) {
    String? optional(String key) {
      final value = raw[key]?.toString().trim();
      return value == null || value.isEmpty ? null : value;
    }

    return NotificationPlatformInteraction(
      deepLink: optional('deepLink'),
      reply: optional('reply'),
      conversationId: optional('conversationId'),
      callId: optional('callId'),
      reason: optional('reason'),
      source: optional('source') ?? '',
    );
  }
}

class LiveActivityPushTokenUpdate {
  final String runId;
  final String conversationId;
  final String activityId;
  final String updateToken;
  final int revision;

  const LiveActivityPushTokenUpdate({
    required this.runId,
    this.conversationId = '',
    required this.activityId,
    required this.updateToken,
    required this.revision,
  });

  factory LiveActivityPushTokenUpdate.fromMap(Map<Object?, Object?> raw) {
    return LiveActivityPushTokenUpdate(
      runId: (raw['runId'] ?? '').toString().trim(),
      conversationId: (raw['conversationId'] ?? '').toString().trim(),
      activityId: (raw['activityId'] ?? '').toString().trim(),
      updateToken: (raw['updateToken'] ?? '').toString().trim(),
      revision: (raw['revision'] as num?)?.toInt() ?? 0,
    );
  }

  bool get isValid =>
      runId.isNotEmpty && activityId.isNotEmpty && updateToken.isNotEmpty;
}

class NotificationPlatformBridge {
  static const MethodChannel _channel = MethodChannel(
    'com.amitia.notifications/control',
  );

  final StreamController<Map<String, Object?>> _tokenChanges =
      StreamController<Map<String, Object?>>.broadcast();
  final StreamController<NotificationPlatformInteraction> _interactions =
      StreamController<NotificationPlatformInteraction>.broadcast();
  final StreamController<LiveActivityPushTokenUpdate>
  _liveActivityTokenChanges =
      StreamController<LiveActivityPushTokenUpdate>.broadcast();

  NotificationPlatformBridge() {
    _channel.setMethodCallHandler(_handleNativeCall);
  }

  Stream<Map<String, Object?>> get tokenChanges => _tokenChanges.stream;
  Stream<NotificationPlatformInteraction> get interactions =>
      _interactions.stream;
  Stream<LiveActivityPushTokenUpdate> get liveActivityTokenChanges =>
      _liveActivityTokenChanges.stream;

  Future<NotificationPlatformState> initialize() async {
    final raw = await _channel.invokeMethod<Map<Object?, Object?>>(
      'initialize',
    );
    return NotificationPlatformState.fromMap(raw ?? const <Object?, Object?>{});
  }

  Future<NotificationPlatformState> capabilities() async {
    final raw = await _channel.invokeMethod<Map<Object?, Object?>>(
      'getCapabilities',
    );
    return NotificationPlatformState.fromMap(raw ?? const <Object?, Object?>{});
  }

  Future<bool> requestPermission() async {
    return await _channel.invokeMethod<bool>('requestPermission') ?? false;
  }

  Future<void> openSettings() => _channel.invokeMethod<void>('openSettings');

  Future<Map<String, dynamic>> floatingBubbleStatus() async =>
      Map<String, dynamic>.from(
        await _channel.invokeMethod<Map<Object?, Object?>>(
              'floatingBubbleStatus',
            ) ??
            {},
      );

  Future<Map<String, dynamic>> enableFloatingBubble() async =>
      Map<String, dynamic>.from(
        await _channel.invokeMethod<Map<Object?, Object?>>(
              'floatingBubbleEnable',
            ) ??
            {},
      );

  Future<Map<String, dynamic>> configureFloatingBubble({required bool previewEnabled}) async =>
      Map<String, dynamic>.from(
        await _channel.invokeMethod<Map<Object?, Object?>>(
              'floatingBubbleConfigure',
              <String, Object>{'previewEnabled': previewEnabled},
            ) ??
            {},
      );

  Future<Map<String, dynamic>> disableFloatingBubble() async =>
      Map<String, dynamic>.from(
        await _channel.invokeMethod<Map<Object?, Object?>>(
              'floatingBubbleDisable',
            ) ??
            {},
      );

  Future<void> openFloatingBubblePermission() async {
    await _channel.invokeMethod<bool>('floatingBubblePermission');
  }

  Future<void> sendLocalNotificationScenario(String scenario) async {
    await _channel.invokeMethod<bool>(
      'sendLocalNotificationScenario',
      <String, String>{'scenario': scenario},
    );
  }

  Future<void> clearExecutionNotifications() =>
      _channel.invokeMethod<void>('clearExecutionNotifications');

  Future<void> clearMessageNotifications() =>
      _channel.invokeMethod<void>('clearMessageNotifications');

  Future<void> clearCallNotifications() =>
      _channel.invokeMethod<void>('clearCallNotifications');

  Future<void> clearReminderNotifications() =>
      _channel.invokeMethod<void>('clearReminderNotifications');

  Future<void> endCall(String callId, {String reason = ''}) async {
    final normalized = callId.trim();
    if (normalized.isEmpty) return;
    await _channel.invokeMethod<void>('endCall', <String, Object?>{
      'callId': normalized,
      'reason': reason.trim(),
    });
  }

  Future<NotificationPlatformInteraction?> consumeInitialInteraction() async {
    final raw = await _channel.invokeMethod<Map<Object?, Object?>>(
      'consumeInitialInteraction',
    );
    if (raw == null || raw.isEmpty) return null;
    return NotificationPlatformInteraction.fromMap(raw);
  }

  Future<void> _handleNativeCall(MethodCall call) async {
    switch (call.method) {
      case 'pushTokenChanged':
        final raw = call.arguments;
        if (raw is Map) {
          _tokenChanges.add(
            raw.map((key, value) => MapEntry(key.toString(), value)),
          );
        }
      case 'notificationInteraction':
        final raw = call.arguments;
        if (raw is Map) {
          _interactions.add(
            NotificationPlatformInteraction.fromMap(
              Map<Object?, Object?>.from(raw),
            ),
          );
        }
      case 'liveActivityTokenChanged':
        final raw = call.arguments;
        if (raw is Map) {
          final update = LiveActivityPushTokenUpdate.fromMap(
            Map<Object?, Object?>.from(raw),
          );
          if (update.isValid) {
            _liveActivityTokenChanges.add(update);
          }
        }
    }
  }

  Future<void> dispose() async {
    await _tokenChanges.close();
    await _interactions.close();
    await _liveActivityTokenChanges.close();
    _channel.setMethodCallHandler(null);
  }
}
