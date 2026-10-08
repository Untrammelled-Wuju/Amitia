import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/core_configuration_guard.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api extends Fake implements BackendServiceApi {
  @override
  int generation = 1;
}

void main() {
  late MobileDeploymentConfig deployment;
  late _Api api;
  late Map<String, dynamic> policy;
  late CoreConfigurationGuard guard;

  setUp(() {
    deployment = const MobileDeploymentConfig(
      mode: MobileDeploymentMode.cloud,
      remoteCoreUri: 'https://core-b.example',
    );
    api = _Api();
    policy = {
      'coreId': 'core-b',
      'coordinationAvailable': true,
      'canAdminister': true,
      'policy': {
        'coordinated': true,
        'providerEpoch': 1,
        'modeRevision': 1,
        'permissionRevision': 1,
      },
    };
    guard = CoreConfigurationGuard(
      deployment: () => deployment,
      api: () => api,
      coordination: () async => policy,
    );
  });

  test('普通绑定和统筹关闭均不能配置 Core', () async {
    policy['canAdminister'] = false;
    expect((await guard.capture()).canConfigure, false);
    policy['canAdminister'] = true;
    (policy['policy'] as Map)['coordinated'] = false;
    expect((await guard.capture()).canConfigure, false);
  });

  test('未知策略不能降级到本机配置', () async {
    policy.remove('policy');
    await expectLater(guard.capture(), throwsStateError);
  });

  test('原 Core 管理员表单在撤权后拒绝操作', () async {
    final intent = await guard.capture();
    policy['canAdminister'] = false;
    await expectLater(guard.validate(intent), throwsStateError);
  });

  test('撤权后重新授予管理员不能恢复旧编辑意图', () async {
    final intent = await guard.capture();
    (policy['policy'] as Map)['permissionRevision'] = 3;
    await expectLater(guard.validate(intent), throwsStateError);
  });

  test('云端权限版本缺失或无效时拒绝进入配置', () async {
    (policy['policy'] as Map)['providerEpoch'] = 0;
    await expectLater(guard.capture(), throwsStateError);
  });

  test('Core 切换即使新 Core 也授予管理员仍拒绝旧表单', () async {
    final intent = await guard.capture();
    policy['coreId'] = 'core-c';
    await expectLater(guard.validate(intent), throwsStateError);
  });

  test('本机旧表单切到云端不重新绑定到云端配置', () async {
    deployment = MobileDeploymentConfig.local;
    final intent = await guard.capture();
    deployment = const MobileDeploymentConfig(
      mode: MobileDeploymentMode.cloud,
      remoteCoreUri: 'https://core-c.example',
    );
    await expectLater(guard.validate(intent), throwsStateError);
  });

  test('策略读取中更换连接时丢弃加载结果', () async {
    final response = Completer<Map<String, dynamic>>();
    guard = CoreConfigurationGuard(
      deployment: () => deployment,
      api: () => api,
      coordination: () => response.future,
    );
    final capturing = guard.capture();
    api = _Api();
    response.complete(policy);
    await expectLater(capturing, throwsStateError);
  });
}
