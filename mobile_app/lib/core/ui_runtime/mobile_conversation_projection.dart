import 'ui_provider.dart';

class MobileConversationEvent {
  const MobileConversationEvent({
    required this.id,
    required this.eventType,
    required this.conversationId,
    required this.timestamp,
    required this.payload,
    this.sequence,
  });

  final String id;
  final String eventType;
  final String conversationId;
  final DateTime timestamp;
  final Map<String, dynamic> payload;
  final int? sequence;
}

class MobileConversationNode {
  const MobileConversationNode({
    required this.nodeId,
    required this.contributionId,
    required this.extensionId,
    required this.nodeType,
    required this.conversationId,
    required this.groupKey,
    required this.status,
    required this.anchorTimestamp,
    required this.updatedAt,
    required this.eventType,
    required this.payload,
    required this.events,
    this.anchorSeq,
    this.title,
  });

  final String nodeId;
  final String contributionId;
  final String extensionId;
  final String nodeType;
  final String conversationId;
  final String groupKey;
  final String status;
  final DateTime anchorTimestamp;
  final DateTime updatedAt;
  final int? anchorSeq;
  final String? title;
  final String eventType;
  final Map<String, dynamic> payload;
  final List<MobileConversationEvent> events;

  Map<String, dynamic> toJson() => {
    'nodeId': nodeId,
    'contributionId': contributionId,
    'extensionId': extensionId,
    'nodeType': nodeType,
    'conversationId': conversationId,
    'groupKey': groupKey,
    'status': status,
    'anchorTimestamp': anchorTimestamp.toIso8601String(),
    'updatedAt': updatedAt.toIso8601String(),
    if (anchorSeq != null) 'anchorSeq': anchorSeq,
    if (title != null) 'title': title,
    'eventType': eventType,
    'payload': payload,
    'events': events.map((event) => {
      'id': event.id,
      'eventType': event.eventType,
      'conversationId': event.conversationId,
      'timestamp': event.timestamp.toIso8601String(),
      if (event.sequence != null) 'sequence': event.sequence,
      'payload': event.payload,
    }).toList(growable: false),
  };
}

class MobileConversationProjection {
  const MobileConversationProjection._();

  static List<MobileConversationNode> assemble({
    required List<MobileConversationEvent> events,
    required List<UIContributionSnapshotEntry> contributions,
  }) {
    final orderedEvents = [...events]..sort(_compareEvent);
    final result = <MobileConversationNode>[];

    for (final contribution in contributions) {
      final projection = (contribution.dataContract['projection'] as Map?)?.cast<String, dynamic>();
      if (projection == null) continue;
      final eventTypes = _stringList(projection['event_types'] ?? projection['eventTypes']);
      if (eventTypes.isEmpty) continue;
      final startEvents = _stringList(projection['start_events'] ?? projection['startEvents']);
      final endEvents = _stringList(projection['end_events'] ?? projection['endEvents']);
      final keyPath = (projection['key_path'] ?? projection['keyPath'] ?? '').toString().trim();
      final titlePath = (projection['title_path'] ?? projection['titlePath'] ?? '').toString().trim();
      final nodeType = (projection['node_type'] ?? projection['nodeType'] ?? contribution.kind).toString();
      final maxEvents = ((projection['max_events'] ?? projection['maxEvents']) as num?)?.toInt() ?? 100;
      final groups = <String, List<MobileConversationEvent>>{};

      for (final event in orderedEvents) {
        if (!eventTypes.contains(event.eventType)) continue;
        final key = keyPath.isEmpty ? event.id : (_lookup(event.payload, keyPath)?.toString().trim() ?? '');
        if (key.isEmpty) continue;
        (groups[key] ??= <MobileConversationEvent>[]).add(event);
      }

      for (final entry in groups.entries) {
        final group = entry.value;
        var startIndex = 0;
        if (startEvents.isNotEmpty) {
          startIndex = group.indexWhere((event) => startEvents.contains(event.eventType));
          if (startIndex < 0) continue;
        }
        final active = group.sublist(startIndex);
        if (active.isEmpty) continue;
        final first = active.first;
        final last = active.last;
        String? title;
        if (titlePath.isNotEmpty) {
          for (var index = active.length - 1; index >= 0; index--) {
            final value = _lookup(active[index].payload, titlePath)?.toString().trim() ?? '';
            if (value.isNotEmpty) { title = value; break; }
          }
        }
        final completed = endEvents.isNotEmpty && active.any((event) => endEvents.contains(event.eventType));
        final retained = active.length > maxEvents ? active.sublist(active.length - maxEvents) : [...active];
        result.add(MobileConversationNode(
          nodeId: '${contribution.contributionId}:${entry.key}',
          contributionId: contribution.contributionId,
          extensionId: contribution.extensionId,
          nodeType: nodeType,
          conversationId: first.conversationId,
          groupKey: entry.key,
          status: completed ? 'completed' : 'active',
          anchorTimestamp: first.timestamp,
          updatedAt: last.timestamp,
          anchorSeq: first.sequence,
          title: title,
          eventType: last.eventType,
          payload: last.payload,
          events: retained,
        ));
      }
    }

    result.sort((a, b) => compareTimeline(a.anchorSeq, a.anchorTimestamp, b.anchorSeq, b.anchorTimestamp));
    return result;
  }

  static int compareTimeline(int? aSeq, DateTime aTime, int? bSeq, DateTime bTime) {
    if (aSeq != null && bSeq != null && aSeq != bSeq) return aSeq.compareTo(bSeq);
    final byTime = aTime.compareTo(bTime);
    if (byTime != 0) return byTime;
    if (aSeq != null && bSeq == null) return -1;
    if (aSeq == null && bSeq != null) return 1;
    return 0;
  }

  static List<MobileConversationEvent> messageEvents({
    required String conversationId,
    required List<Map<String, dynamic>> messages,
  }) => List.generate(messages.length, (index) {
    final message = messages[index];
    final rawTime = message['createdAt'] ?? message['time'] ?? message['timestamp'];
    final timestamp = rawTime is DateTime
        ? rawTime
        : DateTime.tryParse(rawTime?.toString() ?? '') ?? DateTime.fromMillisecondsSinceEpoch(index);
    final sequence = _int(message['seq'] ?? message['sequence']);
    return MobileConversationEvent(
      id: (message['id'] ?? 'message-$index').toString(),
      eventType: 'message_created',
      conversationId: conversationId,
      timestamp: timestamp,
      sequence: sequence,
      payload: message,
    );
  });
}

int _compareEvent(MobileConversationEvent a, MobileConversationEvent b) =>
    MobileConversationProjection.compareTimeline(a.sequence, a.timestamp, b.sequence, b.timestamp);

List<String> _stringList(dynamic value) => value is List
    ? value.map((item) => item.toString().trim()).where((item) => item.isNotEmpty).toList(growable: false)
    : const <String>[];

int? _int(dynamic value) {
  if (value is int) return value;
  return int.tryParse(value?.toString() ?? '');
}

dynamic _lookup(Map<String, dynamic> value, String path) {
  dynamic current = value;
  for (final segment in path.split('.')) {
    if (current is! Map) return null;
    current = current[segment];
  }
  return current;
}
