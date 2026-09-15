import '../backend_transport/backend_service_api.dart';
import '../models/profile.dart';

class ProfileService {
  final BackendServiceApi _api;

  ProfileService(this._api);

  Future<List<ProfileDto>> list({
    String spaceId = '',
    String characterId = '',
    String category = '',
    String keyword = '',
    int page = 1,
    int pageSize = 100,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/profiles',
      queryParameters: {
        if (spaceId.isNotEmpty) 'spaceId': spaceId,
        if (characterId.isNotEmpty) 'characterId': characterId,
        if (category.isNotEmpty) 'category': category,
        if (keyword.trim().isNotEmpty) 'keyword': keyword.trim(),
        'page': page,
        'pageSize': pageSize,
      },
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map>()
        .map((e) => ProfileDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<ProfileDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/profiles', data: data);
    return resp == null ? null : ProfileDto.fromJson(resp);
  }

  Future<ProfileDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/profiles/$id', data: data);
    return resp == null ? null : ProfileDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/profiles/$id');
    return true;
  }

  Future<List<ProfileDto>> getBySpace({
    String spaceId = 'default',
    String characterId = '',
  }) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/profiles/by-space',
      queryParameters: {
        'spaceId': spaceId,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
    if (resp == null) return const [];
    return resp
        .whereType<Map>()
        .map((e) => ProfileDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<bool> extract({
    String spaceId = 'default',
    String characterId = '',
    required String conversationId,
    required List<Map<String, String>> messages,
  }) async {
    await _api.post(
      '/api/profiles/extract',
      data: {
        'spaceId': spaceId,
        'characterId': characterId,
        'conversationId': conversationId,
        'messages': messages,
      },
    );
    return true;
  }

  Future<Map<String, dynamic>?> systemPrompt({
    String spaceId = 'default',
    String characterId = '',
  }) {
    return _api.get<Map<String, dynamic>>(
      '/api/profiles/system-prompt',
      queryParameters: {
        'spaceId': spaceId,
        if (characterId.isNotEmpty) 'characterId': characterId,
      },
    );
  }
}
