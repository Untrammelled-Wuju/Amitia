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

  Future<Map<String, dynamic>?> psycheSnapshot(String characterId) async {
    final id = characterId.trim();
    if (id.isEmpty) {
      throw ArgumentError.value(characterId, 'characterId', 'characterId 不能为空');
    }
    final state = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/runtime-state',
      queryParameters: {'characterId': id},
    );
    if (state == null) return null;
    return _projectOwnedRuntimeState(state);
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
      fields: {'roleAuthority': ?roleAuthority},
    );
    return resp;
  }
}

Map<String, dynamic> _projectOwnedRuntimeState(Map<String, dynamic> state) {
  final psyche = _map(state['psyche']);
  final relationship = _map(state['relationship']);
  final valence = _number(psyche['valence'], 0.5);
  final stress = _number(psyche['stress'], 0);
  final needs = <String, dynamic>{};
  final rawNeeds = state['needs'];
  if (rawNeeds is List) {
    for (final raw in rawNeeds) {
      final item = _map(raw);
      final kind = (item['kind'] ?? '').toString();
      if (kind.isNotEmpty) needs[kind] = _number(item['level'], 0.5);
    }
  }
  final beliefs = <Map<String, dynamic>>[];
  final rawBeliefs = state['beliefs'];
  if (rawBeliefs is List) {
    for (final raw in rawBeliefs) {
      final item = _map(raw);
      beliefs.add(<String, dynamic>{
        'key': (item['key'] ?? '').toString(),
        'value': (item['value'] ?? '').toString(),
        'confidence': _number(item['confidence'], 0),
        'conflicted': item['conflicted'] == true,
      });
    }
  }
  final updatedAt = (state['updatedAt'] ?? '').toString();
  return <String, dynamic>{
    'emotion': <String, dynamic>{
      'positive': valence,
      'negative': 1 - valence,
      'arousal': _number(psyche['arousal'], 0.5),
      'dominance': _number(psyche['dominance'], 0.5),
      'updatedAt': updatedAt,
    },
    'mood': <String, dynamic>{
      'valence': _number(psyche['moodValence'], 0.5),
      'tension': _number(psyche['moodArousal'], 0.5),
      'pad': '',
      'updatedAt': updatedAt,
    },
    'stress': stress,
    'energy': 1 - _number(psyche['fatigue'], 0.3),
    'needs': needs,
    'beliefs': beliefs,
    'relationship': <String, dynamic>{
      'trust': _number(relationship['trust'], 0.5),
      'familiarity': _number(relationship['familiarity'], 0),
      'security': _number(relationship['security'], 0.5),
      'tension': _number(relationship['tension'], 0),
      'repairConfidence': _number(relationship['repairConfidence'], 0.5),
      'boundary': _number(relationship['boundary'], 0.5),
    },
    'affectLabel': stress > 0.55
        ? '紧张'
        : valence > 0.55
        ? '积极'
        : valence < 0.45
        ? '消极'
        : '平静',
    'collectedAt': updatedAt,
  };
}

Map<String, dynamic> _map(Object? value) =>
    value is Map ? Map<String, dynamic>.from(value) : <String, dynamic>{};

double _number(Object? value, double fallback) =>
    value is num && value.isFinite ? value.toDouble() : fallback;
