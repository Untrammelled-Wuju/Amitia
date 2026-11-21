import '../backend_transport/backend_service_api.dart';
import '../backend_transport/role_management_target.dart';
import '../models/character.dart';
import 'role_authority.dart';

class CharacterService {
  final BackendServiceApi _api;
  final RoleManagementTarget? Function()? _target;

  CharacterService(this._api, {RoleManagementTarget? Function()? target})
    : _target = target;

  RoleManagementTarget? _resolve(RoleManagementTarget? target) =>
      target ?? _target?.call();

  Map<String, String>? _headers(
    String? roleAuthority,
    RoleManagementTarget? target,
  ) =>
      withRoleManagementTarget(roleAuthorityHeaders(roleAuthority), _resolve(target));

  Future<String> authority({RoleManagementTarget? target}) async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/characters/authority',
      headers: withRoleManagementTarget(null, _resolve(target)),
    );
    final value = (response?['roleAuthority'] ?? '').toString();
    roleAuthorityHeaders(value);
    return value;
  }

  Future<List<CharacterDto>> list({
    bool includeDisabled = false,
    RoleManagementTarget? target,
  }) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/characters',
      queryParameters: {if (includeDisabled) 'includeDisabled': true},
      headers: withRoleManagementTarget(null, _resolve(target)),
    );
    if (resp == null) return [];
    return resp
        .map((e) => CharacterDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<CharacterDto?> getById(
    String id, {
    RoleManagementTarget? target,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/characters/$id',
      headers: withRoleManagementTarget(null, _resolve(target)),
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> setActive(
    String id, {
    String? roleAuthority,
    RoleManagementTarget? target,
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/$id/active',
      headers: _headers(roleAuthority, target),
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> create(
    Map<String, dynamic> data, {
    String? roleAuthority,
    RoleManagementTarget? target,
  }) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters',
      data: data,
      headers: _headers(roleAuthority, target),
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> update(
    String id,
    Map<String, dynamic> data, {
    String? roleAuthority,
    RoleManagementTarget? target,
  }) async {
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/characters/$id',
      data: data,
      headers: _headers(roleAuthority, target),
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> duplicate(
    String id, {
    String? name,
    String? roleAuthority,
    RoleManagementTarget? target,
  }) async {
    final source = await _api.get<Map<String, dynamic>>(
      '/api/characters/$id',
      headers: withRoleManagementTarget(null, _resolve(target)),
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
    return create(payload, roleAuthority: roleAuthority, target: target);
  }

  Future<CharacterDto?> setDefault(
    String id, {
    String? roleAuthority,
    RoleManagementTarget? target,
  }) => update(
    id,
    const {'isDefault': true},
    roleAuthority: roleAuthority,
    target: target,
  );

  Future<CharacterDto?> archive(
    String id, {
    String? roleAuthority,
    RoleManagementTarget? target,
  }) => update(
    id,
    const {'status': 'disabled'},
    roleAuthority: roleAuthority,
    target: target,
  );

  Future<CharacterDto?> restore(
    String id, {
    String? roleAuthority,
    RoleManagementTarget? target,
  }) => update(
    id,
    const {'status': 'enabled'},
    roleAuthority: roleAuthority,
    target: target,
  );

  Future<bool> delete(
    String id, {
    String? roleAuthority,
    RoleManagementTarget? target,
  }) async {
    await _api.delete(
      '/api/characters/$id',
      headers: _headers(roleAuthority, target),
    );
    return true;
  }
}
