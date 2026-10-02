import 'package:amitia_app/core/settings/chat_appearance_preferences.dart';
import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:amitia_app/core/widgets/character_avatar.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/features/settings/presentation/widgets/chat_appearance_settings.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  test('AI identity defaults on and restores independent toggles', () async {
    SharedPreferences.setMockInitialValues({});
    final avatar = AiAvatarPreferencesNotifier();
    final name = AiNamePreferencesNotifier();
    await avatar.init();
    await name.init();
    expect(avatar.state, true);
    expect(name.state, true);
    await avatar.setEnabled(false);
    expect(name.state, true);
    await name.setEnabled(false);
    final restoredAvatar = AiAvatarPreferencesNotifier();
    final restoredName = AiNamePreferencesNotifier();
    await restoredAvatar.init();
    await restoredName.init();
    expect(restoredAvatar.state, false);
    expect(restoredName.state, false);
    avatar.dispose();
    name.dispose();
    restoredAvatar.dispose();
    restoredName.dispose();
  });

  testWidgets(
    'each split AI bubble shows the independently selected identity',
    (tester) async {
      for (final avatar in [true, false]) {
        for (final name in [true, false]) {
          await tester.pumpWidget(
            MaterialApp(
              home: Scaffold(
                body: AmitiaMessageBubble(
                  messageStyle: ChatMessageStyle.bubble,
                  aiAvatarEnabled: avatar,
                  aiNameEnabled: name,
                  showAvatar: false,
                  showHeader: false,
                  characterName: '测试角色',
                  message: ChatMessage(
                    id: 'a',
                    role: MessageRole.assistant,
                    type: MessageType.text,
                    content: '第一段[AMITIA_BR]第二段',
                    time: DateTime(2026, 10, 2),
                  ),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();
          expect(
            find.byType(CharacterAvatar),
            avatar ? findsNWidgets(2) : findsNothing,
          );
          expect(find.text('测试角色'), name ? findsNWidgets(2) : findsNothing);
          for (final content in ['第一段', '第二段']) {
            expect(find.text(content, findRichText: true), findsOneWidget);
            if (avatar) {
              final index = content == '第一段' ? 0 : 1;
              final avatarRect = tester.getRect(
                find.byType(CharacterAvatar).at(index),
              );
              final contentRect = tester.getRect(
                find.text(content, findRichText: true),
              );
              expect(avatarRect.right, lessThan(contentRect.left));
              expect(avatarRect.top, lessThanOrEqualTo(contentRect.top));
              if (name) {
                expect(
                  tester.getRect(find.text('测试角色').at(index)).left,
                  greaterThan(avatarRect.right),
                );
              }
            }
          }
          expect(tester.takeException(), isNull);
        }
      }
    },
  );
  test(
    'glass defaults off and restores independently of message style',
    () async {
      SharedPreferences.setMockInitialValues({});
      final notifier = UserMessageMaterialNotifier();
      await notifier.init();
      expect(notifier.state, UserMessageMaterial.solid);
      await notifier.setMaterial(UserMessageMaterial.frosted);
      final restored = UserMessageMaterialNotifier();
      await restored.init();
      expect(restored.state, UserMessageMaterial.frosted);
      final style = ChatAppearancePreferencesNotifier();
      await style.init();
      expect(style.state, ChatMessageStyle.flow);
      await restored.setMaterial(UserMessageMaterial.water);
      final waterRestored = UserMessageMaterialNotifier();
      await waterRestored.init();
      expect(waterRestored.state, UserMessageMaterial.water);
      waterRestored.dispose();
      await restored.setMaterial(UserMessageMaterial.solid);
      expect(
        (await SharedPreferences.getInstance()).getString(
          userMessageMaterialStorageKey,
        ),
        'solid',
      );
      notifier.dispose();
      restored.dispose();
      style.dispose();
    },
  );
  test(
    'restores legacy glass but new solid overrides the legacy value',
    () async {
      SharedPreferences.setMockInitialValues({
        userMessageGlassStorageKey: true,
      });
      final notifier = UserMessageMaterialNotifier();
      await notifier.init();
      expect(notifier.state, UserMessageMaterial.frosted);
      await notifier.setMaterial(UserMessageMaterial.solid);
      final restored = UserMessageMaterialNotifier();
      await restored.init();
      expect(restored.state, UserMessageMaterial.solid);
      notifier.dispose();
      restored.dispose();
    },
  );
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
    await tester.tap(find.text('显示 AI 头像'));
    await tester.pumpAndSettle();
    expect(container.read(aiAvatarPreferencesProvider), false);
    expect(container.read(aiNamePreferencesProvider), true);
    await tester.tap(find.text('显示 AI 名称'));
    await tester.pumpAndSettle();
    expect(container.read(aiNamePreferencesProvider), false);
    await tester.tap(find.text('用户消息磨砂玻璃'));
    await tester.pumpAndSettle();
    expect(
      container.read(userMessageMaterialProvider),
      UserMessageMaterial.frosted,
    );
    await tester.tap(find.text('用户消息水玻璃'));
    await tester.pumpAndSettle();
    expect(
      container.read(userMessageMaterialProvider),
      UserMessageMaterial.water,
    );
    expect(
      tester
          .widget<SwitchListTile>(
            find.widgetWithText(SwitchListTile, '用户消息磨砂玻璃'),
          )
          .value,
      false,
    );
    await tester.tap(find.text('用户消息水玻璃'));
    await tester.pumpAndSettle();
    expect(
      container.read(userMessageMaterialProvider),
      UserMessageMaterial.solid,
    );
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
        expect(find.byType(AmitiaMessageView), findsNWidgets(3));
        expect(tester.takeException(), isNull);
        await tester.longPress(find.textContaining('这是user消息'));
        await tester.pumpAndSettle();
        expect(find.text('引用'), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  }
}
