import '../backend_transport/backend_service_api.dart';
import '../models/memory.dart';

class MemoryService {
  final BackendServiceApi _api;

  MemoryService(this._api);

  Future<List<MemoryDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/memories');
    if (resp == null) return [];
    return resp.map((e) => MemoryDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<MemoryDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/memories', data: data);
    if (resp == null) return null;
    return MemoryDto.fromJson(resp);
  }

  Future<MemoryDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/memories/$id', data: data);
    if (resp == null) return null;
    return MemoryDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/memories/$id');
    return true;
  }

  Future<bool> deleteAll() async {
    await _api.delete('/api/memories');
    return true;
  }

  Future<List<MemoryDto>> search(String query) async {
    final resp = await _api.post<List<dynamic>>('/api/memories/search', data: {'query': query});
    if (resp == null) return [];
    return resp.map((e) => MemoryDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<List<MemoryDto>> vectorSearch(String query) async {
    final resp = await _api.post<List<dynamic>>('/api/memories/vector-search', data: {'query': query});
    if (resp == null) return [];
    return resp.map((e) => MemoryDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<List<Map<String, dynamic>>> timeline() async {
    final resp = await _api.get<List<dynamic>>('/api/memories/timeline');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> rebuildIndex() async {
    final resp = await _api.post<Map<String, dynamic>>('/api/memories/rebuild-index');
    return resp;
  }

  Future<Map<String, dynamic>?> vectorStatus() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/memories/vector-status');
    return resp;
  }

  Future<List<MemoryCandidateDto>> listCandidates() async {
    final resp = await _api.get<List<dynamic>>('/api/memory-candidates');
    if (resp == null) return [];
    return resp.map((e) => MemoryCandidateDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<bool> acceptCandidate(String id) async {
    await _api.post('/api/memory-candidates/$id/accept');
    return true;
  }

  Future<bool> rejectCandidate(String id) async {
    await _api.post('/api/memory-candidates/$id/reject');
    return true;
  }

  Future<bool> batchAcceptCandidates(List<String> ids) async {
    await _api.post('/api/memory-candidates/batch-accept', data: {'ids': ids});
    return true;
  }
}
