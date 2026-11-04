import 'dart:io';

import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/characters/presentation/pages/character_teams_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _TeamsApi implements BackendServiceApi {
  final paths = <String>[];

  @override
  int get generation => 1;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    final dynamic value = switch (path) {
      '/api/roles/teams' => [
        {
          'id': 'team-1',
          'name': '研究团队',
          'description': '共享研究上下文',
          'status': 'active',
          'revision': 2,
          'sharedMemory': true,
          'sharedWorkspace': true,
        },
      ],
      '/api/roles' => [
        {'id': 'role-1', 'name': '资料研究员', 'status': 'active'},
      ],
      '/api/roles/teams/templates' => <dynamic>[],
      '/api/roles/teams/team-1' => {
        'team': {
          'id': 'team-1',
          'name': '研究团队',
          'description': '共享研究上下文',
          'status': 'active',
          'revision': 2,
          'sharedMemory': true,
          'sharedWorkspace': true,
        },
        'members': [
          {
            'id': 'member-1',
            'memberType': 'agent',
            'memberId': 'role-1',
            'role': 'lead',
            'responderMode': 'lead',
          },
        ],
        'contexts': [
          {'id': 'context-1', 'kind': 'decision', 'content': '结论必须附来源'},
        ],
        'handoffs': <dynamic>[],
      },
      _ => <String, dynamic>{},
    };
    return value as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  testWidgets('手机团队页显示负责人、共享上下文和团队操作', (tester) async {
    tester.view.physicalSize = const Size(360, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final api = _TeamsApi();
    final router = GoRouter(
      initialLocation: '/characters/teams',
      routes: [
        GoRoute(
          path: '/characters/teams',
          builder: (context, state) => const CharacterTeamsPage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [rawBackendServiceApiProvider.overrideWithValue(api)],
        child: MaterialApp.router(
          theme: AppTheme.lightTheme(),
          routerConfig: router,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('研究团队'), findsWidgets);
    expect(find.text('资料研究员'), findsOneWidget);
    expect(find.text('结论必须附来源'), findsOneWidget);
    expect(find.text('添加成员'), findsOneWidget);
    await tester.tap(find.byTooltip('新建团队'));
    await tester.pumpAndSettle();
    expect(find.text('选择团队成员'), findsOneWidget);
    expect(find.text('资料研究员'), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  test('角色详情不再声明记忆、时间线、主动任务和团队职责入口', () {
    final detail = File(
      'lib/features/characters/presentation/pages/character_detail_page.dart',
    ).readAsStringSync();
    expect(
      detail,
      contains("final _tabs = const ['概览', '设定', '关系', '能力'];"),
    );
    expect(detail, isNot(contains('运行配置')));
    expect(detail, isNot(contains('CharacterRuntimeConfigSection')));
    expect(detail, isNot(contains("title: '角色记忆'")));
    expect(detail, isNot(contains("title: '关系时间线'")));
    expect(File(
      'lib/features/characters/presentation/widgets/character_runtime_config_section.dart',
    ).existsSync(), isFalse);
    final teams = File(
      'lib/features/characters/presentation/pages/character_teams_page.dart',
    ).readAsStringSync();
    expect(teams, isNot(contains('团队预算')));
    expect(teams, isNot(contains('新增预算')));
  });
}
