import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/workshop/presentation/pages/pet_center_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _PetApi implements BackendServiceApi {
  final List<String> requests = [];
  List<Map<String, dynamic>> tasks = [];
  bool fail = false;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    if (path == '/api/desktop-pets/generation-tasks') {
      expect(queryParameters, {'page': 1, 'pageSize': 100});
      if (fail) throw StateError('连接暂时不可用');
      return {'items': tasks, 'total': tasks.length, 'page': 1, 'pageSize': 100}
          as T;
    }
    if (path == '/api/extensions/pet/plugins') {
      return {'plugins': [], 'total': 0, 'page': 1, 'pageSize': 20} as T;
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
  testWidgets('首页使用桌宠任务接口并正常显示空任务', (tester) async {
    final api = _PetApi();
    await _pumpPage(tester, api);
    expect(api.requests, [
      '/api/desktop-pets/generation-tasks',
      '/api/extensions/pet/plugins',
    ]);
    expect(find.text('桌宠制作'), findsOneWidget);
    expect(find.text('创建桌宠'), findsOneWidget);
    expect(find.text('没有进行中的任务'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('新任务字段和终态正确展示', (tester) async {
    final api = _PetApi()
      ..tasks = [
        {
          'id': 'running',
          'name': '生成任务',
          'status': 'running',
          'selectedActionCount': 8,
          'progress': 35,
          'modelName': '测试模型',
        },
        {'id': 'done', 'name': '完成任务', 'status': 'succeeded'},
        {'id': 'failed', 'name': '失败任务', 'status': 'failed'},
      ];
    await _pumpPage(tester, api);
    expect(find.text('8 个动作'), findsOneWidget);
    expect(find.text('35%'), findsOneWidget);
    expect(find.text('生成中'), findsNWidgets(2));
    expect(find.text('完成任务'), findsOneWidget);
    expect(find.text('失败任务'), findsOneWidget);
    expect(find.text('已完成'), findsOneWidget);
    expect(find.text('生成失败'), findsOneWidget);
    expect(find.text('测试模型'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('加载失败保留导航并可以重试恢复', (tester) async {
    final api = _PetApi()..fail = true;
    await _pumpPage(tester, api);
    expect(find.text('桌宠制作'), findsOneWidget);
    expect(find.text('重试'), findsOneWidget);
    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('创建桌宠'), findsOneWidget);
    expect(find.text('重试'), findsNothing);
    expect(api.requests.where((p) => p.endsWith('generation-tasks')).length, 2);
    expect(tester.takeException(), isNull);
  });
}
