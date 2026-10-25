import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
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
import 'package:amitia_app/features/memory/presentation/widgets/historical_memory_card.dart';
import 'package:amitia_app/app/theme/app_theme.dart';
import '../../core/services/owned_task_submission_test.dart' show TaskTransport;
import '../../core/services/owned_memory_management_test.dart' as fixture;

Map<String, dynamic> currentScope(String view) => {
  ...fixture.scope(),
  'memoryRoleId': view.startsWith('role:')
      ? view.substring(5)
      : view == 'global'
      ? '@global-memory'
      : 'role',
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
  Future<List<Map<String, dynamic>>> historicalRoles(
    String characterId,
  ) async => [
    {'id': 'old-role', 'name': '旧角色'},
  ];
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
      'resources': kind != 'memory' ? [record(kind)] : [],
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
    if (historicalRoleId.isNotEmpty)
      'historicalSnapshot': {
        'ownerId': 'phone',
        'resources': [],
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
    int retentionLevel = 0,
    bool? pinned,
    String decayState = '',
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
  test('兼容记录的卡片只展示正文，不拼接内部字段', () {
    final row = {
      ...record('memory'),
      'body': {
        'content': {
          'value': '用户喜欢喝茶',
          'spaceId': 'internal-space',
          'characterId': 'internal-role',
          'sourceConvId': 'internal-conversation',
          'confidence': 60,
        },
      },
    };
    expect(historicalMemoryPreview(row, kind: 'memory'), '用户喜欢喝茶');
    expect(
      historicalMemoryPreview({
        'body': {
          'content': {
            'attributeName': '饮品',
            'attributeValue': '茶',
            'spaceId': 'internal-space',
          },
        },
      }, kind: 'profile'),
      '茶',
    );
    expect(
      historicalMemoryPreview({
        'body': {
          'content': {'event': '一起散步', 'characterId': 'internal-role'},
        },
      }, kind: 'episodic'),
      '一起散步',
    );
  });
  var previewKey = GlobalKey();
  setUpAll(() async {
    final fontPath = Platform.environment['MEMORY_UI_PREVIEW_FONT'];
    final iconsPath = Platform.environment['MEMORY_UI_PREVIEW_ICONS'];
    if (fontPath != null) {
      final loader = FontLoader('AmitiaPreview')
        ..addFont(
          Future.value(
            ByteData.sublistView(await File(fontPath).readAsBytes()),
          ),
        );
      await loader.load();
    }
    if (iconsPath != null) {
      final loader = FontLoader('MaterialIcons')
        ..addFont(
          Future.value(
            ByteData.sublistView(await File(iconsPath).readAsBytes()),
          ),
        );
      await loader.load();
    }
  });
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
  Future<MemorySource> render(
    WidgetTester tester,
    String kind, {
    bool showTools = false,
    double textScale = 1,
    Brightness brightness = Brightness.light,
    bool timeline = false,
    bool coordinated = false,
  }) async {
    previewKey = GlobalKey();
    final api = BackendServiceApi(TaskTransport([]), 1),
        source = MemorySource(BackendServiceApi(TaskTransport([]), 1));
    source.policy = {...source.policy!, 'coordinated': coordinated};
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => OwnedMemoryPage(
            initialKind: kind,
            showTools: showTools,
            timeline: timeline,
          ),
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
        child: RepaintBoundary(
          key: previewKey,
          child: MaterialApp.router(
            debugShowCheckedModeBanner: false,
            routerConfig: router,
            theme:
                (brightness == Brightness.dark
                        ? AppTheme.darkTheme()
                        : AppTheme.lightTheme())
                    .copyWith(
                      textTheme:
                          (brightness == Brightness.dark
                                  ? AppTheme.darkTheme()
                                  : AppTheme.lightTheme())
                              .textTheme
                              .apply(
                                fontFamily:
                                    Platform.environment['MEMORY_UI_PREVIEW_FONT'] ==
                                        null
                                    ? null
                                    : 'AmitiaPreview',
                              ),
                    ),
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: TextScaler.linear(textScale)),
              child: child!,
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return source;
  }

  testWidgets('记忆页面忽略旧版本数据、保留当前入口和操作', (tester) async {
    final source = await render(tester, 'memory');
    expect(find.text('查看范围'), findsNothing);
    expect(find.text('全部记忆'), findsOneWidget);
    expect(find.text('记忆层'), findsNothing);
    expect(find.text('工作角色'), findsNothing);
    await tester.tap(find.byTooltip('筛选与管理'));
    await tester.pumpAndSettle();
    for (final text in ['待审核候选', '从对话生成候选', '查看范围']) {
      expect(find.text(text), findsOneWidget);
    }
    await tester.tap(find.byTooltip('关闭筛选'));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('owned-memory/legacy/old')), findsNothing);
    expect(find.textContaining('保留内容'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('owned-memory/record')));
    await tester.pumpAndSettle();
    for (final text in ['编辑', '到期时间', '归档']) {
      expect(
        find.descendant(
          of: find.byType(BottomSheet),
          matching: find.text(text),
        ),
        findsOneWidget,
      );
    }
    await tester.ensureVisible(find.text('到期时间'));
    await tester.tap(
      find.descendant(
        of: find.byType(BottomSheet),
        matching: find.text('到期时间'),
      ),
    );
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '2027-01-01');
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(source.edits.single['scope']['memoryRoleId'], '@global-memory');
    expect(source.edits.single['characterId'], 'role');
    expect(find.byType(BottomSheet), findsNothing);
  });
  testWidgets('用户画像默认角色范围并可查看全局，图谱页显示真实画布', (tester) async {
    await render(tester, 'profile');
    await tester.tap(find.byTooltip('筛选与管理'));
    await tester.pumpAndSettle();
    expect(find.text('查看范围'), findsOneWidget);
    expect(find.text('全局记忆'), findsOneWidget);
    expect(find.text('记忆层'), findsNothing);
    expect(find.textContaining('默认按角色隔离'), findsOneWidget);
    await tester.tap(find.byTooltip('关闭筛选'));
    await tester.pumpAndSettle();
    await render(tester, 'graph');
    expect(find.byType(OwnedMemoryGraph), findsOneWidget);
    expect(find.byType(InteractiveViewer), findsOneWidget);
  });
  for (final kind in ['memory', 'profile', 'episodic', 'graph', 'timeline']) {
    testWidgets('$kind 历史来源的范围不混入当前角色', (tester) async {
      await render(
        tester,
        kind == 'timeline' ? 'memory' : kind,
        timeline: kind == 'timeline',
        coordinated: true,
      );
      await tester.tap(find.byTooltip('筛选与管理'));
      await tester.pumpAndSettle();
      final source = tester.widget<DropdownButtonFormField<String>>(
        find.byKey(const ValueKey('source/current')),
      );
      source.onChanged!('history:old-role');
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('筛选与管理'));
      await tester.pumpAndSettle();
      final scope = tester.widget<DropdownButtonFormField<String>>(
        find.byKey(const ValueKey('view/all/history:old-role')),
      );
      expect(scope.initialValue, 'all');
      expect(
        find.text(kind == 'profile' ? '原设备用户画像' : '原设备历史 · 旧角色'),
        findsWidgets,
      );
      scope.onChanged!('all');
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('筛选与管理'));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('source/history:old-role')),
        findsOneWidget,
      );
    });
  }
  testWidgets('历史工具卡片在窄屏、大字体和深色下保持可用', (tester) async {
    tester.view.physicalSize = const Size(375, 812);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await render(
      tester,
      'memory',
      showTools: true,
      textScale: 2,
      brightness: Brightness.dark,
    );
    for (final title in ['记忆管理', '情景记忆', '记忆图谱', '记忆时间线', '用户画像', '世界书']) {
      expect(find.text(title), findsOneWidget);
    }
    expect(find.byType(GridView), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
  testWidgets('搜索和筛选可以展开，手机横屏不溢出', (tester) async {
    tester.view.physicalSize = const Size(812, 375);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await render(tester, 'memory');
    await tester.tap(find.byTooltip('搜索记忆'));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('search/memory')), findsOneWidget);
    await tester.tap(find.byTooltip('筛选与管理'));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('筛选记忆'));
    await tester.tap(find.text('筛选记忆'));
    await tester.pumpAndSettle();
    expect(find.text('检索方式'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
  testWidgets('历史页面布局在正常字号下保留原网格比例和页面样式', (tester) async {
    tester.view.physicalSize = const Size(375, 812);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    for (final kind in ['memory', 'profile', 'episodic']) {
      await render(tester, kind, showTools: kind == 'memory');
      if (kind == 'memory') {
        final grid = tester.widget<GridView>(find.byType(GridView));
        final delegate =
            grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount;
        expect(delegate.crossAxisCount, 2);
        expect(delegate.childAspectRatio, 3);
      }
      expect(tester.takeException(), isNull);
      final directory = Platform.environment['MEMORY_UI_PREVIEW_DIR'];
      if (directory != null) {
        final boundary =
            previewKey.currentContext!.findRenderObject()!
                as RenderRepaintBoundary;
        await tester.runAsync(() async {
          final image = await boundary.toImage();
          final data = await image.toByteData(format: ui.ImageByteFormat.png);
          await Directory(directory).create(recursive: true);
          await File(
            '$directory/$kind.png',
          ).writeAsBytes(data!.buffer.asUint8List());
          image.dispose();
        });
      }
    }
  });
}
