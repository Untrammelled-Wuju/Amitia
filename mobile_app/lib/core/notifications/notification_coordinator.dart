import 'dart:async';
import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

import '../backend_transport/backend_service_api.dart';
import '../native_bridge/device_timezone_cache.dart';
import '../services/chat_service.dart';
import 'notification_call_action_outbox.dart';
import 'notification_platform_bridge.dart';
import 'notification_reply_outbox.dart';

class NotificationCallLifecycleEvent {
  const NotificationCallLifecycleEvent({
    required this.callId,
    required this.conversationId,
    required this.reason,
  });

  final String callId;
  final String conversationId;
  final String reason;
}

class NotificationCoordinator with WidgetsBindingObserver {
  NotificationCoordinator({
    required BackendServiceApi api,
    required ChatService chat,
    required GoRouter router,
    NotificationPlatformBridge? platform,
  }) : _api = api,
       _chat = chat,
       _router = router,
       _platform = platform ?? NotificationPlatformBridge();

  final BackendServiceApi _api;
  final ChatService _chat;
  final GoRouter _router;
  final NotificationPlatformBridge _platform;
  final NotificationReplyOutbox _replyOutbox = NotificationReplyOutbox();
  final NotificationCallActionOutbox _callActionOutbox =
      NotificationCallActionOutbox();
  final StreamController<NotificationCallLifecycleEvent> _callLifecycleEvents =
      StreamController<NotificationCallLifecycleEvent>.broadcast();

  StreamSubscription<Map<String, Object?>>? _tokenSubscription;
  StreamSubscription<NotificationPlatformInteraction>? _interactionSubscription;
  StreamSubscription<LiveActivityPushTokenUpdate>?
  _liveActivityTokenSubscription;
  NotificationPlatformState? _platformState;
  Timer? _retryTimer;
  bool _started = false;
  bool _disposed = false;
  bool _refreshing = false;
  bool _refreshPending = false;
  bool _suppressRegistration = false;
  bool _drainingReplies = false;
  bool _drainingCallActions = false;
  Completer<void>? _refreshIdleCompleter;
  int _retryAttempt = 0;
  String? _registeredDeviceId;
  String? _activeConversationId;
  String? _lastInteractionFingerprint;
  DateTime? _lastInteractionAt;

  NotificationPlatformState? get platformState => _platformState;
  Stream<NotificationCallLifecycleEvent> get callLifecycleEvents =>
      _callLifecycleEvents.stream;

  Future<void> start() async {
    if (_started || _disposed) return;
    _started = true;
    WidgetsBinding.instance.addObserver(this);
    _router.routerDelegate.addListener(_handleRouteChanged);
    _handleRouteChanged();
    _tokenSubscription = _platform.tokenChanges.listen((_) {
      unawaited(refreshRegistration());
    });
    _interactionSubscription = _platform.interactions.listen((interaction) {
      unawaited(_handleInteraction(interaction));
    });
    _liveActivityTokenSubscription = _platform.liveActivityTokenChanges.listen(
      (update) => unawaited(_syncLiveActivityToken(update)),
    );
    try {
      _platformState = await _platform.initialize();
      await refreshRegistration();
      final initial = await _platform.consumeInitialInteraction();
      if (initial != null) await _handleInteraction(initial);
      await _drainPendingReplies();
      await _drainPendingCallActions();
    } catch (_) {
      _scheduleRetry();
    }
  }

  Future<bool> requestPermission() async {
    final granted = await _platform.requestPermission();
    _platformState = await _platform.capabilities();
    await refreshRegistration();
    return granted;
  }

  Future<void> openSystemSettings() => _platform.openSettings();

  Future<Map<String, dynamic>> floatingBubbleStatus() =>
      _platform.floatingBubbleStatus();

  Future<Map<String, dynamic>> enableFloatingBubble() =>
      _platform.enableFloatingBubble();

  Future<Map<String, dynamic>> disableFloatingBubble() =>
      _platform.disableFloatingBubble();

  Future<void> openFloatingBubblePermission() =>
      _platform.openFloatingBubblePermission();

  Future<void> sendLocalNotificationScenario(String scenario) =>
      _platform.sendLocalNotificationScenario(scenario);

