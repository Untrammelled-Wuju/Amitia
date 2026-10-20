import 'package:amitia_app/features/chat/runtime/agent_event_reducer.dart';
import 'package:amitia_app/features/chat/runtime/citation_snapshot.dart';
import 'package:flutter_test/flutter_test.dart';

Map<String, dynamic> event(int sequence, String type, {Map<String, dynamic>? override}) => <String, dynamic>{
  'version': 1,
  'eventId': 'event-$sequence',
  'eventSequence': sequence,
  'conversationId': 'conversation-1',
  'turnId': 'turn-1',
  'turnSequence': 1,
  'type': type,
  'createdAt': '2026-10-16T01:00:00Z',
  ...?override,
};

void main() {
  final evidence = <Map<String, dynamic>>[
    <String, dynamic>{'marker': '[K1]', 'kind': 'knowledge', 'index': 1, 'sourceId': 'doc-1', 'title': '内部文档'},
    <String, dynamic>{'marker': '[1]', 'kind': 'web', 'index': 1, 'title': '外部网页', 'url': 'https://example.com'},
  ];

  test('bounded citation normalization rejects invalid markers and deduplicates', () {
    final result = normalizeCitationSnapshot(<dynamic>[
      ...evidence,
      evidence.first,
      <String, dynamic>{'marker': '[K1]', 'kind': 'knowledge', 'index': 2},
      <String, dynamic>{'marker': '[3]', 'kind': 'web', 'index': 3, 'excerpt': List<String>.filled(2000, 'a').join()},
    ]);
    expect(result.length, 3);
    expect(result.map((item) => item['marker']).toList(), <String>['[K1]', '[1]', '[3]']);
    expect((result[2]['excerpt'] as String).length, 1200);
    expect(normalizeCitationSnapshot('invalid'), isEmpty);
    expect(normalizeCitationSnapshot(<dynamic>[
      <String, dynamic>{'marker': '[K1]', 'kind': 'knowledge', 'index': 1,
        'excerpt': '  ${List<String>.filled(1201, '😀').join()}  '},
    ]).first['excerpt'], List<String>.filled(1200, '😀').join());
  });

  test('citation snapshots remove secrets and unsafe navigable URLs', () {
    final source = <String, dynamic>{
      'marker': '[K1]', 'kind': 'knowledge', 'index': 1,
      'url': 'file:///home/private',
      'excerpt': List<String>.filled(1400, '字').join(),
      'locator': <String, dynamic>{
        'page': 7, 'row': '12', 'kind': 'pdf_page', 'path': '/private/file',
        'token': 'private', 'field': r'C:\private\key',
      },
    };
    final citations = normalizeCitationSnapshot(<dynamic>[
      source,
      <String, dynamic>{
        'marker': '[1]', 'kind': 'web', 'index': 1,
        'url': 'https://example.com/article?query=keep&api_key=private&access_token=secret#part',
      },
      <String, dynamic>{
        'marker': '[2]', 'kind': 'web', 'index': 2,
        'url': 'https://user:secret@example.com/private',
      },
    ]);
    expect(citations[0]['locator'], <String, dynamic>{'page': 7, 'row': 12, 'kind': 'pdf_page'});
    expect(citations[0]['url'], isNull);
    expect((citations[0]['excerpt'] as String).runes.length, 1200);
    expect(citations[1]['url'], 'https://example.com/article?query=keep#part');
    expect(citations[2]['url'], isNull);
  });

  test('safe parser locator aliases preserve Excel worksheet and row positions', () {
    final got = normalizeCitationSnapshot(<dynamic>[
      <String, dynamic>{'marker': '[K1]', 'kind': 'knowledge', 'index': 1,
        'locator': <String, dynamic>{'sheetName': '年度汇总 2026', 'rowStart': 146, 'rowEnd': 150, 'path': '/private'}},
      <String, dynamic>{'marker': '[K2]', 'kind': 'knowledge', 'index': 2,
        'locator': <String, dynamic>{'pageNumber': 37, 'password': 'private'}},
      <String, dynamic>{'marker': '[K3]', 'kind': 'knowledge', 'index': 3,
        'locator': <String, dynamic>{'paragraphIndex': 86}},
      <String, dynamic>{'marker': '[K4]', 'kind': 'knowledge', 'index': 4,
        'locator': <String, dynamic>{'sheetName': r'C:\private\file', 'rowStart': 12}},
      <String, dynamic>{'marker': '[K5]', 'kind': 'knowledge', 'index': 5,
        'locator': <String, dynamic>{'sheetName': '2026', 'rowStart': 32}},
    ]);
    expect(got.map((entry) => entry['locator']).toList(), <Map<String, dynamic>>[
      <String, dynamic>{'sheet': '年度汇总 2026', 'row': 146},
      <String, dynamic>{'page': 37},
      <String, dynamic>{'paragraph': 86},
      <String, dynamic>{'row': 12},
      <String, dynamic>{'sheet': '2026', 'row': 32},
    ]);
  });

  test('final citation survives duplicate, stale, terminal and replay events', () {
    final reducer = AgentEventReducer();
    expect(reducer.apply(AgentUIEvent.fromJson(event(1, 'turn.started'))), AgentEventApplyResult.applied);
    final complete = event(2, 'text.completed', override: <String, dynamic>{
      'blockId': 'text-1', 'blockSequence': 1, 'revision': 2,
      'messageId': 'message-1', 'status': 'completed',
      'payload': <String, dynamic>{'blockType': 'text', 'content': '内部 [K1] 网页 [1] 未知 [K9]', 'citationSnapshot': evidence},
    });
    expect(reducer.apply(AgentUIEvent.fromJson(complete)), AgentEventApplyResult.applied);
    expect(reducer.apply(AgentUIEvent.fromJson(complete)), AgentEventApplyResult.duplicate);
    expect(reducer.apply(AgentUIEvent.fromJson(event(1, 'text.delta', override: <String, dynamic>{'eventId': 'stale'}))), AgentEventApplyResult.stale);
    final item = reducer.turn('turn-1')!.items.first;
    expect(item.citationSnapshot.map((entry) => entry['marker']).toList(), <String>['[K1]', '[1]']);
    expect(item.messageId, 'message-1');
    expect(reducer.apply(AgentUIEvent.fromJson(event(3, 'turn.completed', override: <String, dynamic>{'messageId': 'message-1'}))), AgentEventApplyResult.applied);
    expect(reducer.turn('turn-1')!.items.first.citationSnapshot, item.citationSnapshot);
    final replay = AgentEventReducer();
    replay.apply(AgentUIEvent.fromJson(event(1, 'turn.started')));
    replay.apply(AgentUIEvent.fromJson(complete));
    expect(replay.turn('turn-1')!.items.first.citationSnapshot, item.citationSnapshot);
  });
}
