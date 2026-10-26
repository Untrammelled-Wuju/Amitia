import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/app/theme/app_colors.dart';
import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/models/memory.dart';
import 'package:amitia_app/core/models/reminder.dart';
import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/ui_runtime/routing/builtin_route_catalog.dart';
import 'package:amitia_app/core/widgets/amitia_icons.dart';
import 'package:amitia_app/core/widgets/amitia_popup_menu.dart';
import 'package:amitia_app/core/widgets/main_tab_scaffold.dart';
import 'package:amitia_app/features/conversations/presentation/pages/conversation_list_page.dart';
import 'package:amitia_app/features/conversations/presentation/pages/conversation_search_page.dart';
import 'package:amitia_app/features/extensions/presentation/pages/extension_center_page.dart';
import 'package:amitia_app/features/memory/presentation/pages/memory_hub_page.dart';
import 'package:amitia_app/features/profile/presentation/pages/character_world_book_hub_page.dart';
import 'package:amitia_app/features/profile/presentation/pages/profile_hub_page.dart';
import 'package:amitia_app/features/tasks/presentation/pages/task_hub_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

List<CharacterDto> _characters() => [
  CharacterDto(
    id: 'c1',
    name: '小夏',
    avatar: '',
    identity: '',
    personality: '',
    speakingStyle: '',
    description: '',
    status: '',
    isActive: 1,
    createdAt: '',
  ),
  CharacterDto(
    id: 'c2',
    name: '小林',
    avatar: '',
    identity: '',
    personality: '',
    speakingStyle: '',
    description: '',
    status: '',
    isActive: 1,
    createdAt: '',
  ),
];

GoRouter _router() {
  return GoRouter(
    initialLocation: AppRoutes.conversations,
    routes: [
      ...buildBuiltinBusinessRoutes(),
      GoRoute(
        path: '/test-detail',
        builder: (context, state) =>
            const Scaffold(body: Center(child: Text('详情页'))),
      ),
    ],
  );
}

