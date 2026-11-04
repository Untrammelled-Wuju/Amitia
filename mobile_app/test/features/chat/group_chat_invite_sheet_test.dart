import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/models/project.dart';
import 'package:amitia_app/features/chat/presentation/widgets/group_chat_invite_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final characters = [
    CharacterDto(id: 'role-a', name: '角色 A'),
    CharacterDto(id: 'role-b', name: '角色 B'),
  ];
  const project = ProjectDto(
    id: 'project-a',
    name: '项目 A',
    workspaceId: 'workspace-a',
    rootUri: 'amitia://workspace/@workspace-a/',
    workspaces: [
      ProjectWorkspaceDto(
        id: 'workspace-a',
        deviceId: 'device-a',
        name: '主目录',
        kind: 'local',
        rootUri: 'amitia://workspace/@workspace-a/',
      ),
    ],
    primaryWorkspaceId: 'workspace-a',
  );

  Future<void> openSheet(
    WidgetTester tester, {
    required bool showProjectSelector,
    bool projectRequired = false,
    List<GroupTeamOption> teams = const [],
  }) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Consumer(
            builder: (context, ref, child) => Scaffold(
              body: TextButton(
                onPressed: () => showGroupChatInviteSheet(
                  context,
                  ref: ref,
                  characters: characters,
                  preferredRoleId: 'role-a',
                  projects: const [project],
                  teams: teams,
                  showProjectSelector: showProjectSelector,
                  projectRequired: projectRequired,
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
  }

  testWidgets('消息模式显示可选工作区且默认不选择', (tester) async {
    await openSheet(tester, showProjectSelector: true);

    expect(find.text('新建群聊'), findsOneWidget);
    expect(find.text('创建到项目'), findsOneWidget);
    expect(find.text('项目 A'), findsOneWidget);
    expect(find.byIcon(Icons.radio_button_checked), findsNothing);
    await tester.tap(find.widgetWithText(CheckboxListTile, '角色 B'));
    await tester.pumpAndSettle();
    final create = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, '创建群聊'),
    );
    expect(create.onPressed, isNotNull);
  });

  testWidgets('项目模式的群聊弹窗显示工作区选择', (tester) async {
    await openSheet(tester, showProjectSelector: true, projectRequired: true);

    expect(find.text('新建群聊'), findsOneWidget);
    expect(find.text('创建到项目'), findsOneWidget);
    expect(find.text('项目 A'), findsOneWidget);
    expect(find.text('主工作区：主目录'), findsOneWidget);
    expect(find.byIcon(Icons.radio_button_checked), findsNothing);
    await tester.tap(find.widgetWithText(CheckboxListTile, '角色 B'));
    await tester.pumpAndSettle();
    var create = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, '创建群聊'),
    );
    expect(create.onPressed, isNull);
    await tester.tap(find.text('项目 A'));
    await tester.pumpAndSettle();
    create = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, '创建群聊'),
    );
    expect(create.onPressed, isNotNull);
  });

  testWidgets('选择团队会复制当前角色成员快照', (tester) async {
    await openSheet(
      tester,
      showProjectSelector: true,
      teams: const [
        GroupTeamOption(
          id: 'team-a',
          name: '团队 A',
          leadCharacterId: 'role-a',
          memberCharacterIds: ['role-a', 'role-b'],
        ),
      ],
    );
    await tester.tap(find.text('选择团队'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('团队 A').last);
    await tester.pumpAndSettle();
    final create = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, '创建群聊'),
    );
    expect(create.onPressed, isNotNull);
    expect(find.text('邀请角色 (1)'), findsOneWidget);
  });
}
