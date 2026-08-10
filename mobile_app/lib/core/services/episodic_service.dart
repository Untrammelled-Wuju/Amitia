import '../backend_transport/backend_service_api.dart';
import '../models/episodic.dart';

class EpisodicService {
  final BackendServiceApi _api;

  EpisodicService(this._api);

  Future<List<EpisodicDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/episodic');
    if (resp == null) return [];
    return resp.map((e) => EpisodicDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<EpisodicDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/episodic', data: data);
    if (resp == null) return null;
    return EpisodicDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/episodic/$id');
    return true;
  }

  Future<List<EpisodicDto>> getByUser() async {
    final resp = await _api.get<List<dynamic>>('/api/episodic/by-user');
    if (resp == null) return [];
    return resp.map((e) => EpisodicDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<EpisodicDto?> getDetail(String id) async {
    final resp = await _api.get<Map<String, dynamic>>(
      '/api/episodic/$id/detail',
      fromJson: (e) => e as Map<String, dynamic>,
    );
    if (resp == null) return null;
    return EpisodicDto.fromJson(resp);
  }

  Future<List<EpisodicDto>> extract() async {
    final resp = await _api.post<List<dynamic>>('/api/episodic/extract');
    if (resp == null) return [];
    return resp.map((e) => EpisodicDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<Map<String, dynamic>?> systemPrompt() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/episodic/system-prompt');
    return resp;
  }
}
