import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/channel_service.dart';
import 'package:amitia_app/core/services/model_config_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/settings/presentation/pages/model_config_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

class VisionSettingsApi implements BackendServiceApi {
  @override
  int get generation => 1;
  bool takeover = true;
  final mutations = <String>[];

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    Object data;
    if (path == '/api/vision/status') {
      data = <String, dynamic>{'mainModelVision': takeover};
    } else if (path.endsWith('/providers')) {
      data = <dynamic>[];
    } else {
      data = <dynamic>[
        <String, dynamic>{
          'id': 1,
          'name': 'Vision configuration',
          'modelName': 'vision',
          'isActive': takeover ? 0 : 1,
          'disabled': takeover,
        },
      ];
    }
    return data as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    mutations.add(path);
    return <String, dynamic>{'success': true} as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  testWidgets('takeover blocks add and existing card without mutation', (
    tester,
  ) async {
    final api = VisionSettingsApi();
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final router = GoRouter(
      initialLocation: '/settings/model/vision',
      routes: <RouteBase>[
        GoRoute(
          path: '/settings/model/vision',
          builder: (context, state) =>
              const ModelConfigPage(modelType: 'vision'),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: <Override>[
          rawBackendServiceApiProvider.overrideWithValue(api),
          visionServiceProvider.overrideWithValue(VisionService(api)),
          modelConfigServiceProvider.overrideWithValue(ModelConfigService(api)),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('主模型已接管'), findsOneWidget);
    await tester.tap(find.byTooltip('新建'));
    await tester.pumpAndSettle();
    expect(find.text('主模型已开启视觉模式，如需单独启用视觉模型，请先关闭文本模型的支持识图功能'), findsOneWidget);
    await tester.tap(find.text('知道了'));
    await tester.pumpAndSettle();
    await tester.tap(
      find
          .ancestor(
            of: find.text('Vision configuration'),
            matching: find.byType(GestureDetector),
          )
          .first,
    );
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsOneWidget);
    expect(api.mutations, isEmpty);
    await tester.tap(find.text('知道了'));
    await tester.pumpAndSettle();
    api.takeover = false;
    await tester.tap(find.byTooltip('刷新'));
    await tester.pumpAndSettle();
    expect(find.text('主模型已接管'), findsNothing);
    await tester.tap(find.text('测试连接'));
    await tester.pumpAndSettle();
    expect(api.mutations, <String>['/api/vision/configs/1/test']);
  });
}
