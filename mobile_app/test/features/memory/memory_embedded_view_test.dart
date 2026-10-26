import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_memory_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/memory/presentation/pages/owned_memory_page.dart';
import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import '../../core/services/owned_task_submission_test.dart' show TaskTransport;
import 'memory_feature_parity_test.dart' show MemorySource, currentScope;

class RecordingMemory extends DeviceOwnedMemoryService {
  RecordingMemory(super.api) : super(isCurrent: () => true);
  final List<String> views = [];
  @override
  Future<Map<String, dynamic>> query(
    String characterId, {
    bool candidates = false,
    String query = '',
    String mode = 'keyword',
    String memoryType = '',
    String source = '',
    String sort = '',
    int retentionLevel = 0,
    bool? pinned,
    String decayState = '',
    String cursor = '',
    Map<String, dynamic>? original,
    int limit = 128,
    String memoryView = '',
  }) async {
    views.add('$characterId|$memoryView');
    return {
      'resources': [
        {
          'kind': 'memory',
          'id': 'record',
          'ownerId': 'phone',
          'roleId': 'role',
          'revision': 1,
          'body': {
            'key': '饮品',
            'content': {'value': '茶'},
          },
        },
      ],
      'executionScope': currentScope(memoryView.isEmpty ? 'role' : memoryView),
      'nextCursor': '',
    };
  }
}

class ColdMemorySource extends MemorySource {
  ColdMemorySource(super.api) {
    roles = [];
  }
  @override
  Future<bool> refresh() async {
    roles = [
      {'id': 'role', 'name': '角色', 'revision': 2},
    ];
    return true;
  }
}

void main() {
  testWidgets('角色详情内嵌记忆视图在角色晚于首帧加载时仍按角色范围取数', (tester) async {
    final source = ColdMemorySource(BackendServiceApi(TaskTransport([]), 1));
    final service = RecordingMemory(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) =>
              const OwnedMemoryPage(characterId: 'role', embedded: true),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          memoryDataSourceProvider.overrideWithValue(source),
          deviceOwnedMemoryServiceProvider.overrideWithValue(service),
        ],
        child: MaterialApp.router(
          routerConfig: router,
          theme: AppTheme.lightTheme(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(service.views, contains('role|role:role'));
    expect(service.views, isNot(contains('role|all')));
    expect(tester.takeException(), isNull);
  });
  testWidgets('角色详情内嵌记忆视图按角色范围加载并保留管理入口', (tester) async {
    final source = MemorySource(BackendServiceApi(TaskTransport([]), 1));
    final service = RecordingMemory(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) =>
              const OwnedMemoryPage(characterId: 'role', embedded: true),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          memoryDataSourceProvider.overrideWithValue(source),
          deviceOwnedMemoryServiceProvider.overrideWithValue(service),
        ],
        child: MaterialApp.router(
          routerConfig: router,
          theme: AppTheme.lightTheme(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(service.views, contains('role|role:role'));
    expect(find.byType(AppBar), findsNothing);
    for (final tooltip in ['搜索记忆', '筛选与管理', '记忆图谱', '新增记忆']) {
      expect(find.byTooltip(tooltip), findsOneWidget);
    }
    expect(tester.takeException(), isNull);
  });
}
