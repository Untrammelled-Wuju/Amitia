import '../backend_transport/backend_service_api.dart';

class SearchApiService {
  final BackendServiceApi _api;

  SearchApiService(this._api);

  Future<List<Map<String, dynamic>>> listCredentials() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/search/credentials',
    );
    final items = response?['items'];
    if (items is! List) return const <Map<String, dynamic>>[];
    return items
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> saveCredential(
    String engineId,
    String value,
  ) async {
    final response = await _api.put<Map<String, dynamic>>(
      '/api/search/credentials/${Uri.encodeComponent(engineId)}',
      data: <String, dynamic>{'value': value},
    );
    return response ?? <String, dynamic>{};
  }

  Future<void> clearCredential(String engineId) async {
    await _api.delete(
      '/api/search/credentials/${Uri.encodeComponent(engineId)}',
    );
  }
}
