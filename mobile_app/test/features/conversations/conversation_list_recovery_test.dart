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
  testWidgets('会话列表在临时 502 后自动恢复且成功后停止重试', (tester) async {
    SharedPreferences.setMockInitialValues({});
    var requests = 0;
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
          conversationListProvider.overrideWith((ref) async {
            requests++;
            if (requests == 1) throw Exception('502 temporary outage');
            return [
              ConversationDto(
                id: 'conversation-a',
                characterId: 'role-a',
                title: '恢复后的会话',
              ),
            ];
          }),
          characterListProvider.overrideWith(
            (ref) async => [CharacterDto(id: 'role-a', name: '角色 A')],
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pump();
    await tester.pump();
    expect(find.textContaining('502 temporary outage'), findsOneWidget);
    expect(requests, 1);

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();
    expect(find.text('恢复后的会话'), findsOneWidget);
    expect(find.textContaining('502 temporary outage'), findsNothing);
    expect(requests, 2);

    await tester.pump(const Duration(seconds: 10));
    expect(requests, 2);
  });

  testWidgets('项目模式在临时失败后自动恢复', (tester) async {
    SharedPreferences.setMockInitialValues({
      'conversations_project_mode': true,
    });
    var sidebarRequests = 0;
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
          conversationListProvider.overrideWith((ref) async => []),
          characterListProvider.overrideWith(
            (ref) async => [CharacterDto(id: 'role-a', name: '角色 A')],
          ),
          conversationSidebarProvider.overrideWith((ref) async {
            sidebarRequests++;
            if (sidebarRequests == 1) throw Exception('project 502');
            return const ConversationSidebarDto(projects: []);
          }),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pump();
    await tester.pump();
    expect(find.textContaining('project 502'), findsOneWidget);
    expect(sidebarRequests, 1);

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();
    expect(find.text('还没有项目'), findsOneWidget);
    expect(find.textContaining('project 502'), findsNothing);
    expect(sidebarRequests, 2);
  });
}
