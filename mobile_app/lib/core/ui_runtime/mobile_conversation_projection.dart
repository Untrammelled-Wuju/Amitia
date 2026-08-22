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

class _ProjectionLifecycle {
  _ProjectionLifecycle({
    required this.groupKey,
    required this.first,
    required this.events,
  });

  final String groupKey;
  final MobileConversationEvent first;
  final List<MobileConversationEvent> events;
}

class MobileConversationProjection {
  const MobileConversationProjection._();

  static List<MobileConversationNode> assemble({
    required List<MobileConversationEvent> events,
    required List<UIContributionSnapshotEntry> contributions,
  }) {
    final orderedEvents = _dedupeEvents(events)..sort(_compareEvent);
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
      final maxEvents = (((projection['max_events'] ?? projection['maxEvents']) as num?)?.toInt() ?? 100).clamp(1, 1000).toInt();
      final active = <String, _ProjectionLifecycle>{};
      final startedGroups = <String>{};

      void publish(_ProjectionLifecycle lifecycle, String status) {
        if (lifecycle.events.isEmpty) return;
        final first = lifecycle.first;
        final last = lifecycle.events.last;
        String? title;
        if (titlePath.isNotEmpty) {
          for (var index = lifecycle.events.length - 1; index >= 0; index--) {
            final value = _lookup(lifecycle.events[index].payload, titlePath)?.toString().trim() ?? '';
            if (value.isNotEmpty) {
              title = value;
              break;
            }
          }
        }
        final retained = lifecycle.events.length > maxEvents
            ? lifecycle.events.sublist(lifecycle.events.length - maxEvents)
            : [...lifecycle.events];
        result.add(MobileConversationNode(
          nodeId: '${contribution.contributionId}:${lifecycle.groupKey}:${first.id}',
          contributionId: contribution.contributionId,
          extensionId: contribution.extensionId,
          nodeType: nodeType,
          conversationId: first.conversationId,
          groupKey: lifecycle.groupKey,
          status: status,
          anchorTimestamp: first.timestamp,
          updatedAt: last.timestamp,
          anchorSeq: first.sequence,
          title: title,
          eventType: last.eventType,
          payload: last.payload,
          events: retained,
        ));
      }

      for (final event in orderedEvents) {
        if (!eventTypes.contains(event.eventType)) continue;
        final groupKey = keyPath.isEmpty
            ? event.id
            : (_lookup(event.payload, keyPath)?.toString().trim() ?? '');
        if (groupKey.isEmpty) continue;
        final isStart = startEvents.contains(event.eventType);
        final isEnd = endEvents.contains(event.eventType);
        var lifecycle = active[groupKey];

        if (startEvents.isNotEmpty) {
          if (isStart) {
            if (!startedGroups.add(groupKey)) {
              if (lifecycle != null) {
                publish(lifecycle, 'completed');
                active.remove(groupKey);
              }
              continue;
            }
            lifecycle = _ProjectionLifecycle(
              groupKey: groupKey,
              first: event,
              events: <MobileConversationEvent>[event],
            );
            active[groupKey] = lifecycle;
          } else if (lifecycle == null) {
            // A projection with explicit start events must not publish partial
            // contexts when history has not reached the start event yet.
            continue;
          } else {
            lifecycle.events.add(event);
          }
        } else {
          lifecycle ??= _ProjectionLifecycle(
            groupKey: groupKey,
            first: event,
            events: <MobileConversationEvent>[],
          );
          active[groupKey] = lifecycle;
          lifecycle.events.add(event);
        }

        if (isEnd && lifecycle != null) {
          publish(lifecycle, 'completed');
          active.remove(groupKey);
        }
      }

      for (final lifecycle in active.values) {
        publish(lifecycle, 'active');
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
    final id = (message['id'] ?? 'message-$index').toString();
    return MobileConversationEvent(
      id: id,
      eventType: 'message_created',
      conversationId: conversationId,
      timestamp: timestamp,
      sequence: sequence,
      payload: {...message, 'messageId': id},
    );
  });

  static List<MobileConversationEvent> durableEvents({
    required String conversationId,
    required List<Map<String, dynamic>> records,
  }) {
    final result = <MobileConversationEvent>[];
    for (var index = 0; index < records.length; index++) {
      final row = records[index];
      final rawPayload = row['payload'];
      if (rawPayload is! Map) continue;
      final envelope = rawPayload.cast<String, dynamic>();
      final eventType = (envelope['type'] ?? envelope['eventType'] ?? '').toString().trim();
      if (eventType.isEmpty) continue;
      final payload = envelope['payload'] is Map
          ? (envelope['payload'] as Map).cast<String, dynamic>()
          : <String, dynamic>{...envelope};
      final rawTime = envelope['timestamp'] ?? row['occurredAt'];
      final timestamp = rawTime is DateTime
          ? rawTime
          : DateTime.tryParse(rawTime?.toString() ?? '') ?? DateTime.fromMillisecondsSinceEpoch(index);
      result.add(MobileConversationEvent(
        id: (envelope['id'] ?? row['eventId'] ?? 'durable-$index').toString(),
        eventType: eventType,
        conversationId: (envelope['conversationId'] ?? conversationId).toString(),
        timestamp: timestamp,
        sequence: _int(row['sequence'] ?? envelope['sequence'] ?? envelope['seq']),
        payload: payload,
      ));
    }
    return result;
  }

  static List<MobileConversationEvent> mergeEvents(Iterable<List<MobileConversationEvent>> sources) {
    final merged = <String, MobileConversationEvent>{};
    for (final source in sources) {
      for (final event in source) {
        final messageId = event.payload['messageId']?.toString().trim() ?? '';
        final key = messageId.isNotEmpty
            ? '${event.conversationId}:${event.eventType}:message:$messageId'
            : '${event.conversationId}:${event.eventType}:${event.id}';
        merged[key] = event;
      }
    }
    return merged.values.toList(growable: false)..sort(_compareEvent);
  }
}

int _compareEvent(MobileConversationEvent a, MobileConversationEvent b) =>
    MobileConversationProjection.compareTimeline(a.sequence, a.timestamp, b.sequence, b.timestamp);

List<MobileConversationEvent> _dedupeEvents(List<MobileConversationEvent> events) =>
    MobileConversationProjection.mergeEvents(<List<MobileConversationEvent>>[events]);

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
