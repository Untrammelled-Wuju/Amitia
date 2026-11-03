class AttentionItem {
  const AttentionItem({
    required this.id,
    required this.kind,
    required this.decision,
    required this.priority,
    required this.requiresAction,
    required this.title,
    required this.summary,
    required this.reason,
    required this.createdAt,
    this.roleId = '',
    this.conversationId = '',
    this.threadId = '',
    this.requestId = '',
    this.coordinationId = '',
    this.approvalId = '',
    this.turnId = '',
    this.workflowId = '',
    this.workflowRunId = '',
    this.payload = const {},
  });

  final String id;
  final String kind;
  final String decision;
  final int priority;
  final bool requiresAction;
  final String title;
  final String summary;
  final String reason;
  final DateTime createdAt;
  final String roleId;
  final String conversationId;
  final String threadId;
  final String requestId;
  final String coordinationId;
  final String approvalId;
  final String turnId;
  final String workflowId;
  final String workflowRunId;
  final Map<String, dynamic> payload;

  factory AttentionItem.fromJson(Map<String, dynamic> value) {
    final id = (value['id'] ?? '').toString().trim();
    final kind = (value['kind'] ?? '').toString().trim();
    final createdAt = DateTime.tryParse((value['createdAt'] ?? '').toString());
    if (id.isEmpty || kind.isEmpty || createdAt == null) {
      throw const FormatException('待处理事项记录无效');
    }
    return AttentionItem(
      id: id,
      kind: kind,
      decision: (value['decision'] ?? '').toString(),
      priority: (value['priority'] as num?)?.toInt() ?? 0,
      requiresAction: value['requiresAction'] == true,
      title: (value['title'] ?? '').toString(),
      summary: (value['summary'] ?? '').toString(),
      reason: (value['reason'] ?? '').toString(),
      createdAt: createdAt,
      roleId: (value['roleId'] ?? '').toString(),
      conversationId: (value['conversationId'] ?? '').toString(),
      threadId: (value['threadId'] ?? '').toString(),
      requestId: (value['requestId'] ?? '').toString(),
      coordinationId: (value['coordinationId'] ?? '').toString(),
      approvalId: (value['approvalId'] ?? '').toString(),
      turnId: (value['turnId'] ?? '').toString(),
      workflowId: (value['workflowId'] ?? '').toString(),
      workflowRunId: (value['workflowRunId'] ?? '').toString(),
      payload: value['payload'] is Map
          ? Map<String, dynamic>.from(value['payload'] as Map)
          : const {},
    );
  }
}

class AttentionBudgetPolicy {
  const AttentionBudgetPolicy({
    this.enabled = true,
    this.shadowMode = true,
    this.maxInterruptionsPerDay = 24,
    this.maxRepeatedConfirmations = 3,
    this.maxConsecutiveFollowups = 2,
    this.maxUnchangedNotifications = 1,
    this.dedupeWindowMinutes = 30,
    this.revision = 0,
  });

  final bool enabled;
  final bool shadowMode;
  final int maxInterruptionsPerDay;
  final int maxRepeatedConfirmations;
  final int maxConsecutiveFollowups;
  final int maxUnchangedNotifications;
  final int dedupeWindowMinutes;
  final int revision;

