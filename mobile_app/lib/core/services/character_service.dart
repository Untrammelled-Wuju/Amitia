import '../backend_transport/backend_service_api.dart';
import '../models/character.dart';
import 'role_authority.dart';

class CharacterService {
  final BackendServiceApi _api;

  CharacterService(this._api);

  Future<String> authority() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/characters/authority',
    );
    final value = (response?['roleAuthority'] ?? '').toString();
    roleAuthorityHeaders(value);
    return value;
  }

  Future<List<CharacterDto>> list({bool includeDisabled = false}) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/characters',
      queryParameters: {if (includeDisabled) 'includeDisabled': true},
    );
    if (resp == null) return [];
    return resp
        .map((e) => CharacterDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<CharacterDto?> getById(String id) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/characters/$id',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> setActive(String id, {String? roleAuthority}) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/$id/active',
      headers: roleAuthorityHeaders(roleAuthority),
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> create(
    Map<String, dynamic> data, {
    String? roleAuthority,
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters',
      data: data,
      headers: roleAuthorityHeaders(roleAuthority),
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> update(
    String id,
    Map<String, dynamic> data, {
    String? roleAuthority,
  }) async {
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/characters/$id',
      data: data,
      headers: roleAuthorityHeaders(roleAuthority),
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> duplicate(
    String id, {
    String? name,
    String? roleAuthority,
  }) async {
    final source = await _api.get<Map<String, dynamic>>(
      '/api/characters/$id',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (source == null) return null;
    if (roleAuthority != null && source['roleAuthority'] != roleAuthority)
      throw StateError('角色数据归属已变化，请重新加载角色');
    const fields = <String>[
      'voiceType',
      'voiceSpeed',
      'voicePitch',
      'voiceVolume',
      'customVoiceId',
      'identity',
      'personality',
      'avatar',
      'speakingStyle',
      'relationshipStyle',
      'characterBase',
      'boundaryRules',
      'description',
      'basePrompt',
      'gender',
      'pronoun',
      'selfReference',
      'genderExpression',
      'lifeIdentity',
      'personalityConfig',
      'chatStyleConfig',
      'sceneRules',
    ];
    final payload = <String, dynamic>{
      'name': name?.trim().isNotEmpty == true
          ? name!.trim()
          : '${(source['name'] ?? '角色').toString()} 副本',
      'isDefault': false,
    };
    for (final field in fields) {
      if (source.containsKey(field)) payload[field] = source[field];
    }
    return create(payload, roleAuthority: roleAuthority);
  }

  Future<CharacterDto?> setDefault(String id, {String? roleAuthority}) =>
      update(id, const {'isDefault': true}, roleAuthority: roleAuthority);

  Future<CharacterDto?> archive(String id, {String? roleAuthority}) =>
      update(id, const {'status': 'disabled'}, roleAuthority: roleAuthority);

  Future<CharacterDto?> restore(String id, {String? roleAuthority}) =>
      update(id, const {'status': 'enabled'}, roleAuthority: roleAuthority);

  Future<bool> delete(String id, {String? roleAuthority}) async {
    await _api.delete(
      '/api/characters/$id',
      headers: roleAuthorityHeaders(roleAuthority),
    );
    return true;
  }
}
