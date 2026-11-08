import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_method.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_request.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_response.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_transport.dart';
import 'package:amitia_app/core/backend_transport/state/backend_http_state.dart';
import 'package:amitia_app/core/services/device_mesh_service.dart';
import 'package:flutter_test/flutter_test.dart';

final class _CapabilityTransport implements BackendHttpTransport {
  final requests = <BackendHttpRequest>[];

  @override
  BackendHttpState get state => BackendHttpState.idle;

  @override
  Future<void> close() async {}

  @override
  Future<BackendHttpResponse> send(BackendHttpRequest request) async {
    requests.add(request);
    if (request.method == BackendHttpMethod.get) {
      return BackendHttpResponse(
        statusCode: 200,
        headers: const <String, String>{},
        data: <String, dynamic>{
          'revision': 3,
          'capabilities': <Map<String, dynamic>>[
            <String, dynamic>{
              'id': 'task.execute',
              'label': '主动任务',
              'description': '允许统筹中心调度设备任务',
              'enabled': true,
            },
          ],
        },
      );
    }
    return BackendHttpResponse(
      statusCode: 200,
      headers: const <String, String>{},
      data: <String, dynamic>{
        'state': <String, dynamic>{'revision': 4},
        'centerSynced': true,
        'syncError': '',
      },
    );
  }
}

void main() {
  test('设备能力开放读取和保存使用本机接口并保留原始结果', () async {
    final transport = _CapabilityTransport();
    final service = DeviceMeshLocalService(BackendServiceApi(transport, 1));

    final state = await service.capabilityExposure();
    expect(state['revision'], 3);
    expect((state['capabilities'] as List).single['enabled'], isTrue);

    final result = await service.setCapabilityExposure(
      capability: 'task.execute',
      enabled: false,
      expectedRevision: 3,
    );
    expect(result['centerSynced'], isTrue);
    expect(transport.requests.first.path, '/internal/device-mesh/capabilities');
    expect(transport.requests.last.method, BackendHttpMethod.put);
    expect(
      transport.requests.last.path,
      '/internal/device-mesh/capabilities/task.execute',
    );
    expect(transport.requests.last.body, <String, dynamic>{
      'enabled': false,
      'expectedRevision': 3,
    });
  });
}
