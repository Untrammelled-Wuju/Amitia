import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/chat/runtime/agent_event_reducer.dart';
import 'package:flutter_test/flutter_test.dart';

Directory _fixtureDirectory() {
  final candidates = <Directory>[
    Directory('../contracts/agent-runtime/v1/fixtures'),
    Directory('contracts/agent-runtime/v1/fixtures'),
    Directory('../../contracts/agent-runtime/v1/fixtures'),
  ];
  return candidates.firstWhere(
    (candidate) => candidate.existsSync(),
    orElse: () => throw StateError('Agent Runtime v1 golden fixtures not found'),
  );
}

AssistantTurnItemDto _initialItem(String turnId, Map<String, dynamic> block) {
  final type = (block['type'] ?? '').toString();
  return AssistantTurnItemDto(
    id: (block['blockId'] ?? '').toString(),
    turnId: turnId,
    conversationId: 'conversation-1',
    sequence: (block['blockSequence'] as num?)?.toInt() ?? 0,
    type: type,
    status: (block['status'] ?? '').toString(),
    content: (block['content'] ?? '').toString(),
    callId: (block['callId'] ?? '').toString(),
    argumentsJson: (block['arguments'] ?? '').toString(),
    resultJson: (block['result'] ?? '').toString(),
  );
}

AssistantTurnDto _initialTurn(Map<String, dynamic> turn) {
  final turnId = (turn['turnId'] ?? '').toString();
  final rawBlocks = turn['blocks'];
  return AssistantTurnDto(
    id: turnId,
    conversationId: 'conversation-1',
    executionId: (turn['executionId'] ?? '').toString(),
    sequence: (turn['turnSequence'] as num?)?.toInt() ?? 0,
    status: (turn['status'] ?? '').toString(),
    items: rawBlocks is List
        ? rawBlocks
            .whereType<Map>()
            .map((raw) => _initialItem(turnId, Map<String, dynamic>.from(raw)))
            .toList(growable: false)
        : const <AssistantTurnItemDto>[],
  );
}

Map<String, dynamic> _normalizedBlock(AssistantTurnItemDto item) {
  final value = <String, dynamic>{
    'blockId': item.id,
    'blockSequence': item.sequence,
    'type': item.type,
    'status': item.status,
  };
  if (item.content.isNotEmpty) value['content'] = item.content;
  if (item.callId.isNotEmpty) value['callId'] = item.callId;
  if (item.argumentsJson.isNotEmpty) value['arguments'] = item.argumentsJson;
  if (item.resultJson.isNotEmpty) value['result'] = item.resultJson;
  return value;
}

Map<String, dynamic> _normalizedTurn(AssistantTurnDto turn) => <String, dynamic>{
  'turnId': turn.id,
  'turnSequence': turn.sequence,
  'executionId': turn.executionId,
  'status': turn.status,
  'blocks': turn.items.map(_normalizedBlock).toList(growable: false),
};

void main() {
  final files = _fixtureDirectory()
      .listSync()
      .whereType<File>()
      .where((file) => file.path.endsWith('.json'))
      .toList()
    ..sort((left, right) => left.path.compareTo(right.path));

  for (final file in files) {
    final fixture = jsonDecode(file.readAsStringSync()) as Map<String, dynamic>;
    test('Agent Runtime v1 golden: ${fixture['name']}', () {
      expect(fixture['version'], 1);
      final initial = Map<String, dynamic>.from(fixture['initialState'] as Map);
      final rawInitialTurns = initial['turns'];
      final reducer = AgentEventReducer();
      reducer.reset(
        turns: rawInitialTurns is List
            ? rawInitialTurns
                .whereType<Map>()
                .map((raw) => _initialTurn(Map<String, dynamic>.from(raw)))
                .toList(growable: false)
            : const <AssistantTurnDto>[],
        lastEventSequence: (initial['lastEventSequence'] as num?)?.toInt() ?? 0,
      );

      final results = <String>[];
      final rawEvents = fixture['events'] as List;
      for (final raw in rawEvents.whereType<Map>()) {
        results.add(
          reducer
              .apply(AgentUIEvent.fromJson(Map<String, dynamic>.from(raw)))
              .name,
        );
      }
      final expectedResults = fixture['expectedApplyResults'];
      if (expectedResults is List) {
        expect(results, expectedResults.map((value) => value.toString()).toList());
      }

      expect(
        <String, dynamic>{
          'lastEventSequence': reducer.lastEventSequence,
          'activeTurnId': reducer.activeTurnId,
          'turns': reducer.turns.map(_normalizedTurn).toList(growable: false),
        },
        Map<String, dynamic>.from(fixture['expectedState'] as Map),
      );
    });
  }
}
