import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:amitia_app/core/native_bridge/native_bridge_platform_dispatcher.dart';
import 'package:amitia_app/core/services/reply_notification_service.dart';
import 'package:amitia_app/core/settings/reply_notification_preferences.dart';

class FakeDispatcher implements NativeBridgePlatformDispatcher {
  final requests = <Map<String, dynamic>>[];
  bool denied = false;
  bool fail = false;
  @override
  Future<Map<String, dynamic>> execute(Map<String, dynamic> request) async {
    requests.add(request);
    if (fail) throw StateError('unavailable');
    return {'status': denied ? 'error' : 'success'};
  }

  @override
  Future<Map<String, dynamic>> health() async => {};
  @override
  Stream<Map<String, dynamic>> get eventStream => const Stream.empty();
  @override
  void setBackendActionHandler(NativeBackendActionHandler? handler) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUp(() => SharedPreferences.setMockInitialValues({}));
  test('defaults off and persists across instances', () async {
    final preferences = ReplyNotificationPreferences();
    await preferences.init();
    expect(preferences.enabled, false);
    expect(preferences.soundEnabled, false);
    expect(preferences.vibrationEnabled, false);
    await preferences.setEnabled(true);
    await preferences.setSoundEnabled(true);
    await preferences.setVibrationEnabled(true);
    final restored = ReplyNotificationPreferences();
    await restored.init();
    expect(restored.enabled, true);
    expect(restored.soundEnabled, true);
    expect(restored.vibrationEnabled, true);
    await restored.setEnabled(false);
    await restored.setSoundEnabled(false);
    expect(restored.vibrationEnabled, true);
    await restored.setVibrationEnabled(false);
    expect(restored.enabled, false);
    expect(restored.soundEnabled, false);
  });
  test('foreground and unknown lifecycle never notify', () async {
    final preferences = ReplyNotificationPreferences();
    await preferences.setEnabled(true);
    await preferences.setSoundEnabled(true);
    await preferences.setVibrationEnabled(true);
    final dispatcher = FakeDispatcher();
    for (final state in [
      AppLifecycleState.resumed,
      AppLifecycleState.inactive,
      null,
    ]) {
      final service = ReplyNotificationService(
        preferences,
        dispatcher,
        platform: 'android',
        lifecycle: () => state,
      );
      await service.completed('chat', 'turn');
    }
    expect(dispatcher.requests, isEmpty);
  });
  test(
    'background completion sends once and checks native foreground again',
    () async {
      final preferences = ReplyNotificationPreferences();
      await preferences.setEnabled(true);
      final dispatcher = FakeDispatcher();
      final service = ReplyNotificationService(
        preferences,
        dispatcher,
        platform: 'android',
        lifecycle: () => AppLifecycleState.paused,
      );
      await service.completed('chat', 'turn');
      await service.completed('chat', 'turn');
      expect(dispatcher.requests, hasLength(1));
      expect(dispatcher.requests.single['operation'], 'notification.post');
      expect(dispatcher.requests.single['payload']['backgroundOnly'], true);
      expect(dispatcher.requests.single['payload']['silent'], true);
      expect(dispatcher.requests.single['payload']['replySound'], false);
      expect(dispatcher.requests.single['payload']['replyVibration'], false);
    },
  );
  test('disabled preference prevents background notifications', () async {
    final dispatcher = FakeDispatcher();
    final service = ReplyNotificationService(
      ReplyNotificationPreferences(),
      dispatcher,
      platform: 'android',
      lifecycle: () => AppLifecycleState.paused,
    );
    await service.completed('chat', 'turn');
    expect(dispatcher.requests, isEmpty);
  });
  test(
    'notification sound and vibration combinations send one background request',
    () async {
      for (var mask = 0; mask < 8; mask++) {
        final preferences = ReplyNotificationPreferences();
        await preferences.setEnabled(mask & 1 != 0);
        await preferences.setSoundEnabled(mask & 2 != 0);
        await preferences.setVibrationEnabled(mask & 4 != 0);
        final dispatcher = FakeDispatcher();
        final service = ReplyNotificationService(
          preferences,
          dispatcher,
          platform: 'android',
          lifecycle: () => AppLifecycleState.paused,
        );
        await service.completed('chat', 'turn');
        await service.completed('chat', 'turn');
        expect(dispatcher.requests, hasLength(mask == 0 ? 0 : 1));
        if (mask != 0) {
          final payload = dispatcher.requests.single['payload'];
          expect(payload['replyVibration'], mask & 4 != 0);
          expect(payload['replySound'], mask & 2 != 0);
          expect(payload['soundOnly'], mask & 1 == 0);
          expect(payload['backgroundOnly'], true);
        }
      }
    },
  );
  test(
    'vibration alone does not run in foreground or after returning to foreground',
    () async {
      final preferences = ReplyNotificationPreferences();
      await preferences.setVibrationEnabled(true);
      final dispatcher = FakeDispatcher();
      var lifecycle = AppLifecycleState.paused;
      final service = ReplyNotificationService(
        preferences,
        dispatcher,
        platform: 'android',
        lifecycle: () => lifecycle,
      );
      final pending = service.completed('chat', 'turn');
      lifecycle = AppLifecycleState.resumed;
      await pending;
      await service.completed('chat', 'front');
      lifecycle = AppLifecycleState.paused;
      await service.completed('chat', 'turn');
      expect(dispatcher.requests, isEmpty);
    },
  );
  test(
    'non Android platforms do not receive unsupported vibration requests',
    () async {
      final preferences = ReplyNotificationPreferences();
      await preferences.setVibrationEnabled(true);
      final dispatcher = FakeDispatcher();
      final service = ReplyNotificationService(
        preferences,
        dispatcher,
        platform: 'ios',
        lifecycle: () => AppLifecycleState.paused,
      );
      await service.completed('chat', 'turn');
      expect(dispatcher.requests, isEmpty);
    },
  );
  test(
    'sound works independently and combines with notification without duplication',
    () async {
      final preferences = ReplyNotificationPreferences();
      await preferences.setSoundEnabled(true);
      final dispatcher = FakeDispatcher();
      final service = ReplyNotificationService(
        preferences,
        dispatcher,
        platform: 'android',
        lifecycle: () => AppLifecycleState.paused,
      );
      await service.completed('chat', 'sound');
      await service.completed('chat', 'sound');
      expect(dispatcher.requests, hasLength(1));
      expect(dispatcher.requests.single['payload']['soundOnly'], true);
      expect(dispatcher.requests.single['payload']['replySound'], true);
      await preferences.setEnabled(true);
      await service.completed('chat', 'both');
      expect(dispatcher.requests, hasLength(2));
      expect(dispatcher.requests.last['payload']['soundOnly'], false);
      expect(dispatcher.requests.last['payload']['replySound'], true);
      await preferences.setSoundEnabled(false);
      await service.completed('chat', 'silent');
      expect(dispatcher.requests.last['payload']['silent'], true);
    },
  );
  test(
    'permission rejection is reported and delivery failure does not break chat',
    () async {
      final preferences = ReplyNotificationPreferences();
      await preferences.setEnabled(true);
      final dispatcher = FakeDispatcher()..denied = true;
      final service = ReplyNotificationService(
        preferences,
        dispatcher,
        platform: 'android',
        lifecycle: () => AppLifecycleState.paused,
      );
      await expectLater(service.requestPermission(), throwsStateError);
      dispatcher.fail = true;
      await expectLater(service.completed('chat', 'turn'), completes);
    },
  );
}
