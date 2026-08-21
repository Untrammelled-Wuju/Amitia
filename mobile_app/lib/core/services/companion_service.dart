import '../backend_transport/backend_service_api.dart';
import '../models/companion.dart';

class CompanionService {
  final BackendServiceApi _api;

  CompanionService(this._api);

  Future<CompanionStateDto?> state({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/state',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return CompanionStateDto.fromJson(resp);
  }

  Future<LifeStateDto?> lifeState({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/state/life',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return LifeStateDto.fromJson(resp);
  }

  Future<Map<String, dynamic>?> mindState({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/mind-state',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<Map<String, dynamic>?> schedule({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/schedule',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<List<Map<String, dynamic>>> todaySchedule({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/schedule/today',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> regenerateSchedule({String? characterId}) async {
    await _api.post(
      '/api/companion/schedule/regenerate',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return true;
  }

  Future<List<Map<String, dynamic>>> fixedEvents({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/fixed-events',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> createFixedEvent(Map<String, dynamic> data, {String? characterId}) async {
    await _api.post('/api/companion/fixed-events', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<bool> updateFixedEvent(String id, Map<String, dynamic> data, {String? characterId}) async {
    await _api.put('/api/companion/fixed-events/$id', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<bool> deleteFixedEvent(String id, {String? characterId}) async {
    await _api.delete('/api/companion/fixed-events/$id', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId});
    return true;
  }

  Future<List<Map<String, dynamic>>> specialEvents({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/special-events',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.map((e) => Map<String, dynamic>.from(e as Map)).toList();
  }

  Future<bool> createSpecialEvent(Map<String, dynamic> data, {String? characterId}) async {
    await _api.post('/api/companion/special-events', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<bool> updateSpecialEvent(String id, Map<String, dynamic> data, {String? characterId}) async {
    await _api.put('/api/companion/special-events/$id', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<bool> deleteSpecialEvent(String id, {String? characterId}) async {
    await _api.delete('/api/companion/special-events/$id', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId});
    return true;
  }

  Future<Map<String, dynamic>?> lifestyleTendency({String? characterId}) {
    return _api.get<Map<String, dynamic>>(
      '/api/companion/lifestyle-tendency',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
  }

  Future<bool> updateLifestyleTendency(Map<String, dynamic> data, {String? characterId}) async {
    await _api.put('/api/companion/lifestyle-tendency', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<Map<String, dynamic>?> workProfile({String? characterId}) {
    return _api.get<Map<String, dynamic>>(
      '/api/companion/work-profile',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
  }

  Future<bool> updateWorkProfile(Map<String, dynamic> data, {String? characterId}) async {
    await _api.put('/api/companion/work-profile', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<Map<String, dynamic>?> sleepSetting({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/sleep-setting',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<bool> updateSleepSetting(Map<String, dynamic> data, {String? characterId}) async {
    await _api.put('/api/companion/sleep-setting', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<Map<String, dynamic>?> activeMessageSetting({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/active-message/setting',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<bool> updateActiveMessageSetting(Map<String, dynamic> data, {String? characterId}) async {
    await _api.put('/api/companion/active-message/setting', queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId}, data: data);
    return true;
  }

  Future<List<Map<String, dynamic>>> delayedReplies({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/delayed-replies',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> cancelDelayedReply(String id, {String? characterId}) async {
    await _api.post(
      '/api/companion/delayed-replies/$id/cancel',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return true;
  }

  Future<Map<String, dynamic>?> debugOverview({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/debug/overview',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<bool> regenerateAll({String? characterId}) async {
    await _api.post(
      '/api/companion/debug/regenerate-all',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return true;
  }

  Future<List<Map<String, dynamic>>> scheduleConflicts({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/schedule/conflicts',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<List<Map<String, dynamic>>> classAdjustments({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/class-adjustments',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<Map<String, dynamic>?> createClassAdjustment(Map<String, dynamic> data, {String? characterId}) {
    return _api.post<Map<String, dynamic>>(
      '/api/companion/class-adjustments',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
      data: data,
    );
  }

  Future<Map<String, dynamic>?> updateClassAdjustment(int id, Map<String, dynamic> data, {String? characterId}) {
    return _api.put<Map<String, dynamic>>(
      '/api/companion/class-adjustments/$id',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
      data: data,
    );
  }

  Future<bool> deleteClassAdjustment(int id, {String? characterId}) async {
    await _api.delete(
      '/api/companion/class-adjustments/$id',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return true;
  }

  Future<List<Map<String, dynamic>>> effectiveClasses({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/classes/effective',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<Map<String, dynamic>?> regenerateTimeline({String? characterId}) {
    return _api.post<Map<String, dynamic>>(
      '/api/companion/timeline/regenerate',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
  }

  Future<List<Map<String, dynamic>>> activeMessageTasksToday({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/active-message/tasks/today',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<Map<String, dynamic>?> regenerateActiveMessageTasks({String? characterId}) {
    return _api.post<Map<String, dynamic>>(
      '/api/companion/active-message/tasks/regenerate',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
  }

  Future<Map<String, dynamic>?> runActiveMessageTask(int id, {String? characterId}) {
    return _api.post<Map<String, dynamic>>(
      '/api/companion/active-message/tasks/$id/run',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
  }

  Future<Map<String, dynamic>?> cancelActiveMessageTask(int id, {String? characterId}) {
    return _api.post<Map<String, dynamic>>(
      '/api/companion/active-message/tasks/$id/cancel',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
  }

  Future<Map<String, dynamic>?> processDelayedReplies({String? characterId}) {
    return _api.post<Map<String, dynamic>>(
      '/api/companion/delayed-replies/process',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
  }

  Future<List<Map<String, dynamic>>> ruleLogs({String? characterId}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/companion/rule-logs',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    if (resp == null) return [];
    return resp.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

}
