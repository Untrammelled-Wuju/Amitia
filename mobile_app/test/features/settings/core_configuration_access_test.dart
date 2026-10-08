import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_backend_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/services/device_mesh_service.dart';
import 'package:amitia_app/core/services/model_config_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/settings/presentation/pages/model_config_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _Api extends Fake implements BackendServiceApi {
  @override
  int generation = 1;
  bool admin = false;
  int configReads = 0;
  int writes = 0;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    dynamic data;
    if (path.endsWith('/coordination/me')) {
      data = {
        'coreId': 'core-b',
        'coordinationAvailable': true,
        'canAdminister': admin,
        'policy': {
          'coordinated': true,
          'providerEpoch': 1,
          'modeRevision': 1,
          'permissionRevision': 1,
        },
      };
    } else if (path.endsWith('/providers')) {
      data = <dynamic>[];
    } else {
      configReads++;
      data = <dynamic>[
        {
          'id': 1,
          'name': '本机旧配置',
          'apiType': 'openai-compatible',
          'modelName': 'test-model',
          'baseUrl': 'https://model.example',
          'isActive': 1,
        },
      ];
    }
    return fromJson != null ? fromJson(data) : data as T;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    writes++;
    return null;
  }
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  Future<ProviderContainer> render(
    WidgetTester tester,
    _Api api, {
    bool cloud = false,
  }) async {
    final container = ProviderContainer(overrides: [
      rawBackendServiceApiProvider.overrideWithValue(api),
      deviceMeshServiceProvider.overrideWithValue(DeviceMeshService(api)),
      modelConfigServiceProvider.overrideWithValue(ModelConfigService(api)),
    ]);
    addTearDown(container.dispose);
    if (cloud) {
      await container.read(mobileDeploymentConfigProvider.notifier).update(
        const MobileDeploymentConfig(
          mode: MobileDeploymentMode.cloud,
          remoteCoreUri: 'https://core-b.example',
        ),
      );
    }
    final router = GoRouter(routes: [
      GoRoute(
        path: '/',
        builder: (_, __) => const ModelConfigPage(modelType: 'text'),
      ),
    ]);
    addTearDown(router.dispose);
    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: MaterialApp.router(routerConfig: router),
    ));
    await tester.pumpAndSettle();
    return container;
  }

  testWidgets('普通绑定设备不读取模型配置且新建按钮禁用', (tester) async {
    final api = _Api();
    await render(tester, api, cloud: true);
    expect(api.configReads, 0);
    expect(find.textContaining('只有开启统筹模式的云端管理员设备'), findsOneWidget);
    expect(
      tester.widget<IconButton>(find.byWidgetPredicate(
        (widget) => widget is IconButton && widget.tooltip == '新建',
      )).onPressed,
      isNull,
    );
    expect(api.writes, 0);
  });

  testWidgets('旧本机编辑表单切云端后无法提交', (tester) async {
    final api = _Api()..admin = true;
    final container = await render(tester, api);
    await tester.tap(find.text('编辑'));
    await tester.pumpAndSettle();
    expect(find.text('保存'), findsOneWidget);
    await container.read(mobileDeploymentConfigProvider.notifier).update(
      const MobileDeploymentConfig(
        mode: MobileDeploymentMode.cloud,
        remoteCoreUri: 'https://core-b.example',
      ),
    );
    final save = find.text('保存');
    await tester.ensureVisible(save);
    await tester.pumpAndSettle();
    await tester.tap(save);
    await tester.pumpAndSettle();
    expect(api.writes, 0);
    expect(find.textContaining('归属或权限已变化'), findsWidgets);
    expect(find.text('已更新配置'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
