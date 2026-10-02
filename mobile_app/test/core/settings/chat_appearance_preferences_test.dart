import 'package:amitia_app/core/settings/chat_appearance_preferences.dart';
import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/features/settings/presentation/widgets/chat_appearance_settings.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  test('defaults to flow and restores the exclusive saved style', () async {
    SharedPreferences.setMockInitialValues({});
    final notifier = ChatAppearancePreferencesNotifier();
    await notifier.init();
    expect(notifier.state, ChatMessageStyle.flow);
    await notifier.setMessageStyle(ChatMessageStyle.bubble);
    final restored = ChatAppearancePreferencesNotifier();
    await restored.init();
    expect(restored.state, ChatMessageStyle.bubble);
    await restored.setMessageStyle(ChatMessageStyle.flow);
    expect(
      (await SharedPreferences.getInstance()).getString(
        chatMessageStyleStorageKey,
      ),
      'flow',
    );
    notifier.dispose();
    restored.dispose();
  });

  testWidgets('settings changes the shared preference', (tester) async {
    SharedPreferences.setMockInitialValues({});
    final container = ProviderContainer();
    addTearDown(container.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(
          home: Scaffold(body: ChatAppearanceSettings()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('气泡消息'));
    await tester.pumpAndSettle();
    expect(
      container.read(chatAppearancePreferencesProvider),
      ChatMessageStyle.bubble,
    );
    await tester.tap(find.text('流式消息'));
    await tester.pumpAndSettle();
    expect(
      container.read(chatAppearancePreferencesProvider),
      ChatMessageStyle.flow,
    );
  });

  for (final brightness in Brightness.values) {
    testWidgets(
      'bubble messages retain rendering and actions at narrow width $brightness',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(360, 800));
        addTearDown(() => tester.binding.setSurfaceSize(null));
        await tester.pumpWidget(
          MaterialApp(
            theme: ThemeData(brightness: brightness),
            home: Scaffold(
              body: ListView(
                children: [
                  for (final role in [
                    MessageRole.user,
                    MessageRole.assistant,
                    MessageRole.system,
                  ])
                    AmitiaMessageBubble(
                      messageStyle: ChatMessageStyle.bubble,
                      showAvatar: false,
                      showHeader: false,
                      characterName: '测试角色',
                      message: ChatMessage(
                        id: role.name,
                        role: role,
                        type: MessageType.text,
                        content: '这是${role.name}消息，包含足够长的内容以检查窄屏换行与布局。',
                        time: DateTime(2026, 10, 2),
                      ),
                      onReply: () {},
                    ),
                ],
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('测试角色'), findsOneWidget);
        expect(find.byType(AmitiaMessageView), findsNWidgets(2));
        expect(tester.takeException(), isNull);
        await tester.longPress(find.textContaining('这是user消息'));
        await tester.pumpAndSettle();
        expect(find.text('引用'), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  }
}
