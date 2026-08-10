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

  Future<Map<String, dynamic>?> exportPack(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/characters/$id/export-pack');
    return resp;
  }

  Future<Map<String, dynamic>?> importPreview(String packPath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/import-pack/preview',
      data: {'packPath': packPath},
    );
    return resp;
  }

  Future<Map<String, dynamic>?> importConfirm(String packPath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/import-pack/confirm',
      data: {'packPath': packPath},
    );
    return resp;
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

  Future<Map<String, dynamic>?> roleProfile() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/companion/role-profile');
    return resp;
  }

  Future<bool> updateRoleProfile(Map<String, dynamic> data) async {
    await _api.put('/api/companion/role-profile', data: data);
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
