import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/models/attention.dart';

void main() {
  test('attention item requires stable identity and timestamp', () {
    final item = AttentionItem.fromJson({
      'id': 'task:one',
      'kind': 'task_failed',
      'decision': 'failed',
      'priority': 80,
      'requiresAction': true,
      'title': 'Agent 任务需要恢复',
      'summary': '检查失败原因',
      'reason': '任务执行失败',
      'createdAt': '2026-11-02T10:00:00Z',
    });
    expect(item.id, 'task:one');
    expect(item.requiresAction, isTrue);
    expect(item.createdAt.isUtc, isTrue);
  });

  test('budget policy preserves explicit shadow mode', () {
    final policy = AttentionBudgetPolicy.fromJson({
      'enabled': true,
      'shadowMode': false,
      'maxInterruptionsPerDay': 12,
      'revision': 4,
    });
    expect(policy.shadowMode, isFalse);
    expect(policy.maxInterruptionsPerDay, 12);
    expect(policy.revision, 4);
  });

  test('timeline events preserve decision details', () {
    final event = AttentionTimelineEvent.fromJson({
      'id': 'decision-one',
      'type': 'access.decision',
      'title': '来源准入',
      'source': 'agent-access',
      'occurredAt': '2026-11-02T10:00:00Z',
      'details': {'triggerMode': 'mention', 'policyRevision': 3},
    });
    expect(event.details['triggerMode'], 'mention');
    expect(event.details['policyRevision'], 3);
  });

  test('source policy preserves scope and revision', () {
    final policy = AgentSourcePolicy.fromJson({
      'id': 'policy-one',
      'spaceId': 'space-one',
      'scopeType': 'sender',
      'channelId': 'qq',
      'conversationId': 'group-one',
      'senderId': 'owner',
      'explicitMode': 'allow',
      'inferredMode': 'observe',
      'permissionCeiling': 'request_approval',
      'enabled': true,
      'revision': 4,
    });
    expect(policy.scopeType, 'sender');
    expect(policy.senderId, 'owner');
    expect(policy.toJson()['revision'], 4);
  });
}
