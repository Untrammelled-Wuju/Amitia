import '../backend_transport/backend_service_api.dart';

class TemporalService {
  final BackendServiceApi _api;
  final Expando<Map<String, dynamic>> _relationshipAuthorities = Expando();

  Future<Map<String, dynamic>> _relationshipView(
    String characterId, {
    String episodeId = '',
    int limit = 50,
  }) async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/relationship-time',
      queryParameters: {
        'characterId': characterId,
        if (episodeId.isNotEmpty) 'episodeId': episodeId,
        'limit': limit,
      },
    );
    final scope = result?['executionScope'];
    if (result == null ||
        result['ownerId'] is! String ||
        result['roleId'] != characterId ||
        scope is! Map ||
        scope['resourceOwnerId'] != result['ownerId'] ||
        scope['roleId'] != characterId) {
      throw StateError('关系时间数据归属无法确认');
    }
    return result;
  }

  TemporalService(this._api);

  Future<Map<String, dynamic>?> config() async {
    return _api.get<Map<String, dynamic>>('/api/temporal/profile');
  }

  Future<Map<String, dynamic>?> snapshot({
    String characterId = '',
    String channel = 'mobile',
  }) {
    return _api.get<Map<String, dynamic>>(
      '/api/temporal/snapshot',
      queryParameters: <String, dynamic>{
        if (characterId.trim().isNotEmpty) 'characterId': characterId.trim(),
        'channel': channel,
      },
    );
  }

  Future<bool> updateConfig(Map<String, dynamic> data) async {
    await _api.put('/api/temporal/profile', data: data);
    return true;
  }

  Future<Map<String, dynamic>?> suggestTimezone(String timezone) {
    return _api.post<Map<String, dynamic>>(
      '/api/temporal/timezone-suggestion',
      data: {'timezone': timezone},
    );
  }

  Future<Map<String, dynamic>?> acceptTimezoneSuggestion() {
    return _api.post<Map<String, dynamic>>(
      '/api/temporal/timezone-suggestion/accept',
    );
  }

  Future<Map<String, dynamic>?> rejectTimezoneSuggestion() {
    return _api.post<Map<String, dynamic>>(
      '/api/temporal/timezone-suggestion/reject',
    );
  }

  Future<Map<String, dynamic>?> characterProfile(String characterId) {
    return _api.get<Map<String, dynamic>>(
      '/api/temporal/characters/${Uri.encodeComponent(characterId)}/profile',
    );
  }

  Future<bool> updateCharacterProfile(
    String characterId,
    Map<String, dynamic> data,
  ) async {
    await _api.put(
      '/api/temporal/characters/${Uri.encodeComponent(characterId)}/profile',
      data: data,
    );
    return true;
  }

  Future<Map<String, dynamic>?> relationshipTimeSettings(
    String characterId,
  ) async {
    final result = await _relationshipView(characterId);
    final settings = Map<String, dynamic>.from(result['settings'] as Map);
    _relationshipAuthorities[settings] = Map<String, dynamic>.from(
      result['executionScope'] as Map,
    );
    return settings;
  }

  Future<bool> updateRelationshipTimeSettings(
    String characterId,
    Map<String, dynamic> data,
  ) async {
    final expected = _relationshipAuthorities[data];
    if (expected == null) throw StateError('请重新加载关系时间设置');
    final result = await _api.put<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/relationship-time',
      queryParameters: {'characterId': characterId},
      data: {'settings': data, 'expectedExecutionScope': expected},
    );
    if (result == null ||
        result['roleId'] != characterId ||
        result['ownerId'] != expected['resourceOwnerId']) {
      throw StateError('关系时间保存归属无法确认');
    }
    _relationshipAuthorities[data] = Map<String, dynamic>.from(
      result['executionScope'] as Map,
    );
    data['updatedAt'] = (result['settings'] as Map)['updatedAt'];
    return true;
  }

  Future<Map<String, dynamic>?> relationshipTimeState(
    String characterId,
  ) async {
    final result = await _relationshipView(characterId);
    return Map<String, dynamic>.from(result['state'] as Map);
  }

  Future<List<Map<String, dynamic>>> anchors({
    String characterId = '',
    String status = '',
    int limit = 200,
  }) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/temporal/anchors',
      queryParameters: <String, dynamic>{
        if (characterId.trim().isNotEmpty) 'characterId': characterId.trim(),
        if (status.trim().isNotEmpty) 'status': status.trim(),
        'limit': limit,
      },
    );
    if (resp == null) return const <Map<String, dynamic>>[];
    return resp
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>?> createAnchor(Map<String, dynamic> data) {
    return _api.post<Map<String, dynamic>>('/api/temporal/anchors', data: data);
  }

  Future<Map<String, dynamic>?> updateAnchor(
    String id,
    Map<String, dynamic> data, {
    String characterId = '',
  }) {
    final payload = Map<String, dynamic>.from(data);
    payload['characterId'] = characterId.trim();
    return _api.put<Map<String, dynamic>>(
      '/api/temporal/anchors/${Uri.encodeComponent(id)}',
      data: payload,
    );
  }

  Future<bool> deleteAnchor(String id, {String characterId = ''}) async {
    await _api.delete(
      '/api/temporal/anchors/${Uri.encodeComponent(id)}',
      queryParameters: <String, dynamic>{
        if (characterId.trim().isNotEmpty) 'characterId': characterId.trim(),
      },
    );
    return true;
  }

  Future<Map<String, dynamic>?> confirmAnchor(
    String id, {
    String characterId = '',
  }) {
    return _api.post<Map<String, dynamic>>(
      '/api/temporal/anchors/${Uri.encodeComponent(id)}/confirm',
      queryParameters: <String, dynamic>{
        if (characterId.trim().isNotEmpty) 'characterId': characterId.trim(),
      },
    );
  }

  Future<List<Map<String, dynamic>>> reunionEpisodes(
    String characterId, {
    int limit = 50,
  }) async {
    final result = await _relationshipView(characterId, limit: limit);
    return (result['episodes'] as List? ?? const [])
        .whereType<Map>()
        .take(limit.clamp(1, 200))
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>?> reunionEpisode(
    String characterId,
    String episodeId,
  ) async {
    final result = await _relationshipView(characterId, episodeId: episodeId);
    final episode = result['episode'];
    if (episode is! Map || episode['id'] != episodeId) {
      throw StateError('重逢记录不存在或不属于当前所有者');
    }
    return Map<String, dynamic>.from(episode);
  }
}
