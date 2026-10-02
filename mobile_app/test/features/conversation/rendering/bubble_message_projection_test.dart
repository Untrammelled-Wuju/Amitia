import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/settings/chat_appearance_preferences.dart';
import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:amitia_app/features/conversation/rendering/amrp.dart';
import 'package:amitia_app/features/conversation/rendering/bubble_message_projection.dart';
import 'package:amitia_app/features/conversation/rendering/markdown/amitia_markdown.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

ChatMessage message(
  String content, {
  MessageRole role = MessageRole.assistant,
  MessageStatus status = MessageStatus.sent,
  MessageType type = MessageType.text,
  AssistantTurnDto? turn,
}) => ChatMessage(
  id: 'm1',
  role: role,
  type: type,
  content: content,
  status: status,
  time: DateTime(2026, 10, 2),
  assistantTurn: turn,
  mediaUrl: type == MessageType.text ? null : '/asset',
  resourceUri: '/asset',
);

void main() {
  test(
    'buffers incomplete segments and flushes completion or interruption',
    () {
      expect(splitBubbleText('第一段[AMI', complete: false), isEmpty);
      expect(splitBubbleText('第一段[AMITIA_BR]第二段', complete: false), ['第一段']);
      expect(splitBubbleText('第一段[AMITIA_BR]第二段', complete: true), [
        '第一段',
        '第二段',
      ]);
      expect(
        splitBubbleText('[AMITIA_BR]第一段[AMITIA_BR][AMITIA_BR]', complete: true),
        ['第一段'],
      );
      expect(splitBubbleText('中断的尾段[AMITI', complete: true), ['中断的尾段']);
      final interrupted = projectBubbleContent(
        message: AmrpMessage.fromChatMessage(
          message('剩余消息', status: MessageStatus.interrupted),
        ),
      );
      expect((interrupted.single as BubbleText).content, '剩余消息');
    },
  );

  test('preserves fenced and inline code delimiter literals', () {
    const content =
        '`[AMITIA_BR]`\n```js\nconst a = "[AMITIA_BR]";\n```[AMITIA_BR]结束';
    expect(splitBubbleText(content, complete: true), [
      content.substring(0, content.lastIndexOf('[AMITIA_BR]')),
      '结束',
    ]);
  });

  test(
    'orders independent thinking, paired tools and completed text with stable keys',
    () {
      final turn = AssistantTurnDto(
        id: 't1',
        status: 'running',
        items: const [
          AssistantTurnItemDto(
            id: 'r1',
            type: 'reasoning',
            sequence: 1,
            status: 'running',
            content: '不逐字展示',
          ),
          AssistantTurnItemDto(
            id: 'c1',
            type: 'tool_call',
            sequence: 2,
            status: 'running',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'x1',
            type: 'text',
            sequence: 3,
            status: 'running',
            content: '已读取[AMITIA_BR]未完成',
          ),
          AssistantTurnItemDto(
            id: 'result1',
            type: 'tool_result',
            sequence: 4,
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
        ],
      );
      final normalized = AmrpMessage.fromChatMessage(
        message('', status: MessageStatus.streaming),
      );
      final projected = projectBubbleContent(message: normalized, turn: turn);
      expect(projected.map((item) => item.runtimeType).toList(), [
        BubbleThinking,
        BubbleTools,
        BubbleText,
      ]);
      expect((projected.first as BubbleThinking).block.content, isEmpty);
      expect((projected[1] as BubbleTools).items, hasLength(2));
      expect(
        projectBubbleContent(
          message: normalized,
          turn: turn,
        ).map((item) => item.key),
        projected.map((item) => item.key),
      );
      final completed = projectBubbleContent(
        message: normalized,
        turn: AssistantTurnDto(
          id: 't1',
          status: 'interrupted',
          items: turn.items,
        ),
      );
      expect((completed.last as BubbleText).content, '未完成');
    },
  );

  for (final role in [MessageRole.user, MessageRole.assistant]) {
    test(
      '$role media projects dedicated audio, image, video and file bubbles',
      () {
        final types = <Type>[];
        for (final type in [
          MessageType.audio,
          MessageType.image,
          MessageType.video,
          MessageType.file,
        ]) {
          final projected = projectBubbleContent(
            message: AmrpMessage.fromChatMessage(
              message('', role: role, type: type),
            ),
          );
          types.add((projected.single as BubbleRichContent).block.runtimeType);
        }
        expect(types, [
          AmrpAudioBlock,
          AmrpImageBlock,
          AmrpVideoBlock,
          AmrpFileBlock,
        ]);
      },
    );
  }

  testWidgets(
    'shows complete bubbles without token streaming, then releases the final segment',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(360, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      Future<void> pump(MessageStatus status) => tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ListView(
              children: [
                AmitiaMessageBubble(
                  messageStyle: ChatMessageStyle.bubble,
                  message: message('已完成[AMITIA_BR]暂不显示', status: status),
                ),
              ],
            ),
          ),
        ),
      );
      await pump(MessageStatus.streaming);
      await tester.pump();
      var markdown = tester
          .widgetList<AmitiaMarkdownView>(find.byType(AmitiaMarkdownView))
          .toList();
      expect(markdown.map((view) => view.source), ['已完成']);
      expect(markdown.every((view) => !view.streaming), isTrue);
      expect(find.text('正在回复…'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await pump(MessageStatus.sent);
      await tester.pumpAndSettle();
      markdown = tester
          .widgetList<AmitiaMarkdownView>(find.byType(AmitiaMarkdownView))
          .toList();
      expect(markdown.map((view) => view.source), ['已完成', '暂不显示']);
      expect(find.text('正在回复…'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );
}