Widget _app(
  GoRouter router, {
  List<ConversationDto> conversations = const <ConversationDto>[],
  List<ReminderDto> tasks = const <ReminderDto>[],
  List<MemoryDto> memories = const <MemoryDto>[],
}) {
  return ProviderScope(
    overrides: [
      conversationListProvider.overrideWith((ref) async => conversations),
      characterListProvider.overrideWith((ref) async => _characters()),
      currentSpaceProfileProvider.overrideWith((ref) async => null),
      reminderListProvider.overrideWith((ref) async => tasks),
      recentMemoryPageProvider.overrideWith((ref, cursor) async {
        final start = int.tryParse(cursor) ?? 0;
        final page = memories.skip(start).take(10).toList(growable: false);
        final next = start + page.length;
        return MemoryPage(
          items: page,
          nextCursor: next < memories.length ? '$next' : '',
        );
      }),
      installedExtensionViewProvider.overrideWith(
        (ref) async => ExtensionCenterView(
          installed: const [],
          discover: const [],
          updates: const [],
          needsAction: const [],
        ),
      ),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void _expectRootHeaderSurface(WidgetTester tester) {
  final context = tester.element(find.byType(MainTabScaffold));
  final appBar = tester.widget<AppBar>(find.byType(AppBar));
  final expectedIconBrightness =
      ThemeData.estimateBrightnessForColor(context.surfacePrimary) ==
          Brightness.dark
      ? Brightness.light
      : Brightness.dark;
  expect(appBar.backgroundColor, context.surfacePrimary);
  expect(appBar.backgroundColor, isNot(context.backgroundPrimary));
  expect(appBar.toolbarHeight, 50);
  expect(
    appBar.systemOverlayStyle?.statusBarIconBrightness,
    expectedIconBrightness,
  );
}

void _expectRootContentTop(WidgetTester tester, Finder page) {
  final listView = tester.widget<ListView>(
    find.descendant(of: page, matching: find.byType(ListView)).first,
  );
  expect(listView.padding, const EdgeInsets.fromLTRB(16, 12, 16, 20));
}

void main() {
  testWidgets('五个底部入口可以切换', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(500, 1200);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    await tester.pumpWidget(_app(_router()));
    await tester.pumpAndSettle();

    expect(find.byType(ConversationListPage), findsOneWidget);
    expect(find.byType(MainTabScaffold), findsOneWidget);
    expect(find.byKey(const ValueKey('main-tab-消息')), findsOneWidget);
    expect(find.byKey(const ValueKey('main-tab-任务')), findsOneWidget);
    expect(find.byKey(const ValueKey('main-tab-记忆')), findsOneWidget);
    expect(find.byKey(const ValueKey('main-tab-插件')), findsOneWidget);
    expect(find.byKey(const ValueKey('main-tab-我的')), findsOneWidget);
    expect(find.byKey(const ValueKey('message-search-field')), findsOneWidget);
    _expectRootHeaderSurface(tester);
    expect(
      tester
          .widget<Padding>(find.byKey(const ValueKey('message-search-region')))
          .padding,
      const EdgeInsets.fromLTRB(16, 12, 16, 4),
    );

    await tester.tap(find.byKey(const ValueKey('main-tab-任务')));
    await tester.pumpAndSettle();
    expect(find.byType(TaskHubPage), findsOneWidget);
    _expectRootHeaderSurface(tester);
    _expectRootContentTop(tester, find.byType(TaskHubPage));
    expect(find.text('主动任务'), findsOneWidget);
    expect(find.text('持续事项'), findsOneWidget);
    expect(find.text('提醒、计划与定时任务'), findsNothing);
    expect(find.text('长期跟踪和跨对话进展'), findsNothing);
    expect(find.text('暂无任务'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('main-tab-记忆')));
    await tester.pumpAndSettle();
    expect(find.byType(MemoryHubPage), findsOneWidget);
    _expectRootHeaderSurface(tester);
    _expectRootContentTop(tester, find.byType(MemoryHubPage));
    for (final label in ['知识库', '记忆总览', '情景记忆', '记忆图谱', '时间线']) {
      expect(find.text(label), findsOneWidget);
    }
    expect(find.text('用户画像'), findsOneWidget);
    expect(find.text('世界书'), findsOneWidget);
    expect(find.text('全部长期记忆'), findsNothing);
    expect(find.text('关系与概念连接'), findsNothing);
    expect(find.text('暂无最近记忆'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('main-tab-插件')));
    await tester.pumpAndSettle();
    expect(find.text('扩展包'), findsOneWidget);
    _expectRootHeaderSurface(tester);
    _expectRootContentTop(tester, find.byType(ExtensionCenterPage));

    await tester.tap(find.byKey(const ValueKey('main-tab-我的')));
    await tester.pumpAndSettle();
    expect(find.byType(ProfileHubPage), findsOneWidget);
    _expectRootHeaderSurface(tester);
    _expectRootContentTop(tester, find.byType(ProfileHubPage));
    expect(find.text('我的空间'), findsOneWidget);
    for (final label in ['角色', '创意工坊', '游戏模式', '渠道接入', '设置']) {
      expect(find.text(label), findsOneWidget);
    }
    expect(find.text('管理和切换对话角色'), findsNothing);
    expect(find.text('角色卡、工作流与桌宠'), findsNothing);

    await tester.tap(find.byKey(const ValueKey('main-tab-消息')));
    await tester.pumpAndSettle();
    expect(find.byType(ConversationListPage), findsOneWidget);
    _expectRootHeaderSurface(tester);
  });

  testWidgets('任务和记忆预览支持向下滚动加载更多', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(500, 800);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    final tasks = <ReminderDto>[
      for (var index = 1; index <= 12; index++)
        ReminderDto(
          id: 'task-$index',
          title: '计划任务 $index',
          enabled: 1,
          localRemindAt: DateTime.now()
              .add(Duration(hours: index))
              .toIso8601String(),
        ),
    ];
    final memories = <MemoryDto>[
      for (var index = 1; index <= 12; index++)
        MemoryDto(
          id: 'memory-$index',
          key: '最近记忆 $index',
          value: '记忆内容 $index',
          updatedAt: DateTime.now()
              .subtract(Duration(minutes: index))
              .toIso8601String(),
        ),
    ];

    await tester.pumpWidget(_app(_router(), tasks: tasks, memories: memories));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('main-tab-任务')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('planned-task-task-10')), findsOneWidget);
    expect(find.byKey(const ValueKey('planned-task-task-11')), findsNothing);

    final taskList = find.descendant(
      of: find.byType(TaskHubPage),
      matching: find.byType(ListView),
    );
    await tester.drag(taskList, const Offset(0, -900));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('planned-task-task-11')), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('main-tab-记忆')));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('recent-memory-memory-10')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('recent-memory-memory-11')), findsNothing);

    final memoryList = find.descendant(
      of: find.byType(MemoryHubPage),
      matching: find.byType(ListView),
    );
    await tester.drag(memoryList, const Offset(0, -1200));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('recent-memory-memory-11')),
      findsOneWidget,
    );
  });

  testWidgets('我的角色入口先进入角色与世界书二级页', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(500, 1200);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    final router = _router();
    await tester.pumpWidget(_app(router));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('main-tab-我的')));
    await tester.pumpAndSettle();

    await tester.tap(find.text('角色'));
    await tester.pumpAndSettle();
    expect(find.byType(CharacterWorldBookHubPage), findsOneWidget);
    expect(find.text('角色管理'), findsOneWidget);
    expect(find.text('世界书'), findsOneWidget);
  });

  testWidgets('角色与世界书二级页提供两个独立入口', (tester) async {
    final router = GoRouter(
      initialLocation: AppRoutes.meCharacters,
      routes: [
        GoRoute(
          path: AppRoutes.meCharacters,
          builder: (context, state) => const CharacterWorldBookHubPage(),
        ),
        GoRoute(
          path: AppRoutes.characters,
          builder: (context, state) => const Scaffold(body: Text('角色管理目标页')),
        ),
        GoRoute(
          path: AppRoutes.memoryWorldBook,
          builder: (context, state) => const Scaffold(body: Text('世界书目标页')),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();

    await tester.tap(find.text('角色管理'));
    await tester.pumpAndSettle();
    expect(find.text('角色管理目标页'), findsOneWidget);

    router.pop();
    await tester.pumpAndSettle();
    await tester.tap(find.text('世界书'));
    await tester.pumpAndSettle();
    expect(find.text('世界书目标页'), findsOneWidget);
  });

  testWidgets('消息页加号提供新对话和新群聊', (tester) async {
    await tester.pumpWidget(_app(_router()));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('message-create-button')));
    await tester.pumpAndSettle();
    expect(find.byType(AmitiaPopupSurface), findsOneWidget);
    expect(find.text('新对话'), findsOneWidget);
    expect(find.text('新群聊'), findsOneWidget);

    await tester.tap(find.text('新群聊'));
    await tester.pumpAndSettle();
    expect(find.text('新建群聊'), findsOneWidget);
    expect(find.text('默认发言角色'), findsOneWidget);
    expect(find.text('邀请角色 (0)'), findsOneWidget);
    expect(find.text('小夏'), findsWidgets);
    expect(find.text('小林'), findsWidgets);
  });

  testWidgets('消息列表采用无日期分组的 QQ 式行布局', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(500, 1200);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    final conversation = ConversationDto(
      id: 'c1',
      characterId: 'c1',
      title: '小夏',
      messageCount: 1,
      updatedAt: DateTime.now().toIso8601String(),
    );

    await tester.pumpWidget(
      _app(_router(), conversations: <ConversationDto>[conversation]),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('conversation-c1')), findsOneWidget);
    expect(find.text('今天'), findsNothing);
    expect(find.text('昨天'), findsNothing);
    expect(find.text('更早'), findsNothing);
    expect(find.text('1 条消息'), findsOneWidget);
    expect(
      tester.getSize(
        find.byWidgetPredicate(
          (widget) =>
              widget is AmitiaIcon && widget.glyph == AmitiaIconGlyph.search,
        ),
      ),
      const Size.square(17),
    );

    await tester.tap(find.byKey(const ValueKey('message-search-field')));
    await tester.pumpAndSettle();
    expect(find.byType(ConversationSearchPage), findsOneWidget);
    expect(find.byType(MainTabScaffold), findsNothing);

    await tester.enterText(find.byType(TextField), '小夏');
    await tester.pump();
    expect(find.byKey(const ValueKey('search-result-c1')), findsOneWidget);
  });

  testWidgets('二级页面隐藏底栏，返回后恢复根页面', (tester) async {
    final router = _router();
    await tester.pumpWidget(_app(router));
    await tester.pumpAndSettle();

    router.push('/test-detail');
    await tester.pumpAndSettle();
    expect(find.text('详情页'), findsOneWidget);
    expect(find.byType(MainTabScaffold), findsNothing);

    router.pop();
    await tester.pumpAndSettle();
    expect(find.byType(ConversationListPage), findsOneWidget);
    expect(find.byType(MainTabScaffold), findsOneWidget);
  });

  testWidgets('非消息根页面按系统返回键回到消息', (tester) async {
    await tester.pumpWidget(_app(_router()));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('main-tab-任务')));
    await tester.pumpAndSettle();
    expect(find.byType(TaskHubPage), findsOneWidget);

    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();

    expect(find.byType(ConversationListPage), findsOneWidget);
    expect(find.byType(TaskHubPage), findsNothing);
  });

  testWidgets('消息根页面连续两次返回才退出应用', (tester) async {
    var exitCalls = 0;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'SystemNavigator.pop') {
          exitCalls++;
        }
        return null;
      },
    );
    addTearDown(() {
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      );
    });

    await tester.pumpWidget(_app(_router()));
    await tester.pumpAndSettle();

    await tester.binding.handlePopRoute();
    await tester.pump();
    expect(exitCalls, 0);
    expect(find.text('再按一次退出应用'), findsOneWidget);

    await tester.binding.handlePopRoute();
    await tester.pump();
    expect(exitCalls, 1);
  });
}
