import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_theme.dart';
import 'package:amitia_app/features/conversation/rendering/amitia_message_view.dart';
import 'package:amitia_app/features/conversation/rendering/rich_blocks/amitia_rich_blocks.dart';
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

  testWidgets('思考、工具执行流和最终正文按事件顺序排列', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-order',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'text-1',
            turnId: 'turn-order',
            sequence: 4,
            type: 'text',
            status: 'completed',
            content: '最终回复',
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-order',
            sequence: 3,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
            resultJson: '"ok"',
          ),
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-order',
            sequence: 2,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'read_file',
            argumentsJson: '{"path":"a.go"}',
            durationMs: 1200,
          ),
          AssistantTurnItemDto(
            id: 'thinking-1',
            turnId: 'turn-order',
            sequence: 1,
            type: 'reasoning',
            status: 'completed',
            content: '分析',
            durationMs: 1200,
          ),
        ],
      ),
    );

    final thinking = tester.getTopLeft(find.byType(AmitiaThinkingBlock).first);
    final tools = tester.getTopLeft(
      find.byKey(const ValueKey('tool-stream-container')),
    );
    final finalText = tester.getTopLeft(find.text('最终回复'));
    expect(thinking.dy, lessThan(tools.dy));
    expect(tools.dy, lessThan(finalText.dy));
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

    expect(find.text('修改文件 · 1/3 完成'), findsOneWidget);
  });

  testWidgets('工具行显示动作和持续时间', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-duration',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-duration',
            sequence: 1,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'workspace.read',
            argumentsJson: '{"path":"main.go"}',
            durationMs: 1234,
          ),
        ],
      ),
    );
    await tester.tap(find.byKey(const ValueKey('tool-stream-toggle')));
    await tester.pumpAndSettle();
    expect(find.text('读取文件'), findsOneWidget);
    expect(find.text('完成 · 1.2s'), findsOneWidget);
  });

  testWidgets('纯文本工具错误按原文展示且不当作JSON解析', (tester) async {
    const message =
        'capability: no available provider: no executable instance for workspace.write';
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-plain-result',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'call-plain',
            turnId: 'turn-plain-result',
            sequence: 1,
            type: 'tool_call',
            status: 'failed',
            callId: 'call-plain',
            toolName: 'workspace.write',
          ),
          AssistantTurnItemDto(
            id: 'result-plain',
            turnId: 'turn-plain-result',
            sequence: 2,
            type: 'tool_result',
            status: 'failed',
            callId: 'call-plain',
            toolName: 'workspace.write',
            resultJson: message,
          ),
        ],
      ),
    );
    await tester.tap(find.byKey(const ValueKey('tool-stream-toggle')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('tool-call-expand-call-plain')));
    await tester.pumpAndSettle();
    expect(find.text(message), findsOneWidget);
  });

  testWidgets('重启后待对账的工具不显示为运行中', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-reconcile',
        conversationId: 'conversation-1',
        status: 'needs_reconciliation',
        items: [
          AssistantTurnItemDto(
            id: 'call-reconcile',
            turnId: 'turn-reconcile',
            sequence: 1,
            type: 'tool_call',
            status: 'needs_reconciliation',
            callId: 'call-reconcile',
            toolName: 'workspace.write',
          ),
        ],
      ),
    );
    expect(find.text('1 个工具 · 1 个待确认恢复'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('tool-stream-toggle')));
    await tester.pumpAndSettle();
    expect(find.text('待确认恢复'), findsOneWidget);
    expect(find.text('运行中'), findsNothing);
  });

  testWidgets('调用状态未更新时采用结果终态', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-result-state',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-result-state',
            sequence: 1,
            type: 'tool_call',
            status: 'running',
            callId: 'call-1',
            toolName: 'android.display.status',
            durationMs: 22,
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-result-state',
            sequence: 2,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'android.display.status',
            resultJson: '{"state":"ready"}',
          ),
        ],
      ),
    );
    await tester.tap(find.byKey(const ValueKey('tool-stream-toggle')));
    await tester.pumpAndSettle();

    expect(find.text('完成 · 22ms'), findsOneWidget);
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
    await tester.binding.setSurfaceSize(const Size(1000, 1200));
    addTearDown(() => tester.binding.setSurfaceSize(null));
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
            toolName: 'unknown_tool_with_a_very_long_name_that_must_be_trimmed',
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
      find.text('unknown_tool_with_a_very_long_name_that_must_be_trimmed'),
    );
    expect(toolName.maxLines, 1);
    expect(toolName.overflow, TextOverflow.ellipsis);
    expect(find.text('修改文件'), findsOneWidget);
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
    await tester.tap(find.byKey(const ValueKey('tool-call-expand-call-2')));
    await tester.pumpAndSettle();
    expect(find.text('write_file'), findsOneWidget);
  });

  testWidgets('只有工具调用记录时仍展示调用结果', (tester) async {
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-call-result',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-call-result',
            sequence: 1,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'android.display.status',
            resultJson: '{"state":"ready"}',
          ),
        ],
      ),
    );
    await tester.tap(find.byKey(const ValueKey('tool-stream-toggle')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('tool-call-expand-call-1')));
    await tester.pumpAndSettle();

    expect(find.textContaining('"state": "ready"'), findsOneWidget);
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

  testWidgets('相同序号时执行过程保持在最终回复上方', (tester) async {
    await tester.binding.setSurfaceSize(const Size(600, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-order',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-order',
            sequence: 0,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'android.display.status',
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-order',
            sequence: 0,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'android.display.status',
            resultJson: '{"state":"ready"}',
          ),
          AssistantTurnItemDto(
            id: 'text-1',
            turnId: 'turn-order',
            sequence: 0,
            type: 'text',
            status: 'completed',
            content: '最终回复',
          ),
        ],
      ),
    );

    final toolRect = tester.getRect(
      find.byKey(const ValueKey('tool-stream-container')),
    );
    final finalTextRect = tester.getRect(find.text('最终回复'));
    expect(toolRect.top, lessThan(finalTextRect.top));
  });

  testWidgets('旧数据顺序颠倒时最终回复仍排在执行过程之后', (tester) async {
    await tester.binding.setSurfaceSize(const Size(600, 1000));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await _pumpTurn(
      tester,
      const AssistantTurnDto(
        id: 'turn-order-legacy',
        conversationId: 'conversation-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'text-1',
            turnId: 'turn-order-legacy',
            sequence: 1,
            type: 'text',
            status: 'completed',
            content: '最终回复',
          ),
          AssistantTurnItemDto(
            id: 'call-1',
            turnId: 'turn-order-legacy',
            sequence: 2,
            type: 'tool_call',
            status: 'completed',
            callId: 'call-1',
            toolName: 'android.display.status',
          ),
          AssistantTurnItemDto(
            id: 'result-1',
            turnId: 'turn-order-legacy',
            sequence: 3,
            type: 'tool_result',
            status: 'completed',
            callId: 'call-1',
            toolName: 'android.display.status',
            resultJson: '{"state":"ready"}',
          ),
        ],
      ),
    );

    final toolRect = tester.getRect(
      find.byKey(const ValueKey('tool-stream-container')),
    );
    final finalTextRect = tester.getRect(find.text('最终回复'));
    expect(toolRect.top, lessThan(finalTextRect.top));
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
