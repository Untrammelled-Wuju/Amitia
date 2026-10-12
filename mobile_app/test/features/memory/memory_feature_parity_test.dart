import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/services/device_owned_memory_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/memory/presentation/pages/owned_memory_page.dart';
import 'package:amitia_app/features/memory/presentation/widgets/owned_memory_graph.dart';
import '../../core/services/owned_task_submission_test.dart' show TaskTransport;
import '../../core/services/owned_memory_management_test.dart' as fixture;

Map<String, dynamic> currentScope(String view) => {
  ...fixture.scope(),
  'memoryRoleId': view.startsWith('role:')
      ? view.substring(5)
      : '@global-memory',
  if (view == 'all') 'memoryViewAll': true,
};
Map<String, dynamic> record(String kind) => {
  'kind': kind,
  'id': 'record',
  'ownerId': 'phone',
  'roleId': '@global-memory',
  'revision': 1,
  'executionScope': {...currentScope('global')},
  'body': {
    'key': '饮品',
    'content': {'value': '茶', 'source': '用户', 'target': '茶', 'relation': '喜欢'},
  },
};

class MemorySource extends DeviceOwnedChatService {
  MemorySource(super.api) {
    enabled = true;
    roles = [
      {'id': 'role', 'name': '角色', 'revision': 2},
      {'id': 'other', 'name': '另一角色', 'revision': 1},
    ];
    policy = {'selectedRole': 'role', 'coordinated': false};
    coreId = 'core';
  }
  List<Map> edits = [];
  @override
  Future<bool> refresh() async => true;
  @override
  Future<Map<String, dynamic>> data(
    String kind, {
    required String characterId,
    String conversationId = '',
    String cursor = '',
    String legacyCursor = '',
    String historicalRoleId = '',
    String historicalCursor = '',
    String historicalLegacyCursor = '',
    String memoryView = '',
  }) async => {
    'executionScope': currentScope(memoryView),
    'snapshot': {
      'ownerId': 'phone',
      'resources': kind == 'graph' ? [record('graph')] : [],
      'legacyMemories': kind == 'memory'
          ? [
              {
                'id': 'old',
                'key': '历史',
                'value': '保留内容',
                'scope': 'character',
                'characterId': 'role',
              },
            ]
          : [],
      'nextCursors': {},
    },
  };
  @override
  Future<void> edit(
    String kind,
    String id, {
    Map<String, dynamic> changes = const {},
    bool deleted = false,
    bool clear = false,
    String? characterId,
    Map<String, dynamic>? expectedScope,
    String? expectedOwnerId,
    int? expectedRevision,
  }) async {
    edits.add({
      'kind': kind,
      'id': id,
      'changes': changes,
      'scope': expectedScope,
      'characterId': characterId,
    });
  }
}

class MemoryManagement extends DeviceOwnedMemoryService {
  MemoryManagement(super.api) : super(isCurrent: () => true);
  @override
  Future<Map<String, dynamic>> query(
    String characterId, {
    bool candidates = false,
    String query = '',
    String mode = 'keyword',
    String memoryType = '',
    String source = '',
    String sort = '',
    String cursor = '',
    Map<String, dynamic>? original,
    int limit = 128,
    String memoryView = '',
  }) async => {
    'resources': [record('memory')],
    'executionScope': currentScope(memoryView),
    'nextCursor': '',
  };
}

void main() {
  test('手机本机模式使用同一记忆通道，不依赖云端绑定', () async {
    SharedPreferences.setMockInitialValues({});
    final transport = TaskTransport([
      fixture.state(),
      {
        'roles': [
          {'id': 'role', 'name': '角色', 'revision': 2},
        ],
      },
    ]);
    final api = BackendServiceApi(transport, 1);
    final container = ProviderContainer(
      overrides: [
        rawBackendServiceApiProvider.overrideWithValue(api),
        backendServiceProvider.overrideWithValue(api),
      ],
    );
    addTearDown(container.dispose);
    expect(await container.read(ownedMemoryModeProvider.future), isTrue);
    expect(container.read(memoryDataSourceProvider).enabled, isTrue);
    expect(transport.requests.length, 2);
  });
  test('图谱解析真实关系、去重并隔离同名角色节点', () {
    final rows = [
      record('graph'),
      record('graph'),
      {...record('graph'), 'roleId': 'other'},
    ];
    final graph = buildOwnedMemoryGraph(rows);
    expect(graph.nodes.length, 4);
    expect(graph.links.length, 2);
    final unknown = buildOwnedMemoryGraph([
      {
        'kind': 'graph',
        'id': 'opaque',
        'ownerId': 'phone',
        'roleId': 'role',
        'body': {'content': '原记录'},
      },
    ]);
    expect(unknown.nodes.length, 1);
    expect(unknown.links, isEmpty);
  });
  testWidgets('图谱画布实际显示节点，支持搜索邻接和重置', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: OwnedMemoryGraph(rows: [record('graph')])),
      ),
    );
    expect(find.byType(InteractiveViewer), findsOneWidget);
    expect(find.text('用户'), findsOneWidget);
    expect(find.text('茶'), findsOneWidget);
    await tester.enterText(find.byType(TextField), '用户');
    await tester.pump();
    expect(find.text('茶'), findsOneWidget);
    await tester.tap(find.byTooltip('重置图谱'));
    await tester.pump();
    expect(tester.takeException(), isNull);
  });
  Future<MemorySource> render(WidgetTester tester, String kind) async {
    final api = BackendServiceApi(TaskTransport([]), 1),
        source = MemorySource(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => OwnedMemoryPage(initialKind: kind),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        key: ValueKey(kind),
        overrides: [
          memoryDataSourceProvider.overrideWithValue(source),
          deviceOwnedMemoryServiceProvider.overrideWithValue(
            MemoryManagement(api),
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    return source;
  }

  testWidgets('记忆页面保留历史数据、默认全部、无重复分类并补齐操作', (tester) async {
    final source = await render(tester, 'memory');
    expect(find.text('查看范围'), findsOneWidget);
    expect(find.text('全部记忆'), findsOneWidget);
    expect(find.text('记忆层'), findsNothing);
    expect(find.text('工作角色'), findsNothing);
    for (final text in ['编辑', '到期时间', '归档', '待审核候选', '从对话生成候选']) {
      expect(find.text(text), findsOneWidget);
    }
    await tester.drag(find.byType(ListView).last, const Offset(0, -600));
    await tester.pumpAndSettle();
    expect(find.text('历史'), findsOneWidget);
    expect(find.textContaining('保留内容'), findsOneWidget);
    await tester.drag(find.byType(ListView).last, const Offset(0, 600));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('到期时间'));
    await tester.tap(find.text('到期时间'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '2027-01-01');
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(source.edits.single['scope']['memoryRoleId'], '@global-memory');
    expect(source.edits.single['characterId'], 'role');
  });
  testWidgets('用户画像无角色范围选项，图谱页显示真实画布', (tester) async {
    await render(tester, 'profile');
    expect(find.text('查看范围'), findsNothing);
    expect(find.text('记忆层'), findsNothing);
    expect(find.textContaining('全局共享'), findsOneWidget);
    await render(tester, 'graph');
    expect(find.byType(OwnedMemoryGraph), findsOneWidget);
    expect(find.byType(InteractiveViewer), findsOneWidget);
  });
}
