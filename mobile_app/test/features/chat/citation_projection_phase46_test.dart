import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/chat/runtime/citation_projection.dart';
import 'package:flutter_test/flutter_test.dart';

const existing = <Map<String, dynamic>>[
  <String, dynamic>{'marker': '[K1]', 'kind': 'knowledge', 'index': 1},
];
const updated = <Map<String, dynamic>>[
  <String, dynamic>{'marker': '[K2]', 'kind': 'knowledge', 'index': 2},
];

AssistantTurnItemDto textItem({
  List<Map<String, dynamic>> snapshots = const <Map<String, dynamic>>[],
  bool provided = false,
}) => AssistantTurnItemDto(
  id: 'text-1', turnId: 'turn-1', conversationId: 'conversation-1',
  sequence: 1, type: 'text', status: 'completed', revision: 3,
  messageId: 'message-1', content: 'answer', citationSnapshot: snapshots,
  citationSnapshotProvided: provided,
);

void main() {
  test('explicit empty completion clears persisted citations', () {
    expect(projectCitationSnapshot(existing, textItem(provided: true)), isEmpty);
  });
  test('explicit newer completion replaces persisted citations', () {
    expect(projectCitationSnapshot(existing, textItem(snapshots: updated, provided: true)), updated);
  });
  test('missing citation field preserves persisted citations', () {
    expect(projectCitationSnapshot(existing, textItem()), existing);
    expect(projectCitationSnapshot(existing, null), existing);
  });
  test('legacy nonempty completion still supplies citations', () {
    expect(projectCitationSnapshot(existing, textItem(snapshots: updated)), updated);
  });
  test('history without citations receives newer data', () {
    expect(projectCitationSnapshot(const <Map<String, dynamic>>[],
      textItem(snapshots: updated, provided: true)), updated);
  });
}
