import 'package:amitia_app/features/chat/presentation/chat_route_state.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  testWidgets('草稿转正式会话时替换当前页并保留返回栈', (tester) async {
    final router = GoRouter(
      initialLocation: '/',
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => Scaffold(
            body: TextButton(
              onPressed: () => context.push('/chat?draftId=draft-1'),
              child: const Text('打开草稿'),
            ),
          ),
        ),
        GoRoute(
          path: '/chat',
          builder: (context, state) => Scaffold(
            body: TextButton(
              onPressed: () =>
                  replaceChatRouteWithConversation(context, 'conversation-1'),
              child: Text(
                state.uri.queryParameters['conversationId'] ?? 'draft',
              ),
            ),
          ),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();

    await tester.tap(find.text('打开草稿'));
    await tester.pumpAndSettle();
    expect(find.text('draft'), findsOneWidget);
    expect(router.canPop(), isTrue);

    await tester.tap(find.text('draft'));
    await tester.pumpAndSettle();
    expect(find.text('conversation-1'), findsOneWidget);
    expect(router.canPop(), isTrue);

    router.pop();
    await tester.pumpAndSettle();
    expect(find.text('打开草稿'), findsOneWidget);
  });

  test('相同会话路由在运行时已变空白时重新加载', () {
    expect(
      shouldApplyConversationRoute(
        previousConversationId: 'conversation-1',
        nextConversationId: 'conversation-1',
        previousProjectId: '',
        nextProjectId: '',
        runtimeConversationId: '',
        runtimeProjectId: '',
        routeSyncedConversationId: '',
      ),
      isTrue,
    );
  });

  test('相同会话路由在运行时一致时保持当前页面', () {
    expect(
      shouldApplyConversationRoute(
        previousConversationId: 'conversation-1',
        nextConversationId: 'conversation-1',
        previousProjectId: '',
        nextProjectId: '',
        runtimeConversationId: 'conversation-1',
        runtimeProjectId: '',
        routeSyncedConversationId: '',
      ),
      isFalse,
    );
  });

  test('切换会话标识时重新加载', () {
    expect(
      shouldApplyConversationRoute(
        previousConversationId: 'conversation-1',
        nextConversationId: 'conversation-2',
        previousProjectId: '',
        nextProjectId: '',
        runtimeConversationId: 'conversation-1',
        runtimeProjectId: '',
        routeSyncedConversationId: '',
      ),
      isTrue,
    );
  });

  test('空白路由在运行时仍有会话时准备新对话', () {
    expect(
      shouldApplyConversationRoute(
        previousConversationId: '',
        nextConversationId: '',
        previousProjectId: '',
        nextProjectId: '',
        runtimeConversationId: 'conversation-1',
        runtimeProjectId: '',
        routeSyncedConversationId: '',
      ),
      isTrue,
    );
  });

  test('已同步的创建会话路由不重复打开', () {
    expect(
      shouldApplyConversationRoute(
        previousConversationId: '',
        nextConversationId: 'conversation-1',
        previousProjectId: '',
        nextProjectId: '',
        runtimeConversationId: 'conversation-1',
        runtimeProjectId: '',
        routeSyncedConversationId: 'conversation-1',
      ),
      isFalse,
    );
  });
}
