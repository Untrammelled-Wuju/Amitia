import '../backend_transport/backend_service_api.dart';

class KernelTaskService {
  final BackendServiceApi _api;

  KernelTaskService(this._api);

  Future<List<Map<String, dynamic>>> definitions() async {
    final response = await _api.get<Map<String, dynamic>>('/api/extensions/task-definitions');
    return _items(response);
  }

  Future<List<Map<String, dynamic>>> runs({int limit = 200}) async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/extensions/tasks',
      queryParameters: {'limit': limit},
    );
    return _items(response);
  }

  Future<Map<String, dynamic>?> enqueue(Map<String, dynamic> payload) {
    return _api.post<Map<String, dynamic>>('/api/extensions/tasks', data: payload);
  }

  Future<Map<String, dynamic>> runtimeDetail(String taskRunId) async {
    final run = await _api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId');
    if (run == null || run.isEmpty) {
      throw StateError('任务不存在');
    }
    final values = await Future.wait<Map<String, dynamic>>([
      _optionalMap(_api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/progress')),
      _optionalMap(_api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/result')),
      _optionalMap(_api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/checkpoint')),
    ]);
    return {
      'run': run,
      'progress': values[0],
      'result': values[1],
      'checkpoint': values[2],
    };
  }

  Future<void> pause(String taskRunId, {required int generation, String reason = 'user_requested'}) async {
    await _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks/$taskRunId/pause',
      data: {'generation': generation, 'reason': reason},
    );
  }

  Future<void> resume(String taskRunId, {required int generation, String resumeKind = 'resume'}) async {
    await _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks/$taskRunId/resume',
      data: {'generation': generation, 'resumeKind': resumeKind},
    );
  }

  Future<void> cancel(String taskRunId, {String reason = 'user_requested'}) async {
    await _api.post<Map<String, dynamic>>(
      '/api/extensions/tasks/$taskRunId/cancel',
      data: {'reason': reason},
    );
  }

  Future<void> retry(String taskRunId) async {
    await _api.post<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/retry');
  }

  Future<void> recover(String taskRunId) async {
    await _api.post<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/recover');
  }

  static List<Map<String, dynamic>> _items(Map<String, dynamic>? page) {
    final rows = page?['items'];
    if (rows is! List) return const [];
    return rows.whereType<Map>().map((row) => Map<String, dynamic>.from(row)).toList(growable: false);
  }

  static Future<Map<String, dynamic>> _optionalMap(Future<Map<String, dynamic>?> request) async {
    try {
      return await request ?? const <String, dynamic>{};
    } catch (_) {
      return const <String, dynamic>{};
    }
  }
}
