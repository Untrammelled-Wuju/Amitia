import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/core_configuration_intent.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_backend_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/services/device_mesh_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/services/search_api_service.dart';
import 'package:amitia_app/features/settings/presentation/pages/search_api_settings_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:go_router/go_router.dart';

class SearchAuthorityApi extends Fake implements BackendServiceApi {
  @override
  int get generation => 1;
  bool admin = false;
  int permission = 1;
  int reads = 0;
  int deletes = 0;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (path.endsWith('/coordination/me')) {
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
    }
    reads++;
    expect(CoreConfigurationIntent.current?.coreId, 'core');
    return {
          'items': [
            {'engineId': 'search', 'name': 'Core 搜索', 'configured': true},
          ],
        }
        as T;
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    deletes++;
  }
}

Future<ProviderContainer> containerFor(SearchAuthorityApi api) async {
  final container = ProviderContainer(
    overrides: [
      rawBackendServiceApiProvider.overrideWithValue(api),
      searchApiServiceProvider.overrideWithValue(SearchApiService(api)),
      deviceMeshServiceProvider.overrideWithValue(DeviceMeshService(api)),
    ],
  );
  await container
      .read(mobileDeploymentConfigProvider.notifier)
      .update(
        const MobileDeploymentConfig(
          mode: MobileDeploymentMode.cloud,
          remoteCoreUri: 'https://core.example',
        ),
      );
  return container;
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('普通绑定设备不读取Core搜索凭据', (tester) async {
    final api = SearchAuthorityApi();
    final container = await containerFor(api);
    addTearDown(container.dispose);
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, __) => const SearchApiSettingsPage()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    expect(api.reads, 0);
    expect(find.byType(TextFormField), findsNothing);
    expect(find.textContaining('只有开启统筹模式'), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('管理员清除确认框不能复用撤权后重新授予的原权限', (tester) async {
    final api = SearchAuthorityApi()..admin = true;
    final container = await containerFor(api);
    addTearDown(container.dispose);
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, __) => const SearchApiSettingsPage()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    expect(api.reads, 1);
    await tester.tap(find.text('清除'));
    await tester.pumpAndSettle();
    api.permission = 3;
    await tester.tap(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(TextButton, '清除'),
      ),
    );
    await tester.pumpAndSettle();
    expect(api.deletes, 0);
    expect(find.byType(TextFormField), findsNothing);
    await tester.pumpWidget(const SizedBox());
  });
}
