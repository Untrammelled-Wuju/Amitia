import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/system_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _SafetyApi extends Fake implements BackendServiceApi {
  final puts = <MapEntry<String, Map<String, dynamic>>>[];

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final payload = Map<String, dynamic>.from(data as Map);
    puts.add(MapEntry(path, payload));
    return <String, dynamic>{
          'preventEmotionalBlackmail': true,
          'violationAction': 'rewrite',
          'revision': 4,
          'status': 'resolved',
        }
        as T;
  }
}

void main() {
  test('安全配置保存携带原revision并确认新版本', () async {
    final api = _SafetyApi();
    final updated = await SafetyService(api).updateConfig({
      'preventEmotionalBlackmail': true,
      'violationAction': 'rewrite',
      'expectedRevision': 3,
    });
    expect(updated?.revision, 4);
    expect(updated?.violationAction, 'rewrite');
    expect(api.puts.single.key, '/api/safety/config');
    expect(api.puts.single.value['expectedRevision'], 3);
  });

  test('安全事件单条处理只提交事件编号和版本', () async {
    final api = _SafetyApi();
    final result = await SafetyService(
      api,
    ).handleSafetyEvent('incident/1', 2);
    expect(api.puts.single.key, '/api/safety/events/incident%2F1/handle');
    expect(api.puts.single.value, {'expectedRevision': 2});
    expect(result['status'], 'resolved');
  });
}
