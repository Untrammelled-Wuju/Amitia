import '../backend_transport/backend_service_api.dart';
import '../models/reminder.dart';

class ReminderService {
  final BackendServiceApi _api;

  ReminderService(this._api);

  Future<List<ReminderDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/reminders');
    if (resp == null) return [];
    return resp.map((e) => ReminderDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<ReminderDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/reminders', data: data);
    if (resp == null) return null;
    return ReminderDto.fromJson(resp);
  }

  Future<ReminderDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/reminders/$id', data: data);
    if (resp == null) return null;
    return ReminderDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/reminders/$id');
    return true;
  }

  Future<bool> toggle(String id, bool enabled) async {
    await _api.post('/api/reminders/$id/toggle', data: {'enabled': enabled});
    return true;
  }

  Future<bool> trigger(String id) async {
    await _api.post('/api/reminders/$id/trigger');
    return true;
  }

  Future<bool> test(String id) async {
    await _api.post('/api/reminders/$id/test');
    return true;
  }

  Future<Map<String, dynamic>?> status() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/reminders/status');
    return resp;
  }

  Future<Map<String, dynamic>?> cleanupConfig() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/reminders/cleanup-config');
    return resp;
  }

  Future<bool> setCleanupConfig(Map<String, dynamic> data) async {
    await _api.put('/api/reminders/cleanup-config', data: data);
    return true;
  }
}
