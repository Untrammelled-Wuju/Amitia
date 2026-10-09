import 'package:amitia_app/core/native_bridge/native_bridge_platform_dispatcher.dart';
import 'package:amitia_app/core/services/background_keep_alive_service.dart';
import 'package:amitia_app/features/settings/presentation/widgets/background_keep_alive_settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class KeepAliveDispatcher implements NativeBridgePlatformDispatcher {
  bool enabled = false;
  bool active = false;
  bool unrestricted = false;
  bool permissionDenied = false;
  bool startFailed = false;
  bool powerSaveMode = false;
  String dataRestriction = 'disabled';
  bool legacyStatus = false;
  final requests = <Map<String, dynamic>>[];
  @override
  Future<Map<String, dynamic>> execute(Map<String, dynamic> request) async {
    requests.add(request);
    if (request['operation'] == 'notification.request_permission') {
      return {
        'status': permissionDenied ? 'error' : 'success',
        'error': {'message': '通知权限未授予'},
      };
    }
    if (request['operation'] == 'device.keep_alive.set') {
      if (startFailed) {
        return {
          'status': 'error',
          'error': {'message': '系统限制了后台保活'},
        };
      }
      enabled = request['payload']['enabled'] as bool;
      active = enabled;
    }
    return {
      'status': 'success',
      'result': {
        'enabled': enabled,
        'active': active,
        'batteryUnrestricted': unrestricted,
        if (!legacyStatus) 'powerSaveMode': powerSaveMode,
        if (!legacyStatus) 'backgroundDataRestriction': dataRestriction,
      },
    };
  }

  @override
  Future<Map<String, dynamic>> health() async => {};
  @override
  Stream<Map<String, dynamic>> get eventStream => const Stream.empty();
  @override
  void setBackendActionHandler(NativeBackendActionHandler? handler) {}
}

Widget settings(BackgroundKeepAliveService service) => ProviderScope(
  overrides: [backgroundKeepAliveServiceProvider.overrideWithValue(service)],
  child: const MaterialApp(home: Scaffold(body: BackgroundKeepAliveSettings())),
);

