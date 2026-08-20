import '../backend_transport/backend_service_api.dart';

class TemporalService {
  final BackendServiceApi _api;

  TemporalService(this._api);

  Future<Map<String, dynamic>?> config() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/temporal/profile');
    return resp;
  }

  Future<bool> updateConfig(Map<String, dynamic> data) async {
    await _api.put('/api/temporal/profile', data: data);
    return true;
  }

  Future<List<Map<String, dynamic>>> anchors() async {
    final resp = await _api.get<List<dynamic>>('/api/temporal/anchors');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
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
