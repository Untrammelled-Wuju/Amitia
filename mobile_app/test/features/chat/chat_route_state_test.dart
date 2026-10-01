import 'package:amitia_app/features/chat/presentation/chat_route_state.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
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
