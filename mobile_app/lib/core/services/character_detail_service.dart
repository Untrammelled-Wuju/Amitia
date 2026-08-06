import '../api/api_client.dart';
import '../models/character.dart';

class CharacterDetailService {
  final ApiClient _api = ApiClient();

  Future<CharacterDto?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/$id/test',
      data: {'message': '你好'},
    );
    if (resp.data == null) return null;
    return CharacterDto.fromJson(resp.data!);
  }

  Future<Map<String, dynamic>?> exportPack(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/characters/$id/export-pack');
    return resp.data;
  }

  Future<Map<String, dynamic>?> importPreview(String packPath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/import-pack/preview',
      data: {'packPath': packPath},
    );
    return resp.data;
  }

  Future<Map<String, dynamic>?> importConfirm(String packPath) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/import-pack/confirm',
      data: {'packPath': packPath},
    );
    return resp.data;
  }

  Future<List<Map<String, dynamic>>> packHistory() async {
    final resp = await _api.get<List<dynamic>>('/api/characters/packs/history');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> templates() async {
    final resp = await _api.get<List<dynamic>>('/api/character-templates');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createFromTemplate(String templateId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/character-templates/$templateId/create-character',
    );
    return resp.data;
  }

  Future<Map<String, dynamic>?> roleProfile() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/companion/role-profile');
    return resp.data;
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
    return resp.data;
  }
}
