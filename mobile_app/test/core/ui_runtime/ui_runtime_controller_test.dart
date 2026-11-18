import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_invalidation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
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

  test('身份或缓存归属读取失败后保留原因并允许重试', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final service = _CountingExtensionService();
    final identityFailure = StateError('设备身份暂不可读');
    final identityStack = StackTrace.fromString('identity resolver stack');
    final cacheFailure = StateError('当前空间暂不可读');
    var identityAttempts = 0;
    var namespaceAttempts = 0;
    final controller = UIRuntimeController(
      service,
      onLastKnownGood: (_) {},
      cacheNamespace: () async {
        if (++namespaceAttempts == 1) throw cacheFailure;
        return 'scope-retry';
      },
      meshDeviceId: () async {
        if (++identityAttempts == 1) {
          Error.throwWithStackTrace(identityFailure, identityStack);
        }
        return 'test-device';
      },
    );
    addTearDown(controller.dispose);

    await controller.ensureLoaded();
    expect(controller.state.error, same(identityFailure));
    expect(controller.state.stackTrace.toString(), identityStack.toString());
    expect(service.calls, 0);

    await controller.ensureLoaded();
    expect(controller.state.error, same(cacheFailure));
    expect(service.calls, 0);

    await controller.ensureLoaded();
    expect(controller.state.hasError, isFalse);
    expect(controller.state.valueOrNull, isNotNull);
    expect(service.calls, 1);
    expect(identityAttempts, 3);
    expect(namespaceAttempts, 2);
  });

  test('身份查询结束前销毁控制器不会发请求或更新已释放状态', () async {
    final service = _CountingExtensionService();
    final identity = Completer<String?>();
    final controller = UIRuntimeController(
      service,
      onLastKnownGood: (_) => fail('已销毁的控制器还在更新缓存状态'),
      cacheNamespace: () async => 'disposed',
      meshDeviceId: () => identity.future,
    );

    final loading = controller.ensureLoaded();
    controller.dispose();
    identity.complete('test-device');
    await loading;
    expect(service.calls, 0);
  });
}
