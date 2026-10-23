import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/chat/runtime/agent_event_reducer.dart';
import 'package:flutter_test/flutter_test.dart';

const citation = <String, dynamic>{
  'marker': '[K1]',
  'kind': 'knowledge',
  'index': 1,
  'sourceId': 'source-a',
};

AssistantTurnDto historic(
  String status,
  int revision, {
  String executionId = 'execution-a',
  String updatedAt = '',
}) => AssistantTurnDto(
  id: 'turn-a',
  conversationId: 'conversation-a',
  requestId: 'request-a',
  executionId: executionId,
  sequence: 1,
  status: status,
  updatedAt: updatedAt,
  items: <AssistantTurnItemDto>[
    AssistantTurnItemDto(
      id: 'text-a',
      turnId: 'turn-a',
      conversationId: 'conversation-a',
      sequence: 1,
      type: 'text',
      revision: revision,
      status: 'completed',
      messageId: 'message-a',
      isFinal: true,
      content: '可靠引用 [K1]',
      citationSnapshot: const <Map<String, dynamic>>[citation],
    ),
  ],
);

AgentUIEvent event(
  String type,
  int sequence, {
  String blockId = '',
  int revision = 0,
  Map<String, dynamic> payload = const <String, dynamic>{},
}) {
  return AgentUIEvent(
    version: 1,
    eventId: 'e-$sequence',
    eventSequence: sequence,
    conversationId: 'conversation-a',
    requestId: 'request-a',
    executionId: 'execution-a',
    turnId: 'turn-a',
    turnSequence: 1,
    blockId: blockId,
    blockSequence: 1,
    messageId: 'message-a',
    messageSequence: 1,
    callId: '',
    revision: revision,
    type: type,
    status: '',
    payload: payload,
    createdAt: '2026-10-16T10:00:00Z',
  );
}

