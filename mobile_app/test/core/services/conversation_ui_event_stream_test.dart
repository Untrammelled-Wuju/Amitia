import 'dart:convert';

import 'package:amitia_app/core/services/extension_service.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('decodes chunked conversation UI SSE events without polling', () async {
    final source = Stream<List<int>>.fromIterable([
      utf8.encode(
        'event: connected\ndata: {"conversationId":"chat","afterSequence":0}\n\n'
        'event: conversation_ui_event\nid: 7\ndata: {"eventId":"event-7","sequence":7,',
      ),
      utf8.encode(
        '"occurredAt":"2026-11-07T10:00:00Z","payload":{"type":"message.created"}}\n\n',
      ),
    ]);

    final events = await decodeConversationUIEventStream(source).toList();

    expect(events, hasLength(1));
    expect(events.single['eventId'], 'event-7');
    expect(events.single['sequence'], 7);
    expect(events.single['payload'], {'type': 'message.created'});
  });

  test('preserves the original conversation UI stream error', () async {
    final source = Stream<List<int>>.value(
      utf8.encode(
        'event: stream_error\ndata: {"error":"sql: database is locked","afterSequence":9}\n\n',
      ),
    );

    await expectLater(
      decodeConversationUIEventStream(source).toList(),
      throwsA(
        isA<StateError>().having(
          (error) => error.message,
          'message',
          contains('sql: database is locked'),
        ),
      ),
    );
  });
}
