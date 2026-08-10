import '../backend_transport/backend_service_api.dart';
import '../models/companion.dart';

class CompanionService {
  final BackendServiceApi _api;

  CompanionService(this._api);

  Future<CompanionStateDto?> state() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/state',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return CompanionStateDto.fromJson(resp);
  }

  Future<LifeStateDto?> lifeState() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/state/life',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return LifeStateDto.fromJson(resp);
  }

  Future<Map<String, dynamic>?> mindState() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/companion/mind-state');
    return resp;
  }

  Future<Map<String, dynamic>?> schedule() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/companion/schedule');
    return resp;
  }

  Future<List<Map<String, dynamic>>> todaySchedule() async {
    final resp = await _api.get<List<dynamic>>('/api/companion/schedule/today');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> regenerateSchedule() async {
    await _api.post('/api/companion/schedule/regenerate');
    return true;
  }

  Future<List<Map<String, dynamic>>> fixedEvents() async {
    final resp = await _api.get<List<dynamic>>('/api/companion/fixed-events');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> createFixedEvent(Map<String, dynamic> data) async {
    await _api.post('/api/companion/fixed-events', data: data);
    return true;
  }

  Future<bool> updateFixedEvent(String id, Map<String, dynamic> data) async {
    await _api.put('/api/companion/fixed-events/$id', data: data);
    return true;
  }

  Future<bool> deleteFixedEvent(String id) async {
    await _api.delete('/api/companion/fixed-events/$id');
    return true;
  }

  Future<Map<String, dynamic>?> sleepSetting() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/companion/sleep-setting');
    return resp;
  }

  Future<bool> updateSleepSetting(Map<String, dynamic> data) async {
    await _api.put('/api/companion/sleep-setting', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> activeMessageSetting() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/companion/active-message/setting');
    return resp;
  }

  Future<bool> updateActiveMessageSetting(Map<String, dynamic> data) async {
    await _api.put('/api/companion/active-message/setting', data: data);
    return true;
  }

  Future<List<Map<String, dynamic>>> delayedReplies() async {
    final resp = await _api.get<List<dynamic>>('/api/companion/delayed-replies');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> cancelDelayedReply(String id) async {
    await _api.post('/api/companion/delayed-replies/$id/cancel');
    return true;
  }

  Future<Map<String, dynamic>?> debugOverview() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/companion/debug/overview');
    return resp;
  }

  Future<bool> regenerateAll() async {
    await _api.post('/api/companion/debug/regenerate-all');
    return true;
  }
}
