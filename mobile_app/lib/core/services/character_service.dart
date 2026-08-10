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

  Future<bool> delete(String id) async {
    await _api.delete('/api/characters/$id');
    return true;
  }
}
