import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:amitia_app/core/settings/chat_appearance_preferences.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final style in ChatMessageStyle.values) {
    for (final type in [
      MessageType.text,
      MessageType.file,
      MessageType.audio,
      MessageType.video,
      MessageType.image,
    ]) {
      testWidgets('$style $type glass is clipped, reversible and user only', (
        tester,
      ) async {
        Future<void> render(
          UserMessageMaterial material,
          MessageRole role,
        ) async {
          await tester.pumpWidget(
            MaterialApp(
              home: Scaffold(
                body: AmitiaMessageBubble(
                  messageStyle: style,
                  userMessageMaterial: material,
                  showAvatar: false,
                  showHeader: false,
                  message: ChatMessage(
                    id: 'test',
                    role: role,
                    type: type,
                    content: '消息',
                    fileName: '附件',
                    time: DateTime(2026, 10, 2),
                  ),
                ),
              ),
            ),
          );
          await tester.pump();
        }

        await render(UserMessageMaterial.solid, MessageRole.user);
        expect(find.byType(BackdropFilter), findsNothing);
        await render(UserMessageMaterial.frosted, MessageRole.user);
        expect(find.byType(BackdropFilter), findsWidgets);
        expect(
          find.ancestor(
            of: find.byType(BackdropFilter).first,
            matching: find.byType(ClipRRect),
          ),
          findsWidgets,
        );
        expect(tester.takeException(), isNull);
        await render(UserMessageMaterial.water, MessageRole.user);
        expect(find.byType(BackdropFilter), findsWidgets);
        expect(
          find.ancestor(
            of: find.byType(BackdropFilter).first,
            matching: find.byType(ClipRRect),
          ),
          findsWidgets,
        );
        final surface = tester.widget<Container>(
          find
              .descendant(
                of: find.byType(BackdropFilter).first,
                matching: find.byType(Container),
              )
              .first,
        );
        expect((surface.decoration as BoxDecoration).gradient, isNotNull);
        expect(
          (surface.decoration as BoxDecoration).color!.a,
          closeTo(0.60, 0.01),
        );
        await render(UserMessageMaterial.solid, MessageRole.user);
        expect(find.byType(BackdropFilter), findsNothing);
        await render(UserMessageMaterial.water, MessageRole.assistant);
        expect(find.byType(BackdropFilter), findsNothing);
        expect(tester.takeException(), isNull);
      });
    }
  }
}