  Future<void> refreshRegistration() async {
    if (_disposed || _suppressRegistration) return;
    if (_refreshing) {
      _refreshPending = true;
      return;
    }
    _refreshing = true;
    _refreshIdleCompleter = Completer<void>();
    try {
      final state = await _platform.capabilities();
      _platformState = state;
      final platform = state.platform.isNotEmpty
          ? state.platform
          : (Platform.isIOS ? 'ios' : 'android');
      final payload = <String, dynamic>{
        'platform': platform,
        'deviceName': Platform.localHostname,
        'osVersion': Platform.operatingSystemVersion,
        'locale': WidgetsBinding.instance.platformDispatcher.locale
            .toLanguageTag(),
        if (state.appearance != null) 'appearance': state.appearance,
        if (DeviceTimezoneCache.hasValue)
          'timezone': DeviceTimezoneCache.ianaTimezone,
        'preferredProvider':
            state.provider ?? (platform == 'ios' ? 'apns' : 'fcm'),
        'systemNotificationsEnabled': state.notificationsEnabled,
        if (platform == 'android')
          'nativeDataProviders': state.nativeDataProviders,
        if (platform == 'android' && state.pushToken != null)
          'fcmToken': state.pushToken,
        if (platform == 'ios' && state.pushToken != null)
          'apnsToken': state.pushToken,
        if (state.voipToken != null) 'voipToken': state.voipToken,
        if (state.vendorTokens['hms']?.isNotEmpty == true)
          'hmsToken': state.vendorTokens['hms'],
        if (state.vendorTokens['mipush']?.isNotEmpty == true)
          'miPushToken': state.vendorTokens['mipush'],
        if (state.vendorTokens['oppo']?.isNotEmpty == true)
          'oppoToken': state.vendorTokens['oppo'],
        if (state.vendorTokens['vivo']?.isNotEmpty == true)
          'vivoToken': state.vendorTokens['vivo'],
        if (state.vendorTokens['honor']?.isNotEmpty == true)
          'honorToken': state.vendorTokens['honor'],
        if (state.liveActivityPushToStartToken != null)
          'liveActivityPushToStartToken': state.liveActivityPushToStartToken,
        if (state.invalidatedProviders.isNotEmpty)
          'clearTokens': state.invalidatedProviders,
        'liveActivitySupported': state.liveActivitySupported,
        'dynamicIslandSupported': state.dynamicIslandSupported,
        'progressStyleSupported': state.progressStyleSupported,
        'communicationSupported': state.communicationNotificationSupported,
      };
      final registered = await _api.post<Map<String, dynamic>>(
        '/api/notifications/devices/register',
        data: payload,
      );
      final registeredDeviceId = (registered?['deviceId'] ?? '')
          .toString()
          .trim();
      if (registeredDeviceId.isNotEmpty) {
        _registeredDeviceId = registeredDeviceId;
      }
      _retryAttempt = 0;
      _retryTimer?.cancel();
      _retryTimer = null;
      await _sendPresence();
      unawaited(_drainPendingReplies());
      unawaited(_drainPendingCallActions());
    } catch (_) {
      _scheduleRetry();
    } finally {
      _refreshing = false;
      final idle = _refreshIdleCompleter;
      _refreshIdleCompleter = null;
      if (idle != null && !idle.isCompleted) {
        idle.complete();
      }
      if (_refreshPending && !_disposed && !_suppressRegistration) {
        _refreshPending = false;
        unawaited(refreshRegistration());
      }
    }
  }

  Future<void> updatePreferences({
    bool? pushEnabled,
    bool? messagePushEnabled,
    bool? executionActivityEnabled,
    bool? callPushEnabled,
    bool? reminderPushEnabled,
    String? previewMode,
    bool? soundEnabled,
  }) async {
    final deviceId = await _effectiveDeviceId();
    if (deviceId == null) {
      await refreshRegistration();
      return;
    }
    await _api.patch<Map<String, dynamic>>(
      '/api/notifications/devices/${Uri.encodeComponent(deviceId)}/preferences',
      data: <String, dynamic>{
        'pushEnabled': ?pushEnabled,
        'messagePushEnabled': ?messagePushEnabled,
        'executionActivityEnabled': ?executionActivityEnabled,
        'callPushEnabled': ?callPushEnabled,
        'reminderPushEnabled': ?reminderPushEnabled,
        'previewMode': ?previewMode,
        'soundEnabled': ?soundEnabled,
      },
    );
    if (pushEnabled == false || messagePushEnabled == false) {
      try {
        await _platform.clearMessageNotifications();
      } catch (_) {}
    }
    if (pushEnabled == false || executionActivityEnabled == false) {
      try {
        await _platform.clearExecutionNotifications();
      } catch (_) {}
    }
    if (pushEnabled == false || callPushEnabled == false) {
      try {
        await _platform.clearCallNotifications();
      } catch (_) {}
    }
    if (pushEnabled == false || reminderPushEnabled == false) {
      try {
        await _platform.clearReminderNotifications();
      } catch (_) {}
    }
  }

