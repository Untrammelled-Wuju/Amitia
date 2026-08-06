import '../api/api_client.dart';

class TemporalService {
  final ApiClient _api = ApiClient();

  Future<Map<String, dynamic>?> config() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/temporal/config');
    return resp.data;
  }

  Future<bool> updateConfig(Map<String, dynamic> data) async {
    await _api.put('/api/temporal/config', data: data);
    return true;
  }

  Future<List<Map<String, dynamic>>> anchors() async {
    final resp = await _api.get<List<dynamic>>('/api/temporal/anchors');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<bool> createAnchor(Map<String, dynamic> data) async {
    await _api.post('/api/temporal/anchors', data: data);
    return true;
  }

  Future<bool> updateAnchor(String id, Map<String, dynamic> data) async {
    await _api.put('/api/temporal/anchors/$id', data: data);
    return true;
  }

  Future<bool> deleteAnchor(String id) async {
    await _api.delete('/api/temporal/anchors/$id');
    return true;
  }
}
