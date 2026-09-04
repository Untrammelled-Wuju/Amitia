import '../backend_transport/backend_service_api.dart';
import '../models/worldbook.dart';

class WorldBookService {
  final BackendServiceApi _api;

  WorldBookService(this._api);

  Future<List<WorldBookDto>> list({
    String matchType = '',
    String characterId = '',
    String keyword = '',
    int page = 1,
    int pageSize = 100,
  }) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/world-book',
      queryParameters: {
        if (matchType.isNotEmpty) 'matchType': matchType,
        if (characterId.isNotEmpty) 'characterId': characterId,
        if (keyword.trim().isNotEmpty) 'keyword': keyword.trim(),
        'page': page,
        'pageSize': pageSize,
      },
    );
    final items = resp?['items'];
    if (items is! List) return const [];
    return items
        .whereType<Map>()
        .map((e) => WorldBookDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<WorldBookDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/world-book', data: data);
    return resp == null ? null : WorldBookDto.fromJson(resp);
  }

  Future<WorldBookDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/world-book/$id', data: data);
    return resp == null ? null : WorldBookDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/world-book/$id');
    return true;
  }

  Future<bool> deleteAll() async {
    await _api.delete('/api/world-book');
    return true;
  }

  Future<List<WorldBookMatchDto>> testMatch(String text) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/world-book/match', data: {'text': text});
    final matches = resp?['matches'];
    if (matches is! List) return const [];
    return matches
        .whereType<Map>()
        .map((e) => WorldBookMatchDto.fromJson(Map<String, dynamic>.from(e)))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>?> systemPrompt({String userMessage = ''}) {
    return _api.get<Map<String, dynamic>>(
      '/api/world-book/system-prompt',
      queryParameters: {if (userMessage.isNotEmpty) 'userMessage': userMessage},
    );
  }
}
