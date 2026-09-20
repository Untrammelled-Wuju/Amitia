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
          ),
        ),
      ),
    );

    expect(find.text('复制'), findsNothing);
    expect(find.text('复制 Markdown'), findsNothing);
    expect(find.text('回复'), findsNothing);
    expect(find.text('重新生成'), findsNothing);

    await tester.tap(find.byIcon(Icons.copy_outlined));
    await tester.pumpAndSettle();

    expect(find.text('复制纯文本'), findsOneWidget);
    expect(find.text('复制 Markdown'), findsOneWidget);
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

    expect(find.text('思考完成（2.85s）'), findsOneWidget);
    await tester.tap(find.text('思考完成（2.85s）'));
    await tester.pumpAndSettle();

    expect(find.text('复制'), findsNothing);
    expect(find.byType(Scrollbar), findsNothing);
    expect(
      tester.getSize(find.byType(SelectableText)).height,
      greaterThan(260),
    );
  });
}
