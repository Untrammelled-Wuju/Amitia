import '../backend_transport/backend_service_api.dart';
import '../models/character.dart';

class CharacterService {
  final BackendServiceApi _api;

  CharacterService(this._api);

  Future<List<CharacterDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/characters');
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

  Future<CharacterDto?> setActive(String id) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters/$id/active',
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/characters',
      data: data,
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }

  Future<CharacterDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>(
      '/api/characters/$id',
      data: data,
    );
    if (resp == null) return null;
    return CharacterDto.fromJson(resp);
  }


  Future<CharacterDto?> duplicate(String id, {String? name}) async {
    final source = await _api.get<Map<String, dynamic>>(
      '/api/characters/$id',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (source == null) return null;
    const fields = <String>[
      'voiceType', 'voiceSpeed', 'voicePitch', 'voiceVolume', 'customVoiceId',
      'identity', 'personality', 'avatar', 'speakingStyle', 'relationshipStyle',
      'characterBase', 'boundaryRules', 'description', 'gender', 'pronoun',
      'selfReference', 'genderExpression', 'lifeIdentity', 'personalityConfig',
      'chatStyleConfig', 'sceneRules',
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
    return create(payload);
  }

  Future<CharacterDto?> setDefault(String id) => update(id, const {'isDefault': true});

  Future<CharacterDto?> archive(String id) => update(id, const {'status': 'disabled'});

  Future<bool> delete(String id) async {
    await _api.delete('/api/characters/$id');
    return true;
  }
}
