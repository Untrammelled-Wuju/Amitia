import 'package:amitia_app/core/widgets/amitia_message.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_theme.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/features/conversation/rendering/amrp.dart';
import 'package:amitia_app/features/conversation/rendering/rich_blocks/amitia_rich_blocks.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('user message actions open from long press menu', (tester) async {
    var edited = false;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AmitiaMessageBubble(
            message: ChatMessage(
              id: 'user-1',
              role: MessageRole.user,
              type: MessageType.text,
              content: '你好',
              time: DateTime(2026, 9, 20, 12, 34),
            ),
            onReply: () {},
            onCopy: () {},
            onEdit: () => edited = true,
          ),
        ),
      ),
    );

    expect(find.text('12:34'), findsNothing);

    await tester.longPress(find.text('你好'));
    await tester.pumpAndSettle();

    expect(find.text('显示时间'), findsOneWidget);
    expect(find.text('引用'), findsOneWidget);
    expect(find.text('复制'), findsOneWidget);
    expect(find.text('修改'), findsOneWidget);

    await tester.tap(find.text('修改'));
    await tester.pumpAndSettle();

    expect(edited, isTrue);
  });

  testWidgets('assistant message copy action selects copy mode', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AmitiaMessageView(
            message: ChatMessage(
              id: 'assistant-1',
              role: MessageRole.assistant,
              type: MessageType.text,
              content: '你好',
              time: DateTime(2026, 9, 20, 12, 35),
            ),
            onReply: () {},
          ),
        ),
      ),
    );

    expect(find.text('复制'), findsNothing);
    expect(find.text('复制 Markdown'), findsNothing);
    expect(find.text('回复'), findsNothing);
    expect(find.byIcon(Icons.refresh_rounded), findsNothing);

    await tester.tap(find.byIcon(Icons.copy_outlined));
    await tester.pumpAndSettle();

    expect(find.text('复制纯文本'), findsOneWidget);
    expect(find.text('复制 Markdown'), findsOneWidget);

    final copyRect = tester.getRect(find.byIcon(Icons.copy_outlined));
    final replyRect = tester.getRect(find.byIcon(Icons.format_quote_rounded));
    expect(copyRect.width, 15);
    expect(replyRect.left - copyRect.left, lessThanOrEqualTo(40));
  });

  testWidgets('assistant streaming hides copy and reply actions', (
    tester,
  ) async {
    final message = ChatMessage(
      id: 'assistant-streaming',
      role: MessageRole.assistant,
      type: MessageType.text,
      content: '正在生成',
      time: DateTime(2026, 9, 20, 12, 36),
      status: MessageStatus.streaming,
    );
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AmitiaMessageView(message: message, onReply: () {}),
        ),
      ),
    );

    expect(find.byIcon(Icons.copy_outlined), findsNothing);
    expect(find.byIcon(Icons.format_quote_rounded), findsNothing);
  });

  testWidgets('assistant continuation hides header and uses compact spacing', (
    tester,
  ) async {
    final message = ChatMessage(
      id: 'assistant-continuation',
      characterId: 'character-1',
      role: MessageRole.assistant,
      type: MessageType.text,
      content: '第二条消息',
      time: DateTime(2026, 9, 20, 12, 36),
    );
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: AmitiaMessageBubble(
            message: message,
            showAvatar: false,
            showHeader: false,
            compactBottom: true,
          ),
        ),
      ),
    );

    expect(find.text('Amitia'), findsNothing);
    expect(find.text('12:36'), findsNothing);
    final view = tester.widget<AmitiaMessageView>(
      find.byType(AmitiaMessageView),
    );
    expect(view.characterId, 'character-1');
    final padding = tester.widget<Padding>(
      find
          .descendant(
            of: find.byType(AmitiaMessageBubble),
            matching: find.byType(Padding),
          )
          .first,
    );
    expect(padding.padding.resolve(TextDirection.ltr).bottom, 10);
  });

  testWidgets('thinking content expands naturally without copy action', (
    tester,
  ) async {
    final content = List<String>.generate(
      80,
      (index) => '思考内容第 ${index + 1} 行',
    ).join('\n');
    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(extensions: [AmitiaMessageTheme.light]),
        home: Scaffold(
          body: SingleChildScrollView(
            child: AmitiaThinkingBlock(
              block: AmrpThinkingBlock(
                content: content,
                state: AmrpMessageState.completed,
                duration: const Duration(milliseconds: 2850),
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.text('思考完成（2.9 秒）'), findsOneWidget);
    expect(find.byIcon(Icons.chevron_right_rounded), findsOneWidget);
    await tester.tap(find.text('思考完成（2.9 秒）'));
    await tester.pumpAndSettle();

    expect(find.text('复制'), findsNothing);
    expect(find.byType(Scrollbar), findsNothing);
    expect(
      tester.getSize(find.byType(SelectableText)).height,
      greaterThan(260),
    );
  });

  testWidgets('thinking placeholder cannot expand without content', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(extensions: [AmitiaMessageTheme.light]),
        home: Scaffold(
          body: AmitiaThinkingBlock(
            block: const AmrpThinkingBlock(
              content: '',
              state: AmrpMessageState.streaming,
            ),
          ),
        ),
      ),
    );

    expect(find.text('思考中'), findsOneWidget);
    expect(find.byIcon(Icons.chevron_right_rounded), findsNothing);
    await tester.tap(find.text('思考中'));
    await tester.pump();
    expect(find.byType(SelectableText), findsNothing);
  });
}
