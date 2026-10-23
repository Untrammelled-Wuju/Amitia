import '../backend_transport/backend_service_api.dart';

Future<Map<String, dynamic>> readUsageView(
  BackendServiceApi api,
  String path,
) async {
  try {
    final result = await api.get<Map<String, dynamic>>(path);
    if (result == null || result['scope'] is! Map) throw StateError('使用统计响应无效');
    return result;
  } catch (error) {
    return {'error': error.toString()};
  }
}

String usageScopeLabel(Map<String, dynamic> data) =>
    (data['scope'] as Map?)?['deviceId'] == null ? '当前 Core' : '本设备在当前 Core';

String usageNumberLabel(Object? value, {String missing = '不可用'}) =>
    value is num && value.isFinite ? '$value' : missing;
