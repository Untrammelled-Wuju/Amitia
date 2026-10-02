import 'package:amitia_app/core/native_bridge/native_bridge_platform_dispatcher.dart';
import 'package:amitia_app/core/services/screen_awake_service.dart';
import 'package:amitia_app/features/settings/presentation/widgets/screen_awake_settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class ScreenDispatcher implements NativeBridgePlatformDispatcher {
  bool enabled = false;
  bool fail = false;
  bool ignoreSave = false;
  final requests = <Map<String, dynamic>>[];
  @override
  Future<Map<String, dynamic>> execute(Map<String, dynamic> request) async {
    requests.add(request);
    if (fail) return {'status': 'error'};
    if (request['operation'] == 'display.keep_awake.set' && !ignoreSave) {
      enabled = request['payload']['enabled'] as bool;
    }
    return {
      'status': 'success',
      'result': {'enabled': enabled},
    };
  }

  @override
  Future<Map<String, dynamic>> health() async => {};
  @override
  Stream<Map<String, dynamic>> get eventStream => const Stream.empty();
  @override
  void setBackendActionHandler(NativeBackendActionHandler? handler) {}
}

Widget screenSettings(ScreenAwakeService service) => ProviderScope(
  overrides: [screenAwakeServiceProvider.overrideWithValue(service)],
  child: const MaterialApp(home: Scaffold(body: ScreenAwakeSettings())),
);

void main() {
  test('loads native preference and restores it in a new service', () async {
    final dispatcher = ScreenDispatcher();
    final service = ScreenAwakeService(dispatcher, platform: 'android');
    expect(await service.load(), false);
    expect(await service.setEnabled(true), true);
    expect(
      await ScreenAwakeService(dispatcher, platform: 'android').load(),
      true,
    );
    expect(await service.setEnabled(false), false);
    expect(dispatcher.requests[1]['payload'], {'enabled': true});
    expect(dispatcher.requests[1]['platform'], 'android');
  });
  test('reports native failure instead of showing success', () async {
    final dispatcher = ScreenDispatcher()..fail = true;
    final service = ScreenAwakeService(dispatcher, platform: 'android');
    await expectLater(service.load(), throwsStateError);
    await expectLater(service.setEnabled(true), throwsStateError);
  });
  test('detects a preference that was not saved', () async {
    final dispatcher = ScreenDispatcher()..ignoreSave = true;
    await expectLater(
      ScreenAwakeService(dispatcher, platform: 'android').setEnabled(true),
      throwsStateError,
    );
  });
  test('uses the same preference operations on iOS', () async {
    final dispatcher = ScreenDispatcher();
    await ScreenAwakeService(dispatcher, platform: 'ios').setEnabled(true);
    expect(dispatcher.requests.single['platform'], 'ios');
    expect(dispatcher.requests.single['operation'], 'display.keep_awake.set');
  });
  testWidgets('switch reads saved state and updates native preference', (
    tester,
  ) async {
    final dispatcher = ScreenDispatcher()..enabled = true;
    await tester.pumpWidget(
      screenSettings(ScreenAwakeService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch)).value, true);
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    expect(dispatcher.enabled, false);
    expect(tester.widget<Switch>(find.byType(Switch)).value, false);
  });
  testWidgets('failed save keeps the previous switch state', (tester) async {
    final dispatcher = ScreenDispatcher();
    await tester.pumpWidget(
      screenSettings(ScreenAwakeService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    dispatcher.fail = true;
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch)).value, false);
    expect(find.text('屏幕常亮设置保存失败，请重试'), findsOneWidget);
  });
  testWidgets('failed load disables switch and can be retried', (tester) async {
    final dispatcher = ScreenDispatcher()..fail = true;
    await tester.pumpWidget(
      screenSettings(ScreenAwakeService(dispatcher, platform: 'android')),
    );
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch)).onChanged, isNull);
    dispatcher.fail = false;
    await tester.tap(find.text('重试读取屏幕设置'));
    await tester.pumpAndSettle();
    expect(tester.widget<Switch>(find.byType(Switch)).onChanged, isNotNull);
  });
}
