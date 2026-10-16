import '../backend_transport/backend_service_api.dart';

class UserFeedbackService {
  final BackendServiceApi _api;

  UserFeedbackService(this._api);

  Future<Map<String, dynamic>> settings() async {
    final result = await _api.get<Map<String, dynamic>>('/api/user-feedback/settings');
    return result ?? const {};
  }

  Future<Map<String, dynamic>> saveUploadUrl(String uploadUrl) async {
    final result = await _api.put<Map<String, dynamic>>(
      '/api/user-feedback/settings',
      data: {'uploadUrl': uploadUrl},
    );
    return result ?? const {};
  }

  Future<Map<String, dynamic>> preview(Map<String, dynamic> payload) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/user-feedback/diagnostics/preview',
      data: payload,
    );
    return result ?? const {};
  }

  Future<Map<String, dynamic>> submit(Map<String, dynamic> payload) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/user-feedback/tickets',
      data: payload,
    );
    return result ?? const {};
  }

  Future<List<Map<String, dynamic>>> tickets({int limit = 20}) async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/user-feedback/tickets',
      queryParameters: {'limit': limit},
    );
    final rows = result?['tickets'];
    if (rows is! List) return const [];
    return rows
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> retry(String id) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/user-feedback/tickets/${Uri.encodeComponent(id)}/retry',
    );
    return result ?? const {};
  }
}
