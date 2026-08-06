import '../api/api_client.dart';
import '../models/profile.dart';

class ProfileService {
  final ApiClient _api = ApiClient();

  Future<List<ProfileDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/profiles');
    if (resp.data == null) return [];
    return resp.data!.map((e) => ProfileDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<ProfileDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/profiles', data: data);
    if (resp.data == null) return null;
    return ProfileDto.fromJson(resp.data!);
  }

  Future<ProfileDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/profiles/$id', data: data);
    if (resp.data == null) return null;
    return ProfileDto.fromJson(resp.data!);
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
    if (resp.data == null) return null;
    return ProfileDto.fromJson(resp.data!);
  }

  Future<ProfileDto?> extract(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/profiles/extract', data: {'text': text});
    if (resp.data == null) return null;
    return ProfileDto.fromJson(resp.data!);
  }

  Future<Map<String, dynamic>?> systemPrompt() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/profiles/system-prompt');
    return resp.data;
  }
}
