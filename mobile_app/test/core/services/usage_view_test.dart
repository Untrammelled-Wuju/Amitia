import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/usage_view.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api extends Fake implements BackendServiceApi {
  bool fail = false;
  Map<String, dynamic>? response;
  String? requestedPath;
  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requestedPath = path;
    if (fail) throw StateError('offline');
    return response as T?;
  }
}

void main() {
  test('读取实际用量接口并保留设备范围', () async {
    final api = _Api()
      ..response = {
        'scope': {'deviceId': 'phone'},
        'todayCalls': 2,
        'todayTokens': null,
      };
    final usage = await readUsageView(api, '/api/usage/overview');
    expect(api.requestedPath, '/api/usage/overview');
    expect(usageScopeLabel(usage), '本设备在当前 Core');
    expect(usageNumberLabel(usage['todayCalls']), '2');
    expect(usageNumberLabel(usage['todayTokens'], missing: '未知'), '未知');
    expect(usageNumberLabel(0), '0');
  });
  test('请求失败和空响应明确显示错误', () async {
    final api = _Api();
    expect((await readUsageView(api, '/api/usage/daily'))['error'], isNotNull);
    api.fail = true;
    expect(
      (await readUsageView(api, '/api/usage/models'))['error'],
      contains('offline'),
    );
    expect(usageNumberLabel(null), '不可用');
  });
}
