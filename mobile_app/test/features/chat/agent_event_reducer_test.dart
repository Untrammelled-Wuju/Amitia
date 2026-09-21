import 'package:amitia_app/features/chat/runtime/agent_event_reducer.dart';
import 'package:flutter_test/flutter_test.dart';

AgentUIEvent _event({
  required int sequence,
  required String id,
  required String type,
  String turnId = 'turn-1',
  String blockId = '',
  int blockSequence = 0,
  int revision = 1,
  String status = '',
  String callId = '',
  Map<String, dynamic> payload = const <String, dynamic>{},
  String messageId = '',
}) {
  return AgentUIEvent(
    version: 1,
    eventId: id,
    eventSequence: sequence,
    conversationId: 'conversation-1',
    requestId: 'request-1',
    executionId: 'execution-1',
    turnId: turnId,
    turnSequence: 1,
    blockId: blockId,
    blockSequence: blockSequence,
    messageId: messageId,
    messageSequence: 0,
    callId: callId,
    revision: revision,
    type: type,
    status: status,
    payload: payload,
    createdAt: '2026-09-21T10:00:00Z',
  );
}

void main() {
  test('唯一 v1 流将 queued、text delta 和 terminal 归入同一 Turn', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: const [], lastEventSequence: 0);

    expect(
      reducer.apply(_event(sequence: 1, id: 'evt-1', type: 'turn.queued', status: 'queued')),
      AgentEventApplyResult.applied,
    );
    expect(reducer.activeTurnId, 'turn-1');

    expect(
      reducer.apply(
        _event(
          sequence: 2,
          id: 'evt-2',
          type: 'text.started',
          blockId: 'block-text',
          blockSequence: 1,
          status: 'streaming',
          payload: const <String, dynamic>{'blockType': 'text'},
        ),
      ),
      AgentEventApplyResult.applied,
    );
    reducer.apply(
      _event(
        sequence: 3,
        id: 'evt-3',
        type: 'text.delta',
        blockId: 'block-text',
        blockSequence: 1,
        revision: 2,
        status: 'streaming',
        payload: const <String, dynamic>{'delta': '你'},
      ),
    );
    reducer.apply(
      _event(
        sequence: 4,
        id: 'evt-4',
        type: 'text.delta',
        blockId: 'block-text',
        blockSequence: 1,
        revision: 3,
        status: 'streaming',
        payload: const <String, dynamic>{'delta': '好'},
      ),
    );

    expect(reducer.turn('turn-1')!.items.single.content, '你好');

    reducer.apply(
      _event(
        sequence: 5,
        id: 'evt-5',
        type: 'turn.completed',
        status: 'completed',
        messageId: 'message-1',
      ),
    );
    expect(reducer.turn('turn-1')!.status, 'completed');
    expect(reducer.turn('turn-1')!.items.single.messageId, 'message-1');
    expect(reducer.activeTurnId, isEmpty);
  });

  test('重复事件只应用一次', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: const [], lastEventSequence: 0);
    final event = _event(sequence: 1, id: 'evt-1', type: 'turn.queued', status: 'queued');

    expect(reducer.apply(event), AgentEventApplyResult.applied);
    expect(reducer.apply(event), AgentEventApplyResult.duplicate);
  });

  test('普通序列缺口要求恢复而不是继续猜状态', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: const [], lastEventSequence: 10);

    expect(
      reducer.apply(_event(sequence: 13, id: 'evt-13', type: 'turn.queued')),
      AgentEventApplyResult.gap,
    );
    expect(reducer.lastEventSequence, 10);
  });

  test('approval expired 恢复 Turn 运行状态', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: const [], lastEventSequence: 0);
    reducer.apply(_event(sequence: 1, id: 'evt-1', type: 'turn.started', status: 'running'));
    reducer.apply(_event(sequence: 2, id: 'evt-2', type: 'approval.requested', status: 'waiting_approval'));
    expect(reducer.turn('turn-1')!.status, 'waiting_approval');
    reducer.apply(_event(sequence: 3, id: 'evt-3', type: 'approval.expired', status: 'running'));
    expect(reducer.turn('turn-1')!.status, 'running');
  });

  test('低 revision 不能覆盖更新后的 Block', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: const [], lastEventSequence: 0);
    reducer.apply(_event(sequence: 1, id: 'evt-1', type: 'turn.queued'));
    reducer.apply(
      _event(
        sequence: 2,
        id: 'evt-2',
        type: 'block.checkpoint',
        blockId: 'block-text',
        blockSequence: 1,
        revision: 5,
        payload: const <String, dynamic>{'blockType': 'text', 'content': 'new'},
      ),
    );

    expect(
      reducer.apply(
        _event(
          sequence: 3,
          id: 'evt-3',
          type: 'block.checkpoint',
          blockId: 'block-text',
          blockSequence: 1,
          revision: 4,
          payload: const <String, dynamic>{'blockType': 'text', 'content': 'old'},
        ),
      ),
      AgentEventApplyResult.stale,
    );
    expect(reducer.turn('turn-1')!.items.single.content, 'new');
  });
}
