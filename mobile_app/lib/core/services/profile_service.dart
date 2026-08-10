import '../backend_transport/backend_service_api.dart';
import '../models/profile.dart';

class ProfileService {
  final BackendServiceApi _api;

  ProfileService(this._api);

  Future<List<ProfileDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/profiles');
    if (resp == null) return [];
    return resp.map((e) => ProfileDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<ProfileDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/profiles', data: data);
    if (resp == null) return null;
    return ProfileDto.fromJson(resp);
  }

  Future<ProfileDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/profiles/$id', data: data);
    if (resp == null) return null;
    return ProfileDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/profiles/$id');
    return true;
  }

  Future<ProfileDto?> getByUser() async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/profiles/by-user',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return ProfileDto.fromJson(resp);
  }

  Future<ProfileDto?> extract(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/profiles/extract', data: {'text': text});
    if (resp == null) return null;
    return ProfileDto.fromJson(resp);
  }

  Future<Map<String, dynamic>?> systemPrompt() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/profiles/system-prompt');
    return resp;
  }
}
