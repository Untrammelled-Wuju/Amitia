import '../backend_transport/backend_service_api.dart';

class PrivacyService {
  final BackendServiceApi _api;

  PrivacyService(this._api);

  Future<Map<String, dynamic>> scan() async {
    final result = await _api.post<Map<String, dynamic>>('/api/privacy/scan');
    return result ?? const {};
  }

  Future<Map<String, dynamic>> results() async {
    final result = await _api.get<Map<String, dynamic>>('/api/privacy/scan-results');
    return result ?? const {};
  }

  Future<Map<String, dynamic>> mask(List<int> ids) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/mask',
      data: {'ids': ids, 'confirmToken': '确认脱敏'},
    );
    return result ?? const {};
  }

  Future<Map<String, dynamic>> deletionStats() async {
    final result = await _api.get<Map<String, dynamic>>('/api/privacy/deletion/stats');
    return result ?? const {};
  }

  Future<Map<String, dynamic>> requestDeletion({
    required String targetId,
    required String targetType,
    required String scope,
    String reason = '',
  }) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/deletion/request',
      data: {
        'targetId': targetId,
        'targetType': targetType,
        'scope': scope,
        'reason': reason,
      },
    );
    return result ?? const {};
  }

  Future<Map<String, dynamic>> deletionStatus(String id) async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/privacy/deletion/status/${Uri.encodeComponent(id)}',
    );
    return result ?? const {};
  }

  Future<Map<String, dynamic>> runDeletionCleanup() async {
    final result = await _api.post<Map<String, dynamic>>('/api/privacy/deletion/cleanup');
    return result ?? const {};
  }

  Future<List<Map<String, dynamic>>> deletionSecurityTests({
    required String targetId,
    required String targetType,
  }) async {
    final result = await _api.post<dynamic>(
      '/api/privacy/deletion/security-tests',
      data: {'targetId': targetId, 'targetType': targetType},
    );
    if (result is! List) return const [];
    return result.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }
}
