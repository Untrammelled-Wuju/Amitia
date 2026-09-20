import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_theme.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('空助手回合仍显示思考完成和最终正文', (tester) async {
    final message = ChatMessage(
      id: 'assistant-1',
      role: MessageRole.assistant,
      type: MessageType.text,
      content: '最终回复',
      reasoningContent: '思考内容',
      reasoningDurationMs: 1200,
      time: DateTime(2026, 9, 20),
      status: MessageStatus.delivered,
      assistantTurn: const AssistantTurnDto(
        id: 'turn-1',
        conversationId: 'conversation-1',
        status: 'completed',
      ),
    );

    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(extensions: [AmitiaMessageTheme.light]),
        home: Scaffold(body: AmitiaMessageView(message: message)),
      ),
    );

    expect(find.text('思考完成（1.2 秒）'), findsOneWidget);
    expect(find.text('最终回复'), findsOneWidget);

    await tester.tap(find.text('思考完成（1.2 秒）'));
    await tester.pumpAndSettle();

    expect(find.text('思考内容'), findsOneWidget);
  });

  testWidgets('运行中的空助手回合显示正在思考', (tester) async {
    final message = ChatMessage(
      id: 'assistant-1',
      role: MessageRole.assistant,
      type: MessageType.text,
      content: '',
      time: DateTime(2026, 9, 20),
      status: MessageStatus.streaming,
      assistantTurn: const AssistantTurnDto(
        id: 'turn-1',
        conversationId: 'conversation-1',
        status: 'running',
      ),
    );

    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(extensions: [AmitiaMessageTheme.light]),
        home: Scaffold(body: AmitiaMessageView(message: message)),
      ),
    );

    expect(find.text('思考中'), findsOneWidget);
  });
}
