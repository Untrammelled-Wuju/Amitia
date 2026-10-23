import 'dart:math';
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
    if (result?['status'] != 'completed' || result?['complete'] != true) {
      throw StateError('隐私扫描未完整完成，请重试');
    }
    return result!;
  }

  Future<Map<String, dynamic>> results() async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/privacy/scan-results',
    );
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
        .map(
          (finding) => {
            'id':
                (finding['id'] ??
                        finding['recordId'] ??
                        finding['messageId'] ??
                        '')
                    .toString(),
            'sourceTable':
                (finding['source_table'] ??
                        finding['sourceTable'] ??
                        'messages')
                    .toString(),
          },
        )
        .where((item) => (item['id'] ?? '').toString().isNotEmpty)
        .toList(growable: false);
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/mask',
      data: {'items': items, 'confirmToken': '确认脱敏'},
    );
    final expected = items
        .map((item) => '${item['sourceTable']}/${item['id']}')
        .toSet()
        .length;
    if (result?['masked'] != true || result?['maskedCount'] != expected) {
      throw StateError('数据持有方未确认全部脱敏，请重新扫描');
    }
    return result!;
  }

  Future<Map<String, dynamic>> deletionStats() async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/privacy/deletion/stats',
    );
    return result ?? const {};
  }

  Future<List<Map<String, dynamic>>> previewDeletion({
    required String targetId,
    required String targetType,
    required String scope,
    String reason = '',
    String legacyDeletionId = '',
  }) async {
    final payload = <String, dynamic>{
      'targetId': targetId,
      'targetType': targetType,
      'scope': scope,
      'reason': reason,
      if (legacyDeletionId.isNotEmpty) 'legacyDeletionId': legacyDeletionId,
    };
    payload['requestId'] = _requestId();
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/deletion/preview',
      data: payload,
    );
    final targets = result?['targets'];
    if (targets is! List || targets.isEmpty) throw StateError('当前数据范围没有找到删除目标');
    return targets
        .whereType<Map>()
        .map(
          (target) => <String, dynamic>{
            ...payload,
            'target': Map<String, dynamic>.from(target),
          },
        )
        .toList(growable: false);
  }

  Future<Map<String, dynamic>> requestDeletion({
    required Map<String, dynamic> intent,
  }) async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/deletion/request',
      data: intent,
    );
    if (result == null ||
        result['id'] == null ||
        ![
          'pending',
          'cleaning',
          'completed',
          'failed',
        ].contains(result['status'])) {
      throw StateError('数据持有方未返回有效删除回执');
    }
    return result;
  }

  Future<Map<String, dynamic>> deletionStatus(String id) async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/privacy/deletion/status/${Uri.encodeComponent(id)}',
    );
    return result ?? const {};
  }

  Future<Map<String, dynamic>> runDeletionCleanup() async {
    final result = await _api.post<Map<String, dynamic>>(
      '/api/privacy/deletion/cleanup',
    );
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
    return result
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList(growable: false);
  }

  static String _requestId() {
    final bytes = List<int>.generate(16, (_) => Random.secure().nextInt(256));
    bytes[6] = (bytes[6] & 15) | 64;
    bytes[8] = (bytes[8] & 63) | 128;
    final value = bytes
        .map((byte) => byte.toRadixString(16).padLeft(2, '0'))
        .join();
    return '${value.substring(0, 8)}-${value.substring(8, 12)}-${value.substring(12, 16)}-${value.substring(16, 20)}-${value.substring(20)}';
  }
}