  factory AttentionBudgetPolicy.fromJson(Map<String, dynamic> value) {
    return AttentionBudgetPolicy(
      enabled: value['enabled'] != false,
      shadowMode: value['shadowMode'] != false,
      maxInterruptionsPerDay:
          (value['maxInterruptionsPerDay'] as num?)?.toInt() ?? 24,
      maxRepeatedConfirmations:
          (value['maxRepeatedConfirmations'] as num?)?.toInt() ?? 3,
      maxConsecutiveFollowups:
          (value['maxConsecutiveFollowups'] as num?)?.toInt() ?? 2,
      maxUnchangedNotifications:
          (value['maxUnchangedNotifications'] as num?)?.toInt() ?? 1,
      dedupeWindowMinutes:
          (value['dedupeWindowMinutes'] as num?)?.toInt() ?? 30,
      revision: (value['revision'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
    'enabled': enabled,
    'shadowMode': shadowMode,
    'maxInterruptionsPerDay': maxInterruptionsPerDay,
    'maxRepeatedConfirmations': maxRepeatedConfirmations,
    'maxConsecutiveFollowups': maxConsecutiveFollowups,
    'maxUnchangedNotifications': maxUnchangedNotifications,
    'dedupeWindowMinutes': dedupeWindowMinutes,
    'revision': revision,
  };

  AttentionBudgetPolicy copyWith({
    bool? enabled,
    bool? shadowMode,
    int? maxInterruptionsPerDay,
    int? maxRepeatedConfirmations,
    int? maxConsecutiveFollowups,
    int? maxUnchangedNotifications,
    int? dedupeWindowMinutes,
    int? revision,
  }) {
    return AttentionBudgetPolicy(
      enabled: enabled ?? this.enabled,
      shadowMode: shadowMode ?? this.shadowMode,
      maxInterruptionsPerDay:
          maxInterruptionsPerDay ?? this.maxInterruptionsPerDay,
      maxRepeatedConfirmations:
          maxRepeatedConfirmations ?? this.maxRepeatedConfirmations,
      maxConsecutiveFollowups:
          maxConsecutiveFollowups ?? this.maxConsecutiveFollowups,
      maxUnchangedNotifications:
          maxUnchangedNotifications ?? this.maxUnchangedNotifications,
      dedupeWindowMinutes: dedupeWindowMinutes ?? this.dedupeWindowMinutes,
      revision: revision ?? this.revision,
    );
  }
}

class AttentionBudgetStats {
  const AttentionBudgetStats({
    required this.policy,
    this.todayInterruptions = 0,
    this.todaySuppressed = 0,
    this.todayObserved = 0,
    this.repeatedConfirmations = 0,
    this.consecutiveFollowups = 0,
    this.unchangedNotifications = 0,
  });

  final AttentionBudgetPolicy policy;
  final int todayInterruptions;
  final int todaySuppressed;
  final int todayObserved;
  final int repeatedConfirmations;
  final int consecutiveFollowups;
  final int unchangedNotifications;

  factory AttentionBudgetStats.fromJson(Map<String, dynamic> value) {
    return AttentionBudgetStats(
      policy: AttentionBudgetPolicy.fromJson(
        value['policy'] is Map
            ? Map<String, dynamic>.from(value['policy'] as Map)
            : const {},
      ),
      todayInterruptions: (value['todayInterruptions'] as num?)?.toInt() ?? 0,
      todaySuppressed: (value['todaySuppressed'] as num?)?.toInt() ?? 0,
      todayObserved: (value['todayObserved'] as num?)?.toInt() ?? 0,
      repeatedConfirmations:
          (value['repeatedConfirmations'] as num?)?.toInt() ?? 0,
      consecutiveFollowups:
          (value['consecutiveFollowups'] as num?)?.toInt() ?? 0,
      unchangedNotifications:
          (value['unchangedNotifications'] as num?)?.toInt() ?? 0,
    );
  }
}

class AttentionTimelineEvent {
  const AttentionTimelineEvent({
    required this.id,
    required this.type,
    required this.title,
    required this.source,
    required this.occurredAt,
    this.summary = '',
    this.status = '',
    this.details = const {},
  });

  final String id;
  final String type;
  final String title;
  final String source;
  final DateTime occurredAt;
  final String summary;
  final String status;
  final Map<String, dynamic> details;

  factory AttentionTimelineEvent.fromJson(Map<String, dynamic> value) {
    final occurredAt = DateTime.tryParse(
      (value['occurredAt'] ?? '').toString(),
    );
    if (occurredAt == null) throw const FormatException('决策时间无效');
    return AttentionTimelineEvent(
      id: (value['id'] ?? '').toString(),
      type: (value['type'] ?? '').toString(),
      title: (value['title'] ?? '').toString(),
      source: (value['source'] ?? '').toString(),
      occurredAt: occurredAt,
      summary: (value['summary'] ?? '').toString(),
      status: (value['status'] ?? '').toString(),
      details: value['details'] is Map
          ? Map<String, dynamic>.from(value['details'] as Map)
          : const {},
    );
  }
}

class AgentSourcePolicy {
  const AgentSourcePolicy({
    this.id = '',
    this.spaceId = '',
    this.scopeType = 'channel',
    this.channelId = '',
    this.accountId = '',
    this.conversationId = '',
    this.senderId = '',
    this.explicitMode = 'allow',
    this.inferredMode = 'observe',
    this.triggerMode = 'direct_or_mention',
    this.permissionCeiling = 'request_approval',
    this.enabled = true,
    this.revision = 0,
  });

  final String id;
  final String spaceId;
  final String scopeType;
  final String channelId;
  final String accountId;
  final String conversationId;
  final String senderId;
  final String explicitMode;
  final String inferredMode;
  final String triggerMode;
  final String permissionCeiling;
  final bool enabled;
  final int revision;

  factory AgentSourcePolicy.fromJson(Map<String, dynamic> value) {
    return AgentSourcePolicy(
      id: (value['id'] ?? '').toString(),
      spaceId: (value['spaceId'] ?? '').toString(),
      scopeType: (value['scopeType'] ?? 'channel').toString(),
      channelId: (value['channelId'] ?? '').toString(),
      accountId: (value['accountId'] ?? '').toString(),
      conversationId: (value['conversationId'] ?? '').toString(),
      senderId: (value['senderId'] ?? '').toString(),
      explicitMode: (value['explicitMode'] ?? 'allow').toString(),
      inferredMode: (value['inferredMode'] ?? 'observe').toString(),
      triggerMode: (value['triggerMode'] ?? 'direct_or_mention').toString(),
      permissionCeiling: (value['permissionCeiling'] ?? 'request_approval')
          .toString(),
      enabled: value['enabled'] != false,
      revision: (value['revision'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
    if (id.isNotEmpty) 'id': id,
    if (spaceId.isNotEmpty) 'spaceId': spaceId,
    'scopeType': scopeType,
    'channelId': channelId.trim(),
    'accountId': accountId.trim(),
    'conversationId': conversationId.trim(),
    'senderId': senderId.trim(),
    'explicitMode': explicitMode,
    'inferredMode': inferredMode,
    'triggerMode': triggerMode,
    'permissionCeiling': permissionCeiling,
    'enabled': enabled,
    'revision': revision,
  };
}
