import '../backend_transport/backend_service_api.dart';
import '../models/model_config.dart';

class ModelConfigService {
  final BackendServiceApi _api;

  ModelConfigService(this._api);

  Future<List<ModelConfigDto>> list() async {
    final resp = await _api.get<List<dynamic>>('/api/model/configs');
    if (resp == null) return [];
    return resp.map((e) => ModelConfigDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<ModelConfigDto?> create(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/model/configs', data: data);
    if (resp == null) return null;
    return ModelConfigDto.fromJson(resp);
  }

  Future<ModelConfigDto?> update(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/model/configs/$id', data: data);
    if (resp == null) return null;
    return ModelConfigDto.fromJson(resp);
  }

  Future<bool> delete(String id) async {
    await _api.delete('/api/model/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/model/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/model/configs/$id/test');
    return resp;
  }

  Future<Map<String, dynamic>?> testStandalone(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/model/test', data: data);
    return resp;
  }

  Future<Map<String, dynamic>?> routes() async {
    final resp = await _api.get<Map<String, dynamic>>('/api/model/routes');
    return resp;
  }

  Future<bool> updateRoutes(Map<String, dynamic> data) async {
    await _api.put('/api/model/routes', data: data);
    return true;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/model/providers');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> detectModels() async {
    final resp = await _api.post<List<dynamic>>('/api/model/detect-models');
    if (resp == null) return [];
    return resp.map((e) => e as Map<String, dynamic>).toList();
  }
}
