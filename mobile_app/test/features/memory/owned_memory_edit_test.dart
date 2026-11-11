import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:amitia_app/app/theme/app_colors.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_memory_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/services/shared_core_data_access.dart';
import 'package:amitia_app/features/memory/presentation/pages/owned_memory_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import '../../core/services/owned_task_submission_test.dart' show TaskTransport;
import 'memory_feature_parity_test.dart' show MemorySource, currentScope;

class AuthorityMemory extends DeviceOwnedMemoryService {
  AuthorityMemory(super.api) : super(isCurrent: () => true);

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
  }) async => {
    'resources': [
      {
        'kind': 'memory',
        'id': 'record',
        'ownerId': 'phone',
        'roleId': 'role',
        'revision': 1,
        'executionScope': currentScope('role'),
        '_memoryAuthority': Object(),
        'body': {
          'key': '饮品',
          'content': {'value': '茶'},
        },
      },
    ],
    'executionScope': currentScope(memoryView),
    'nextCursor': '',
  };
}

void main() {
  testWidgets('内部权限标记不阻止记忆编辑弹窗', (tester) async {
    final source = MemorySource(BackendServiceApi(TaskTransport([]), 1));
    final service = AuthorityMemory(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => const OwnedMemoryPage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          memoryDataSourceProvider.overrideWithValue(source),
          deviceOwnedMemoryServiceProvider.overrideWithValue(service),
          sharedCoreDataAccessProvider.overrideWith(
            (ref) async => const SharedCoreDataAccess.local(),
          ),
        ],
        child: MaterialApp.router(
          routerConfig: router,
          theme: AppTheme.lightTheme(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('编辑'));
    await tester.pumpAndSettle();
    expect(find.text('编辑记忆内容'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('只读模式点击新增记忆时解释权限原因', (tester) async {
    final source = MemorySource(BackendServiceApi(TaskTransport([]), 1));
    final service = AuthorityMemory(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => const OwnedMemoryPage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          memoryDataSourceProvider.overrideWithValue(source),
          deviceOwnedMemoryServiceProvider.overrideWithValue(service),
          sharedCoreDataAccessProvider.overrideWith(
            (ref) async => const SharedCoreDataAccess(
              read: true,
              write: false,
              reason: '当前设备未获统筹中心管理员授权，只能查看中心数据',
              coreId: 'core',
            ),
          ),
        ],
        child: MaterialApp.router(
          routerConfig: router,
          theme: AppTheme.lightTheme(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final action = tester.widget<FloatingActionButton>(
      find.byType(FloatingActionButton),
    );
    expect(action.onPressed, isNotNull);
    expect(action.backgroundColor, isNot(AppColors.light.accentPrimary));
    await tester.tap(find.byType(FloatingActionButton));
    await tester.pumpAndSettle();
    expect(find.textContaining('当前设备未获统筹中心管理员授权'), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  testWidgets('只读模式的总览管理入口解释权限原因', (tester) async {
    final source = MemorySource(BackendServiceApi(TaskTransport([]), 1));
    final service = AuthorityMemory(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => const OwnedMemoryPage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          memoryDataSourceProvider.overrideWithValue(source),
          deviceOwnedMemoryServiceProvider.overrideWithValue(service),
          sharedCoreDataAccessProvider.overrideWith(
            (ref) async => const SharedCoreDataAccess(
              read: true,
              write: false,
              reason: '当前设备无写入权限',
              coreId: 'core',
            ),
          ),
        ],
        child: MaterialApp.router(
          routerConfig: router,
          theme: AppTheme.lightTheme(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final button = find.ancestor(
      of: find.text('生成候选'),
      matching: find.byType(TextButton),
    );
    expect(tester.widget<TextButton>(button).onPressed, isNull);
    await tester.tap(find.text('生成候选'));
    await tester.pumpAndSettle();
    expect(find.textContaining('当前设备无写入权限'), findsWidgets);
  });

  testWidgets('没有可用角色时刷新说明原因', (tester) async {
    final source = MemorySource(BackendServiceApi(TaskTransport([]), 1))
      ..roles = [];
    final service = AuthorityMemory(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => const OwnedMemoryPage(),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          memoryDataSourceProvider.overrideWithValue(source),
          deviceOwnedMemoryServiceProvider.overrideWithValue(service),
          sharedCoreDataAccessProvider.overrideWith(
            (ref) async => const SharedCoreDataAccess.local(),
          ),
        ],
        child: MaterialApp.router(
          routerConfig: router,
          theme: AppTheme.lightTheme(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('刷新'));
    await tester.tap(find.text('刷新'));
    await tester.pump();
    expect(find.text('请先创建或选择可用角色。'), findsOneWidget);
  });
}
