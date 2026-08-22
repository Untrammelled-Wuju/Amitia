import '../backend_transport/backend_service_api.dart';

class TemporalService {
  final BackendServiceApi _api;

  TemporalService(this._api);

  Future<Map<String, dynamic>?> config() async {
    return _api.get<Map<String, dynamic>>('/api/temporal/profile');
  }

  Future<bool> updateConfig(Map<String, dynamic> data) async {
    await _api.put('/api/temporal/profile', data: data);
    return true;
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

  Future<Map<String, dynamic>?> relationshipTimeSettings(String characterId) {
    return _api.get<Map<String, dynamic>>(
      '/api/temporal/characters/${Uri.encodeComponent(characterId)}/relationship-time/settings',
    );
  }

  Future<bool> updateRelationshipTimeSettings(
    String characterId,
    Map<String, dynamic> data,
  ) async {
    await _api.put(
      '/api/temporal/characters/${Uri.encodeComponent(characterId)}/relationship-time/settings',
      data: data,
    );
    return true;
  }

  Future<Map<String, dynamic>?> relationshipTimeState(String characterId) {
    return _api.get<Map<String, dynamic>>(
      '/api/temporal/characters/${Uri.encodeComponent(characterId)}/relationship-time/state',
    );
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
    final resp = await _api.get<dynamic>(
      '/api/temporal/characters/${Uri.encodeComponent(characterId)}/reunion-episodes',
      queryParameters: {'limit': limit},
    );
    if (resp is List) {
      return resp
          .whereType<Map>()
          .map((item) => Map<String, dynamic>.from(item))
          .toList(growable: false);
    }
    if (resp is Map) {
      final items = resp['items'];
      if (items is List) {
        return items
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
      }
    }
    return const <Map<String, dynamic>>[];
  }

  Future<Map<String, dynamic>?> reunionEpisode(
    String characterId,
    String episodeId,
  ) {
    return _api.get<Map<String, dynamic>>(
      '/api/temporal/characters/${Uri.encodeComponent(characterId)}/reunion-episodes/${Uri.encodeComponent(episodeId)}',
    );
  }
}
