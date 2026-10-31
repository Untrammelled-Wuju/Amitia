import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/models/project.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/conversations/presentation/pages/conversation_list_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  testWidgets('消息页切换项目模式后展示项目列表和新项目入口', (tester) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    SharedPreferences.setMockInitialValues({});
    final project = ProjectDto(
      id: 'project-a',
      name: '项目 A',
      workspaceId: 'workspace-a',
      workspaces: const [
        ProjectWorkspaceDto(
          id: 'workspace-a',
          deviceId: 'device-a',
          name: '主目录',
          kind: 'local',
          rootUri: 'amitia://workspace/@workspace-a/',
        ),
      ],
      primaryWorkspaceId: 'workspace-a',
      conversations: [
        ConversationDto(
          id: 'conversation-a',
          projectId: 'project-a',
          characterId: 'role-a',
          title: '项目内对话',
        ),
      ],
    );
    final router = GoRouter(
      initialLocation: '/conversations',
      routes: [
        GoRoute(
          path: '/conversations',
          builder: (context, state) => const ConversationListPage(),
        ),
      ],
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          conversationListProvider.overrideWith((ref) async => project.conversations),
          characterListProvider.overrideWith(
            (ref) async => [CharacterDto(id: 'role-a', name: '角色 A')],
          ),
          conversationSidebarProvider.overrideWith(
            (ref) async => ConversationSidebarDto(projects: [project]),
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('消息'), findsOneWidget);
    await tester.longPress(find.text('项目内对话'));
    await tester.pumpAndSettle();
    expect(find.text('重命名'), findsOneWidget);
    expect(find.text('置顶'), findsWidgets);
    expect(find.text('归档'), findsWidgets);
    expect(find.text('标为已读'), findsWidgets);
    expect(find.text('删除'), findsOneWidget);
    await tester.tapAt(const Offset(10, 10));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('conversation-view-switch')));
    await tester.pumpAndSettle();
    expect(find.text('切换为项目模式'), findsOneWidget);

    await tester.tap(find.text('切换'));
    await tester.pumpAndSettle();
    expect(find.text('项目'), findsOneWidget);
    expect(find.text('项目 A'), findsOneWidget);
    expect(find.text('项目内对话'), findsNothing);
    final projectHeader = find.byKey(
      const ValueKey('project-header-project-a'),
    );
    expect(tester.widget<InkWell>(projectHeader).borderRadius, isNull);
    expect(
      tester.getSize(projectHeader).width,
      tester
          .getSize(find.byKey(const ValueKey('project-conversation-list')))
          .width,
    );

    await tester.tap(find.text('项目 A'));
    await tester.pumpAndSettle();
    expect(find.text('项目内对话'), findsOneWidget);

    await tester.longPress(find.text('项目内对话'));
    await tester.pumpAndSettle();
    expect(find.text('重命名'), findsOneWidget);
    expect(find.text('标为已读'), findsWidgets);
    expect(find.text('调整所属项目'), findsOneWidget);
    await tester.tapAt(const Offset(10, 10));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('批量选择'));
    await tester.pumpAndSettle();
    expect(find.text('选择对话'), findsOneWidget);
    await tester.tap(find.text('项目内对话'));
    await tester.pumpAndSettle();
    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isTrue);
    await tester.longPress(find.text('项目内对话'));
    await tester.pumpAndSettle();
    expect(find.text('归档'), findsWidgets);
    expect(find.text('移动到项目'), findsOneWidget);
    await tester.tapAt(const Offset(10, 10));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('退出多选'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('message-create-button')));
    await tester.pumpAndSettle();
    expect(find.text('新项目'), findsOneWidget);

    await tester.tapAt(const Offset(10, 10));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('conversation-view-switch')));
    await tester.pumpAndSettle();
    expect(find.text('切换为消息模式'), findsOneWidget);

    await tester.tap(find.text('切换'));
    await tester.pumpAndSettle();
    expect(find.text('消息'), findsOneWidget);
    expect(find.text('项目'), findsNothing);
  });
}
