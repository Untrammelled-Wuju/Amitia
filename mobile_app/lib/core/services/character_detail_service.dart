import '../backend_transport/backend_service_api.dart';
import '../models/character.dart';

class CharacterDetailService {
  final BackendServiceApi _api;

  CharacterDetailService(this._api);

  Future<CharacterDto?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/$id/test',
      data: {'message': '你好'},
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<List<Map<String, dynamic>>> packHistory() async {
    final resp = await _api.get<List<dynamic>>('/api/characters/packs/history');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> templates() async {
    final resp = await _api.get<List<dynamic>>('/api/character-templates');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createFromTemplate(String templateId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/character-templates/$templateId/create-character',
    );
    return resp;
  }

  Future<Map<String, dynamic>?> character(String id) async {
    return _api.get<Map<String, dynamic>>('/api/characters/$id');
  }

  Future<Map<String, dynamic>?> updateCharacter(String id, Map<String, dynamic> data) async {
    return _api.put<Map<String, dynamic>>('/api/characters/$id', data: data);
  }

  Future<Map<String, dynamic>?> roleProfile({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/role-profile',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
    );
    return resp;
  }

  Future<bool> updateRoleProfile(Map<String, dynamic> data, {String? characterId}) async {
    await _api.put(
      '/api/companion/role-profile',
      queryParameters: {if (characterId != null && characterId.isNotEmpty) 'characterId': characterId},
      data: data,
    );
    return true;
  }

  Future<Map<String, dynamic>?> uploadAvatar(String id, String filePath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/$id/avatar',
      data: {'filePath': filePath},
    );
    return resp;
  }
}
