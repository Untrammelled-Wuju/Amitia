import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/chat/runtime/agent_event_reducer.dart';
import 'package:flutter_test/flutter_test.dart';

const citation = <String, dynamic>{
  'marker': '[K1]', 'kind': 'knowledge', 'index': 1, 'sourceId': 'source-1',
};

AssistantTurnDto historic() => const AssistantTurnDto(
  id: 'turn-1', conversationId: 'conversation-1', characterId: 'character-1',
  requestId: 'request-1', executionId: 'execution-1', sequence: 1,
  status: 'running',
  items: <AssistantTurnItemDto>[
    AssistantTurnItemDto(
      id: 'text-1', turnId: 'turn-1', conversationId: 'conversation-1',
      sequence: 1, revision: 5, type: 'text', status: 'completed',
      content: '引用 [K1]', messageId: 'message-1', isFinal: true,
      citationSnapshot: <Map<String, dynamic>>[citation],
    ),
  ],
);

Map<String, dynamic> active(Map<String, dynamic> block) => <String, dynamic>{
  'turnId': 'turn-1', 'turnSequence': 1, 'requestId': 'request-1',
  'executionId': 'execution-1', 'status': 'running',
  'blocks': <Map<String, dynamic>>[block],
};

void main() {
  test('重连后活动 Snapshot 不丢失历史正文及引用快照', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: <AssistantTurnDto>[historic()], lastEventSequence: 10,
      activeTurn: active(<String, dynamic>{
        'blockId': 'text-1', 'blockSequence': 1, 'type': 'text',
        'status': 'completed', 'revision': 5,
      }));
    final turn = reducer.turn('turn-1')!;
    expect(turn.conversationId, 'conversation-1');
    expect(turn.characterId, 'character-1');
    expect(turn.items.single.messageId, 'message-1');
    expect(turn.items.single.content, '引用 [K1]');
    expect(turn.items.single.citationSnapshot, <Map<String, dynamic>>[citation]);
  });

  test('较旧活动块不能覆盖较新引用', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: <AssistantTurnDto>[historic()], lastEventSequence: 10,
      activeTurn: active(<String, dynamic>{
        'blockId': 'text-1', 'blockSequence': 1, 'type': 'text',
        'status': 'running', 'revision': 4, 'content': '旧正文',
        'payload': <String, dynamic>{'citationSnapshot': <dynamic>[]},
      }));
    expect(reducer.turn('turn-1')!.items.single.revision, 5);
    expect(reducer.turn('turn-1')!.items.single.content, '引用 [K1]');
    expect(reducer.turn('turn-1')!.items.single.citationSnapshot, <Map<String, dynamic>>[citation]);
  });

  test('新版本明确空正文及引用可以清空旧快照', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: <AssistantTurnDto>[historic()], lastEventSequence: 10,
      activeTurn: active(<String, dynamic>{
        'blockId': 'text-1', 'blockSequence': 1, 'type': 'text',
        'status': 'completed', 'revision': 6, 'content': '',
        'payload': <String, dynamic>{'citationSnapshot': <dynamic>[]},
      }));
    expect(reducer.turn('turn-1')!.items.single.content, isEmpty);
    expect(reducer.turn('turn-1')!.items.single.citationSnapshot, isEmpty);
  });

  test('历史 DTO 明确空引用能清除实时引用，缺失字段则保留', () {
    final source = historic();
    final reducer = AgentEventReducer();
    reducer.reset(turns: <AssistantTurnDto>[source], lastEventSequence: 10);
    final withoutField = AssistantTurnItemDto.fromJson(<String, dynamic>{
      'id': 'text-1', 'turnId': 'turn-1', 'conversationId': 'conversation-1',
      'messageId': 'message-1', 'sequence': 1, 'revision': 5,
      'type': 'text', 'status': 'completed', 'content': '引用 [K1]',
    });
    expect(withoutField.citationSnapshotProvided, isFalse);
    reducer.mergeTurns(<AssistantTurnDto>[
      AssistantTurnDto(
        id: 'turn-1', conversationId: 'conversation-1', sequence: 1,
        status: 'completed', items: <AssistantTurnItemDto>[withoutField],
      ),
    ]);
    expect(reducer.turn('turn-1')!.items.single.citationSnapshot, <Map<String, dynamic>>[citation]);

    final explicitlyCleared = AssistantTurnItemDto.fromJson(<String, dynamic>{
      'id': 'text-1', 'turnId': 'turn-1', 'conversationId': 'conversation-1',
      'messageId': 'message-1', 'sequence': 1, 'revision': 5,
      'type': 'text', 'status': 'completed', 'content': '无来源',
      'citationSnapshot': <dynamic>[],
    });
    expect(explicitlyCleared.citationSnapshotProvided, isTrue);
    reducer.mergeTurns(<AssistantTurnDto>[
      AssistantTurnDto(
        id: 'turn-1', conversationId: 'conversation-1', sequence: 1,
        status: 'completed', items: <AssistantTurnItemDto>[explicitlyCleared],
      ),
    ]);
    expect(reducer.turn('turn-1')!.items.single.citationSnapshot, isEmpty);
  });

  test('恢复后完成事件只消费一次且终态仍保留引用', () {
    final reducer = AgentEventReducer();
    reducer.reset(turns: <AssistantTurnDto>[historic()], lastEventSequence: 20,
      activeTurn: active(<String, dynamic>{
        'blockId': 'text-1', 'blockSequence': 1, 'type': 'text',
        'status': 'streaming', 'revision': 5,
      }));
    const completed = AgentUIEvent(
      version: 1, eventId: 'event-21', eventSequence: 21,
      conversationId: 'conversation-1', requestId: 'request-1',
      executionId: 'execution-1', turnId: 'turn-1', turnSequence: 1,
      blockId: 'text-1', blockSequence: 1, messageId: 'message-1',
      messageSequence: 1, callId: '', revision: 6,
      type: 'text.completed', status: 'completed',
      payload: <String, dynamic>{
        'blockType': 'text', 'content': '答案 [K1]',
        'citationSnapshot': <Map<String, dynamic>>[citation],
      },
      createdAt: '2026-10-16T00:00:00Z',
    );
    expect(reducer.apply(completed), AgentEventApplyResult.applied);
    expect(reducer.apply(completed), AgentEventApplyResult.duplicate);
    final terminal = AgentUIEvent(
      version: 1, eventId: 'event-22', eventSequence: 22,
      conversationId: 'conversation-1', requestId: 'request-1',
      executionId: 'execution-1', turnId: 'turn-1', turnSequence: 1,
      blockId: '', blockSequence: 0, messageId: 'message-1',
      messageSequence: 1, callId: '', revision: 0,
      type: 'turn.completed', status: 'completed',
      payload: const <String, dynamic>{},
      createdAt: '2026-10-16T00:00:01Z',
    );
    expect(reducer.apply(terminal), AgentEventApplyResult.applied);
    expect(reducer.turn('turn-1')!.items.single.citationSnapshot, <Map<String, dynamic>>[citation]);
    expect(reducer.turn('turn-1')!.status, 'completed');
  });
}
