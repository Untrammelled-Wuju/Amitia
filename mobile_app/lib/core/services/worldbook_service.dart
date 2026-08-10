import '../backend_transport/backend_service_api.dart';
import '../models/worldbook.dart';

class WorldBookService {
  final BackendServiceApi _api;

  WorldBookService(this._api);

  Future<List<WorldBookDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/world-book');
    if (resp == null) return [];
    return resp.map((e) => WorldBookDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<WorldBookDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/world-book', data: data);
    if (resp == null) return null;
    return WorldBookDto.fromJson(resp);
  }

  Future<WorldBookDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/world-book/$id', data: data);
    if (resp == null) return null;
    return WorldBookDto.fromJson(resp);
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
    return resp;
  }

  Future<Map<String, dynamic>?> systemPrompt() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/world-book/system-prompt');
    return resp;
  }
}
