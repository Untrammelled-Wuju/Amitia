import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_invalidation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _FakeBackendServiceApi extends Fake implements BackendServiceApi {
  @override
  int get generation => 1;
}

class _CountingExtensionService extends ExtensionService {
  _CountingExtensionService() : super(_FakeBackendServiceApi());

  int calls = 0;

  @override
  Future<Map<String, dynamic>> getUISnapshot(
    String platform, {
    String deviceId = '',
  }) async {
    calls++;
    return <String, dynamic>{
      'providers': <dynamic>[],
      'slots': <dynamic>[],
      'profile': <String, dynamic>{
        'profileId': 'default',
        'name': 'Default',
        'selections': <String, String>{},
      },
      'profileLayers': <dynamic>[],
      'providerContext': <String, dynamic>{
        'platform': platform,
        'localRuntime': true,
      },
      'resolved': <String, dynamic>{},
      'providerVersion': 1,
    };
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('初始化期间的失效通知不重复请求 UI 快照', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final service = _CountingExtensionService();
    final controller = UIRuntimeController(
      service,
      onLastKnownGood: (_) {},
      cacheNamespace: () async => 'startup-dedup',
      meshDeviceId: () async => null,
    );
    addTearDown(controller.dispose);

    UIRuntimeInvalidationBus.notifyChanged();
    await controller.ensureLoaded();
    await Future<void>.delayed(const Duration(milliseconds: 350));

    expect(service.calls, 1);
  });
}
