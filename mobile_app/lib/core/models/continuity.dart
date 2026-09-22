String _stringValue(dynamic value) => value?.toString() ?? '';

DateTime? _dateTimeValue(dynamic value) {
  final raw = _stringValue(value).trim();
  if (raw.isEmpty) return null;
  return DateTime.tryParse(raw);
}

class ContinuityThreadDto {
  const ContinuityThreadDto({
    required this.id,
    required this.spaceId,
    required this.characterId,
    required this.parentThreadId,
    required this.title,
    required this.goal,
    required this.status,
    required this.summary,
    required this.currentState,
    required this.nextAction,
    required this.priority,
    required this.confidence,
    required this.revision,
    required this.createdAt,
    required this.updatedAt,
    required this.lastActiveAt,
    required this.completedAt,
  });

  factory ContinuityThreadDto.fromJson(Map<String, dynamic> json) {
    return ContinuityThreadDto(
      id: _stringValue(json['id']),
      spaceId: _stringValue(json['spaceId']),
      characterId: _stringValue(json['characterId']),
      parentThreadId: _stringValue(json['parentThreadId']),
      title: _stringValue(json['title']),
      goal: _stringValue(json['goal']),
      status: _stringValue(json['status']),
      summary: _stringValue(json['summary']),
      currentState: _stringValue(json['currentState']),
      nextAction: _stringValue(json['nextAction']),
      priority: (json['priority'] as num?)?.toInt() ?? 0,
      confidence: (json['confidence'] as num?)?.toDouble() ?? 0,
      revision: (json['revision'] as num?)?.toInt() ?? 0,
      createdAt: _dateTimeValue(json['createdAt']),
      updatedAt: _dateTimeValue(json['updatedAt']),
      lastActiveAt: _dateTimeValue(json['lastActiveAt']),
      completedAt: _dateTimeValue(json['completedAt']),
    );
  }

  final String id;
  final String spaceId;
  final String characterId;
  final String parentThreadId;
  final String title;
  final String goal;
  final String status;
  final String summary;
  final String currentState;
  final String nextAction;
  final int priority;
  final double confidence;
  final int revision;
  final DateTime? createdAt;
  final DateTime? updatedAt;
  final DateTime? lastActiveAt;
  final DateTime? completedAt;

  bool get terminal => status == 'completed' || status == 'cancelled';
  bool get waiting => status == 'waiting';
  bool get paused => status == 'paused';
}

class ContinuityWaitDto {
  const ContinuityWaitDto({
    required this.id,
    required this.threadId,
    required this.waitType,
    required this.status,
    required this.description,
    required this.conditionJson,
    required this.resumeHint,
    required this.dueAt,
    required this.resolvedAt,
    required this.resolvedBy,
    required this.autoResume,
    required this.wakeState,
    required this.wakeAttempts,
    required this.lastWakeError,
    required this.createdAt,
  });

  factory ContinuityWaitDto.fromJson(Map<String, dynamic> json) {
    final condition = json['conditionJson'];
    return ContinuityWaitDto(
      id: _stringValue(json['id']),
      threadId: _stringValue(json['threadId']),
      waitType: _stringValue(json['waitType']),
      status: _stringValue(json['status']),
      description: _stringValue(json['description']),
      conditionJson: condition is Map || condition is List
          ? condition.toString()
          : _stringValue(condition),
      resumeHint: _stringValue(json['resumeHint']),
      dueAt: _dateTimeValue(json['dueAt']),
      resolvedAt: _dateTimeValue(json['resolvedAt']),
      resolvedBy: _stringValue(json['resolvedBy']),
      autoResume: json['autoResume'] == true,
      wakeState: _stringValue(json['wakeState']),
      wakeAttempts: (json['wakeAttempts'] as num?)?.toInt() ?? 0,
      lastWakeError: _stringValue(json['lastWakeError']),
      createdAt: _dateTimeValue(json['createdAt']),
    );
  }

  final String id;
  final String threadId;
  final String waitType;
  final String status;
  final String description;
  final String conditionJson;
  final String resumeHint;
  final DateTime? dueAt;
  final DateTime? resolvedAt;
  final String resolvedBy;
  final bool autoResume;
  final String wakeState;
  final int wakeAttempts;
  final String lastWakeError;
  final DateTime? createdAt;

  bool get open => status == 'waiting';
}

class ContinuityEventDto {
  const ContinuityEventDto({
    required this.id,
    required this.eventType,
    required this.sourceType,
    required this.sourceId,
    required this.payloadJson,
    required this.occurredAt,
  });

  factory ContinuityEventDto.fromJson(Map<String, dynamic> json) {
    return ContinuityEventDto(
      id: _stringValue(json['id']),
      eventType: _stringValue(json['eventType']),
      sourceType: _stringValue(json['sourceType']),
      sourceId: _stringValue(json['sourceId']),
      payloadJson: _stringValue(json['payloadJson']),
      occurredAt: _dateTimeValue(json['occurredAt']),
    );
  }

  final String id;
  final String eventType;
  final String sourceType;
  final String sourceId;
  final String payloadJson;
  final DateTime? occurredAt;
}

class ContinuityDetailDto {
  const ContinuityDetailDto({
    required this.thread,
    required this.waits,
    required this.events,
  });

  factory ContinuityDetailDto.fromJson(Map<String, dynamic> json) {
    final waits = (json['waits'] as List?) ?? const <dynamic>[];
    final events = (json['events'] as List?) ?? const <dynamic>[];
    return ContinuityDetailDto(
      thread: ContinuityThreadDto.fromJson(
        Map<String, dynamic>.from((json['thread'] as Map?) ?? const {}),
      ),
      waits: waits
          .whereType<Map>()
          .map(
            (item) =>
                ContinuityWaitDto.fromJson(Map<String, dynamic>.from(item)),
          )
          .toList(growable: false),
      events: events
          .whereType<Map>()
          .map(
            (item) =>
                ContinuityEventDto.fromJson(Map<String, dynamic>.from(item)),
          )
          .toList(growable: false),
    );
  }

  final ContinuityThreadDto thread;
  final List<ContinuityWaitDto> waits;
  final List<ContinuityEventDto> events;
}
