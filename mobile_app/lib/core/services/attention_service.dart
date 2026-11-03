import '../backend_transport/backend_service_api.dart';
import '../models/attention.dart';

class AttentionService {
  const AttentionService(this._api);

  final BackendServiceApi _api;

  Future<List<AttentionItem>> items() async {
    final rows = await _api.get<List<dynamic>>('/api/attention/items');
    if (rows == null || rows.any((row) => row is! Map)) {
      throw StateError('待处理事项响应无效');
    }
    return rows
        .map(
          (row) =>
              AttentionItem.fromJson(Map<String, dynamic>.from(row as Map)),
        )
        .toList(growable: false);
  }

  Future<AttentionBudgetStats> budget() async {
    final value = await _api.get<Map<String, dynamic>>('/api/attention/budget');
    if (value == null) throw StateError('注意力预算响应无效');
    return AttentionBudgetStats.fromJson(value);
  }

  Future<AttentionBudgetPolicy> saveBudget(AttentionBudgetPolicy policy) async {
    final value = await _api.put<Map<String, dynamic>>(
      '/api/attention/budget',
      data: policy.toJson(),
    );
    if (value == null) throw StateError('注意力预算未确认保存');
    return AttentionBudgetPolicy.fromJson(value);
  }

  Future<List<AttentionTimelineEvent>> timeline({
    String requestId = '',
    String threadId = '',
    String coordinationId = '',
    String approvalId = '',
    String workflowRunId = '',
  }) async {
    final rows = await _api.get<List<dynamic>>(
      '/api/attention/timeline',
      queryParameters: {
        if (requestId.isNotEmpty) 'requestId': requestId,
        if (threadId.isNotEmpty) 'threadId': threadId,
        if (coordinationId.isNotEmpty) 'coordinationId': coordinationId,
        if (approvalId.isNotEmpty) 'approvalId': approvalId,
        if (workflowRunId.isNotEmpty) 'workflowRunId': workflowRunId,
      },
    );
    if (rows == null || rows.any((row) => row is! Map)) {
      throw StateError('决策时间线响应无效');
    }
    return rows
        .map(
          (row) => AttentionTimelineEvent.fromJson(
            Map<String, dynamic>.from(row as Map),
          ),
        )
        .toList(growable: false);
  }

  Future<List<AgentSourcePolicy>> policies() async {
    final rows = await _api.get<List<dynamic>>('/api/agent-access/policies');
    if (rows == null || rows.any((row) => row is! Map)) {
      throw StateError('来源授权策略响应无效');
    }
    return rows
        .map(
          (row) =>
              AgentSourcePolicy.fromJson(Map<String, dynamic>.from(row as Map)),
        )
        .toList(growable: false);
  }

  Future<AgentSourcePolicy> savePolicy(AgentSourcePolicy policy) async {
    final value = await _api.put<Map<String, dynamic>>(
      '/api/agent-access/policies',
      data: policy.toJson(),
    );
    if (value == null) throw StateError('来源授权策略未确认保存');
    return AgentSourcePolicy.fromJson(value);
  }

  Future<void> deletePolicy(String id, int revision) async {
    await _api.delete(
      '/api/agent-access/policies/${Uri.encodeComponent(id)}',
      queryParameters: {'revision': revision.toString()},
    );
  }
}
