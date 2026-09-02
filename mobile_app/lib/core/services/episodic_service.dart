import '../backend_transport/backend_service_api.dart';
import '../models/episodic.dart';

class EpisodicService {
  final BackendServiceApi _api;

  EpisodicService(this._api);

  Future<List<EpisodicDto>> list({
    String userId = '',
    String characterId = '',
    String sceneType = '',
    int page = 1,
    int pageSize = 100,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/episodic',
      queryParameters: {
        if (userId.isNotEmpty) 'userId': userId,
        if (characterId.isNotEmpty) 'characterId': characterId,
        if (sceneType.isNotEmpty) 'sceneType': sceneType,
        'page': page,
        'pageSize': pageSize,
      },
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map>()
        .map((e) => EpisodicDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<EpisodicDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/episodic', data: data);
    return resp == null ? null : EpisodicDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/episodic/$id');
    return true;
  }

  Future<List<EpisodicDto>> getByUser({
    String userId = 'default',
    String characterId = '',
  }) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/episodic/by-user',
      queryParameters: {
        'userId': userId,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    if (resp == null) return const [];
    return resp
        .whereType<Map>()
        .map((e) => EpisodicDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>?> getDetail(String id) {
    return _api.get<Map<String, dynamic>>('/api/episodic/$id');
  }

  Future<bool> extract({
    String userId = 'default',
    String characterId = '',
    required String conversationId,
    required List<Map<String, String>> messages,
  }) async {
    await _api.post(
      '/api/episodic/extract',
      data: {
        'userId': userId,
        'characterId': characterId,
        'conversationId': conversationId,
        'messages': messages,
      },
    );
    return true;
  }

  Future<Map<String, dynamic>?> systemPrompt({
    String userId = 'default',
    String characterId = '',
  }) {
    return _api.get<Map<String, dynamic>>(
      '/api/episodic/system-prompt',
      queryParameters: {
        'userId': userId,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
  }
}
