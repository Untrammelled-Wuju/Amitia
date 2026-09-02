import '../backend_transport/backend_service_api.dart';
import '../models/reminder.dart';

class ReminderService {
  final BackendServiceApi _api;

  ReminderService(this._api);

  Future<List<ReminderDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/reminders');
    if (resp == null) return const [];
    return resp
        .whereType<Map>()
        .map((e) => ReminderDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<ReminderDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/reminders', data: data);
    return resp == null ? null : ReminderDto.fromJson(resp);
  }

  Future<ReminderDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/reminders/$id', data: data);
    return resp == null ? null : ReminderDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/reminders/$id');
    return true;
  }

  Future<ReminderDto?> toggle(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/reminders/$id/toggle');
    return resp == null ? null : ReminderDto.fromJson(resp);
  }

  Future<Map<String, dynamic>> trigger(String id) async {
    return await _api.post<Map<String, dynamic>>('/api/reminders/$id/trigger') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> test(String id) async {
    return await _api.post<Map<String, dynamic>>('/api/reminders/$id/test') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> status() async {
    return await _api.get<Map<String, dynamic>>('/api/reminders/status') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> cleanupConfig() async {
    return await _api.get<Map<String, dynamic>>('/api/reminders/cleanup-config') ?? <String, dynamic>{};
  }

  Future<bool> setCleanupConfig(String cleanupDays) async {
    await _api.put('/api/reminders/cleanup-config', data: {'cleanupDays': cleanupDays});
    return true;
  }

  Future<List<Map<String, dynamic>>> prospective({String characterId = ''}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/reminders/prospective',
      queryParameters: {if (characterId.isNotEmpty) 'characterId': characterId},
    );
    return (resp ?? const [])
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> queueSummary() async {
    return await _api.get<Map<String, dynamic>>('/api/reminders/queue-summary') ?? <String, dynamic>{};
  }

  Future<Map<String, dynamic>> triggerHistory({
    int page = 1,
    int pageSize = 20,
    String state = '',
  }) async {
    return await _api.get<Map<String, dynamic>>(
          '/api/reminders/trigger-history',
          queryParameters: {
            'page': page,
            'pageSize': pageSize,
            if (state.isNotEmpty) 'state': state,
          },
        ) ??
        <String, dynamic>{'items': <dynamic>[], 'total': 0};
  }

  Future<bool> clearBackpressure() async {
    await _api.post('/api/reminders/clear-backpressure');
    return true;
  }
}
