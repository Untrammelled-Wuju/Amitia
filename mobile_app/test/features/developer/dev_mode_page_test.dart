import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error_code.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/developer/presentation/pages/dev_mode_page.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _DevApi implements BackendServiceApi {
  Map<String, dynamic> data = {'enabled': false, 'workspaces': [], 'total': 0};
  Object? error;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    expect(path, '/api/extensions/dev-mode/workspaces');
    if (error != null) throw error!;
    return (fromJson == null ? data : fromJson(data)) as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Future<void> _render(WidgetTester tester, _DevApi api) async {
  final router = GoRouter(
    routes: [GoRoute(path: '/', builder: (_, _) => const DevModePage())],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [backendServiceProvider.overrideWithValue(api)],
      child: MaterialApp.router(routerConfig: router),
    ),
  );
  await tester.pumpAndSettle();
}

BackendTransportError _forbidden(String message) {
  final request = RequestOptions(path: '/api/extensions/dev-mode/workspaces');
  return BackendTransportError(
    code: BackendTransportErrorCode.authenticationFailed,
    statusCode: 403,
    cause: DioException.badResponse(
      statusCode: 403,
      requestOptions: request,
      response: Response(
        requestOptions: request,
        statusCode: 403,
        data: {'error': message},
      ),
    ),
  );
}

void main() {
  testWidgets('未启用状态正常展示且不提供注册操作', (tester) async {
    await _render(tester, _DevApi());
    expect(find.text('开发模式未启用'), findsOneWidget);
    expect(find.byTooltip('注册工作区'), findsNothing);
    expect(find.text('重试'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('兼容旧 Runtime 的业务禁用 403', (tester) async {
    final api = _DevApi()..error = _forbidden('developer mode is disabled');
    await _render(tester, api);
    expect(find.text('开发模式未启用'), findsOneWidget);
    expect(find.text('重试'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('真实权限 403 保留失败状态并支持重试恢复', (tester) async {
    final api = _DevApi()..error = _forbidden('permission denied');
    await _render(tester, api);
    expect(find.text('开发模式未启用'), findsNothing);
    expect(find.text('重试'), findsOneWidget);
    expect(find.byTooltip('注册工作区'), findsNothing);
    api.error = null;
    api.data = {'enabled': true, 'workspaces': [], 'total': 0};
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('暂无开发工作区'), findsOneWidget);
    expect(find.byTooltip('注册工作区'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('兼容已启用的旧版列表响应', (tester) async {
    final api = _DevApi()..data = {'workspaces': [], 'total': 0};
    await _render(tester, api);
    expect(find.text('暂无开发工作区'), findsOneWidget);
    expect(find.byTooltip('注册工作区'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