void main() {
  test('defaults off and reloads the native preference', () async {
    final dispatcher = KeepAliveDispatcher();
    final service = BackgroundKeepAliveService(dispatcher, platform: 'android');
    expect((await service.load()).enabled, false);
    expect((await service.setEnabled(true)).active, true);
    expect(
      (await BackgroundKeepAliveService(
        dispatcher,
        platform: 'android',
      ).load()).enabled,
      true,
    );
    expect(
      dispatcher.requests[1]['operation'],
      'notification.request_permission',
    );
    expect(dispatcher.requests[2]['operation'], 'device.keep_alive.set');
  });
  test('permission denial does not start keep alive', () async {
    final dispatcher = KeepAliveDispatcher()..permissionDenied = true;
    await expectLater(
      BackgroundKeepAliveService(
        dispatcher,
        platform: 'android',
      ).setEnabled(true),
      throwsStateError,
    );
    expect(dispatcher.requests, hasLength(1));
    expect(dispatcher.enabled, false);
  });
  test('disable does not require notification permission', () async {
    final dispatcher = KeepAliveDispatcher()
      ..enabled = true
      ..active = true
      ..permissionDenied = true;
    final status = await BackgroundKeepAliveService(
      dispatcher,
      platform: 'android',
    ).setEnabled(false);
    expect(status.enabled, false);
    expect(dispatcher.requests.single['operation'], 'device.keep_alive.set');
  });
  test(
    'startup refusal is reported without falsely enabling preference',
    () async {
      final dispatcher = KeepAliveDispatcher()..startFailed = true;
      await expectLater(
        BackgroundKeepAliveService(
          dispatcher,
          platform: 'android',
        ).setEnabled(true),
        throwsStateError,
      );
      expect(dispatcher.enabled, false);
    },
  );
  test('battery settings use the native settings operation', () async {
    final dispatcher = KeepAliveDispatcher();
    await BackgroundKeepAliveService(
      dispatcher,
      platform: 'android',
    ).openBatterySettings();
    expect(
      dispatcher.requests.single['operation'],
      'device.keep_alive.battery_settings',
    );
  });
  test('unsupported platforms never call Android keep alive', () async {
    final dispatcher = KeepAliveDispatcher();
    await expectLater(
      BackgroundKeepAliveService(dispatcher, platform: 'ios').load(),
      throwsUnsupportedError,
    );
    expect(dispatcher.requests, isEmpty);
  });
  test('legacy native status does not imply network permission', () async {
    final dispatcher = KeepAliveDispatcher()..legacyStatus = true;
    final status = await BackgroundKeepAliveService(
      dispatcher,
      platform: 'android',
    ).load();
    expect(status.powerSaveMode, isNull);
    expect(status.backgroundDataRestriction, 'unknown');
  });
  testWidgets('system shortcuts are available while keep alive is disabled', (
    tester,
  ) async {
    final dispatcher = KeepAliveDispatcher();
    await tester.pumpWidget(
      settings(BackgroundKeepAliveService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    expect(find.text('后台保活未开启'), findsOneWidget);
    expect(find.text('电池优化：未豁免'), findsOneWidget);
    await tester.tap(find.text('后台流量设置'));
    await tester.pumpAndSettle();
    expect(
      dispatcher.requests.last['operation'],
      'device.keep_alive.network_settings',
    );
    await tester.tap(find.text('应用系统设置'));
    await tester.pumpAndSettle();
    expect(
      dispatcher.requests.last['operation'],
      'device.keep_alive.app_settings',
    );
    expect(dispatcher.enabled, false);
    expect(find.text('电池优化：未豁免'), findsOneWidget);
  });
  testWidgets('returning from settings refreshes actual system states', (
    tester,
  ) async {
    final dispatcher = KeepAliveDispatcher()
      ..powerSaveMode = true
      ..dataRestriction = 'restricted';
    await tester.pumpWidget(
      settings(BackgroundKeepAliveService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    expect(find.text('省流量模式：后台流量受限'), findsOneWidget);
    expect(find.text('系统省电模式已开启，后台运行仍可能受限'), findsOneWidget);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    dispatcher.unrestricted = true;
    dispatcher.powerSaveMode = false;
    dispatcher.dataRestriction = 'whitelisted';
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pumpAndSettle();
    expect(find.text('电池优化：已豁免'), findsOneWidget);
    expect(find.text('省流量模式：已允许本应用不受限用量'), findsOneWidget);
    expect(find.text('系统省电模式未开启'), findsOneWidget);
    expect(find.text('后台保活未开启'), findsOneWidget);
    expect(find.text('打开系统省电设置'), findsOneWidget);
  });
  testWidgets('legacy status renders unavailable values explicitly', (
    tester,
  ) async {
    final dispatcher = KeepAliveDispatcher()..legacyStatus = true;
    await tester.pumpWidget(
      settings(BackgroundKeepAliveService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    expect(find.text('省流量模式：状态暂不可用'), findsOneWidget);
    expect(find.text('系统省电模式状态暂不可用'), findsOneWidget);
    expect(find.text('应用系统设置'), findsOneWidget);
  });
  testWidgets('switch enables protection and exposes battery settings', (
    tester,
  ) async {
    final dispatcher = KeepAliveDispatcher();
    await tester.pumpWidget(
      settings(BackgroundKeepAliveService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch)).value, false);
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch)).value, true);
    expect(find.text('打开系统省电设置'), findsOneWidget);
    await tester.tap(find.text('打开系统省电设置'));
    await tester.pumpAndSettle();
    expect(
      dispatcher.requests.last['operation'],
      'device.keep_alive.battery_settings',
    );
  });
  testWidgets('failed enable retains the disabled switch', (tester) async {
    final dispatcher = KeepAliveDispatcher()..startFailed = true;
    await tester.pumpWidget(
      settings(BackgroundKeepAliveService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch)).value, false);
    expect(find.text('系统限制了后台保活'), findsOneWidget);
  });
}
