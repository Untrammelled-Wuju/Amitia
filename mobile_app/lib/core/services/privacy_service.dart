import '../backend_transport/backend_service_api.dart';

class PrivacyService {
  final BackendServiceApi _api;

  PrivacyService(this._api);

  Future<Map<String, dynamic>> scan({
    List<String> scope = const ['messages', 'memories', 'import_items'],
  }) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/scan',
      data: {'scope': scope},
    );
    return result ?? const {};
  }

  Future<Map<String, dynamic>> results() async {
    final result = await _api.get<Map<String, dynamic>>('/api/privacy/scan-results');
    return result ?? const {};
  }

  Future<Map<String, dynamic>> scanResult(String id) async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/privacy/scan-results/${Uri.encodeComponent(id)}',
    );
    final nested = result?['result'];
    return nested is Map ? Map<String, dynamic>.from(nested) : const {};
  }

  Future<Map<String, dynamic>> mask(List<Map<String, dynamic>> findings) async {
    final items = findings
        .map((finding) => {
              'id': (finding['id'] ?? finding['recordId'] ?? finding['messageId'] ?? '').toString(),
              'sourceTable': (finding['source_table'] ?? finding['sourceTable'] ?? 'messages').toString(),
            })
        .where((item) => (item['id'] ?? '').toString().isNotEmpty)
        .toList(growable: false);
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/mask',
      data: {'items': items, 'confirmToken': '确认脱敏'},
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
