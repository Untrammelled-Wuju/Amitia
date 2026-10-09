import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/core_configuration_intent.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_backend_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/services/device_mesh_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/services/voice_service.dart';
import 'package:amitia_app/features/settings/presentation/pages/asr_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

class ProbeApi extends Fake implements BackendServiceApi {
  @override
  int get generation => 1;
  bool admin = true;
  int permission = 1;
  int configReads = 0;
  int multipartWrites = 0;
  Map<String, String>? fields;
  Map<String, List<String>>? files;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (path.endsWith('/coordination/me'))
      return {
            'coreId': 'core',
            'coordinationAvailable': true,
            'canAdminister': admin,
            'policy': {
              'coordinated': true,
              'providerEpoch': 1,
              'modeRevision': 1,
              'permissionRevision': permission,
            },
          }
          as T;
    if (path.endsWith('/configs')) {
      configReads++;
      return [
            {
              'id': 'config',
              'name': 'Core ASR',
              'isActive': true,
              'hasApiKey': true,
            },
          ]
          as T;
    }
    return <dynamic>[] as T;
  }

  @override
  Future<T?> postMultipart<T>(
    String path, {
    Map<String, String> fields = const {},
    Map<String, List<String>> files = const {},
    Map<String, dynamic>? queryParameters,
    T Function(dynamic)? fromJson,
  }) async {
    multipartWrites++;
    this.fields = fields;
    this.files = files;
    expect(CoreConfigurationIntent.current?.coreId, 'core');
    return (path.endsWith('/upload')
            ? {'url': 'owned-private-audio'}
            : {'taskId': 'task'})
        as T;
  }
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));
  for (final admin in [false, true]) {
    testWidgets(admin ? '撤权重新授予后原ASR上传按钮不发请求' : '普通绑定不能读取ASR配置或测试音频', (
      tester,
    ) async {
      final api = ProbeApi()..admin = admin;
      final container = ProviderContainer(
        overrides: [
          rawBackendServiceApiProvider.overrideWithValue(api),
          asrServiceProvider.overrideWithValue(ASRService(api)),
          deviceMeshServiceProvider.overrideWithValue(DeviceMeshService(api)),
        ],
      );
      addTearDown(container.dispose);
      await container
          .read(mobileDeploymentConfigProvider.notifier)
          .update(
            const MobileDeploymentConfig(
              mode: MobileDeploymentMode.cloud,
              remoteCoreUri: 'https://core.example',
            ),
          );
      final router = GoRouter(
        routes: [GoRoute(path: '/', builder: (_, __) => const AsrPage())],
      );
      addTearDown(router.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();
      if (admin) {
        expect(api.configReads, 1);
        api.permission = 3;
        await tester.ensureVisible(find.text('选择并上传音频'));
        await tester.tap(find.text('选择并上传音频'));
        await tester.pumpAndSettle();
        expect(find.text('重新加载'), findsOneWidget);
        expect(find.text('选择并上传音频'), findsNothing);
      } else {
        expect(api.configReads, 0);
        expect(find.text('选择并上传音频'), findsNothing);
      }
      expect(api.multipartWrites, 0);
      await tester.pumpWidget(const SizedBox());
    });
  }

  test('ASR测试上传实际文件并按后端表单契约提交，不传本机URI替代音频', () async {
    final api = ProbeApi();
    final service = ASRService(api);
    const intent = CoreConfigurationIntent(
      generation: 1,
      coreId: 'core',
      policyRevision: '1:1:1',
      canConfigure: true,
      isCurrent: current,
    );
    await intent.run(() => service.uploadAudio('fixture.wav'));
    expect(api.files, {
      'audio': ['fixture.wav'],
    });
    await intent.run(
      () => service.submitUrl('owned-private-audio', language: 'zh-CN'),
    );
    expect(api.fields, {
      'audioUrl': 'owned-private-audio',
      'language': 'zh-CN',
    });
  });
}

bool current() => true;
