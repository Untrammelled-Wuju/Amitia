import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../native_bridge/native_bridge_platform_dispatcher.dart';
import '../native_bridge/providers/native_bridge_relay_provider.dart';
import '../settings/reply_notification_preferences.dart';

bool replyNotificationInBackground(AppLifecycleState? state) =>
    state == AppLifecycleState.paused ||
    state == AppLifecycleState.hidden ||
    state == AppLifecycleState.detached;
final replyNotificationServiceProvider = Provider(
  (ref) => ReplyNotificationService(
    ref.read(replyNotificationPreferencesProvider.notifier),
    ref.read(nativeBridgePlatformDispatcherProvider),
  ),
);

class ReplyNotificationService {
  final ReplyNotificationPreferences preferences;
  final NativeBridgePlatformDispatcher dispatcher;
  final AppLifecycleState? Function() lifecycle;
  final String? platform;
  final Set<String> _completed = {};
  ReplyNotificationService(
    this.preferences,
    this.dispatcher, {
    AppLifecycleState? Function()? lifecycle,
    String? platform,
  }) : lifecycle = lifecycle ?? (() => WidgetsBinding.instance.lifecycleState),
       platform =
           platform ??
           switch (defaultTargetPlatform) {
             TargetPlatform.android => 'android',
             TargetPlatform.iOS => 'ios',
             _ => null,
           };
  Future<void> requestPermission() async {
    if (platform == null) throw UnsupportedError('当前平台不支持系统通知');
    final result = await dispatcher.execute({
      'protocolVersion': 1,
      'requestId': 'reply-permission-${DateTime.now().microsecondsSinceEpoch}',
      'platform': platform,
      'operation': 'notification.request_permission',
      'payload': <String, dynamic>{},
    });
    if (!['ok', 'success'].contains(result['status'])) {
      throw StateError('系统通知权限未授予，请在系统设置中允许通知');
    }
  }

  Future<void> completed(String conversationId, String turnId) async {
    final id = '$conversationId:$turnId';
    if (conversationId.isEmpty || turnId.isEmpty || _completed.contains(id)) {
      return;
    }
    _completed.add(id);
    if (_completed.length > 500) _completed.remove(_completed.first);
    try {
      await preferences.init();
      final vibration = platform == 'android' && preferences.vibrationEnabled;
      if ((!preferences.enabled && !preferences.soundEnabled && !vibration) ||
          platform == null ||
          !replyNotificationInBackground(lifecycle())) {
        return;
      }
      await dispatcher.execute({
        'protocolVersion': 1,
        'requestId': 'reply-$id',
        'platform': platform,
        'operation': 'notification.post',
        'payload': {
          'title': 'Amitia · 回复完成',
          'body': 'AI 已完成回复，打开应用查看。',
          'channel': 'amitia_agent',
          'silent': !preferences.soundEnabled,
          'replySound': preferences.soundEnabled,
          'replyVibration': vibration,
          'soundOnly': !preferences.enabled,
          'backgroundOnly': true,
        },
      });
    } catch (_) {}
  }
}
