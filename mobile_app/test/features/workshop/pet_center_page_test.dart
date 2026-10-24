import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/workshop/presentation/pages/pet_center_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _PetApi implements BackendServiceApi {
  final List<String> requests = [];
  List<Map<String, dynamic>> plugins = [];
  bool fail = false;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    if (path == '/api/extensions/pet/plugins') {
      expect(queryParameters, {'page': '1', 'pageSize': '20'});
      if (fail) throw StateError('连接暂时不可用');
      return {
        'plugins': plugins,
        'total': plugins.length,
        'page': 1,
        'pageSize': 20,
      } as T;
    }
    throw StateError('不应调用接口: $path');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Future<void> _pumpPage(WidgetTester tester, _PetApi api) async {
  tester.view.physicalSize = const Size(800, 1400);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final router = GoRouter(
    routes: [GoRoute(path: '/', builder: (_, _) => const PetCenterPage())],
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

void main() {
  testWidgets('桌宠中心只展示安装、插件与运行入口', (tester) async {
    final api = _PetApi();
    await _pumpPage(tester, api);
    expect(api.requests, ['/api/extensions/pet/plugins']);
    expect(find.text('桌宠中心'), findsOneWidget);
    expect(find.text('安装管理'), findsOneWidget);
    expect(find.text('桌宠运行'), findsOneWidget);
    expect(find.text('暂无已安装的桌宠插件'), findsOneWidget);
    expect(find.text('创建桌宠'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('已启用插件显示运行概览', (tester) async {
    final api = _PetApi()
      ..plugins = [
        {
          'extensionId': 'com.example.pet',
          'pluginId': 'pet-main',
          'name': '测试桌宠',
          'description': '桌宠运行插件',
          'version': '1.2.3',
          'enabled': true,
          'installState': 'installed',
        },
      ];
    await _pumpPage(tester, api);
    expect(find.text('测试桌宠'), findsOneWidget);
    expect(find.text('已启用'), findsOneWidget);
    expect(find.text('已安装 1 个桌宠插件，其中 1 个已启用'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('加载失败后可以重试', (tester) async {
    final api = _PetApi()..fail = true;
    await _pumpPage(tester, api);
    expect(find.text('重试'), findsOneWidget);
    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('安装管理'), findsOneWidget);
    expect(find.text('重试'), findsNothing);
    expect(api.requests.length, 2);
    expect(tester.takeException(), isNull);
  });
}
