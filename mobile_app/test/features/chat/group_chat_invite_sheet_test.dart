import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/models/project.dart';
import 'package:amitia_app/features/chat/presentation/widgets/group_chat_invite_sheet.dart';
import 'package:flutter/material.dart';
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
  }) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () => showGroupChatInviteSheet(
                context,
                characters: characters,
                preferredRoleId: 'role-a',
                projects: const [project],
                showProjectSelector: showProjectSelector,
              ),
              child: const Text('open'),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
  }

  testWidgets('正常消息模式的群聊弹窗不显示工作区选择', (tester) async {
    await openSheet(tester, showProjectSelector: false);

    expect(find.text('新建群聊'), findsOneWidget);
    expect(find.text('选择工作区'), findsNothing);
    expect(find.text('项目 A'), findsNothing);
  });

  testWidgets('项目模式的群聊弹窗显示工作区选择', (tester) async {
    await openSheet(tester, showProjectSelector: true);

    expect(find.text('新建群聊'), findsOneWidget);
    expect(find.text('选择工作区'), findsOneWidget);
    expect(find.text('项目 A'), findsOneWidget);
    expect(find.text('主工作区：主目录'), findsOneWidget);
  });
}