void main() {
  test('恢复完成快照替换中断，旧中断快照不能回退状态', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: [historic('interrupted', 5)], lastEventSequence: 5);
    reducer.mergeTurns([historic('completed', 6)]);
    expect(reducer.turn('turn-a')!.status, 'completed');
    reducer.mergeTurns([historic('interrupted', 5)]);
    expect(reducer.turn('turn-a')!.status, 'completed');
  });
  test('同一执行保存完成事件清除中断，迟到中断不回退', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: [historic('interrupted', 5)], lastEventSequence: 5);
    expect(
      reducer.apply(event('turn.completed', 6, payload: {'saved': true})),
      AgentEventApplyResult.applied,
    );
    expect(reducer.turn('turn-a')!.status, 'completed');
    expect(
      reducer.apply(event('turn.interrupted', 7)),
      AgentEventApplyResult.stale,
    );
  });
  test('旧完成快照和其他执行不能覆盖当前状态', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: [historic('interrupted', 5, updatedAt: '2026-10-16T10:02:00Z')],
      lastEventSequence: 5,
    );
    reducer.mergeTurns([
      historic('completed', 6, updatedAt: '2026-10-16T10:01:00Z'),
    ]);
    expect(reducer.turn('turn-a')!.status, 'interrupted');
    reducer.mergeTurns([historic('completed', 8, executionId: 'foreign')]);
    expect(reducer.turn('turn-a')!.status, 'interrupted');
    expect(reducer.turn('turn-a')!.items.single.revision, 6);
  });
  test('完成后迟到启动和审批事件不能重启任务', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('completed', 5)],
      lastEventSequence: 5,
    );
    expect(
      reducer.apply(event('turn.started', 6)),
      AgentEventApplyResult.stale,
    );
    expect(
      reducer.apply(event('approval.approved', 7)),
      AgentEventApplyResult.stale,
    );
    expect(reducer.turn('turn-a')!.status, 'completed');
    expect(reducer.activeTurnId, isEmpty);
  });

  test('失败后冲突终态及取消后的迟到增量均被拒绝', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('failed', 5)],
      lastEventSequence: 5,
    );
    expect(
      reducer.apply(event('turn.completed', 6)),
      AgentEventApplyResult.stale,
    );
    expect(reducer.turn('turn-a')!.status, 'failed');
    final cancelled = AgentEventReducer();
    cancelled.reset(
      turns: <AssistantTurnDto>[historic('interrupted', 5)],
      lastEventSequence: 5,
    );
    expect(
      cancelled.apply(
        event(
          'text.delta',
          6,
          blockId: 'text-a',
          revision: 6,
          payload: const <String, dynamic>{'delta': '错误增量'},
        ),
      ),
      AgentEventApplyResult.stale,
    );
    expect(cancelled.turn('turn-a')!.items.single.content, '可靠引用 [K1]');
  });

  test('取消后的迟到完成不能补写回复', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('interrupted', 5)],
      lastEventSequence: 5,
    );
    expect(
      reducer.apply(
        event(
          'text.completed',
          6,
          blockId: 'text-a',
          revision: 6,
          payload: const <String, dynamic>{'content': '取消后的内容'},
        ),
      ),
      AgentEventApplyResult.stale,
    );
    expect(reducer.turn('turn-a')!.items.single.content, '可靠引用 [K1]');
  });

  test('高修订的迟到 text.completed 仍可补齐引用快照', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('completed', 5)],
      lastEventSequence: 5,
    );
    expect(
      reducer.apply(
        event(
          'text.completed',
          6,
          blockId: 'text-a',
          revision: 6,
          payload: const <String, dynamic>{
            'content': '新引用 [K1]',
            'citationSnapshot': <Map<String, dynamic>>[citation],
          },
        ),
      ),
      AgentEventApplyResult.applied,
    );
    expect(reducer.turn('turn-a')!.status, 'completed');
    expect(
      reducer.turn('turn-a')!.items.single.citationSnapshot,
      <Map<String, dynamic>>[citation],
    );
  });

  test('旧历史终态覆盖运行状态，但不会降低正文修订号', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('running', 8)],
      lastEventSequence: 5,
    );
    reducer.mergeTurns(<AssistantTurnDto>[historic('completed', 5)]);
    expect(reducer.turn('turn-a')!.status, 'completed');
    expect(reducer.turn('turn-a')!.items.single.revision, 8);
  });

  test('权威历史关闭任务后，清除活动标记', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('running', 8)],
      lastEventSequence: 5,
    );
    expect(
      reducer.apply(event('turn.started', 6)),
      AgentEventApplyResult.applied,
    );
    expect(reducer.activeTurnId, 'turn-a');
    reducer.mergeTurns(<AssistantTurnDto>[historic('completed', 5)]);
    expect(reducer.turn('turn-a')!.status, 'completed');
    expect(reducer.activeTurnId, isEmpty);
  });

  test('新的运行历史不能复活已经终止的 Turn', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('interrupted', 5)],
      lastEventSequence: 5,
    );
    reducer.mergeTurns(<AssistantTurnDto>[historic('running', 9)]);
    expect(reducer.turn('turn-a')!.status, 'interrupted');
    expect(reducer.turn('turn-a')!.items.single.revision, 9);
  });

  test('旧活动快照不能复活历史完成任务', () {
    final reducer = AgentEventReducer();
    reducer.reset(
      turns: <AssistantTurnDto>[historic('completed', 5)],
      lastEventSequence: 5,
      activeTurn: <String, dynamic>{
        'turnId': 'turn-a',
        'status': 'running',
        'blocks': <Map<String, dynamic>>[
          <String, dynamic>{
            'blockId': 'text-a',
            'blockSequence': 1,
            'revision': 4,
            'status': 'running',
            'type': 'text',
          },
        ],
      },
    );
    expect(reducer.turn('turn-a')!.status, 'completed');
    expect(reducer.activeTurnId, isEmpty);
  });
}