  Future<Map<String, dynamic>> serverCapabilities() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/notifications/capabilities',
    );
    return response ?? const <String, dynamic>{};
  }

  Future<List<Map<String, dynamic>>> devices() async {
    final response = await _api.get<dynamic>('/api/notifications/devices');
    final source = response is List
        ? response
        : response is Map && response['items'] is List
        ? response['items'] as List
        : const <dynamic>[];
    return source
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .toList(growable: false);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    unawaited(_sendPresence());
    if (state == AppLifecycleState.resumed) {
      unawaited(refreshRegistration());
    }
  }

  void _handleRouteChanged() {
    final uri = _router.routerDelegate.currentConfiguration.uri;
    final next = uri.path == '/chat'
        ? uri.queryParameters['conversationId']?.trim()
        : null;
    if (next == _activeConversationId) return;
    _activeConversationId = next;
    unawaited(_sendPresence());
  }

  Future<void> _sendPresence() async {
    if (_disposed) return;
    try {
      await _api.post<Map<String, dynamic>>(
        '/api/notifications/presence',
        data: <String, dynamic>{
          'foreground':
              WidgetsBinding.instance.lifecycleState ==
              AppLifecycleState.resumed,
          'activeConversationId': _activeConversationId ?? '',
        },
      );
    } catch (_) {}
  }

  Future<String?> _effectiveDeviceId() async {
    final registered = _registeredDeviceId?.trim();
    if (registered != null && registered.isNotEmpty) return registered;
    await refreshRegistration();
    final refreshed = _registeredDeviceId?.trim();
    return refreshed == null || refreshed.isEmpty ? null : refreshed;
  }

  Future<void> _syncLiveActivityToken(
    LiveActivityPushTokenUpdate update,
  ) async {
    if (_disposed || !update.isValid) return;
    try {
      await _api.post<Map<String, dynamic>>(
        '/api/notifications/live-activities/token',
        data: <String, dynamic>{
          'runId': update.runId,
          if (update.conversationId.isNotEmpty)
            'conversationId': update.conversationId,
          'activityId': update.activityId,
          'updateToken': update.updateToken,
          'revision': update.revision,
        },
      );
    } catch (_) {
      _scheduleRetry();
    }
  }

  Future<Map<String, dynamic>?> currentDevice() async {
    await refreshRegistration();
    final deviceId = await _effectiveDeviceId();
    if (deviceId == null) return null;
    final rows = await devices();
    for (final row in rows) {
      if ((row['deviceId'] ?? '').toString().trim() == deviceId) {
        return row;
      }
    }
    return null;
  }

  Future<Map<String, dynamic>?> testNotification() async {
    final deviceId = await _effectiveDeviceId();
    if (deviceId == null) return null;
    return _api.post<Map<String, dynamic>>(
      '/api/notifications/devices/${Uri.encodeComponent(deviceId)}/test',
    );
  }

  Future<bool> answerIncomingCall({
    required String callId,
    required String conversationId,
  }) async {
    final normalizedCallId = callId.trim();
    if (_disposed || _suppressRegistration || normalizedCallId.isEmpty) {
      return false;
    }
    final pending = PendingNotificationCallAction(
      operation: 'answer',
      callId: normalizedCallId,
      conversationId: conversationId.trim(),
      reason: 'answered',
      createdAtMillis: DateTime.now().millisecondsSinceEpoch,
    );
    await _callActionOutbox.put(pending);
    return _sendPendingCallAction(pending);
  }

  Future<void> endIncomingCall({
    required String callId,
    required String conversationId,
    String reason = 'user_ended',
  }) async {
    final normalizedCallId = callId.trim();
    if (_disposed || normalizedCallId.isEmpty) return;
    try {
      await _platform.endCall(normalizedCallId, reason: reason);
    } catch (_) {}
    final pending = PendingNotificationCallAction(
      operation: 'end',
      callId: normalizedCallId,
      conversationId: conversationId.trim(),
      reason: reason.trim().isEmpty ? 'user_ended' : reason.trim(),
      createdAtMillis: DateTime.now().millisecondsSinceEpoch,
    );
    await _callActionOutbox.put(pending);
    await _sendPendingCallAction(pending);
  }

  Future<void> revokeCurrentRegistrationForDeploymentTransition() async {
    if (_disposed) return;
    _suppressRegistration = true;
    _retryTimer?.cancel();
    _retryTimer = null;
    _refreshPending = false;

    // The device owns OS presentation. Clear transient execution/call surfaces
    // locally before changing Core ownership so a dead old Cloud Core cannot
    // leave a stale Live Activity, task progress card, or incoming-call UI.
    try {
      await _platform.clearExecutionNotifications();
    } catch (_) {}
    try {
      await _platform.clearCallNotifications();
    } catch (_) {}

    try {
      final inFlight = _refreshIdleCompleter;
      if (inFlight != null && !inFlight.isCompleted) {
        try {
          await inFlight.future.timeout(const Duration(seconds: 5));
        } catch (_) {}
      }
      final deviceId = _registeredDeviceId?.trim();
      if (deviceId == null || deviceId.isEmpty) return;
      await _api.delete(
        '/api/notifications/devices/${Uri.encodeComponent(deviceId)}',
      );
      _registeredDeviceId = null;
    } catch (_) {
      // Deployment switching must remain possible when the old Core is
      // unreachable. Keep registration suppressed until the caller has either
      // reconciled the new Core or rolled back the previous deployment.
      _registeredDeviceId = null;
    }
  }

  void resumeRegistrationAfterDeploymentTransition() {
    if (_disposed) return;
    _suppressRegistration = false;
    unawaited(refreshRegistration());
    unawaited(_drainPendingReplies());
    unawaited(_drainPendingCallActions());
  }

  Future<void> _handleInteraction(
    NotificationPlatformInteraction interaction,
  ) async {
    if (_disposed) return;
    final fingerprint = <String>[
      interaction.source,
      interaction.deepLink ?? '',
      interaction.conversationId ?? '',
      interaction.reply ?? '',
      interaction.callId ?? '',
      interaction.reason ?? '',
    ].join('\u001f');
    final now = DateTime.now();
    if (_lastInteractionFingerprint == fingerprint &&
        _lastInteractionAt != null &&
        now.difference(_lastInteractionAt!) < const Duration(seconds: 2)) {
      return;
    }
    _lastInteractionFingerprint = fingerprint;
    _lastInteractionAt = now;
    final conversationId =
        interaction.conversationId ??
        _conversationFromDeepLink(interaction.deepLink);
    if (interaction.source == 'callEnded') {
      final callId = interaction.callId?.trim() ?? '';
      if (callId.isNotEmpty && !_callLifecycleEvents.isClosed) {
        _callLifecycleEvents.add(
          NotificationCallLifecycleEvent(
            callId: callId,
            conversationId: conversationId ?? '',
            reason: interaction.reason?.trim() ?? '',
          ),
        );
      }
      return;
    }
    final reply = interaction.reply?.trim();
    if (reply != null && reply.isNotEmpty && conversationId != null) {
      final pending = PendingNotificationReply(
        id: 'notification-${DateTime.now().microsecondsSinceEpoch}',
        conversationId: conversationId,
        message: reply,
        createdAtMillis: DateTime.now().millisecondsSinceEpoch,
      );
      await _replyOutbox.put(pending);
      await _sendPendingReply(pending);
    }
    final deepLink = interaction.deepLink;
    if (deepLink != null) {
      final uri = Uri.tryParse(deepLink);
      if (uri != null &&
          uri.scheme == 'amitia' &&
          uri.host == 'call' &&
          uri.queryParameters['action'] == 'decline') {
        final callId = uri.queryParameters['call']?.trim();
        if (callId != null && callId.isNotEmpty) {
          final pending = PendingNotificationCallAction(
            operation: 'end',
            callId: callId,
            conversationId: conversationId ?? '',
            reason: 'user_declined',
            createdAtMillis: DateTime.now().millisecondsSinceEpoch,
          );
          await _callActionOutbox.put(pending);
          await _sendPendingCallAction(pending);
        }
        return;
      }
      _routeDeepLink(deepLink);
    } else if (conversationId != null) {
      _router.go(
        Uri(
          path: '/chat',
          queryParameters: <String, String>{'conversationId': conversationId},
        ).toString(),
      );
    }
  }

  Future<bool> _sendPendingCallAction(
    PendingNotificationCallAction pending,
  ) async {
    if (_disposed || _suppressRegistration) return false;
    final operation = pending.operation == 'answer' ? 'answer' : 'end';
    try {
      await _api.post<Map<String, dynamic>>(
        '/api/notifications/calls/${Uri.encodeComponent(pending.callId)}/$operation',
        data: <String, dynamic>{
          'conversationId': pending.conversationId,
          if (operation == 'end') 'reason': pending.reason,
        },
      );
      await _callActionOutbox.remove(pending.callId);
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<void> _drainPendingCallActions() async {
    if (_disposed || _suppressRegistration || _drainingCallActions) return;
    _drainingCallActions = true;
    try {
      final pending = await _callActionOutbox.load();
      for (final action in pending) {
        if (_disposed || _suppressRegistration) break;
        if (!await _sendPendingCallAction(action)) break;
      }
    } finally {
      _drainingCallActions = false;
    }
  }

  Future<bool> _sendPendingReply(PendingNotificationReply pending) async {
    if (_disposed || _suppressRegistration) return false;
    try {
      await _chat.submitMessage(
        message: pending.message,
        conversationId: pending.conversationId,
        clientMessageId: pending.id,
      );
      await _replyOutbox.remove(pending.id);
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<void> _drainPendingReplies() async {
    if (_disposed || _suppressRegistration || _drainingReplies) return;
    _drainingReplies = true;
    try {
      final pending = await _replyOutbox.load();
      for (final reply in pending) {
        if (_disposed || _suppressRegistration) break;
        if (!await _sendPendingReply(reply)) break;
      }
    } finally {
      _drainingReplies = false;
    }
  }

  String? _conversationFromDeepLink(String? raw) {
    if (raw == null || raw.isEmpty) return null;
    final uri = Uri.tryParse(raw);
    if (uri == null || uri.scheme != 'amitia' || uri.pathSegments.isEmpty) {
      return null;
    }
    if (uri.host != 'chat' && uri.host != 'call') return null;
    return uri.pathSegments.first;
  }

  void _routeDeepLink(String raw) {
    final uri = Uri.tryParse(raw);
    if (uri == null || uri.scheme != 'amitia') return;
    switch (uri.host) {
      case 'chat':
        if (uri.pathSegments.isEmpty) return;
        final conversationId = uri.pathSegments.first;
        _router.go(
          Uri(
            path: '/chat',
            queryParameters: <String, String>{
              'conversationId': conversationId,
              'messageId': ?uri.queryParameters['message'],
              'runId': ?uri.queryParameters['run'],
            },
          ).toString(),
        );
      case 'settings':
        if (uri.pathSegments.join('/') == 'notifications') {
          _router.go('/settings/notifications');
        } else {
          _router.go('/settings');
        }
      case 'reminder':
        _router.go('/reminders');
      case 'call':
        if (uri.pathSegments.isNotEmpty) {
          _router.go(
            Uri(
              path: '/call/${uri.pathSegments.first}',
              queryParameters: <String, String>{
                'type': ?uri.queryParameters['type'],
                'caller': ?uri.queryParameters['caller'],
                'call': ?uri.queryParameters['call'],
                'action': ?uri.queryParameters['action'],
              },
            ).toString(),
          );
        }
    }
  }

  void _scheduleRetry() {
    if (_disposed || _suppressRegistration || _retryTimer != null) return;
    _retryAttempt = (_retryAttempt + 1).clamp(1, 6);
    final seconds = 1 << (_retryAttempt - 1);
    _retryTimer = Timer(Duration(seconds: seconds), () {
      _retryTimer = null;
      unawaited(refreshRegistration());
    });
  }

  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    _retryTimer?.cancel();
    _retryTimer = null;
    _suppressRegistration = true;
    final idle = _refreshIdleCompleter;
    _refreshIdleCompleter = null;
    if (idle != null && !idle.isCompleted) {
      idle.complete();
    }
    WidgetsBinding.instance.removeObserver(this);
    _router.routerDelegate.removeListener(_handleRouteChanged);
    await _tokenSubscription?.cancel();
    await _interactionSubscription?.cancel();
    await _liveActivityTokenSubscription?.cancel();
    await _platform.dispose();
    await _callLifecycleEvents.close();
  }
}
