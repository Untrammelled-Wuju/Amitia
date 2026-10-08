import '../backend_transport/backend_service_api.dart';
import 'role_authority.dart';

class CharacterDetailService {
  final BackendServiceApi _api;

  CharacterDetailService(this._api);

  Future<Map<String, dynamic>?> test(String id, String message) async {
    final trimmed = message.trim();
    if (trimmed.isEmpty) {
      throw ArgumentError.value(message, 'message', '测试消息不能为空');
    }
    return _api.post<Map<String, dynamic>>(
      '/api/characters/$id/test',
      data: {'message': trimmed},
    );
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

  Future<Map<String, dynamic>?> createFromTemplate(
    String templateId, {
    String? roleAuthority,
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/character-templates/$templateId/create-character',
      headers: roleAuthorityHeaders(roleAuthority),
    );
    return resp;
  }

  Future<Map<String, dynamic>?> character(String id) async {
    return _api.get<Map<String, dynamic>>('/api/characters/$id');
  }

  Future<Map<String, dynamic>?> updateCharacter(
    String id,
    Map<String, dynamic> data, {
    String? roleAuthority,
  }) async {
    return _api.put<Map<String, dynamic>>(
      '/api/characters/$id',
      data: data,
      headers: roleAuthorityHeaders(roleAuthority),
    );
  }

  Future<Map<String, dynamic>?> roleProfile({String? characterId}) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/companion/role-profile',
      queryParameters: {
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
      },
    );
    return resp;
  }

  Future<bool> updateRoleProfile(
    Map<String, dynamic> data, {
    String? characterId,
    String? roleAuthority,
  }) async {
    await _api.put(
      '/api/companion/role-profile',
      queryParameters: {
        if (characterId != null && characterId.isNotEmpty)
          'characterId': characterId,
      },
      data: data,
      headers: roleAuthorityHeaders(roleAuthority),
    );
    return true;
  }

  Future<Map<String, dynamic>?> uploadAvatar(
    String id,
    String filePath, {
    String? roleAuthority,
  }) async {
    roleAuthorityHeaders(roleAuthority);
    final resp = await _api.postMultipart<Map<String, dynamic>>(
      '/api/characters/$id/avatar',
      files: {
        'avatar': [filePath],
      },
      fields: {if (roleAuthority != null) 'roleAuthority': roleAuthority},
    );
    return resp;
  }
}
