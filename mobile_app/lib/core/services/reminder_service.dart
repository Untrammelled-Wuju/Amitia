import '../backend_transport/backend_service_api.dart';
import '../models/reminder.dart';

class ReminderService {
  final BackendServiceApi _api;

  ReminderService(this._api);

  Map<String, dynamic> _required(Map<String, dynamic>? data) {
    if (data == null) throw StateError('服务未返回有效回执');
    return data;
  }

  ReminderDto _saved(Map<String, dynamic>? value) {
    final data = _required(value);
    if (data['id'] is! String ||
        data['ownerId'] is! String ||
        !const [0, 1, true, false].contains(data['enabled'])) {
      throw StateError('任务记录格式无效');
    }
    final item = ReminderDto.fromJson(data);
    if (data['saved'] != true ||
        item.id.isEmpty ||
        item.generation < 1 ||
        item.ownerId.isEmpty)
      throw StateError('服务未确认任务保存成功');
    return item;
  }

  Future<ReminderDto> action(String id, int generation, String action) async =>
      _saved(
        await _api.post<Map<String, dynamic>>(
          '/api/reminders/$id/action',
          data: {'generation': generation, 'action': action},
        ),
      );

  Future<List<ReminderDto>> list() async {
    final resp = await _api.get<List<dynamic>>(
      '/api/reminders',
      queryParameters: {'kind': 'user_reminder'},
    );
    if (resp == null) throw StateError('服务未返回任务列表');
    if (resp.any((item) => item is! Map)) throw StateError('任务列表包含无效记录');
    return resp
        .map((item) => _saved(Map<String, dynamic>.from(item as Map)))
        .toList(growable: false);
  }

  Future<List<String>> channels() async {
    final values = await _api.get<List<dynamic>>('/api/reminders/channels');
    if (values == null || values.any((value) => value is! String))
      throw StateError('渠道列表响应无效');
    return values.cast<String>();
  }

  Future<ReminderDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/reminders',
      data: data,
    );
    return _saved(resp);
  }

  Future<ReminderDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/reminders/$id',
      data: data,
    );
    return _saved(resp);
  }

  Future<bool> delete(String id, int generation) async {
    _saved(
      await _api.deleteWithResponse<Map<String, dynamic>>(
        '/api/reminders/$id',
        data: {'generation': generation},
      ),
    );
    return true;
  }

  Future<ReminderDto?> toggle(String id, int generation) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/reminders/$id/toggle',
      data: {'generation': generation},
    );
    return _saved(resp);
  }

  Future<Map<String, dynamic>> trigger(String id, int generation) async {
    final result = _required(
      await _api.post<Map<String, dynamic>>(
        '/api/reminders/$id/trigger',
        data: {'generation': generation},
      ),
    );
    if (result['accepted'] != true || result['triggerId'] == null)
      throw StateError('任务未受理');
    return result;
  }

  Future<Map<String, dynamic>> test(String id) async {
    final result = _required(
      await _api.post<Map<String, dynamic>>('/api/reminders/$id/test'),
    );
    if (result['tested'] != true || result['id'] != id)
      throw StateError('服务未返回有效预览结果');
    return result;
  }

  Future<Map<String, dynamic>> status() async {
    final result = _required(
      await _api.get<Map<String, dynamic>>(
        '/api/reminders/status',
        queryParameters: {'kind': 'user_reminder'},
      ),
    );
    if (result['schedulerRunning'] is! bool || result['total'] is! num)
      throw StateError('调度状态响应无效');
    return result;
  }

  Future<Map<String, dynamic>> cleanupConfig() async {
    return _required(
      await _api.get<Map<String, dynamic>>('/api/reminders/cleanup-config'),
    );
  }

  Future<bool> setCleanupConfig(String cleanupDays) async {
    final response = _required(
      await _api.put<Map<String, dynamic>>(
        '/api/reminders/cleanup-config',
        data: {'cleanupDays': cleanupDays},
      ),
    );
    if (response['cleanupDays'].toString() != cleanupDays)
      throw StateError('清理策略未确认保存');
    return true;
  }

  Future<List<Map<String, dynamic>>> prospective({
    String characterId = '',
  }) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/reminders/prospective',
      queryParameters: {
        'kind': 'user_reminder',
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    if (resp == null) throw StateError('服务未返回计划列表');
    if (resp.any((item) => item is! Map)) throw StateError('计划列表包含无效记录');
    return resp
        .map((item) => Map<String, dynamic>.from(item as Map))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> queueSummary() async {
    return _required(
      await _api.get<Map<String, dynamic>>(
        '/api/reminders/queue-summary',
        queryParameters: {'kind': 'user_reminder'},
      ),
    );
  }

  Future<Map<String, dynamic>> triggerHistory({
    int page = 1,
    int pageSize = 20,
    String state = '',
  }) async {
    final result = _required(
      await _api.get<Map<String, dynamic>>(
        '/api/reminders/trigger-history',
        queryParameters: {
          'kind': 'user_reminder',
          'page': page,
          'pageSize': pageSize,
          if (state.isNotEmpty) 'state': state,
        },
      ),
    );
    if (result['items'] is! List ||
        result['total'] is! num ||
        (result['items'] as List).any((item) => item is! Map))
      throw StateError('执行历史响应无效');
    return result;
  }

  Future<bool> clearBackpressure() async {
    final result = _required(
      await _api.post<Map<String, dynamic>>(
        '/api/reminders/clear-backpressure',
        queryParameters: {'kind': 'user_reminder'},
      ),
    );
    if (result['clearedCount'] is! num) throw StateError('服务未确认背压清理结果');
    return true;
  }
}
