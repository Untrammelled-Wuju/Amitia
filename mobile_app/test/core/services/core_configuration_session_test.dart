import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/core_configuration_intent.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/services/core_configuration_guard.dart';
import 'package:amitia_app/core/services/core_configuration_session.dart';
import 'package:flutter_test/flutter_test.dart';

class SessionApi extends Fake implements BackendServiceApi {
  @override
  int generation = 1;
}

void main() {
  late SessionApi api;
  late Map<String, dynamic> policy;
  late CoreConfigurationSession session;
  setUp(() {
    api = SessionApi();
    policy = {
      'coreId': 'core',
      'coordinationAvailable': true,
      'canAdminister': true,
      'policy': {
        'coordinated': true,
        'providerEpoch': 1,
        'modeRevision': 1,
        'permissionRevision': 1,
      },
    };
    session = CoreConfigurationSession(
      CoreConfigurationGuard(
        deployment: () => const MobileDeploymentConfig(
          mode: MobileDeploymentMode.cloud,
          remoteCoreUri: 'https://core.example',
        ),
        api: () => api,
        coordination: () async => policy,
      ),
    );
  });
  tearDown(() => session.close());
  test('管理员读取与写入均携带相同Core原权限配置意图', () async {
    final value = await session.load(() async {
      expect(CoreConfigurationIntent.current?.coreId, 'core');
      return 'Core data';
    });
    expect(value, 'Core data');
    final original = session.intent;
    await session.write(original, () async {
      expect(identical(CoreConfigurationIntent.current, original), true);
      expect(CoreConfigurationIntent.current?.policyRevision, '1:1:1');
    });
  });
  test('普通绑定或OFF管理员都不触发配置读取', () async {
    policy['canAdminister'] = false;
    var reads = 0;
    await expectLater(session.load(() async => reads++), throwsStateError);
    policy['canAdminister'] = true;
    (policy['policy'] as Map)['coordinated'] = false;
    await expectLater(session.load(() async => reads++), throwsStateError);
    expect(reads, 0);
  });
  test('重新加载同Core不能替代已经打开确认框的原配置意图', () async {
    await session.load(() async => 1);
    final original = session.intent;
    await session.load(() async => 2);
    var writes = 0;
    await expectLater(
      session.write(original, () async => writes++),
      throwsStateError,
    );
    expect(writes, 0);
  });
  test('权限ABA禁止提交并清空原配置意图', () async {
    await session.load(() async => 1);
    final original = session.intent;
    (policy['policy'] as Map)['permissionRevision'] = 3;
    var writes = 0;
    await expectLater(
      session.write(original, () async => writes++),
      throwsStateError,
    );
    expect(writes, 0);
    expect(session.intent, isNull);
  });
  test('请求进行中Core变化后丢弃返回值', () async {
    await session.load(() async => 1);
    final original = session.intent;
    final pending = Completer<int>();
    final write = session.write(original, () => pending.future);
    await Future<void>.delayed(Duration.zero);
    policy['coreId'] = 'other';
    pending.complete(2);
    await expectLater(write, throwsStateError);
  });
  test('关闭页面后迟到加载不能恢复表单', () async {
    final pending = Completer<int>();
    final load = session.load(() => pending.future);
    await Future<void>.delayed(Duration.zero);
    session.close();
    pending.complete(1);
    await expectLater(load, throwsStateError);
    expect(session.intent, isNull);
  });
}
