// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

import '../backend_transport/backend_service_api.dart';

class ExtensionTaskService {
  final BackendServiceApi _api;

  ExtensionTaskService(this._api);

  Future<List<Map<String, dynamic>>> listDefinitions({int limit = 200}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/task-definitions',
      queryParameters: {'limit': limit},
    );
    return _items(resp);
  }

  Future<List<Map<String, dynamic>>> listRuns({int limit = 200}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/extensions/tasks',
      queryParameters: {'limit': limit},
    );
    return _items(resp);
  }

  Future<Map<String, dynamic>?> enqueue({
    required String taskDefinitionId,
    required String extensionId,
    required String moduleId,
    required Map<String, dynamic> input,
    int priority = 0,
    String source = 'mobile',
  }) {
    return _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks',
      data: {
        'taskDefinitionId': taskDefinitionId,
        'extensionId': extensionId,
        'moduleId': moduleId,
        'input': input,
        'priority': priority,
        'source': source,
      },
    );
  }

  Future<Map<String, dynamic>> runtimeDetail(String taskRunId) async {
    final id = Uri.encodeComponent(taskRunId);
    final values = await Future.wait<Map<String, dynamic>?>([
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id'),
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id/progress'),
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id/result'),
      _api.get<Map<String, dynamic>>('/api/extensions/tasks/$id/checkpoint'),
    ]);
    return {
      'run': values[0] ?? const <String, dynamic>{},
      'progress': values[1] ?? const <String, dynamic>{},
      'result': values[2] ?? const <String, dynamic>{},
      'checkpoint': values[3] ?? const <String, dynamic>{},
    };
  }

  Future<void> pause(String taskRunId, {required int generation, String reason = 'user_requested'}) async {
    final id = Uri.encodeComponent(taskRunId);
    await _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks/$id/pause',
      data: {'generation': generation, 'reason': reason},
    );
  }

  Future<void> resume(String taskRunId, {required int generation, String resumeKind = 'resume'}) async {
    final id = Uri.encodeComponent(taskRunId);
    await _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks/$id/resume',
      data: {'generation': generation, 'resumeKind': resumeKind},
    );
  }

  Future<void> cancel(String taskRunId, {String reason = 'user_requested'}) async {
    final id = Uri.encodeComponent(taskRunId);
    await _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks/$id/cancel',
      data: {'reason': reason},
    );
  }

  Future<void> retry(String taskRunId) async {
    final id = Uri.encodeComponent(taskRunId);
    await _api.post<Map<String, dynamic>>('/api/extensions/tasks/$id/retry');
  }

  Future<void> recover(String taskRunId) async {
    final id = Uri.encodeComponent(taskRunId);
    await _api.post<Map<String, dynamic>>('/api/extensions/tasks/$id/recover');
  }

  List<Map<String, dynamic>> _items(Map<String, dynamic>? page) {
    final raw = page?['items'];
    if (raw is! List) return const [];
    return raw
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }
}
