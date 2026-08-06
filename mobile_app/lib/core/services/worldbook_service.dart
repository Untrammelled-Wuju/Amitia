import '../api/api_client.dart';
import '../models/worldbook.dart';

class WorldBookService {
  final ApiClient _api = ApiClient();

  Future<List<WorldBookDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/world-book');
    if (resp.data == null) return [];
    return resp.data!.map((e) => WorldBookDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<WorldBookDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/world-book', data: data);
    if (resp.data == null) return null;
    return WorldBookDto.fromJson(resp.data!);
  }

  Future<WorldBookDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/world-book/$id', data: data);
    if (resp.data == null) return null;
    return WorldBookDto.fromJson(resp.data!);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/world-book/$id');
    return true;
  }

  Future<bool> deleteAll() async {
    await _api.delete('/api/world-book');
    return true;
  }

  Future<Map<String, dynamic>?> testMatch(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/world-book/match', data: {'text': text});
    return resp.data;
  }

  Future<Map<String, dynamic>?> systemPrompt() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/world-book/system-prompt');
    return resp.data;
  }
}
