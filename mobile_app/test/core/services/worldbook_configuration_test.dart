import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/core_configuration_intent.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/services/core_configuration_guard.dart';
import 'package:amitia_app/core/services/core_configuration_session.dart';
import 'package:amitia_app/core/services/worldbook_service.dart';
import 'package:flutter_test/flutter_test.dart';

class WorldbookApi extends Fake implements BackendServiceApi {
  final requests = <String>[];
  final intents = <CoreConfigurationIntent?>[];
  @override
  int get generation => 1;
  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    intents.add(CoreConfigurationIntent.current);
    return <String, dynamic>{
          'items': [
            {'id': 'same-id', 'injectContent': '原Core内容'},
          ],
        }
        as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    intents.add(CoreConfigurationIntent.current);
    return null;
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    requests.add(path);
    intents.add(CoreConfigurationIntent.current);
  }
}

void main() {
  late WorldbookApi api;
  late WorldBookService service;
  late Map<String, dynamic> policy;
  late CoreConfigurationSession session;
  setUp(() {
    api = WorldbookApi();
    service = WorldBookService(api);
    policy = {
      'coreId': 'b',
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
          remoteCoreUri: 'https://b.example',
        ),
        api: () => api,
        coordination: () async => policy,
      ),
    );
  });
  tearDown(() => session.close());
  test('普通与未知设备状态不触发世界书服务读取', () async {
    policy['canAdminister'] = false;
    await expectLater(session.load(service.list), throwsStateError);
    policy.remove('canAdminister');
    await expectLater(session.load(service.list), throwsStateError);
    expect(api.requests, isEmpty);
  });
  test('管理员列表与写入使用原Core同一数据范围，换Core旧确认不得删除同ID', () async {
    final entries = await session.load(service.list);
    final original = session.intent;
    expect(entries.single.id, 'same-id');
    await session.write(
      original,
      () => service.create({'injectContent': '内容'}),
    );
    expect(
      api.intents.every(
        (intent) => intent?.coreId == 'b' && intent?.policyRevision == '1:1:1',
      ),
      isTrue,
    );
    policy['coreId'] = 'c';
    await expectLater(
      session.write(original, () => service.delete('same-id')),
      throwsStateError,
    );
    expect(api.requests, ['/api/world-book', '/api/world-book']);
  });
  test('原权限版本被撤销后导入及匹配调用不能继续读取Core内容', () async {
    await session.load(service.list);
    final original = session.intent;
    (policy['policy'] as Map)['permissionRevision'] = 3;
    await expectLater(
      session.write(original, () => service.create({})),
      throwsStateError,
    );
    await expectLater(
      session.write(original, () => service.testMatch('输入')),
      throwsStateError,
    );
    expect(api.requests, ['/api/world-book']);
  });
}
