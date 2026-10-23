import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/privacy_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api extends Fake implements BackendServiceApi {
  Map<String, dynamic> response = {};
  String? path;
  Object? body;
  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    this.path = path;
    body = data;
    return response as T;
  }
}

void main() {
  test('完整扫描保留零风险，缺失完整性标记必须失败', () async {
    final api = _Api();
    final service = PrivacyService(api);
    api.response = {'status': 'completed', 'complete': true, 'totalFound': 0};
    expect((await service.scan())['totalFound'], 0);
    expect(api.path, '/api/privacy/scan');
    api.response = {'status': 'completed', 'totalFound': 0};
    await expectLater(service.scan(), throwsStateError);
  });
  test('脱敏数量不足不能显示成功，并保留扫描回执', () async {
    final api = _Api();
    final service = PrivacyService(api);
    final targets = [
      {'id': 'receipt', 'source_table': 'import_items'},
    ];
    api.response = {'masked': true, 'maskedCount': 0};
    await expectLater(service.mask(targets), throwsStateError);
    api.response = {'masked': true, 'maskedCount': 1};
    expect((await service.mask(targets))['maskedCount'], 1);
    expect((api.body as Map)['items'], [
      {'id': 'receipt', 'sourceTable': 'import_items'},
    ]);
  });
  test('删除确认保留预览来源与版本，失败回执不改为成功', () async {
    final api = _Api();
    final service = PrivacyService(api);
    final target = {
      'id': 'message',
      'kind': 'message',
      'ownerId': 'phone',
      'roleId': 'role',
      'revision': 3,
      'executionScope': {'modeRevision': 4},
    };
    api.response = {
      'targets': [target],
    };
    final intents = await service.previewDeletion(
      targetId: 'message',
      targetType: 'message',
      scope: 'all',
    );
    expect(intents.single['target'], target);
    expect(intents.single['requestId'], isNotEmpty);
    api.response = {
      'id': 'receipt',
      'status': 'failed',
      'retrievalBlocked': false,
    };
    final result = await service.requestDeletion(intent: intents.single);
    expect(result['status'], 'failed');
    expect(result['retrievalBlocked'], false);
    expect(api.body, intents.single);
    api.response = {'targets': <dynamic>[]};
    await expectLater(
      service.previewDeletion(
        targetId: 'missing',
        targetType: 'message',
        scope: 'all',
      ),
      throwsStateError,
    );
  });
}
