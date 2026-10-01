import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_theme.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('失败工具按调用去重计数', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-1',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-1',
            sequence: 1,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'call-2',
            turnId: 'turn-1',
            sequence: 2,
            type: 'tool_call',
            status: 'failed',
            callId: 'call-2',
            toolName: 'write_file',
          ),
          AssistantTurnItemDto(
            id: 'call-3',
            turnId: 'turn-1',
            sequence: 3,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-3',
            toolName: 'calculate',
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-1',
            sequence: 4,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'result-2',
            turnId: 'turn-1',
            sequence: 5,
            type: 'tool_result',
            status: 'failed',
            callId: 'call-2',
            toolName: 'write_file',
          ),
          AssistantTurnItemDto(
            id: 'result-3',
            turnId: 'turn-1',
            sequence: 6,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-3',
            toolName: 'calculate',
          ),
        ],
      ),
    );

    expect(find.text('3 个工具 · 1 个失败'), findsOneWidget);
  });

  testWidgets('执行中完成数只统计工具调用', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-2',
        conversationId: 'conversation-1',
        status: 'running',
        items: [
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-2',
            sequence: 1,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'call-2',
            turnId: 'turn-2',
            sequence: 2,
            type: 'tool_call',
            status: 'running',
            callId: 'call-2',
            toolName: 'write_file',
          ),
          AssistantTurnItemDto(
            id: 'call-3',
            turnId: 'turn-2',
            sequence: 3,
            type: 'tool_call',
            status: 'running',
            callId: 'call-3',
            toolName: 'calculate',
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-2',
            sequence: 4,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
        ],
      ),
    );

    expect(find.text('执行中 · 1/3 完成'), findsOneWidget);
  });

  testWidgets('旧工具结果按调用去重计数', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-3',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-3',
            sequence: 1,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'result-1-copy',
            turnId: 'turn-3',
            sequence: 2,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'result-2',
            turnId: 'turn-3',
            sequence: 3,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-2',
            toolName: 'calculate',
          ),
        ],
      ),
    );

    expect(find.text('2 个工具 · 已完成'), findsOneWidget);
  });

  testWidgets('调用结果不单独渲染为工具条目', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-tool-details',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-tool-details',
            sequence: 1,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName:
                'read_file_with_a_very_long_tool_name_that_must_be_trimmed',
            argumentsJson: '{"path":"a.txt"}',
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-tool-details',
            sequence: 2,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName:
                'read_file_with_a_very_long_tool_name_that_must_be_trimmed',
            resultJson: '{"content":"ok"}',
          ),
          AssistantTurnItemDto(
            id: 'call-2',
            turnId: 'turn-tool-details',
            sequence: 3,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-2',
            toolName: 'write_file',
            argumentsJson: '{"path":"b.txt"}',
          ),
          AssistantTurnItemDto(
            id: 'result-2',
            turnId: 'turn-tool-details',
            sequence: 4,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-2',
            toolName: 'write_file',
            resultJson: '{"written":true}',
          ),
        ],
      ),
    );

    await tester.tap(find.byKey(const ValueKey('tool-stream-toggle')));
    await tester.pumpAndSettle();

    final toolName = tester.widget<Text>(
      find.text('read_file_with_a_very_long_tool_name_that_must_be_trimmed'),
    );
    expect(toolName.maxLines, 1);
    expect(toolName.overflow, TextOverflow.ellipsis);
    expect(find.text('write_file'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('tool-call-expand-call-1')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('tool-call-expand-call-2')),
      findsOneWidget,
    );
    expect(find.text('展开'), findsNWidgets(2));
    expect(find.text('调用结果'), findsNothing);

    await tester.tap(find.byKey(const ValueKey('tool-call-expand-call-1')));
    await tester.pumpAndSettle();

    expect(find.text('调用工具'), findsOneWidget);
    expect(find.text('调用参数'), findsOneWidget);
    expect(find.text('调用结果'), findsOneWidget);
    expect(find.textContaining('"path": "a.txt"'), findsOneWidget);
    expect(find.textContaining('"content": "ok"'), findsOneWidget);
  });

  testWidgets('工具执行流上下间距一致', (tester) async {
    await tester.binding.setSurfaceSize(const Size(600, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-4',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'reasoning-1',
            turnId: 'turn-4',
            sequence: 1,
            type: 'reasoning',
            status: 'completed',
            content: '第一段思考',
            durationMs: 1000,
          ),
          AssistantTurnItemDto(
            id: 'text-empty',
            turnId: 'turn-4',
            sequence: 2,
            type: 'text',
            status: 'completed',
            content: '   ',
          ),
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-4',
            sequence: 3,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-4',
            sequence: 4,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
          ),
          AssistantTurnItemDto(
            id: 'reasoning-2',
            turnId: 'turn-4',
            sequence: 5,
            type: 'reasoning',
            status: 'completed',
            content: '第二段思考',
            durationMs: 2000,
          ),
        ],
      ),
    );

    final thinkingSummary = find.byWidgetPredicate(
      (widget) =>
          widget is Container &&
          widget.padding ==
              const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
    );
    final firstThinking = tester.getRect(thinkingSummary.at(0));
    final toolContainer = tester.getRect(
      find.byKey(const ValueKey('tool-stream-container')),
    );
    final secondThinking = tester.getRect(thinkingSummary.at(1));

    expect(toolContainer.top - firstThinking.bottom, closeTo(8, 0.01));
    expect(secondThinking.top - toolContainer.bottom, closeTo(8, 0.01));
  });
}

Future<void> _pumpTurn(WidgetTester tester, AssistantTurnDto turn) async {
  final message = ChatMessage(
    id: 'assistant-${turn.id}',
    role: MessageRole.assistant,
    type: MessageType.text,
    content: '',
    time: DateTime(2026, 9, 28),
    status: turn.status == 'completed'
        ? MessageStatus.delivered
        : MessageStatus.streaming,
    assistantTurn: turn,
  );
  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(extensions: [AmitiaMessageTheme.light]),
      home: Scaffold(body: AmitiaMessageView(message: message)),
    ),
  );
}
